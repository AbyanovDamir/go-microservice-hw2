package metrics

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"time"
)

type Metrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	Errors   *prometheus.CounterVec
	Registry *prometheus.Registry
}

func New() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(),
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP responses"}, []string{"method", "endpoint", "code"}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP latency", Buckets: []float64{.001, .005, .01, .025, .05, .1, .5, 1}}, []string{"method", "endpoint"}),
		Errors:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_errors_total", Help: "HTTP responses with status >= 400"}, []string{"method", "endpoint", "code"})}
	m.Registry.MustRegister(m.Requests, m.Duration, m.Errors, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *recorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}

// Match without serving to get bounded labels even for limiter rejections.
func (m *Metrics) Middleware(router *mux.Router, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := "unmatched"
		match := &mux.RouteMatch{}
		if router.Match(r, match) && match.Route != nil {
			if path, err := match.Route.GetPathTemplate(); err == nil {
				endpoint = path
			}
		}
		method := r.Method
		switch method {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		default:
			method = "OTHER"
		}
		start := time.Now()
		rw := &recorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.status == 0 {
			rw.status = 200
		}
		code := strconv.Itoa(rw.status)
		m.Requests.WithLabelValues(method, endpoint, code).Inc()
		m.Duration.WithLabelValues(method, endpoint).Observe(time.Since(start).Seconds())
		if rw.status >= 400 {
			m.Errors.WithLabelValues(method, endpoint, code).Inc()
		}
	})
}
