package main

import (
	"fmt"
	"strings"
)

func EncodeString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
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
