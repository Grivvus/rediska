package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

func Listen(
	ctx context.Context, cfg config.RedisConfig,
	st *storage.Storage, logger *slog.Logger,
) error {
	listner, err := net.Listen("tcp", "0.0.0.0:"+cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to bind to port %v: %w", cfg.Port, err)
	}
	// listener.Accept is blocking function
	// so to shutdown on Sigint we must launch another goroutine
	// that will be waiting ctx.Done
	go func() {
		<-ctx.Done()

		logger.Info("Close listner", "host", "0.0.0.0", "port", cfg.Port)
		_ = listner.Close()
	}()

	for {
		connection, err := listner.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Error("", "err", fmt.Errorf("can't accept connection: %w", err))
			return err
		}
		go func() {
			handleConnection(ctx, logger, cfg, connection, st, []net.Conn{})
		}()
	}
}

func handleConnection(
	ctx context.Context, logger *slog.Logger,
	cfg config.RedisConfig, connection net.Conn,
	st *storage.Storage, knownReplicas []net.Conn,
) {
	needed := false
	defer func() {
		if !needed {
			_ = connection.Close()
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("recovered from panic", "r", r)
			_, _ = connection.Write(codec.EncodeError(fmt.Errorf("unknown fatal error: %v", r)))
		}
	}()
	readBuffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := connection.Read(readBuffer)
			if err != nil && !errors.Is(err, io.EOF) {
				logger.Error("error while reading from the connection", "err", err)
				continue
			} else if errors.Is(err, io.EOF) || n == 0 {
				return
			}
			if n == len(readBuffer) {
				logger.Error("can't fit message into buffer", "len buffer", len(readBuffer))
				_, _ = connection.Write(codec.EncodeError(fmt.Errorf("message is too long")))
				// return because now in connection lays some garbage
				// that we couldn't fit into buffer
				// so we could close the connection or read full message
				// and then start processing next one
				return
			}
			logger.Info("", "bytes recieved", n, "conn", connection)
			command, err := codec.DecodeArray(readBuffer)
			if err != nil {
				_, _ = connection.Write(codec.EncodeError(fmt.Errorf("can't parse accepted data: %w", err)))
				continue
			}
			if strings.ToUpper(command[0]) == "PING" {
				handlePing(connection, command)
			} else if strings.ToUpper(command[0]) == "ECHO" {
				handleEcho(connection, command)
			} else if strings.ToUpper(command[0]) == "SET" {
				handleSet(connection, command, st)
			} else if strings.ToUpper(command[0]) == "GET" {
				handleGet(connection, command, st)
			} else if strings.ToUpper(command[0]) == "CONFIG" {
				handleConfig(connection, command, cfg)
			} else if strings.ToUpper(command[0]) == "INFO" {
				_, _ = connection.Write(codec.EncodeBulkString(cfg.GetInfo()))
			} else if strings.ToUpper(command[0]) == "KEYS" {
				handleKeys(connection, command, st)
			} else if strings.ToUpper(command[0]) == "SAVE" {
				handleSave(connection, command, st)
			} else if strings.ToUpper(command[0]) == "REPLCONF" {
				replconfHandle(connection, command, &knownReplicas, &needed)
			} else if strings.ToUpper(command[0]) == "PSYNC" {
				psyncHandle(connection, command)
			}
		}
	}
}
