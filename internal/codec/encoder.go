package codec

import (
	"fmt"
	"strconv"
	"strings"
)

func EncodeString(s string) []byte {
	return []byte(fmt.Sprintf("$%s\r\n%s\r\n", strconv.Itoa(len(s)), s))
}

func EncodeArray(data []string) []byte {
	b := strings.Builder{}
	fmt.Fprintf(&b, "*%v\r\n", len(data))
	for _, s := range data {
		_, _ = b.Write(EncodeString(s))
	}
	return []byte(b.String())
}
