package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")
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
	}
	if settings.RdbDir != "" || settings.RdbFilename != "" {
		LoadSave(settings.RdbDir+"/", settings.RdbFilename)
	}

	listner, err := net.Listen("tcp", "0.0.0.0:"+settings.Port)
	defer func() {
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
	defer connection.Close()
	readBuffer := make([]byte, 100)
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
		if strings.ToUpper(parsedData[0]) == "PING" {
			connection.Write([]byte("+PONG\r\n"))
		} else if strings.ToUpper(parsedData[0]) == "ECHO" {
			connection.Write(readBuffer[14:n])
		} else if strings.ToUpper(parsedData[0]) == "SET" {
			Set(parsedData, connection)
		} else if strings.ToUpper(parsedData[0]) == "GET" {
			Get(parsedData, connection)
		} else if strings.ToUpper(parsedData[0]) == "CONFIG" {
			if strings.ToUpper(parsedData[1]) == "GET" {
				if strings.ToUpper(parsedData[2]) == "DIR" {
					retStr := fmt.Sprintf("*2\r\n$3\r\ndir\r\n$%v\r\n%v\r\n", len(GetSettings().RdbDir), GetSettings().RdbDir)
					connection.Write([]byte(retStr))
				} else if strings.ToUpper(parsedData[2]) == "DBFILENAME" {
					retStr := fmt.Sprintf("*2\r\n$10\r\ndbfilename\r\n$%v\r\n%v\r\n", len(GetSettings().RdbFilename), GetSettings().RdbFilename)
					connection.Write([]byte(retStr))
				}
			}
		} else if strings.ToUpper(parsedData[0]) == "INFO" {
			connection.Write([]byte(GetInfo()))
		} else if strings.ToUpper(parsedData[0]) == "KEYS" {
			if parsedData[1] != "*" {
				panic("KEYS command not fully implemented\n")
			}
			Keys(parsedData, connection, parsedData[1])
		} else if strings.ToUpper(parsedData[0]) == "SAVE" {

		}
	}
}

func Encode(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func Merge(s1, s2 string) string {
	return s1 + s2
}
