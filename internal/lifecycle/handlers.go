package lifecycle

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

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
	err := propagate([]net.Conn{}, codec.EncodeArray(command))
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
		return
	}
	// without timestamp
	if len(command) == 3 {
		st.Set(command[1], command[2])
		_, _ = conn.Write(codec.EncodeSimpleString("OK"))
		return
	}
	// with timestamp
	exp, err := parseExpiration(command)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(err))
		return
	}
	st.SetWithExpiry(command[1], command[2], exp)
	_, _ = conn.Write(codec.EncodeSimpleString("OK"))
}

func propagate(knownReplicas []net.Conn, data []byte) error {
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
	if len(command) == 1 {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("wrong usage of KEYS command, expect some arguments")))
		return
	}
	if command[1] != "*" {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("KEYS command not fully implemented")))
		return
	}
	keys := st.Keys(command, command[1])
	_, _ = conn.Write(codec.EncodeArray(keys))
}

func handleSave(conn net.Conn, command []string, st *storage.Storage) {
	rdb := storage.EncodeToRDB(st)
	var fname string
	if len(command) > 1 {
		fname = command[1]
	} else {
		fname = "dump.rdb"
	}
	f, err := os.Create(fname)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(err))
		return
	}
	_, err = f.Write(rdb)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("can't write data to rdb file: %w", err)))
		return
	}
	_, _ = conn.Write(codec.EncodeSimpleString("OK"))
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

func sendRdbFile(connection net.Conn) error {
	file, err := os.ReadFile("empty.rdb")
	if err != nil {
		return fmt.Errorf("can't read rdb file: %w", err)
	}
	_, _ = connection.Write(codec.EncodeBulkString(string(file)))
	return nil
}
