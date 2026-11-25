package handlers

import (
	"github.com/gorilla/mux"
	"go-microservice/models"
	"go-microservice/services"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCRUDAndInvalidInput(t *testing.T) {
	h := UserHandler{services.NewUserService(nil)}
	r := mux.NewRouter()
	r.HandleFunc("/api/users", h.List).Methods("GET")
	r.HandleFunc("/api/users", h.Create).Methods("POST")
	r.HandleFunc("/api/users/{id}", h.Get).Methods("GET")
	r.HandleFunc("/api/users/{id}", h.Update).Methods("PUT")
	r.HandleFunc("/api/users/{id}", h.Delete).Methods("DELETE")
	cases := []struct {
		method, path, body string
		code               int
		contains           string
	}{
		{"GET", "/api/users", "", 200, "[]"},
		{"POST", "/api/users", `{"name":"Alice","email":"a@example.org"}`, 201, `"id":1`},
		{"GET", "/api/users/1", "", 200, "Alice"},
		{"PUT", "/api/users/1", `{"name":"Bob","email":"b@example.org"}`, 200, "Bob"},
		{"POST", "/api/users", `null`, 400, "error"},
		{"POST", "/api/users", `{"name":"A","email":"bad"}`, 400, "error"},
		{"POST", "/api/users", `{"name":"A","email":"A <a@example.org>"}`, 400, "error"},
		{"POST", "/api/users", `{"name":" ","email":"a@example.org"}`, 400, "error"},
		{"POST", "/api/users", `{"id":12,"name":"A","email":"a@example.org"}`, 400, "error"},
		{"POST", "/api/users", `{"unexpected":1}`, 400, "error"},
		{"POST", "/api/users", `{"name":"A","email":"a@example.org"} {}`, 400, "error"},
		{"POST", "/api/users", `{`, 400, "error"},
		{"POST", "/api/users", strings.Repeat("x", 5000), 400, "error"},
		{"GET", "/api/users/abc", "", 400, "error"},
		{"GET", "/api/users/0", "", 400, "error"},
		{"GET", "/api/users/999", "", 404, "error"},
		{"DELETE", "/api/users/1", "", 204, ""},
		{"GET", "/api/users/1", "", 404, "error"},
		{"PUT", "/api/users/1", `{"name":"A","email":"a@example.org"}`, 404, "error"},
		{"DELETE", "/api/users/1", "", 404, "error"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(c.method, c.path, strings.NewReader(c.body)))
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.contains) {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
}
func TestQueueFullReturns503(t *testing.T) {
	h := UserHandler{services.NewUserService(func(models.Event) bool { return false })}
	w := httptest.NewRecorder()
	h.Create(w, httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"A","email":"a@example.org"}`)))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
