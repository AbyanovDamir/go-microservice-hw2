package metrics

import (
	"github.com/gorilla/mux"
	"go-microservice/utils"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsLabelsStatusAndLimiter(t *testing.T) {
	m := New()
	r := mux.NewRouter()
	r.HandleFunc("/api/users/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }).Methods("GET")
	h := m.Middleware(r, utils.RateLimitMiddleware(rate.NewLimiter(0, 1), r))
	for _, path := range []string{"/api/users/123", "/api/users/456"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(w.Result().Body)
	s := string(body)
	for _, want := range []string{`http_requests_total{code="404",endpoint="/api/users/{id}",method="GET"} 1`, `http_errors_total{code="429",endpoint="/api/users/{id}",method="GET"} 1`, `http_request_duration_seconds_count{endpoint="/api/users/{id}",method="GET"} 2`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(s, "/123") || strings.Contains(s, "/456") {
		t.Fatal("unbounded ID labels")
	}
}
