package encoder

import (
	"fmt"
	"strconv"
	"strings"
)

func EncodeString(s string) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(s), s))
}

func EncodeArray(data []string) []byte {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("*%v\r\n", len(data)))
	for _, s := range data {
		b.WriteString(fmt.Sprintf("$%v\r\n", len(s)))
		b.WriteString(s)
		b.WriteString("\r\n")
	}
	fmt.Println("encoded:", b.String())
	return []byte(b.String())
}

func Parse(buffer []byte) ([][]string, error) {
	var ret [][]string
	var buf []string
	stringified := string(buffer)
	splited := strings.Split(stringified, "\r\n")
	fmt.Println(splited)
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
			index := FindBytes(buffer[n*2+2:], '*') + n*2 + 2
			if index != -1 {
				innerParse, err := Parse(buffer[index:])
				if err != nil {
					return nil, fmt.Errorf("parsing error: %w", err)
				}
				ret = append(ret, innerParse...)
				fmt.Println(string(buffer[index:]))
			}
		}
	} else if strings.HasPrefix(string(buffer), "$") {
		ret = append(ret, []string{splited[1]})
	}
	return ret, nil
}

func Merge(s ...string) string {
	b := strings.Builder{}
	for _, part := range s {
		b.WriteString(part)
	}
	return b.String()
}

func FindBytes(source []byte, target byte) int {
	for i := range source {
		if source[i] == target {
			return i
		}
	}
	return -1
}

func SplitByteArray(arr []byte, sep byte) [][]byte {
	res := make([][]byte, 0)
	start := 1
	for i := 1; i < len(arr); i++ {
		if arr[i] == sep {
			res = append(res, arr[start:i])
			start = i + 1
		}
	}
	if start < len(arr) {
		res = append(res, arr[start:])
	}
	return res
}
