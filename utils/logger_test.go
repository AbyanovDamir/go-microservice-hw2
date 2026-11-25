package utils

import (
	"context"
	"errors"
	"go-microservice/models"
	"io"
	"log/slog"
	"sync"
	"testing"
)

type fakeSink struct {
	mu     sync.Mutex
	calls  int
	events []models.Event
}

func (s *fakeSink) Save(ctx context.Context, e models.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		return errors.New("transient")
	}
	s.events = append(s.events, e)
	return nil
}
func TestDispatcherRetriesAndDrains(t *testing.T) {
	s := &fakeSink{}
	d := NewDispatcher(s, slog.New(slog.NewTextHandler(io.Discard, nil)), 10)
	for i := 1; i <= 3; i++ {
		if !d.Enqueue(models.Event{Sequence: uint64(i), User: models.User{ID: i}}) {
			t.Fatal("unexpected queue overflow")
		}
	}
	d.Close()
	n, p, f := d.Stats()
	if n != 0 || p != 3 || f != 0 || s.calls != 4 || len(s.events) != 3 {
		t.Fatalf("%d %d %d %+v", n, p, f, s)
	}
	for i, e := range s.events {
		if e.Sequence != uint64(i+1) {
			t.Fatal("event reorder")
		}
	}
}
