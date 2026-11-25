package handlers

import (
	"context"
	"go-microservice/utils"
	"net/http"
	"time"
)

type Checker interface{ Check(context.Context) error }

func IntegrationStatus(s Checker, d *utils.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		depth, processed, failed := d.Stats()
		if s == nil {
			JSON(w, 200, map[string]any{"mode": "local", "s3_enabled": false, "queue_depth": depth, "processed": processed, "failed": failed})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Check(ctx); err != nil {
			failure(w, 503, "S3 unavailable")
			return
		}
		JSON(w, 200, map[string]any{"s3_enabled": true, "queue_depth": depth, "processed": processed, "failed": failed})
	}
}
