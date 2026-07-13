package storage

import (
	"bytes"
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
	st := genTestStorage()

	encoded := EncodeToRDB(st)
	decodedRDB, err := DecodeRDB(bytes.NewReader(encoded))
	require.NoError(t, err)
	t.Log(decodedRDB)
	empty := emptyStorage()
	decodedRDB.Apply(empty)
	assert.Equal(t, st.storage, empty.storage, "both storages should be equal")
	assert.Equal(t, st.timestamps, empty.timestamps, "timestamps should stay the same")
}

func TestDecodeEmpty(t *testing.T) {
	f, err := os.Open("../../empty.rdb")
	require.NoError(t, err)
	decodedRDB, err := DecodeRDB(f)
	assert.NoError(t, err)
	t.Log(decodedRDB)
}

func genTestStorage() *Storage {
	st := NewStorage(*config.Default())
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

func emptyStorage() *Storage {
	return NewStorage(*config.Default())
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
