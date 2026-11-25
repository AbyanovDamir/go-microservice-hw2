package utils

import (
	"context"
	"go-microservice/models"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type EventSink interface {
	Save(context.Context, models.Event) error
}

// One bounded worker preserves event order; retries reuse the same S3 key.
type Dispatcher struct {
	queue     chan models.Event
	wg        sync.WaitGroup
	log       *slog.Logger
	sink      EventSink
	failed    atomic.Uint64
	processed atomic.Uint64
}

func NewDispatcher(sink EventSink, logger *slog.Logger, capacity int) *Dispatcher {
	d := &Dispatcher{queue: make(chan models.Event, capacity), sink: sink, log: logger}
	d.wg.Add(1)
	go d.run()
	return d
}

// Close must be called only after all HTTP handlers have stopped.
func (d *Dispatcher) Close() { close(d.queue); d.wg.Wait() }
func (d *Dispatcher) Enqueue(e models.Event) bool {
	select {
	case d.queue <- e:
		return true
	default:
		return false
	}
}
func (d *Dispatcher) Stats() (int, uint64, uint64) {
	return len(d.queue), d.processed.Load(), d.failed.Load()
}
func (d *Dispatcher) run() {
	defer d.wg.Done()
	for e := range d.queue {
		d.log.Info("audit", "sequence", e.Sequence, "action", e.Action, "user_id", e.User.ID)
		if d.sink != nil {
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				if attempt > 0 {
					time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				err = d.sink.Save(ctx, e)
				cancel()
				if err == nil {
					break
				}
			}
			if err != nil {
				d.failed.Add(1)
				d.log.Error("backup failed", "sequence", e.Sequence, "error", err)
			}
		}
		// A local notification adapter; no external email is sent.
		d.log.Info("notification", "event", e.Action, "user_id", e.User.ID, "adapter", "log")
		d.processed.Add(1)
	}
}
