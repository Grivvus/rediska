package codec

import (
	"fmt"
	"strconv"
	"strings"
)

func EncodeBulkString(s string) []byte {
	return fmt.Appendf(nil, "$%s\r\n%s\r\n", strconv.Itoa(len(s)), s)
}

func NullBulkString() []byte {
	return []byte("$-1\r\n")
}

func EncodeSimpleString(s string) []byte {
	return []byte("+" + s + "\r\n")
}

func EncodeArray(data []string) []byte {
	b := strings.Builder{}
	fmt.Fprintf(&b, "*%v\r\n", len(data))
	for _, s := range data {
		_, _ = b.Write(EncodeBulkString(s))
	}
	return []byte(b.String())
}

func EncodeError(err error) []byte {
	return []byte("-" + err.Error() + "\r\n")
}
