package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPILimiterBurstAndSustainedRate(t *testing.T) {
	limiter := NewAPILimiter(5000)
	now := time.Now()
	if !limiter.AllowN(now, 5000) {
		t.Fatal("initial burst was not available")
	}
	if limiter.AllowN(now, 1) {
		t.Fatal("bucket was not exhausted")
	}
	next := now.Add(time.Second)
	if limiter.AllowN(next, 1001) {
		t.Fatal("admitted more than 1000 tokens per second")
	}
	if !limiter.AllowN(next, 1000) {
		t.Fatal("did not replenish 1000 tokens in one second")
	}
	if limiter.AllowN(next, 1) {
		t.Fatal("accepted a token beyond the replenished budget")
	}
}

func TestRateLimitHTTPResponse(t *testing.T) {
	limiter := NewAPILimiter(5000)
	if !limiter.AllowN(time.Now(), 5000) {
		t.Fatal("could not exhaust bucket")
	}
	// An immediate retry must fail; avoid timing dependence by reserving all
	// tokens for a future interval before exercising the HTTP boundary.
	reservation := limiter.ReserveN(time.Now(), 5000)
	defer reservation.Cancel()
	handler := RateLimitMiddleware(limiter, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("rejected request reached handler") }))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/users", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatalf("unexpected response: %d %v", w.Code, w.Header())
	}
}
