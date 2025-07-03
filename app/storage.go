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

// could be RWMutex
var storageMu sync.Mutex = *new(sync.Mutex)
var storage map[string]string = make(map[string]string)
var timeMu sync.Mutex = *new(sync.Mutex)
var timestamps map[string]NowAndDuration = make(map[string]NowAndDuration)

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
		if time.Since(nad.now) > nad.expires {
			retStr := "$-1\r\n"
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

func Parse(buffer []byte) []string {
	splited := strings.Split(string(buffer), "\r\n")
	var ret []string
	for i := 1; i < len(splited); i++ {
		if i%2 == 0 {
			ret = append(ret, splited[i])
		}
	}
	return ret
}
