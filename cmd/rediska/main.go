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
	"strconv"
	"strings"
	"syscall"
	"time"

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
		f, err := os.Open(cfg.RdbDir + cfg.RdbFilename)
		if err != nil {
			slog.Error("can't open rdb file", "err", err)
			return
		}
		rdb, err := storage.DecodeRDB(f)
		if err != nil {
			slog.Error("can't parse rdb file", "err", err)
			return
		}
		rdb.Apply(st)
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
				_, _ = connection.Write(codec.EncodeError(fmt.Errorf("message is too long")))
				// return because now in connection lays some garbage
				// that we couldn't fit into buffer
				// so we could close the connection or read full message
				// and then start processing next one
				return
			}
			slog.Info("", "bytes recieved", n, "conn", connection)
			parsedData, err := codec.Parse(readBuffer)
			if err != nil {
				_, _ = connection.Write(codec.EncodeError(fmt.Errorf("can't parse accepted data: %w", err)))
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
					_, _ = connection.Write(codec.EncodeBulkString(cfg.GetInfo()))
				} else if strings.ToUpper(command[0]) == "KEYS" {
					handleKeys(connection, command, st)
				} else if strings.ToUpper(command[0]) == "SAVE" {
					_, _ = connection.Write(codec.EncodeError(fmt.Errorf("SAVE command is not implemented yet")))
				} else if strings.ToUpper(command[0]) == "REPLCONF" {
					replconfHandle(connection, command, &knownReplicas, &needed)
				} else if strings.ToUpper(command[0]) == "PSYNC" {
					psyncHandle(connection, command)
				}
			}
		}
	}
}

func handlePing(conn net.Conn, command []string) {
	_ = command
	_, _ = conn.Write(codec.EncodeSimpleString("PONG"))
}

func handleEcho(conn net.Conn, command []string) {
	_, _ = conn.Write(codec.EncodeArray(command[1:]))
}

func handleGet(conn net.Conn, command []string, st *storage.Storage) {
	msg, err := st.Get(command[1])
	// expired or no value
	if err != nil {
		_, _ = conn.Write(codec.NullBulkString())
		return
	}
	_, _ = conn.Write(codec.EncodeBulkString(msg))
}

func handleSet(conn net.Conn, command []string, st *storage.Storage) {
	err := Propagate([]net.Conn{}, codec.EncodeArray(command))
	if err != nil {
		slog.Error("can't propagate SET command", "err", err)
	}
	if len(command) != 3 && len(command) != 5 {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf(
			`unexpected number of arguments,
			exepcted 3 for value without timestamp and 5 for value with timestamp,
			but got %v`,
			len(command),
		)))
	}
	// without timestamp
	if len(command) == 3 {
		st.Set(command[1], command[2])
		return
	}
	// with timestamp
	exp, err := parseExpiration(command)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(err))
		return
	}
	st.SetWithExpiry(command[1], command[2], exp)
}

func parseExpiration(command []string) (time.Time, error) {
	if command[3] != "px" {
		return time.Time{}, fmt.Errorf(
			"unexpected message format, expect 'px' for timestamp, got '%v'",
			command[3],
		)
	}
	parsed, err := strconv.Atoi(command[4])
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid data for time delay\n Can't parse %v to int",
			command[4],
		)
	}
	exp := time.Now().Add(time.Duration(parsed) * time.Millisecond)
	return exp, nil
}

func handleConfig(conn net.Conn, command []string, cfg config.RedisConfig) {
	if strings.ToUpper(command[1]) == "GET" {
		if strings.ToUpper(command[2]) == "DIR" {
			_, _ = conn.Write(codec.EncodeArray([]string{"dir", cfg.RdbDir}))
		} else if strings.ToUpper(command[2]) == "DBFILENAME" {
			_, _ = conn.Write([]byte(codec.EncodeArray(
				[]string{"dbfilename", cfg.RdbFilename},
			)))
		}
	}
}

func handleKeys(conn net.Conn, command []string, st *storage.Storage) {
	if command[1] != "*" {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("KEYS command not fully implemented")))
		return
	}
	keys := st.Keys(command, command[1])
	_, _ = conn.Write(codec.EncodeArray(keys))
}

func replconfHandle(conn net.Conn, command []string, knownReplicas *[]net.Conn, neededFlag *bool) {
	if command[1] == "listening-port" {
		*knownReplicas = append(*knownReplicas, conn)
		*neededFlag = true
	}
	_, _ = conn.Write(codec.EncodeSimpleString("OK"))
}

func psyncHandle(conn net.Conn, command []string) {
	_ = command
	const masterID = "8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb"
	_, _ = conn.Write(codec.EncodeSimpleString("FULLRESYNC " + masterID + " 0"))
	err := sendRdbFile(conn)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("error appeared during PSYNC: %w", err)))
	}
}

func Propagate(knownReplicas []net.Conn, data []byte) error {
	var compositError error
	for _, conn := range knownReplicas {
		slog.Info(
			"propagate to replica",
			"addr", conn.RemoteAddr().String(),
			"data", data,
		)
		_, err := conn.Write(data)
		compositError = errors.Join(compositError, fmt.Errorf("can't propagate to connection %v: %w", conn, err))
	}

	return compositError
}

func sendRdbFile(connection net.Conn) error {
	file, err := os.ReadFile("empty.rdb")
	if err != nil {
		return fmt.Errorf("can't read rdb file: %w", err)
	}
	_, _ = connection.Write(codec.EncodeBulkString(string(file)))
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
