package storage

import (
	"fmt"
	"hash/crc64"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
)

type RDBValueType byte

const (
	String             RDBValueType = iota // 0
	List                                   // 1
	Set                                    // 2
	SortedSet                              // 3
	Hash                                   // 4
	Zipmap             = iota + 4          // 0
	Ziplist                                //10
	Intset                                 // 11
	SortedSetInZiplist                     // 12

	//Since RDBv4
	HashmapInZiplist // 13
	// Since RDBv7
	ListInQuicklist // 14
)

const magicString = "REDIS"
const rdbVersion = "0011"

func EncodeToRDB(st *Storage) []byte {
	var sb strings.Builder
	_, _ = sb.WriteString(magicString)
	_, _ = sb.WriteString(rdbVersion)
	_, _ = sb.Write(encodeAuxilaryField(st))
	_, _ = sb.Write(encodeDBSelector(st))
	_, _ = sb.Write(encodeValues(st))
	_, _ = sb.Write([]byte{'F', 'F'})

	crc := crc64.Checksum([]byte(sb.String()), nil)
	_, _ = fmt.Fprintf(&sb, "%v", crc)
	return []byte(sb.String())
}

func encodeAuxilaryField(_ *Storage) []byte {
	return []byte("FA")
}

func encodeDBSelector(_ *Storage) []byte {
	const defaultDBSelector = "00"
	return []byte("FE" + " " + defaultDBSelector)
}

func encodeValues(st *Storage) []byte {
	st.storageMu.RLock()
	st.timeMu.RLock()
	defer st.storageMu.RUnlock()
	defer st.timeMu.RUnlock()

	var encoded []byte
	for k, v := range st.storage {
		t, hasTime := st.timestamps[k]
		if hasTime {
			encoded = append(encoded, encodeStringValue(k, v, &t)...)
		}
		encoded = append(encoded, encodeStringValue(k, v, nil)...)
	}

	return encoded
}

func encodeStringValue(key, value string, timestamp *time.Time) []byte {
	enc := make([]byte, 0, len(key)+len(value)+32)
	if timestamp != nil {
		enc = append(enc, []byte("FC ")...)
		enc = append(enc, []byte(strconv.FormatInt(timestamp.UnixMilli(), 10))...)
		enc = append(enc, '\n')
	}
	enc = append(enc, byte(String), '\n')
	enc = append(enc, codec.EncodeString(key)...)
	enc = append(enc, '\n')
	enc = append(enc, codec.EncodeString(value)...)
	enc = append(enc, '\n')
	return enc
}
