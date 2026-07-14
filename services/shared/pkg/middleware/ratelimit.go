package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redisClient *redis.Client
	limit       int
	window      time.Duration
}

func NewRateLimiter(redisAddr string, limit int, window time.Duration) *RateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &RateLimiter{
		redisClient: client,
		limit:       limit,
		window:      window,
	}
}

func NewRateLimiterWithClient(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redisClient: client,
		limit:       limit,
		window:      window,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		key := fmt.Sprintf("ratelimit:%s", ip)

		pipe := rl.redisClient.Pipeline()
		now := time.Now().UnixNano()
		windowStart := now - rl.window.Nanoseconds()

		pipe.ZRemRangeByScore(r.Context(), key, "0", fmt.Sprintf("%d", windowStart))
		countCmd := pipe.ZCard(r.Context(), key)
		_, err := pipe.Exec(r.Context())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		count, _ := countCmd.Result()
		if count >= int64(rl.limit) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", rl.window.Seconds()))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		pipe2 := rl.redisClient.Pipeline()
		pipe2.ZAdd(r.Context(), key, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d", now)})
		pipe2.Expire(r.Context(), key, rl.window)
		pipe2.Exec(r.Context())

		next.ServeHTTP(w, r)
	})
}
