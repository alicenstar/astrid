package license

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type contextKey string

const StatusKey contextKey = "licenseStatus"

type Status struct {
	Expired         bool
	UpdateAvailable bool
	UpdateVersion   string
}

func StatusMiddleware(client *Client) func(http.Handler) http.Handler {
	var (
		mu     sync.RWMutex
		status Status
	)

	go func() {
		// Short initial interval to handle SDK not being ready at startup.
		interval := 10 * time.Second
		for {
			s := Status{}

			s.Expired = client.IsExpired()

			if update, err := client.CheckForUpdates(); err == nil && update != nil {
				s.UpdateAvailable = true
				s.UpdateVersion = update.VersionLabel
			}

			mu.Lock()
			status = s
			mu.Unlock()

			time.Sleep(interval)
			interval = 5 * time.Minute
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.RLock()
			s := status
			mu.RUnlock()
			ctx := context.WithValue(r.Context(), StatusKey, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetStatus(r *http.Request) Status {
	s, _ := r.Context().Value(StatusKey).(Status)
	return s
}
