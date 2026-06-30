package codec

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"strings"
)

func Parse(buffer []byte) ([][]string, error) {
	var ret [][]string
	var buf []string
	stringified := string(buffer)
	splited := strings.Split(stringified, "\r\n")
	log.Println(splited)
	if buffer[0] == '*' {
		n, err := strconv.Atoi(splited[0][1:])
		if err != nil {
			return nil, fmt.Errorf("can't parse array: %w", err)
		}
		for i := range n {
			_, err := strconv.Atoi(splited[i*2+1][1:])
			if err != nil {
				return nil, fmt.Errorf("can't parse number: %w", err)
			}
			buf = append(buf, splited[i*2+2])
		}
		ret = append(ret, buf[:])
		if n*2+2 < len(splited) {
			index := findBytes(buffer[n*2+2:], '*') + n*2 + 2
			if index != -1 {
				innerParse, err := Parse(buffer[index:])
				if err != nil {
					return nil, fmt.Errorf("parsing error: %w", err)
				}
				ret = append(ret, innerParse...)
				log.Println(string(buffer[index:]))
			}
		}
	} else if strings.HasPrefix(string(buffer), "$") {
		ret = append(ret, []string{splited[1]})
	}
	return ret, nil
}

func findBytes(source []byte, target byte) int {
	for i := range source {
		if source[i] == target {
			return i
		}
	}
	return -1
}

func DecodeString(encoded []byte) (string, error) {
	if encoded[0] != '$' {
		return "", fmt.Errorf("missing '$' at the start of the encoded string")
	}
	i := 0
	for i < len(encoded)-1 && encoded[i] != '\r' {
		i++
	}
	if encoded[i] != '\r' || encoded[i+1] != '\n' {
		return "", fmt.Errorf(
			"invalid encoding at %v: expected \\r\\n",
			i+1,
		)
	}
	lenBytes := encoded[1:i]
	encodedLen, err := strconv.Atoi(string(lenBytes))
	if err != nil {
		return "", fmt.Errorf("can't parse encoded string length: %w", err)
	}
	if i+2+encodedLen > len(encoded) {
		return "", fmt.Errorf("invalid encoded length: out of range")
	}
	encodedStringAsBytes := encoded[i+2 : i+2+encodedLen]
	return string(encodedStringAsBytes), nil
}

func DecodeArray(encoded []byte) ([]string, error) {
	if encoded[0] != '*' {
		return nil, fmt.Errorf("missing '*' at the start of the encoded array")
	}
	i := 0
	for i < len(encoded)-1 && encoded[i] != '\r' {
		i++
	}
	if encoded[i] != '\r' || encoded[i+1] != '\n' {
		return nil, fmt.Errorf("invalid encoding at %v: expected \\r\\n", i+1)
	}
	lenBytes := encoded[1:i]
	encodedLen, err := strconv.Atoi(string(lenBytes))
	if err != nil {
		return nil, fmt.Errorf("can't parse encoded array length: %w", err)
	}
	elements := encoded[i+2:]
	decodedStrings := make([]string, 0, encodedLen)

	for i := range encodedLen {
		decodedStr, err := DecodeString(elements)
		if err != nil {
			return nil, fmt.Errorf("can't decode array's %v'th element: %w", len(decodedStrings), err)
		}
		decodedStrings = append(decodedStrings, decodedStr)
		idx := bytes.IndexRune(elements[1:], '$')
		if i != encodedLen-1 {
			if idx == -1 {
				return nil, fmt.Errorf("invalid number of elements in array")
			}
			elements = elements[idx+1:]
		}
	}

	return decodedStrings, nil
}
