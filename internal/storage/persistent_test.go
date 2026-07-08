package storage_test

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeCycle(t *testing.T) {
	t.Skip()
	st := genTestStorage()

	encoded := storage.EncodeToRDB(st)
	decodedRDB, err := storage.DecodeRDB(bytes.NewReader(encoded))
	assert.NoError(t, err)
	empty := emptyStorage()
	decodedRDB.Apply(empty)
	assert.Equal(t, st, empty, "both storages should be equal")
}

func genTestStorage() *storage.Storage {
	st := storage.NewStorage(*config.Default())
	nKeys := rand.IntN(1000) + 100
	for range nKeys {
	}

	return st
}

func emptyStorage() *storage.Storage {
	return storage.NewStorage(*config.Default())
}
