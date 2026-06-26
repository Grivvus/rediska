package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/encoder"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

func main() {
	fmt.Println("Rediska startup")
	cfg := config.Default()
	st := storage.NewStorage(*cfg)
	args := os.Args
	for i, arg := range args {
		if arg == "--dir" {
			cfg.RdbDir = args[i+1]
		}
		if arg == "--dbfilename" {
			cfg.RdbFilename = args[i+1]
		}
		if arg == "--port" {
			cfg.Port = args[i+1]
		}
		if arg == "--replicaof" {
			cfg.Role = config.ReplicaRole
			hostPort := strings.Split(args[i+1], " ")
			masterHost := hostPort[0]
			masterPort := hostPort[1]
			fmt.Println(masterHost, masterPort)
			cfg.MasterHost = masterHost
			cfg.MasterPort = masterPort
			go Handshake(*cfg, st)
		}
	}

	if cfg.RdbDir != "" || cfg.RdbFilename != "" {
		slog.Warn("can't load config save from file")
		// LoadSave(cfg.RdbDir+"/", cfg.RdbFilename)
	}

	listen(*cfg, st)
}

func listen(cfg config.RedisConfig, st *storage.Storage) error {
	listner, err := net.Listen("tcp", "0.0.0.0:"+cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to bind to port %v: %w", cfg.Port, err)
	}
	defer func() {
		slog.Info("Close listner", "host", "0.0.0.0", "port", cfg.Port)
		_ = listner.Close()
	}()
	for {
		connection, err := listner.Accept()
		if err != nil {
			return fmt.Errorf("can't accept connection: %w", err)
		}
		go handleConnection(cfg, connection, st, []net.Conn{})
	}
}

func handleConnection(
	cfg config.RedisConfig, connection net.Conn,
	st *storage.Storage, knownReplicas []net.Conn,
) error {
	needed := false
	defer func() {
		if !needed {
			_ = connection.Close()
		}
	}()
	readBuffer := make([]byte, 1024)
	for {
		n, err := connection.Read(readBuffer)
		if n == 0 {
			break
		}
		if err != nil {
			return fmt.Errorf("error accepting connection: %w", err)
		}
		log.Printf("%v bytes recieved\n", n)
		parsedData, err := encoder.Parse(readBuffer)
		if err != nil {
			return fmt.Errorf("can't parse accepted data: %w", err)
		}
		fmt.Println(parsedData)
		if len(parsedData) == 0 {
			continue
		}
		for _, command := range parsedData {
			if strings.ToUpper(command[0]) == "PING" {
				connection.Write([]byte("+PONG\r\n"))
			} else if strings.ToUpper(command[0]) == "ECHO" {
				connection.Write(readBuffer[14:n])
			} else if strings.ToUpper(command[0]) == "SET" {
				Propagate([]net.Conn{}, encoder.EncodeArray(command))
				msg, err := st.Set(command)
				if err != nil {
				}
				if msg != nil {
					_, _ = connection.Write(msg)
				}
			} else if strings.ToUpper(command[0]) == "GET" {
				msg := st.Get(command)
				if msg != nil {
					_, _ = connection.Write(msg)
				}
			} else if strings.ToUpper(command[0]) == "CONFIG" {
				if strings.ToUpper(command[1]) == "GET" {
					if strings.ToUpper(command[2]) == "DIR" {
						retStr := fmt.Sprintf("*2\r\n$3\r\ndir\r\n$%v\r\n%v\r\n", len(cfg.RdbDir), cfg.RdbDir)
						connection.Write([]byte(retStr))
					} else if strings.ToUpper(command[2]) == "DBFILENAME" {
						retStr := fmt.Sprintf("*2\r\n$10\r\ndbfilename\r\n$%v\r\n%v\r\n", len(cfg.RdbFilename), cfg.RdbFilename)
						connection.Write([]byte(retStr))
					}
				}
			} else if strings.ToUpper(command[0]) == "INFO" {
				connection.Write(encoder.EncodeString(cfg.GetInfo()))
			} else if strings.ToUpper(command[0]) == "KEYS" {
				if command[1] != "*" {
					return fmt.Errorf("KEYS command not fully implemented")
				}
				st.Keys(command, command[1])
			} else if strings.ToUpper(command[0]) == "SAVE" {
			} else if strings.ToUpper(command[0]) == "REPLCONF" {
				const retStr = "+OK\r\n"
				if command[1] == "listening-port" {
					knownReplicas = append(knownReplicas, connection)
					needed = true
				}
				connection.Write([]byte(retStr))
			} else if strings.ToUpper(command[0]) == "PSYNC" {
				const masterID = "8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb"
				retStr := fmt.Sprintf("+FULLRESYNC %s 0\r\n", masterID)
				connection.Write([]byte(retStr))
				sendRdbFile(connection)
			}
		}
	}

	return nil
}

func Propagate(knownReplicas []net.Conn, data []byte) {
	for _, conn := range knownReplicas {
		slog.Info(
			"propagate to replica",
			"addr", conn.RemoteAddr().String(),
			"data", data,
		)
		conn.Write(data)
	}
}

func sendRdbFile(connection net.Conn) error {
	file, err := os.ReadFile("empty.rdb")
	if err != nil {
		return fmt.Errorf("can't read rdb file: %w", err)
	}
	length := len(file)
	_, err = connection.Write([]byte(fmt.Sprintf("$%d\r\n%s", length, file)))
	if err != nil {
		return fmt.Errorf("can't write rdb file to replica connection: %w", err)
	}
	return nil
}

func Ping(conn net.Conn) {
	s := "*1\r\n$4\r\nPING\r\n"
	conn.Write([]byte(s))
}

func Handshake(cfg config.RedisConfig, st *storage.Storage) error {
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
	go handleConnection(cfg, conn, st, []net.Conn{})
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
	s := fmt.Sprintf("*3\r\n$8\r\nREPLCONF\r\n$14\r\nlistening-port\r\n$4\r\n%v\r\n", cfg.Port)
	conn.Write([]byte(s))
}

func ReplconfCapa(conn net.Conn) {
	s := "*3\r\n$8\r\nREPLCONF\r\n$4\r\ncapa\r\n$6\r\npsync2\r\n"
	conn.Write([]byte(s))
}

func Psync(conn net.Conn) {
	s := "*3\r\n$5\r\nPSYNC\r\n$1\r\n?\r\n$2\r\n-1\r\n"
	conn.Write([]byte(s))
}
