package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"go-microservice/handlers"
	"go-microservice/metrics"
	"go-microservice/services"
	"go-microservice/utils"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var sink utils.EventSink
	var checker handlers.Checker
	if env("S3_ENABLED", "true") == "true" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		s, err := services.NewIntegrationService(env("MINIO_ENDPOINT", "localhost:9000"), env("MINIO_ACCESS_KEY", "minioadmin"), env("MINIO_SECRET_KEY", "minioadmin123"), env("MINIO_BUCKET", "users"), hex.EncodeToString(b), env("MINIO_SECURE", "false") == "true")
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		for {
			err = s.EnsureBucket(ctx)
			if err == nil || ctx.Err() != nil {
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
		cancel()
		if err != nil {
			return fmt.Errorf("S3 initialization: %w", err)
		}
		sink = s
		checker = s
	}
	d := utils.NewDispatcher(sink, logger, 10000)
	users := services.NewUserService(d.Enqueue)
	h := handlers.UserHandler{Service: users}
	router := mux.NewRouter()
	router.HandleFunc("/api/users", h.List).Methods("GET")
	router.HandleFunc("/api/users", h.Create).Methods("POST")
	router.HandleFunc("/api/users/{id}", h.Get).Methods("GET")
	router.HandleFunc("/api/users/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/api/users/{id}", h.Delete).Methods("DELETE")
	router.HandleFunc("/api/integration/status", handlers.IntegrationStatus(checker, d)).Methods("GET")
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.JSON(w, 404, map[string]string{"error": "route not found"})
	})
	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.JSON(w, 405, map[string]string{"error": "method not allowed"})
	})
	m := metrics.New()
	m.Registry.MustRegister(newQueueCollector(d))
	burst, err := strconv.Atoi(env("RATE_BURST", "5000"))
	if err != nil || burst < 1 {
		return errors.New("RATE_BURST must be positive")
	}
	limited := m.Middleware(router, utils.RateLimitMiddleware(utils.NewAPILimiter(burst), router))
	root := http.NewServeMux()
	root.Handle("/metrics", m.Handler())
	root.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	root.Handle("/", limited)
	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: root, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-done:
		d.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		return err
	} // Process exit cancels a forced shutdown; do not close an active queue.
	d.Close()
	return nil
}
