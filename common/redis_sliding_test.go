package common

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisIncrWithSlidingExpirationRefreshesTTL(t *testing.T) {
	server := miniredis.RunT(t)
	previousEnabled, previousClient := RedisEnabled, RDB
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	RedisEnabled, RDB = true, client
	t.Cleanup(func() {
		_ = client.Close()
		RedisEnabled, RDB = previousEnabled, previousClient
	})

	count, err := RedisIncrWithSlidingExpiration("test:violation", 2*time.Hour)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	server.FastForward(90 * time.Minute)
	count, err = RedisIncrWithSlidingExpiration("test:violation", 2*time.Hour)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	ttl, err := client.TTL(t.Context(), "test:violation").Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Hour+59*time.Minute)

	server.FastForward(2*time.Hour + time.Second)
	count, err = RedisIncrWithSlidingExpiration("test:violation", 2*time.Hour)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRedisIncrWithSlidingExpirationRejectsUnavailableRedis(t *testing.T) {
	previousEnabled, previousClient := RedisEnabled, RDB
	RedisEnabled, RDB = false, nil
	t.Cleanup(func() {
		RedisEnabled, RDB = previousEnabled, previousClient
	})

	_, err := RedisIncrWithSlidingExpiration("test:violation", time.Hour)
	assert.Error(t, err)
}
