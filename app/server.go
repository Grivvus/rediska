package main

import (
	"fmt"
	"net"
	"os"
)

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
	read_buffer := make([]byte, 100)
    for {
        n, err := connection.Read(read_buffer)
        if n == 0 {
            break
        }
        if err != nil {
            fmt.Println("Error accepting connection: ", err.Error())
            os.Exit(1)
        }
        fmt.Printf("%v bytes recieved\n", n)
        if string(read_buffer[:n]) == "*1\r\n$4\r\nPING\r\n" {
            connection.Write([]byte("+PONG\r\n"))
        }
    }
}
