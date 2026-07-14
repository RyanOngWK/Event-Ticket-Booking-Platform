//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/example/ticket-platform/services/ticket/internal/lock"
)

func TestConcurrencyLockContention(t *testing.T) {
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			for retries := 0; retries < 200; retries++ {
				acquired, err := rl.Acquire(ctx, "lock:event:1", 30*time.Second)
				if err != nil {
					t.Errorf("goroutine %d: acquire error: %v", id, err)
					return
				}
				if acquired {
					mu.Lock()
					successCount++
					mu.Unlock()

					time.Sleep(10 * time.Millisecond)

					if err := rl.Release(ctx, "lock:event:1"); err != nil {
						t.Errorf("goroutine %d: release error: %v", id, err)
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	if successCount != numGoroutines {
		t.Errorf("expected %d successful acquires, got %d", numGoroutines, successCount)
	}
}

func TestConcurrencyOnlyOneLockHolder(t *testing.T) {
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())

	var wg sync.WaitGroup
	simultaneousHolders := 0
	maxSimultaneous := 0
	var mu sync.Mutex
	numGoroutines := 100

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				acquired, err := rl.Acquire(ctx, "lock:event:2", 5*time.Second)
				if err != nil {
					return
				}
				if acquired {
					mu.Lock()
					simultaneousHolders++
					if simultaneousHolders > maxSimultaneous {
						maxSimultaneous = simultaneousHolders
					}
					mu.Unlock()

					time.Sleep(5 * time.Millisecond)

					mu.Lock()
					simultaneousHolders--
					mu.Unlock()

					rl.Release(ctx, "lock:event:2")
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	if maxSimultaneous > 1 {
		t.Errorf("max simultaneous lock holders was %d, expected 1", maxSimultaneous)
	}
}

func TestConcurrencyLockFairness(t *testing.T) {
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())

	ctx := context.Background()
	key := "lock:fairness:test"

	ok, err := rl.Acquire(ctx, key, 10*time.Second)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if !ok {
		t.Fatal("initial acquire should succeed")
	}

	var wg sync.WaitGroup
	acquiredCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquired, _ := rl.Acquire(ctx, key, 1*time.Second)
			if acquired {
				mu.Lock()
				acquiredCount++
				mu.Unlock()
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)

	if acquiredCount > 0 {
		t.Errorf("no goroutine should acquire while lock is held: %d acquired", acquiredCount)
	}

	rl.Release(ctx, key)

	wg.Wait()

	fmt.Printf("goroutines that acquired after release: %d\n", acquiredCount)
}
