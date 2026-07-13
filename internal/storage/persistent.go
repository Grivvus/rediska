package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	crc64 "github.com/codecrafters-io/redis-starter-go/internal/crc64jones"
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

	crc := crc64.CRC64(0, sb.String())
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
		enc = binary.LittleEndian.AppendUint64(enc, uint64(timestamp.UnixMilli()))
	}
	enc = append(enc, byte(String))
	enc = append(enc, codec.EncodeBulkString(key)...)
	enc = append(enc, codec.EncodeBulkString(value)...)
	return enc
}

type auxilaryField struct {
	redisVersion string
	creationTime time.Time
}

type resizeHint struct {
	valueTableSize     int
	timestampTableSize int
}

type RDBStructure struct {
	magic            [5]byte
	rdbVersion       [4]byte
	aux              auxilaryField
	databaseSelector int
	resizeHint       resizeHint
	values           map[string]string
	timestamps       map[string]time.Time
	crcSum           [8]byte
}

func (r RDBStructure) Apply(st *Storage) {
	st.storageMu.Lock()
	st.timeMu.Lock()
	defer st.storageMu.Unlock()
	defer st.timeMu.Unlock()

	st.storage = maps.Clone(r.values)
	st.timestamps = maps.Clone(r.timestamps)
}

func DecodeRDB(r io.Reader) (RDBStructure, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return RDBStructure{}, fmt.Errorf("can't read rdb file: %w", err)
	}
	if !slices.Equal(raw[0:5], []byte(magicString)) {
		return RDBStructure{}, fmt.Errorf("missing magic bytes at the start of the rdb file")
	}
	n := len(raw)
	crcBin := raw[n-8 : n]
	crc := binary.LittleEndian.Uint64(crcBin)
	crcCalculated := crc64.CRC64(0, string(raw[0:n-8]))
	if crc != crcCalculated {
		return RDBStructure{}, fmt.Errorf(
			"crc64 sum didn't match, rdb file may be corrupted: expected %v, got %v",
			crc, crcCalculated,
		)
	}
	i := 0
	for i < len(raw)-1 && !(raw[i] == 'F' && raw[i+1] == 'E') {
		i++
	}
	if raw[i] != 'F' || raw[i+1] != 'E' {
		return RDBStructure{}, fmt.Errorf("can't find database selector block in rdb file")
	}
	rdb := RDBStructure{
		values:     make(map[string]string),
		timestamps: make(map[string]time.Time),
	}
	// skip FE + DB_SELECTOR
	raw = raw[i+2+2:]
	if raw[i] == 'F' && raw[i+1] == 'B' {
		// parse resizedb fields
		return RDBStructure{}, fmt.Errorf("not implemented")
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
			unixMS := binary.LittleEndian.Uint64(unixMSBytes)
			if unixMS > math.MaxInt64 {
				panic("can't convert uint64 to int64")
			}
			ts := time.UnixMilli(int64(unixMS))
			timestamp = &ts

			// FC + 8 bytes timestamp
			i += 10

		case 'D':
			// sec timestamp is 4 byte long
			unixSecBytes := encoded[i+2 : i+2+4]
			unixSec := binary.LittleEndian.Uint32(unixSecBytes)
			ts := time.Unix(int64(unixSec), 0)
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
	// skip '$LEN', encoded key
	i += len(strconv.Itoa(len(key))) + len(key) + 5 // + '$' + \r\n x2
	val, err = codec.DecodeString(encoded[i:])
	if err != nil {
		return 0, "", "", nil, fmt.Errorf("can't decode value: %w", err)
	}
	i += len(strconv.Itoa(len(val))) + len(val) + 5 // + '$' + \r\n x2

	return
}
