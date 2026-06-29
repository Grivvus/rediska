package codec_test

import (
	"strings"
	"testing"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/stretchr/testify/assert"
)

func TestStringEncoding(t *testing.T) {
	cases := []struct {
		given          string
		expectedOutput []byte
	}{
		{"OK", []byte("$2\r\nOK\r\n")},
		{"", []byte("$0\r\n\r\n")},
		{"hello", []byte("$5\r\nhello\r\n")},
		{"привет", []byte("$12\r\nпривет\r\n")},
		{"a\r\nb", []byte("$4\r\na\r\nb\r\n")},
		{"😀", []byte("$4\r\n😀\r\n")},
		{"line1\nline2", []byte("$11\r\nline1\nline2\r\n")},
		{strings.Repeat("x", 1000), []byte("$1000\r\n" + strings.Repeat("x", 1000) + "\r\n")},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.expectedOutput, codec.EncodeString(tc.given))
	}
}

func TestArrayEncoding(t *testing.T) {
	cases := []struct {
		given    []string
		expected []byte
	}{
		{[]string{"PING"}, []byte("*1\r\n$4\r\nPING\r\n")},
		{[]string{"SET", "key", "value"}, []byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n")},
		{[]string{}, []byte("*0\r\n")},
		{[]string{""}, []byte("*1\r\n$0\r\n\r\n")},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.expected, codec.EncodeArray(tc.given))
	}
}
