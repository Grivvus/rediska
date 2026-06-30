package storage

import (
	"encoding/binary"
	"fmt"
	"hash/crc64"
	"io"
	"slices"
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
	crcBin := binary.LittleEndian.AppendUint64(nil, crc)
	_, _ = sb.Write(crcBin)
	return []byte(sb.String())
}

func encodeAuxilaryField(_ *Storage) []byte {
	return []byte("FA")
}

func encodeDBSelector(_ *Storage) []byte {
	const defaultDBSelector = "00"
	return []byte("FE" + defaultDBSelector)
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
		enc = append(enc, 'F', 'C')
		enc = append(enc, []byte(strconv.FormatInt(timestamp.UnixMilli(), 10))...)
	}
	enc = append(enc, byte(String))
	enc = append(enc, codec.EncodeString(key)...)
	enc = append(enc, codec.EncodeString(value)...)
	return enc
}

type rdbStructure struct {
	magic            [5]byte
	rdbVersion       [4]byte
	aux              auxilaryField
	databaseSelector int
	resizeHint       resizeHint
	values           map[string]string
	timestamps       map[string]time.Time
	crcSum           [8]byte
}

type auxilaryField struct {
	redisVersion string
	creationTime time.Time
}

type resizeHint struct {
	valueTableSize     int
	timestampTableSize int
}

func DecodeRDB(r io.Reader) (rdbStructure, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return rdbStructure{}, fmt.Errorf("can't read rdb file: %w", err)
	}
	if !slices.Equal(raw[0:5], []byte(magicString)) {
		return rdbStructure{}, fmt.Errorf("missing magic bytes at the start of the rdb file")
	}
	n := len(raw)
	crcBin := raw[n-8 : n]
	crc := binary.LittleEndian.Uint64(crcBin)
	crcCalculated := crc64.Checksum(raw[0:n-8], nil)
	if crc != crcCalculated {
		return rdbStructure{}, fmt.Errorf("crc64 sum didn't match, rdb file may be corrupted")
	}
	i := 0
	for i < len(raw)-1 && raw[i] != 'F' && raw[i+1] != 'E' {
		i++
	}
	if raw[i] != 'F' && raw[i+1] != 'E' {
		return rdbStructure{}, fmt.Errorf("can't find database selector block in rdb file")
	}
	rdb := rdbStructure{}
	raw = raw[i+2:]
	if raw[i] == 'F' && raw[i+1] == 'B' {
		// parse resizedb fields
	}
	for len(raw) > 0 && (raw[0] != 'F' && raw[1] != 'F') {
		shift, key, value, timestamp, err := parseKeyValuePair(raw)
		if err != nil {
			return rdb, err
		}
		rdb.values[key] = value
		if timestamp != nil {
			rdb.timestamps[key] = *timestamp
		}
		raw = raw[shift:]
	}

	return rdb, nil
}

func parseKeyValuePair(
	encoded []byte,
) (i int, key string, val string, timestamp *time.Time, err error) {
	if encoded[i] == 'F' {
		switch encoded[i+1] {
		case 'C':
			// ms timestamp is 8 byte long
			unixMSBytes := encoded[i+2 : i+2+8]
			unixMS, err := strconv.ParseInt(string(unixMSBytes), 10, 64)
			if err != nil {
				return 0, "", "", nil, fmt.Errorf("can't parse expiry timestamp: %w", err)
			}
			ts := time.UnixMilli(unixMS)
			timestamp = &ts

			// FC + 8 bytes timestamp
			i += 10

		case 'D':
			// sec timestamp is 4 byte long
			unixSecBytes := encoded[i+2 : i+2+4]
			unixSec, err := strconv.ParseInt(string(unixSecBytes), 10, 32)
			if err != nil {
				return 0, "", "", nil, fmt.Errorf("can't prase expiry timestamp: %w", err)
			}
			ts := time.Unix(unixSec, 0)
			timestamp = &ts

			// FD + 4 bytes timestamp
			i += 6
		default:
			return 0, "", "", nil, fmt.Errorf("invalid parsing state at %v", i+1)
		}
	}
	valueType := RDBValueType(encoded[i])
	i++
	if valueType != String {
		return 0, "", "", nil, fmt.Errorf("unsupported value type: %v", valueType)
	}
	key, err = codec.DecodeString(encoded[i:])
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("can't decode key: %w", err)
	}
	// skip '$' of the key
	i++
	for i < len(encoded) && encoded[i] != '$' {
		i++
	}
	val, err = codec.DecodeString(encoded[i:])
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("can't decode value: %w", err)
	}

	return
}
