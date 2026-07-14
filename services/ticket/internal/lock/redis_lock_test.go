package lock

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func setupRedisLock(t *testing.T) (*RedisLock, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	return NewRedisLock(s.Addr()), s
}

func TestAcquireSuccess(t *testing.T) {
	l, _ := setupRedisLock(t)
	ctx := context.Background()

	ok, err := l.Acquire(ctx, "lock:test", 10*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if !ok {
		t.Error("expected lock to be acquired")
	}
}

func TestAcquireFailsWhenAlreadyLocked(t *testing.T) {
	l, _ := setupRedisLock(t)
	ctx := context.Background()

	ok1, _ := l.Acquire(ctx, "lock:test", 10*time.Second)
	if !ok1 {
		t.Fatal("first acquire should succeed")
	}

	ok2, err := l.Acquire(ctx, "lock:test", 10*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if ok2 {
		t.Error("second acquire should fail when already locked")
	}
}

func TestReleaseAllowsReAcquire(t *testing.T) {
	l, _ := setupRedisLock(t)
	ctx := context.Background()

	ok1, _ := l.Acquire(ctx, "lock:test", 10*time.Second)
	if !ok1 {
		t.Fatal("first acquire should succeed")
	}

	err := l.Release(ctx, "lock:test")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	ok2, err := l.Acquire(ctx, "lock:test", 10*time.Second)
	if err != nil {
		t.Fatalf("Acquire after release failed: %v", err)
	}
	if !ok2 {
		t.Error("should be able to acquire after release")
	}
}

func TestReleaseNonExistentKey(t *testing.T) {
	l, _ := setupRedisLock(t)
	ctx := context.Background()

	err := l.Release(ctx, "lock:nonexistent")
	if err != nil {
		t.Fatalf("Release non-existent should not error: %v", err)
	}
}

func TestAcquireLockTTLExpires(t *testing.T) {
	l, s := setupRedisLock(t)
	ctx := context.Background()

	ok1, _ := l.Acquire(ctx, "lock:test", 1*time.Second)
	if !ok1 {
		t.Fatal("first acquire should succeed")
	}

	s.FastForward(2 * time.Second)

	ok2, err := l.Acquire(ctx, "lock:test", 10*time.Second)
	if err != nil {
		t.Fatalf("Acquire after expiry failed: %v", err)
	}
	if !ok2 {
		t.Error("should be able to acquire after TTL expires")
	}
}
