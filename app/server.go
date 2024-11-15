package main

import (
	"fmt"
	"io"
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
    if dir != "" || dbfilename != ""{
	    LoadSave(dir + "/", dbfilename)
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
		} else if strings.ToUpper(parsedData[0]) == "KEYS" {
			if parsedData[1] != "*" {
				panic("KEYS command not fully implemented\n")
			}
			Keys(parsedData, connection, parsedData[1])
		} else if strings.ToUpper(parsedData[0]) == "SAVE" {

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

func LoadSave(dir, dbfilename string) {
	parsedFile := ParseRDBFile(dir, dbfilename)
    var fileStorage map[string]string
    var fileTime map[string]NowAndDuration
    if parsedFile == nil {
        return
    }
	fileStorage, fileTime = ParseDB(parsedFile[2])
    storageMu.Lock()
    defer storageMu.Unlock()
    for key, value := range fileStorage {
        storage[key] = value
    }
    timeMu.Lock()
    defer timeMu.Unlock()
    for key, value := range fileTime {
        timestamps[key] = value 
    }
}

func ParseRDBFile(dir, dbfilename string) [][]byte {
	var file *os.File
	_, err := os.Stat(dir + dbfilename)
	if err != nil {
		file, err = os.Create(dir + dbfilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unexpected error: couldn't create file; err=%v\n", err)
			os.Exit(1)
		}
	} else {
		file, err = os.Open(dir + dbfilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unexpected error: couldn't read file; err=%v\n", err)
			os.Exit(1)
		}
	}
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unexpected error: couldn't read from file; err=%v\n", err)
		os.Exit(1)
	}
	redisMagicString := data[:9]
	metadataStart := FindBytes(data, 0xfa)
	dbStart := FindBytes(data, 0xfe)
	endOfFile := FindBytes(data, 0xff)
    if dbStart == -1 || metadataStart == -1 || endOfFile == -1 {
        fmt.Fprintf(os.Stderr, "metadataStart = %v, dbStart = %v, endOfFile = %v\n", metadataStart, dbStart, endOfFile)
        return nil
    }
	metadataSection := data[metadataStart:dbStart]
	dbSection := data[dbStart:endOfFile]
	crc64Hash := data[endOfFile+1:]
	return [][]byte{redisMagicString, metadataSection, dbSection, crc64Hash}
}

func ParseDB(dbSection []byte) (map[string]string, map[string]NowAndDuration) {
	dbSections := make([][]byte, 0)
	dbSectionStringSplitted := strings.Split(string(dbSection), string(0xfe))
	for _, str := range dbSectionStringSplitted {
		dbSections = append(dbSections, []byte(str))
	}
    fileStorage := make(map[string]string)
    fileTime := make(map[string]NowAndDuration)
    for _, section := range dbSections {
        ParseDBSection(section, fileStorage, fileTime)
    }
    return fileStorage, fileTime
}

func ParseDBSection(dbSection []byte, fileStorage map[string]string, fileTime map[string]NowAndDuration) {
    dbIndex := dbSection[1]
    var valuesNumber byte
    var expiresNumber byte
    // may needed later
    _ = expiresNumber
    _ = dbIndex
    if dbSection[2] == 0xfb {
        valuesNumber = dbSection[3]
        expiresNumber = dbSection[4]
    }
    i := 5
    for range valuesNumber {
        if dbSection[i] == 0x00 {
            // no time expiration
            // parsing regular value
            i++
            lenOfKey := dbSection[i]
            i++
            key := dbSection[i:i + int(lenOfKey)]
            i += int(lenOfKey)
            lenOfValue := dbSection[i]
            i++
            value := dbSection[i: i + int(lenOfValue)]
            i += int(lenOfKey)
            fileStorage[string(key)] = string(value)
        } else if dbSection[i] == 0xfc {
            // timestamp in millisecond
            // 8 bytes for it
            // 8 bytes unsigned
            panic("Not implemented")
        } else if dbSection[i] == 0xfd {
            // timestamp in seconds
            // 4 bytes value
            // 4 bytes unsigned
            panic("Not implemented")
        } else {
            fmt.Fprintf(os.Stderr, "Unexpected byte on %v index while parsing db section with %v value\n", i, dbSection[i])
            os.Exit(1)
        }
    }
}

func FindBytes(source []byte, target byte) int {
	for i := range source {
		if source[i] == target {
			return i
		}
	}
	return -1
}

func Set(parsedData []string, connection net.Conn) {
	storageMu.Lock()
	defer storageMu.Unlock()
	if len(parsedData) > 3 && parsedData[3] == "px" {
		timeMu.Lock()
		defer timeMu.Unlock()
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
}

func Get(parsedData []string, connection net.Conn) {
	timeMu.Lock()
	storageMu.Lock()
	defer timeMu.Unlock()
	defer storageMu.Unlock()
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
}

func Keys(parsedData []string, connection net.Conn, pattern string) {
	storageMu.Lock()
	defer storageMu.Unlock()
	keys := make([]string, 0)
	for key := range storage {
		keys = append(keys, key)
	}
	l := len(keys)
	retStr := fmt.Sprintf("*%v\r\n", l)
	for i := range l {
		retStr += fmt.Sprintf("$%v\r\n%v\r\n", len(keys[i]), keys[i])
	}
	connection.Write([]byte(retStr))
}

func Save(parsedData []string, connection net.Conn) {

}
