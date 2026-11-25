package services

import (
	"fmt"
	"go-microservice/models"
	"sync"
	"testing"
)

func TestConcurrentCRUDAndEventOrder(t *testing.T) {
	var events []models.Event
	s := NewUserService(func(e models.Event) bool { events = append(events, e); return true })
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := s.Create(models.User{Name: fmt.Sprint(i), Email: "a@example.org"})
			if err != nil {
				t.Error(err)
				return
			}
			if _, err = s.Update(u.ID, models.User{Name: "updated", Email: u.Email}); err != nil {
				t.Error(err)
			}
			if _, err = s.Get(u.ID); err != nil {
				t.Error(err)
			}
			_ = s.GetAll()
			if err = s.Delete(u.ID); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if len(s.GetAll()) != 0 || len(events) != 300 {
		t.Fatalf("users/events: %d/%d", len(s.GetAll()), len(events))
	}
	for i, e := range events {
		if e.Sequence != uint64(i+1) || e.User.ID < 1 {
			t.Fatalf("invalid event: %+v", e)
		}
	}
}

func TestQueueFailureDoesNotMutateStore(t *testing.T) {
	accept := false
	s := NewUserService(func(models.Event) bool { return accept })
	u := models.User{Name: "Alice", Email: "a@example.org"}
	if _, err := s.Create(u); err != ErrBusy {
		t.Fatal(err)
	}
	accept = true
	created, err := s.Create(u)
	if err != nil || created.ID != 1 {
		t.Fatalf("%+v %v", created, err)
	}
	accept = false
	if _, err = s.Update(1, models.User{Name: "changed", Email: u.Email}); err != ErrBusy {
		t.Fatal(err)
	}
	if err = s.Delete(1); err != ErrBusy {
		t.Fatal(err)
	}
	got, _ := s.Get(1)
	if got.Name != "Alice" {
		t.Fatal(got)
	}
}
