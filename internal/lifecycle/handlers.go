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
	if len(command) != 2 {
		_, _ = conn.Write(codec.EncodeError(fmt.Errorf("ERR wrong number of arguments for 'echo' command")))
		return
	}
	_, _ = conn.Write(codec.EncodeBulkString(command[1]))
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

type setCommand struct {
	SetIfNotExist bool // NX flag
	SetIfExist    bool // XX flag
	KeepTTL       bool
	Get           bool

	Key string
	Val string
	Exp *time.Time // EX | PX | EXAT | PXAT
}

func parseSetCommand(raw []string) (setCommand, error) {
	if len(raw) < 3 {
		return setCommand{}, fmt.Errorf("ERR wrong number of arguments for 'set' command")
	}
	key, val := raw[1], raw[2]
	i := 3
	command := setCommand{
		Key: key,
		Val: val,
	}
	for i < len(raw) {
		switch strings.ToUpper(raw[i]) {
		case "EX", "PX", "EXAT", "PXAT":
			if i+1 >= len(raw) {
				return setCommand{}, fmt.Errorf("ERR wrong number of arguments for 'set' command")
			}
			t, err := parseExpiration(raw[i], raw[i+1])
			if err != nil {
				return setCommand{}, err
			}
			command.Exp = &t
			i++
		case "NX":
			command.SetIfNotExist = true
		case "XX":
			command.SetIfExist = true
		case "KEEPTTL":
			command.KeepTTL = true
		case "GET":
			command.Get = true
		default:
			return setCommand{}, fmt.Errorf("ERR unknown option '%v' for 'set' command", raw[i])
		}
		i++
	}

	return command, nil
}

func handleSet(conn net.Conn, command []string, st *storage.Storage) {
	setCommand, err := parseSetCommand(command)
	if err != nil {
		_, _ = conn.Write(codec.EncodeError(err))
		return
	}

	err = propagate([]net.Conn{}, codec.EncodeArray(command))
	if err != nil {
		slog.Error("can't propagate SET command", "err", err)
	}

	if setCommand.Exp != nil {
		st.SetWithExpiry(setCommand.Key, setCommand.Val, *setCommand.Exp)
	} else {
		st.Set(setCommand.Key, setCommand.Val)
	}
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

func parseExpiration(expModifier /* EX, PX, EXAT, PXAT */, expValue string) (time.Time, error) {
	expModifier = strings.ToUpper(expModifier)
	parsed, err := strconv.ParseInt(expValue, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid data for time delay\n can't parse %v to int",
			expValue,
		)
	}
	now := time.Now()
	switch expModifier {
	case "EX":
		return now.Add(time.Duration(parsed) * time.Second), nil
	case "PX":
		return now.Add(time.Duration(parsed) * time.Millisecond), nil
	case "EXAT":
		return time.Unix(parsed, 0), nil
	case "PXAT":
		return time.UnixMilli(parsed), nil
	default:
		return time.Time{}, fmt.Errorf("ERR unknown time modifier '%v'", expModifier)
	}
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
