package services

import (
	"context"
	"go-microservice/models"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Exercises the real minio-go client against a local S3 protocol fixture.
// This is not a test against a running MinIO server.
func TestS3ClientProtocol(t *testing.T) {
	var mu sync.Mutex
	var savedPath, savedBody, auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Query().Has("location") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`))
			return
		}
		if r.Method == "HEAD" {
			w.WriteHeader(200)
			return
		}
		if r.Method == "PUT" {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			savedPath = r.URL.Path
			savedBody = string(b)
			auth = r.Header.Get("Authorization")
			mu.Unlock()
			w.Header().Set("ETag", `"abc123"`)
			w.WriteHeader(200)
			return
		}
		t.Errorf("unexpected S3 request: %s %s", r.Method, r.URL)
		w.WriteHeader(400)
	}))
	defer server.Close()
	s, err := NewIntegrationService(strings.TrimPrefix(server.URL, "http://"), "testkey", "testsecret", "users", "run-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnsureBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(context.Background(), models.Event{Sequence: 1, Action: "DELETE", User: models.User{ID: 7}}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if savedPath != "/users/events/run-1/00000000000000000001.json" || !strings.Contains(savedBody, `"action":"DELETE"`) || !strings.Contains(auth, "AWS4-HMAC-SHA256") {
		t.Fatalf("path=%s body=%s auth=%s", savedPath, savedBody, auth)
	}
}
