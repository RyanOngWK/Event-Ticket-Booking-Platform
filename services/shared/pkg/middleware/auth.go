package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type Auth struct {
	redisClient *redis.Client
}

func NewAuth(client *redis.Client) *Auth {
	return &Auth{redisClient: client}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w, "missing authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := a.redisClient.Get(r.Context(), "session:"+token).Result()
		if err == redis.Nil {
			writeUnauthorized(w, "session expired or invalid")
			return
		}
		if err != nil {
			writeUnauthorized(w, "authentication error")
			return
		}

		a.redisClient.Expire(r.Context(), "session:"+token, 24*time.Hour)

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) ValidateToken(ctx context.Context, token string) (string, error) {
	return a.redisClient.Get(ctx, "session:"+token).Result()
}

func GetUserID(ctx context.Context) string {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
