package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type NowAndDuration struct {
	now     time.Time
	expires time.Duration
}

var dir string
var dbfilename string

var storageMu sync.Mutex = *new(sync.Mutex)
var storage map[string]string = make(map[string]string)
var timeMu sync.Mutex = *new(sync.Mutex)
var timestamps map[string]NowAndDuration = make(map[string]NowAndDuration)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")
	args := os.Args
	for i, arg := range args {
		if arg == "--dir" {
			dir = args[i+1]
		}
		if arg == "--dbfilename" {
			dbfilename = args[i+1]
		}
	}

	listner, err := net.Listen("tcp", "0.0.0.0:6379")
	defer listner.Close()
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
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
		parsedData := parse(readBuffer)
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
					retStr := fmt.Sprintf("*2\r\n$3\r\ndir\r\n$%v\r\n%v\r\n", len(dir), dir)
					connection.Write([]byte(retStr))
				} else if strings.ToUpper(parsedData[2]) == "DBFILENAME" {
					retStr := fmt.Sprintf("*2\r\n$10\r\ndbfilename\r\n$%v\r\n%v\r\n", len(dbfilename), dbfilename)
					connection.Write([]byte(retStr))
				}
			}
		}
	}
}

func parse(buffer []byte) []string {
	splited := strings.Split(string(buffer), "\r\n")
	var ret []string
	for i := 1; i < len(splited); i++ {
		if i%2 == 0 {
			ret = append(ret, splited[i])
		}
	}
	return ret
}

func Set(parsedData []string, connection net.Conn) {
	storageMu.Lock()
	if len(parsedData) > 3 && parsedData[3] == "px" {
		timeMu.Lock()
		parsed, err := strconv.Atoi(parsedData[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid data for time delay\n Can't parse %v to int\n", parsedData[4])
			os.Exit(1)
		}
		nad := new(NowAndDuration)
		nad.expires = time.Duration(parsed * 1_000_000)
		nad.now = time.Now()
		timestamps[parsedData[1]] = *nad
		timeMu.Unlock()
	}
	storage[parsedData[1]] = parsedData[2]
	storageMu.Unlock()
	connection.Write([]byte("+OK\r\n"))
}

func Get(parsedData []string, connection net.Conn) {
	timeMu.Lock()
	storageMu.Lock()
	nad, exist := timestamps[parsedData[1]]
	if !exist {
		retStr := fmt.Sprintf("+%v\r\n", storage[parsedData[1]])
		connection.Write([]byte(retStr))
	} else {
		if time.Now().Sub(nad.now) > nad.expires {
			retStr := fmt.Sprintf("$-1\r\n")
			connection.Write([]byte(retStr))
		} else {
			retStr := fmt.Sprintf("+%v\r\n", storage[parsedData[1]])
			connection.Write([]byte(retStr))
		}
	}
	timeMu.Unlock()
	storageMu.Unlock()
}
