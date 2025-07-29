package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

var knownReplicas = make([]net.Conn, 0)

func main() {
	fmt.Println("Rediska startup")
	settings := GetSettings()
	args := os.Args
	for i, arg := range args {
		if arg == "--dir" {
			settings.RdbDir = args[i+1]
		}
		if arg == "--dbfilename" {
			settings.RdbFilename = args[i+1]
		}
		if arg == "--port" {
			settings.Port = args[i+1]
		}
		if arg == "--replicaof" {
			settings.Role = Replica
			hostPort := strings.Split(args[i+1], " ")
			masterHost := hostPort[0]
			masterPort := hostPort[1]
			fmt.Println(masterHost, masterPort)
			settings.MasterHost = masterHost
			settings.MasterPort = masterPort
			go Handshake()
		}
	}

	if settings.RdbDir != "" || settings.RdbFilename != "" {
		LoadSave(settings.RdbDir+"/", settings.RdbFilename)
	}

	listen(settings)
}

func listen(settings *redisConfig) {
	listner, err := net.Listen("tcp", "0.0.0.0:"+settings.Port)
	defer func() {
		fmt.Printf("Close listner on %v:%v\n", "0.0.0.0", settings.Port)
		err := listner.Close()
		if err != nil {
			panic("failed to Close listner")
		}
	}()
	if err != nil {
		fmt.Println("Failed to bind to port " + config.Port)
		os.Exit(1)
	}
	for {
		connection, err := listner.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(connection)
	}
}

func handleConnection(connection net.Conn) {
	needed := false
	defer func() {
		if !needed {
			err := connection.Close()
			if err != nil {
				panic("Can't close connection, cause " + err.Error())
			}
		}
	}()
	readBuffer := make([]byte, 1024)
	for {
		n, err := connection.Read(readBuffer)
		if n == 0 {
			break
		}
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		fmt.Printf("%v bytes recieved\n", n)
		parsedData := Parse(readBuffer)
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
				Propagate(EncodeArray(command))
				go Set(command, connection)
			} else if strings.ToUpper(command[0]) == "GET" {
				Get(command, connection)
			} else if strings.ToUpper(command[0]) == "CONFIG" {
				if strings.ToUpper(command[1]) == "GET" {
					if strings.ToUpper(command[2]) == "DIR" {
						retStr := fmt.Sprintf("*2\r\n$3\r\ndir\r\n$%v\r\n%v\r\n", len(GetSettings().RdbDir), GetSettings().RdbDir)
						connection.Write([]byte(retStr))
					} else if strings.ToUpper(command[2]) == "DBFILENAME" {
						retStr := fmt.Sprintf("*2\r\n$10\r\ndbfilename\r\n$%v\r\n%v\r\n", len(GetSettings().RdbFilename), GetSettings().RdbFilename)
						connection.Write([]byte(retStr))
					}
				}
			} else if strings.ToUpper(command[0]) == "INFO" {
				connection.Write([]byte(GetInfo()))
			} else if strings.ToUpper(command[0]) == "KEYS" {
				if command[1] != "*" {
					panic("KEYS command not fully implemented\n")
				}
				Keys(command, connection, command[1])
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
}

func Propagate(data []byte) {
	for _, conn := range knownReplicas {
		fmt.Println("writing to conn", conn.RemoteAddr().String())
		conn.Write(data)
	}
}

func sendRdbFile(connection net.Conn) {
	file, err := os.ReadFile("empty.rdb")
	if err != nil {
		panic("Can't read rdb file " + err.Error())
	}
	length := len(file)
	connection.Write([]byte(fmt.Sprintf("$%d\r\n%s", length, file)))
}

func Ping(conn net.Conn) {
	s := "*1\r\n$4\r\nPING\r\n"
	conn.Write([]byte(s))
}

func GetMasterConnection() net.Conn {
	masterConn, err := net.Dial("tcp", GetSettings().MasterHost+":"+GetSettings().MasterPort)
	if err != nil {
		panic("Can't connect to master:" + err.Error())
	}
	return masterConn
}

func Handshake() {
	conn := GetMasterConnection()
	buffer := make([]byte, 100)
	Ping(conn)
	_, err := conn.Read(buffer)
	fmt.Println(string(buffer))
	if err != nil {
		panic("Can't read from master: " + err.Error())
	}
	ReplconfPort(conn)
	_, err = conn.Read(buffer)
	fmt.Println(string(buffer))
	if err != nil {
		panic("Can't read from master: " + err.Error())
	}
	ReplconfCapa(conn)
	_, err = conn.Read(buffer)
	fmt.Println(string(buffer))
	if err != nil {
		panic("Can't read from master: " + err.Error())
	}
	Psync(conn)
	_, err = conn.Read(buffer)
	if err != nil {
		panic("Can't read from master: " + err.Error())
	}
	go handleConnection(conn)
}

func ReplconfPort(conn net.Conn) {
	s := fmt.Sprintf("*3\r\n$8\r\nREPLCONF\r\n$14\r\nlistening-port\r\n$4\r\n%v\r\n", GetSettings().Port)
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
