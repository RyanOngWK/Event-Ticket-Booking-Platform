package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const CorrelationIDKey contextKey = "correlation_id"

func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get("X-Correlation-ID")
		if cid == "" {
			cid = uuid.New().String()
		}
		w.Header().Set("X-Correlation-ID", cid)
		ctx := context.WithValue(r.Context(), CorrelationIDKey, cid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetCorrelationID(ctx context.Context) string {
	cid, ok := ctx.Value(CorrelationIDKey).(string)
	if !ok {
		return ""
	}
	return cid
}
