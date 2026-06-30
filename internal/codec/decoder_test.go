package codec_test

import (
	"strings"
	"testing"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/stretchr/testify/assert"
)

func TestStringDecoder(t *testing.T) {
	strs := []string{"OK", "", "hello", "привет", "a\r\nb", "😀", "line1\nline2", strings.Repeat("x", 1000)}
	for _, s := range strs {
		encoded := codec.EncodeString(s)
		decoded, err := codec.DecodeString(encoded)
		assert.NoError(t, err)
		assert.Equal(t, s, decoded)
	}
}

func TestArrayDecoder(t *testing.T) {
	arrays := [][]string{{"PING"}, {"SET", "key", "value"}, {}, {""}}
	for _, arr := range arrays {
		encoded := codec.EncodeArray(arr)
		decoded, err := codec.DecodeArray(encoded)
		assert.NoError(t, err)
		assert.Equal(t, arr, decoded)
	}
}
