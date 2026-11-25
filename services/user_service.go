package services

import (
	"errors"
	"go-microservice/models"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("user not found")
var ErrBusy = errors.New("event queue is full")

type Enqueue func(models.Event) bool
type UserService struct {
	mu       sync.RWMutex
	users    map[int]models.User
	nextID   int
	sequence uint64
	enqueue  Enqueue
}

func NewUserService(enqueue Enqueue) *UserService {
	return &UserService{users: make(map[int]models.User), nextID: 1, enqueue: enqueue}
}
func (s *UserService) emit(action string, u models.User) error {
	e := models.Event{Sequence: s.sequence + 1, Action: action, User: u, Time: time.Now().UTC()}
	if s.enqueue != nil && !s.enqueue(e) {
		return ErrBusy
	}
	s.sequence++
	return nil
}
func (s *UserService) Create(u models.User) (models.User, error) {
	if err := u.Validate(); err != nil {
		return models.User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = s.nextID
	if err := s.emit("CREATE", u); err != nil {
		return models.User{}, err
	}
	s.nextID++
	s.users[u.ID] = u
	return u, nil
}
func (s *UserService) GetAll() []models.User {
	s.mu.RLock()
	out := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *UserService) Get(id int) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return models.User{}, ErrNotFound
	}
	return u, nil
}
func (s *UserService) Update(id int, u models.User) (models.User, error) {
	if err := u.Validate(); err != nil {
		return models.User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return models.User{}, ErrNotFound
	}
	u.ID = id
	if err := s.emit("UPDATE", u); err != nil {
		return models.User{}, err
	}
	s.users[id] = u
	return u, nil
}
func (s *UserService) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	if err := s.emit("DELETE", u); err != nil {
		return err
	}
	delete(s.users, id)
	return nil
}
