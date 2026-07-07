package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/flags"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, os.Kill, syscall.SIGTERM,
	)
	defer cancel()

	slog.Info("Rediska startup")
	providedFlags := flags.Parse()

	cfg := config.Default()
	cfg.WithFlags(providedFlags)

	st := storage.NewStorage(*cfg)

	if cfg.Role == config.ReplicaRole {
		go func() {
			err := Handshake(ctx, *cfg, st)
			if err != nil {
				slog.Error("error during handshake", "err", err)
			}
		}()
	}

	if cfg.RdbDir != "" || cfg.RdbFilename != "" {
		slog.Warn("can't load config save from file", "cause", "not implemented")
		// LoadSave(cfg.RdbDir+"/", cfg.RdbFilename)
	}

	err := listen(ctx, *cfg, st)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	<-ctx.Done()
	log.Println("rediska shutdown")
}

func listen(
	ctx context.Context, cfg config.RedisConfig, st *storage.Storage,
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

		slog.Info("Close listner", "host", "0.0.0.0", "port", cfg.Port)
		_ = listner.Close()
	}()

	for {
		connection, err := listner.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("", "err", fmt.Errorf("can't accept connection: %w", err))
			return err
		}
		go func() {
			handleConnection(ctx, cfg, connection, st, []net.Conn{})
		}()
	}
}

func handleConnection(
	ctx context.Context, cfg config.RedisConfig, connection net.Conn,
	st *storage.Storage, knownReplicas []net.Conn,
) {
	needed := false
	defer func() {
		if !needed {
			_ = connection.Close()
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
				slog.Error("error while reading from the connection", "err", err)
				continue
			} else if errors.Is(err, io.EOF) || n == 0 {
				return
			}
			if n == len(readBuffer) {
				slog.Error("can't fit message into buffer", "len buffer", len(readBuffer))
				connection.Write(codec.EncodeError(fmt.Errorf("message is too long")))
				// return because now in connection lays some garbage
				// that we couldn't fit into buffer
				// so we could close the connection or read full message
				// and then start processing next one
				return
			}
			slog.Info("", "bytes recieved", n, "conn", connection)
			parsedData, err := codec.Parse(readBuffer)
			if err != nil {
				connection.Write(codec.EncodeError(fmt.Errorf("can't parse accepted data: %w", err)))
				continue
			}
			for _, command := range parsedData {
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
					_, _ = connection.Write(codec.EncodeString(cfg.GetInfo()))
				} else if strings.ToUpper(command[0]) == "KEYS" {
					handleKeys(connection, command, st)
				} else if strings.ToUpper(command[0]) == "SAVE" {
					_, _ = connection.Write(codec.EncodeError(fmt.Errorf("SAVE command is not implemented yet")))
				} else if strings.ToUpper(command[0]) == "REPLCONF" {
					replconfHandle(connection, command, knownReplicas, &needed)
				} else if strings.ToUpper(command[0]) == "PSYNC" {
					psyncHandle(connection, command)
				}
			}
		}
	}
}

func handlePing(conn net.Conn, command []string) {
	_ = command
	_, _ = conn.Write([]byte("+PONG\r\n"))
}

func handleEcho(conn net.Conn, command []string) {
	_, _ = conn.Write(codec.EncodeArray(command[1:]))
}

func handleGet(conn net.Conn, command []string, st *storage.Storage) {
	msg := st.Get(command)
	if msg != nil {
		_, _ = conn.Write(msg)
	}
}

func handleSet(conn net.Conn, command []string, st *storage.Storage) {
	Propagate([]net.Conn{}, codec.EncodeArray(command))
	msg, err := st.Set(command)
	if err != nil {
		msg := codec.EncodeError(fmt.Errorf("error appeared during SET command: %w", err))
		_, _ = conn.Write(msg)
		return
	}
	if msg != nil {
		_, _ = conn.Write(msg)
	}
}

func handleConfig(conn net.Conn, command []string, cfg config.RedisConfig) {
	if strings.ToUpper(command[1]) == "GET" {
		if strings.ToUpper(command[2]) == "DIR" {
			_, _ = conn.Write(codec.EncodeArray([]string{"dir", cfg.RdbDir}))
		} else if strings.ToUpper(command[2]) == "DBFILENAME" {
			_, _ = conn.Write([]byte(codec.EncodeArray([]string{"dbfilename", cfg.RdbFilename})))
		}
	}
}

func handleKeys(conn net.Conn, command []string, st *storage.Storage) {
	if command[1] != "*" {
		conn.Write(codec.EncodeError(fmt.Errorf("KEYS command not fully implemented")))
		return
	}
	st.Keys(command, command[1])
}

func replconfHandle(conn net.Conn, command []string, knownReplicas []net.Conn, neededFlag *bool) {
	const retStr = "+OK\r\n"
	if command[1] == "listening-port" {
		knownReplicas = append(knownReplicas, conn)
		*neededFlag = true
	}
	_, _ = conn.Write([]byte(retStr))
}

func psyncHandle(conn net.Conn, command []string) {
	_ = command
	const masterID = "8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb"
	retStr := fmt.Sprintf("+FULLRESYNC %s 0\r\n", masterID)
	_, _ = conn.Write([]byte(retStr))
	err := sendRdbFile(conn)
	if err != nil {
		conn.Write(codec.EncodeError(fmt.Errorf("error appeared during PSYNC: %w", err)))
	}
}

func Propagate(knownReplicas []net.Conn, data []byte) {
	for _, conn := range knownReplicas {
		slog.Info(
			"propagate to replica",
			"addr", conn.RemoteAddr().String(),
			"data", data,
		)
		_, _ = conn.Write(data)
	}
}

func sendRdbFile(connection net.Conn) error {
	file, err := os.ReadFile("empty.rdb")
	if err != nil {
		return fmt.Errorf("can't read rdb file: %w", err)
	}
	length := len(file)
	_, err = fmt.Fprintf(connection, "$%d\r\n%s", length, file)
	if err != nil {
		return fmt.Errorf("can't write rdb file to replica connection: %w", err)
	}
	return nil
}

func Ping(conn net.Conn) {
	_, _ = conn.Write(codec.EncodeArray([]string{"PING"}))
}

func Handshake(
	ctx context.Context, cfg config.RedisConfig, st *storage.Storage,
) error {
	conn, err := GetMasterConnection(cfg)
	if err != nil {
		return err
	}
	buffer := make([]byte, 100)
	Ping(conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	ReplconfPort(cfg, conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	ReplconfCapa(conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	Psync(conn)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	go func() {
		handleConnection(ctx, cfg, conn, st, []net.Conn{})
	}()
	return nil
}

func GetMasterConnection(cfg config.RedisConfig) (net.Conn, error) {
	masterConn, err := net.Dial("tcp", cfg.MasterHost+":"+cfg.MasterPort)
	if err != nil {
		return nil, fmt.Errorf("can't connect to master: %w", err)
	}
	return masterConn, nil
}

func ReplconfPort(cfg config.RedisConfig, conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"REPLCONF", "listening-port", cfg.Port})))
}

func ReplconfCapa(conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"REPLCONF", "capa", "psync2"})))
}

func Psync(conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"PSYNC", "?", "-1"})))
}
