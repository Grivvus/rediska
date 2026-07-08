package storage_test

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeCycle(t *testing.T) {
	t.Skip("broken")
	st := genTestStorage()

	encoded := storage.EncodeToRDB(st)
	t.Log(string(encoded))
	decodedRDB, err := storage.DecodeRDB(bytes.NewReader(encoded))
	require.NoError(t, err)
	empty := emptyStorage()
	decodedRDB.Apply(empty)
	assert.Equal(t, st, empty, "both storages should be equal")
}

func genTestStorage() *storage.Storage {
	st := storage.NewStorage(*config.Default())
	nKeys := rand.IntN(5) + 2
	for range nKeys {
		key, value := randStringN(30), randStringN(rand.IntN(100)+1)
		if rand.Int()%2 == 1 {
			st.Set(key, value)
		} else {
			ts := time.Now().Add(time.Duration(rand.N(100)) * time.Second)
			st.SetWithExpiry(key, value, ts)
		}
	}

	return st
}

func emptyStorage() *storage.Storage {
	return storage.NewStorage(*config.Default())
}

func randStringN(n int) string {
	const alphaStart = 'a'
	const alphaEnd = 'z'

	b := strings.Builder{}
	for range n {
		b.WriteByte(byte(rand.IntN(int('z'-'a'))) + 'a')
	}
	return b.String()
}
