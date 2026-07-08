package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
)

var ErrValueExpired = fmt.Errorf("the value is expired")
var ErrKeyDoesntExist = fmt.Errorf("the key does not exist")

type Storage struct {
	storage    map[string]string
	storageMu  sync.RWMutex
	timestamps map[string]time.Time
	timeMu     sync.RWMutex
	cfg        config.RedisConfig
}

func NewStorage(cfg config.RedisConfig) *Storage {
	return &Storage{
		storage:    make(map[string]string),
		timestamps: make(map[string]time.Time),
		cfg:        cfg,
	}
}

func (st *Storage) Set(key, value string) {
	st.storageMu.Lock()
	defer st.storageMu.Unlock()
	st.storage[key] = value
}

func (st *Storage) SetWithExpiry(key, value string, expiration time.Time) {
	st.storageMu.Lock()
	st.timeMu.Lock()
	defer st.storageMu.Unlock()
	defer st.timeMu.Unlock()

	st.storage[key] = value
	st.timestamps[key] = expiration
}

func (st *Storage) Get(key string) (value string, err error) {
	st.timeMu.RLock()
	st.storageMu.RLock()
	defer st.timeMu.RUnlock()
	defer st.storageMu.RUnlock()
	expires, ok := st.timestamps[key]
	if ok && time.Now().After(expires) {
		return "", ErrValueExpired
	}

	value, ok = st.storage[key]
	if !ok {
		return "", ErrKeyDoesntExist
	}
	return value, nil
}

func (st *Storage) Keys(parsedData []string, pattern string) []string {
	st.storageMu.RLock()
	defer st.storageMu.RUnlock()
	keys := make([]string, 0)
	for key := range st.storage {
		if matchesPattern(key, pattern) {
			keys = append(keys, key)
		}
	}
	return keys
}

// not implemented
func matchesPattern(key, pattern string) bool {
	_, _ = key, pattern
	return true
}
