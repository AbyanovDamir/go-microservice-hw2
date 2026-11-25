package handlers

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"go-microservice/models"
	"go-microservice/services"
	"io"
	"net/http"
	"strconv"
)

type UserHandler struct{ Service *services.UserService }

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}
func input(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var u *models.User
	if err := dec.Decode(&u); err != nil || u == nil {
		failure(w, 400, "expected a user JSON object up to 4096 bytes")
		return models.User{}, false
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		failure(w, 400, "expected exactly one JSON object")
		return models.User{}, false
	}
	if u.ID != 0 {
		failure(w, 400, "id is assigned by the server")
		return models.User{}, false
	}
	if err := u.Validate(); err != nil {
		failure(w, 400, err.Error())
		return models.User{}, false
	}
	return *u, true
}
func id(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || n < 1 {
		failure(w, 400, "id must be a positive integer")
		return 0, false
	}
	return n, true
}
func serviceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		failure(w, 404, err.Error())
	case errors.Is(err, services.ErrBusy):
		w.Header().Set("Retry-After", "1")
		failure(w, 503, err.Error())
	default:
		failure(w, 400, err.Error())
	}
}
func (h UserHandler) List(w http.ResponseWriter, r *http.Request) { JSON(w, 200, h.Service.GetAll()) }
func (h UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	n, ok := id(w, r)
	if !ok {
		return
	}
	u, err := h.Service.Get(n)
	if err != nil {
		serviceError(w, err)
		return
	}
	JSON(w, 200, u)
}
func (h UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := input(w, r)
	if !ok {
		return
	}
	u, err := h.Service.Create(u)
	if err != nil {
		serviceError(w, err)
		return
	}
	w.Header().Set("Location", "/api/users/"+strconv.Itoa(u.ID))
	JSON(w, 201, u)
}
func (h UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	n, ok := id(w, r)
	if !ok {
		return
	}
	u, ok := input(w, r)
	if !ok {
		return
	}
	u, err := h.Service.Update(n, u)
	if err != nil {
		serviceError(w, err)
		return
	}
	JSON(w, 200, u)
}
func (h UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	n, ok := id(w, r)
	if !ok {
		return
	}
	if err := h.Service.Delete(n); err != nil {
		serviceError(w, err)
		return
	}
	w.WriteHeader(204)
}
