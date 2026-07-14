package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var releaseScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	else
		return 0
	end
`)

type RedisLock struct {
	client *redis.Client
}

func NewRedisLock(addr string) *RedisLock {
	client := redis.NewClient(&redis.Options{Addr: addr})
	return &RedisLock{client: client}
}

func (l *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := l.client.SetNX(ctx, key, "locked", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock: %w", err)
	}
	return ok, nil
}

func (l *RedisLock) Release(ctx context.Context, key string) error {
	err := releaseScript.Run(ctx, l.client, []string{key}, "locked").Err()
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}
