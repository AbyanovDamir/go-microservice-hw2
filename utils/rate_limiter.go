package utils

import (
	"golang.org/x/time/rate"
	"net/http"
)

const APIRequestsPerSecond = 1000

func NewAPILimiter(burst int) *rate.Limiter {
	return rate.NewLimiter(rate.Limit(APIRequestsPerSecond), burst)
}

func RateLimitMiddleware(limiter *rate.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"too many requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
