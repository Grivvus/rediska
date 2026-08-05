package storage

import (
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_KeyIsDeletedAfterExpiry(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.GCPeriodSec = 2
	st := testStorage(*cfg)

	type testDataValue struct {
		key      string
		val      string
		expAfter time.Duration
	}

	toInsert := []testDataValue{
		{
			"k1",
			"v1",
			500 * time.Millisecond,
		},
		{
			"k2",
			"v2",
			20 * time.Millisecond,
		},
		{
			"k3",
			"v3",
			1500 * time.Millisecond,
		},
		{
			"k4",
			"v4",
			5 * time.Second,
		},
	}

	now := time.Now()
	for _, v := range toInsert {
		st.SetWithExpiry(v.key, v.val, now.Add(v.expAfter))
	}

	time.Sleep(time.Duration(cfg.GCPeriodSec+1) * time.Second)
	st.storageMu.RLock()
	defer st.storageMu.RUnlock()
	assert.Less(t, len(st.storage), len(toInsert))
	val, err := st.Get("k4")
	assert.NoError(t, err)
	assert.Equal(t, "v4", val)

	_, err = st.Get("k1")
	assert.ErrorIs(t, err, ErrKeyDoesntExist)
}

func TestStorage_ValueExpiry(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	st := testStorage(*cfg)
	expiry := 20 * time.Millisecond

	st.SetWithExpiry("key", "val", time.Now().Add(expiry))

	time.Sleep(expiry * 2)

	/* value is expired, but gc didn't delete it yet */
	_, err := st.Get("key")
	assert.ErrorIs(t, err, ErrValueExpired)
}

func TestStorage_Del(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	st := testStorage(*cfg)
	key := "key"
	val := "val"

	st.Set(key, val)
	fetchedVal, err := st.Get(key)
	require.NoError(t, err)
	assert.Equal(t, val, fetchedVal)
	_, err = st.Del(key)
	require.NoError(t, err)
	_, errAfter := st.Get(key)
	assert.ErrorIs(t, errAfter, ErrKeyDoesntExist)
}

func testStorage(cfg config.RedisConfig) *Storage {
	return NewStorage(cfg, testLogger())
}
