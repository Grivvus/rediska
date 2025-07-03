package main

import "fmt"

func Encode(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func Merge(s1, s2 string) string {
	return s1 + s2
}

// Multi-merge
func Mmerge(s ...string) string {
	res := ""
	for _, part := range s {
		res += part
	}
	return res
}
