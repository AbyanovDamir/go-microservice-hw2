package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-microservice/models"
)

// Opt-in end-to-end test against the running Compose API and a real MinIO.
func TestComposeCRUDAndS3Archive(t *testing.T) {
	if os.Getenv("RUN_E2E") != "1" {
		t.Skip("set RUN_E2E=1 after docker compose up")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	base := os.Getenv("API_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, base+path, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: %d %s", method, path, response.StatusCode, data)
		}
		t.Logf("%s %s -> %d", method, path, response.StatusCode)
		return data
	}
	var user models.User
	email := fmt.Sprintf("e2e-%d@example.org", time.Now().UnixNano())
	data := request("POST", "/api/users", fmt.Sprintf(`{"name":"E2E test","email":%q}`, email), 201)
	if err := json.Unmarshal(data, &user); err != nil || user.ID < 1 {
		t.Fatalf("invalid user %s", data)
	}
	path := fmt.Sprintf("/api/users/%d", user.ID)
	request("GET", path, "", 200)
	request("PUT", path, fmt.Sprintf(`{"name":"E2E updated","email":%q}`, email), 200)
	request("DELETE", path, "", 204)
	request("GET", path, "", 404)
	request("POST", "/api/users", `{"name":"Invalid","email":"bad"}`, 400)
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}
	key := os.Getenv("MINIO_ACCESS_KEY")
	if key == "" {
		key = "minioadmin"
	}
	secret := os.Getenv("MINIO_SECRET_KEY")
	if secret == "" {
		secret = "minioadmin123"
	}
	s3, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(key, secret, ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var found []models.Event
	var keys []string
	for {
		found = nil
		keys = nil
		for object := range s3.ListObjects(ctx, "users", minio.ListObjectsOptions{Prefix: "events/", Recursive: true}) {
			if object.Err != nil {
				t.Fatal(object.Err)
			}
			stream, err := s3.GetObject(ctx, "users", object.Key, minio.GetObjectOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var event models.Event
			err = json.NewDecoder(stream).Decode(&event)
			_ = stream.Close()
			if err != nil {
				t.Fatal(err)
			}
			if event.User.ID == user.ID && event.User.Email == email {
				found = append(found, event)
				keys = append(keys, object.Key)
			}
		}
		if len(found) >= 3 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("expected 3 archived events, got %d", len(found))
		case <-time.After(200 * time.Millisecond):
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Sequence < found[j].Sequence })
	if len(found) != 3 || found[0].Action != "CREATE" || found[1].Action != "UPDATE" || found[2].Action != "DELETE" {
		t.Fatalf("unexpected events: %+v", found)
	}
	if found[1].User.Name != "E2E updated" || found[2].User.Name != "E2E updated" {
		t.Fatalf("incorrect snapshots: %+v", found)
	}
	for _, event := range found {
		t.Logf("S3 event sequence=%d action=%s user_id=%d", event.Sequence, event.Action, event.User.ID)
	}
	for _, key := range keys {
		t.Logf("S3 object: users/%s", key)
	}
	request("GET", "/api/integration/status", "", 200)
	t.Log("Real MinIO archive verified: CREATE -> UPDATE -> DELETE")
}
