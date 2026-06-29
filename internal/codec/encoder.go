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
	fmt.Fprintf(&b, "*%v\r\n", len(data))
	for _, s := range data {
		fmt.Fprintf(&b, "$%v\r\n", len(s))
		b.WriteString(s)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}
