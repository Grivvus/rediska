package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

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
	dbSections := SplitByteArray(dbSection, 0xfe)
	fileStorage := make(map[string]string)
	fileTime := make(map[string]NowAndDuration)
	for _, section := range dbSections {
		ParseDBSection(section, fileStorage, fileTime)
	}
	return fileStorage, fileTime
}

func ParseDBSection(dbSection []byte, fileStorage map[string]string, fileTime map[string]NowAndDuration) {
	dbIndex := dbSection[0]
	var valuesNumber byte
	var expiresNumber byte
	// may needed later
	_ = expiresNumber
	_ = dbIndex
	if dbSection[1] == 0xfb {
		valuesNumber = dbSection[2]
		expiresNumber = dbSection[3]
	}
	i := 4
	for range valuesNumber {
		if dbSection[i] == 0x00 {
			// no time expiration
			// parsing regular value
			i++
			lenOfKey := dbSection[i]
			i++
			key := dbSection[i : i+int(lenOfKey)]
			i += int(lenOfKey)
			lenOfValue := dbSection[i]
			i++
			value := dbSection[i : i+int(lenOfValue)]
			i += int(lenOfValue)
			fileStorage[string(key)] = string(value)
		} else if dbSection[i] == 0xfc {
			// timestamp in millisecond
			// 8 bytes for it
			// 8 bytes unsigned
			i++
			var milliseconds uint64
			for j := range 8 {
				milliseconds += (uint64(dbSection[i]) << (j * 8))
				i++
			}
			i++
			lenOfKey := dbSection[i]
			i++
			key := dbSection[i : i+int(lenOfKey)]
			i += int(lenOfKey)
			lenOfValue := dbSection[i]
			i++
			value := dbSection[i : i+int(lenOfValue)]
			i += int(lenOfValue)
			fileStorage[string(key)] = string(value)
			t := new(NowAndDuration)
			t.now = time.Now()
			t.expires = time.Duration((milliseconds - uint64(time.Now().Unix()*1000)) * 1_000_000)
			fileTime[string(key)] = *t
		} else if dbSection[i] == 0xfd {
			// timestamp in seconds
			// 4 bytes value
			// 4 bytes unsigned
			var seconds uint64
			i++
			for j := range 4 {
				seconds += (uint64(dbSection[i]) << (j * 8))
				i++
			}
			i++
			lenOfKey := dbSection[i]
			i++
			key := dbSection[i : i+int(lenOfKey)]
			i += int(lenOfKey)
			lenOfValue := dbSection[i]
			i++
			value := dbSection[i : i+int(lenOfValue)]
			i += int(lenOfValue)
			fileStorage[string(key)] = string(value)
			t := new(NowAndDuration)
			t.now = time.Now()
			t.expires = time.Duration((seconds - uint64(time.Now().Unix())) * 1_000_000)
			fileTime[string(key)] = *t
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected byte on %v index while parsing db section with %v value\n", i, dbSection[i])
			os.Exit(1)
		}
	}
}
