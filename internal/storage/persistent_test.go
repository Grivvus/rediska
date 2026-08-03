package storage

import (
	"bytes"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeCycle(t *testing.T) {
	t.Parallel()
	st := genTestStorage()

	encoded := EncodeToRDB(st)
	decodedRDB, err := DecodeRDB(bytes.NewReader(encoded))
	require.NoError(t, err)
	empty := emptyStorage()
	decodedRDB.Apply(empty)
	assert.Equal(t, st.storage, empty.storage, "both storages should be equal")
	// check len of the map because we can't directly compare values in it
	assert.Equal(t, len(st.timestamps), len(empty.timestamps), "timestamps should have the same number of values")
}

func TestDecodeEmpty(t *testing.T) {
	t.Parallel()
	f, err := os.Open("../../empty.rdb")
	require.NoError(t, err)
	src, err := io.ReadAll(f)
	require.NoError(t, err)
	decodedRDB, err := DecodeRDB(bytes.NewReader(src))
	assert.NoError(t, err)
	t.Log(decodedRDB)
}

func genTestStorage() *Storage {
	st := NewStorage(*config.Default(), testLogger())
	nKeys := rand.IntN(5) + 2
	for range nKeys {
		key, value := randStringN(6), randStringN(rand.IntN(30)+1)
		if rand.Int()%2 == 1 {
			st.Set(key, value)
		} else {
			ts := time.Now().Add(time.Duration(rand.N(100)) * time.Second)
			st.SetWithExpiry(key, value, ts)
		}
	}

	return st
}

func emptyStorage() *Storage {
	return NewStorage(*config.Default(), testLogger())
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

func randStringN(n int) string {
	b := strings.Builder{}
	for range n {
		b.WriteByte(byte(rand.IntN(int('z'-'a'))) + 'a')
	}
	return b.String()
}
