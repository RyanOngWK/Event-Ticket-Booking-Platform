package lock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	redsyncRedis "github.com/go-redsync/redsync/v4/redis"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

type RedisLock struct {
	redsync *redsync.Redsync
	clients []*redis.Client
	mu      sync.Mutex
	locks   map[string]*redsync.Mutex
}

// NewRedisLock creates a Redlock coordinator over independent Redis masters.
// A single address is supported for local unit tests, but provides no
// redundancy; use five masters for the Compose deployment.
// NewRedisLock creates a Redlock client backed by the configured Redis masters.
func NewRedisLock(addrs ...string) *RedisLock {
	pools := make([]redsyncRedis.Pool, 0, len(addrs))
	clients := make([]*redis.Client, 0, len(addrs))
	for _, addr := range addrs {
		client := redis.NewClient(&redis.Options{Addr: addr})
		clients = append(clients, client)
		pools = append(pools, goredis.NewPool(client))
	}
	return &RedisLock{
		redsync: redsync.New(pools...),
		clients: clients,
		locks:   make(map[string]*redsync.Mutex),
	}
}

// Close releases the Redis clients owned by the lock manager.
func (l *RedisLock) Close() {
	for _, client := range l.clients {
		_ = client.Close()
	}
}

// Acquire attempts to obtain a quorum lease for the key for the requested TTL.
func (l *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	mutex := l.redsync.NewMutex(key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(1),
		redsync.WithFailFast(true),
	)
	if err := mutex.TryLockContext(ctx); err != nil {
		var taken *redsync.ErrTaken
		if errors.As(err, &taken) {
			return false, nil
		}
		return false, fmt.Errorf("acquire redlock: %w", err)
	}

	l.mu.Lock()
	l.locks[key] = mutex
	l.mu.Unlock()
	return true, nil
}

// Release safely releases the lease owned by this lock manager for the key.
func (l *RedisLock) Release(ctx context.Context, key string) error {
	l.mu.Lock()
	mutex, ok := l.locks[key]
	if !ok {
		l.mu.Unlock()
		return nil
	}
	delete(l.locks, key)
	l.mu.Unlock()

	released, err := mutex.UnlockContext(ctx)

	if err != nil {
		return fmt.Errorf("release redlock: %w", err)
	}
	if !released {
		return errors.New("release redlock did not reach a quorum")
	}
	return nil
}
