package main

import (
	"fmt"
	"net"
	"os"
    "strings"
    "strconv"
    "time"
)

type NowAndDuration struct {
    now time.Time
    expires time.Duration
}

var storage map[string]string = make(map[string]string)
var timestamps map[string]NowAndDuration = make(map[string]NowAndDuration)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

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
		if strings.ToUpper(parsedData[0]) == "PING"{
			connection.Write([]byte("+PONG\r\n"))
		} else if strings.ToUpper(parsedData[0]) == "ECHO"{
			connection.Write(readBuffer[14:n])
		} else if strings.ToUpper(parsedData[0]) == "SET"{
            if len(parsedData) > 3 && parsedData[3] == "px"{
                parsed, err := strconv.Atoi(parsedData[4])
                if err != nil {
                    fmt.Fprintf(os.Stderr, "Invalid data for time delay\n Can't parse %v to int\n", parsedData[4])
                    os.Exit(1)
                }
                nad := new(NowAndDuration)
                nad.expires = time.Duration(parsed * 1_000_000)
                nad.now = time.Now()
                timestamps[parsedData[1]] = *nad
            }
            storage[parsedData[1]] = parsedData[2]
            connection.Write([]byte("+OK\r\n"))
		} else if strings.ToUpper(parsedData[0]) == "GET"{
            nad, exist := timestamps[parsedData[1]]
            if !exist{
                retStr := fmt.Sprintf("+%v\r\n", storage[parsedData[1]])
                connection.Write([]byte(retStr))
            } else {
                if time.Now().Sub(nad.now) > nad.expires{
                    retStr := fmt.Sprintf("$-1\r\n")
                    connection.Write([]byte(retStr))
                } else {
                    retStr := fmt.Sprintf("+%v\r\n", storage[parsedData[1]])
                    connection.Write([]byte(retStr))
                }
            }
		}
	}
}

func parse(buffer []byte) []string {
    splited := strings.Split(string(buffer), "\r\n") 
    var ret []string
    for i := 1; i < len(splited); i++{
        if i % 2 == 0 {
            ret = append(ret, splited[i])
        }
    }
    return ret
}
