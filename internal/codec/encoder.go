package codec

import (
	"fmt"
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
