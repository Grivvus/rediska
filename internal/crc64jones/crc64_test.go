package crc64_test

import (
	"testing"

	crc64 "github.com/codecrafters-io/redis-starter-go/internal/crc64jones"
	"github.com/stretchr/testify/assert"
)

func TestSimple(t *testing.T) {
	t.Parallel()
	checksum := crc64.CRC64(0, "123456789")
	assert.Equal(t, uint64(0xe9c6d914c4b8d9ca), checksum)
}
