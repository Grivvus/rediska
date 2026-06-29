package storage

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/codecrafters-io/redis-starter-go/internal/config"
)

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

func (st *Storage) Set(parsedData []string) (msg []byte, err error) {
	st.storageMu.Lock()
	defer st.storageMu.Unlock()
	if len(parsedData) > 3 && parsedData[3] == "px" {
		st.timeMu.Lock()
		defer st.timeMu.Unlock()
		parsed, err := strconv.Atoi(parsedData[4])
		if err != nil {
			return nil, fmt.Errorf("Invalid data for time delay\n Can't parse %v to int\n", parsedData[4])
		}
		st.timestamps[parsedData[1]] = time.Now().Add(time.Duration(parsed) * time.Millisecond)
	}
	st.storage[parsedData[1]] = parsedData[2]
	if st.cfg.Role == config.MasterRole {
		return []byte("+OK\r\n"), nil
	}
	return nil, nil
}

func (st *Storage) Get(parsedData []string) (msg []byte) {
	st.timeMu.RLock()
	st.storageMu.RLock()
	defer st.timeMu.RUnlock()
	defer st.storageMu.RUnlock()
	expires, exist := st.timestamps[parsedData[1]]
	if !exist {
		retStr := fmt.Sprintf("+%v\r\n", st.storage[parsedData[1]])
		return []byte(retStr)
	}
	if time.Now().After(expires) {
		retStr := "$-1\r\n"
		return []byte(retStr)
	}

	retStr := fmt.Sprintf("+%v\r\n", st.storage[parsedData[1]])
	return []byte(retStr)
}

func (st *Storage) Keys(parsedData []string, pattern string) []byte {
	/*
		we could try to make this in 1 linear pass, not 2
		we need to encode information about number of keys

		we could firstly encode all keys, and then
		append information about number of keys to the left side (start) of the message
	*/
	st.storageMu.RLock()
	defer st.storageMu.RUnlock()
	keys := make([]string, 0)
	for key := range st.storage {
		keys = append(keys, key)
	}
	var sb strings.Builder
	header := fmt.Sprintf("*%v\r\n", len(keys))
	sb.WriteString(header)
	for _, key := range keys {
		_, _ = sb.Write(codec.EncodeString(key))
	}
	return []byte(sb.String())
}
