package storage

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
)

var ErrValueExpired = fmt.Errorf("the value is expired")
var ErrKeyDoesntExist = fmt.Errorf("the key does not exist")

type Storage struct {
	logger     *slog.Logger
	storage    map[string]string
	storageMu  sync.RWMutex
	timestamps map[string]time.Time
	timeMu     sync.RWMutex
	cfg        config.RedisConfig
}

func NewStorage(cfg config.RedisConfig, logger *slog.Logger) *Storage {
	st := &Storage{
		storage:    make(map[string]string),
		timestamps: make(map[string]time.Time),
		cfg:        cfg,
		logger:     logger,
	}
	// do I need ctx here?
	go st.gcWorker()
	return st
}

func (st *Storage) Set(key, value string) {
	// if there's old value with some expiry, delete expiration time
	st.timeMu.Lock()
	delete(st.timestamps, key)
	st.timeMu.Unlock()

	st.storageMu.Lock()
	st.storage[key] = value
	st.storageMu.Unlock()
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

func (st *Storage) Del(key string) (string, error) {
	st.timeMu.Lock()
	defer st.timeMu.Unlock()
	st.storageMu.Lock()
	defer st.storageMu.Unlock()
	val, ok := st.storage[key]
	if !ok {
		return "", nil
	}
	delete(st.storage, key)
	delete(st.timestamps, key)
	return val, nil
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

func (st *Storage) gcWorker() {
	if st.cfg.GCPeriodSec <= 0 {
		panic("invalid GCPeriodSec, it must be a positive number")
	}
	ticker := time.NewTicker(time.Duration(st.cfg.GCPeriodSec) * time.Second)
	defer ticker.Stop()
	for now := range ticker.C {
		st.logger.Info("start gc at", "time", now)
		cnt := 0

		/* need to figure it out, how to improve gc locks */
		/* now it stops any other oprations */

		/* acuire */
		st.timeMu.Lock()
		st.storageMu.Lock()

		for key, tstamp := range st.timestamps {
			if now.After(tstamp) {
				cnt++
				delete(st.timestamps, key)
				delete(st.storage, key)
			}
		}

		/* release */
		st.timeMu.Unlock()
		st.storageMu.Unlock()

		st.logger.Info("end gc at", "time", now, "deleted keys", cnt)
	}
}

// not implemented
func matchesPattern(key, pattern string) bool {
	_, _ = key, pattern
	return true
}
