package codec

import (
	"fmt"
	"strconv"
	"strings"
)

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
			index := findBytes(buffer[n*2+2:], '*') + n*2 + 2
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

func findBytes(source []byte, target byte) int {
	for i := range source {
		if source[i] == target {
			return i
		}
	}
	return -1
}
