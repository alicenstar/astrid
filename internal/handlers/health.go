package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Pinger interface {
	Ping() error
}

type pgPinger struct {
	db interface {
		PingContext(ctx context.Context) error
	}
}

func (p *pgPinger) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.db.PingContext(ctx)
}

// PingerFunc is a function that implements Pinger.
type PingerFunc func() error

func (f PingerFunc) Ping() error { return f() }

func NewPgPinger(db interface{ PingContext(ctx context.Context) error }) Pinger {
	return &pgPinger{db: db}
}

type HealthHandler struct {
	pg    Pinger
	redis Pinger
}

func NewHealthHandler(pg, redis Pinger) *HealthHandler {
	return &HealthHandler{pg: pg, redis: redis}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":   "ok",
		"postgres": "ok",
		"redis":    "ok",
	}
	status := http.StatusOK

	if err := h.pg.Ping(); err != nil {
		resp["postgres"] = "error"
		resp["status"] = "degraded"
		status = http.StatusServiceUnavailable
	}
	if err := h.redis.Ping(); err != nil {
		resp["redis"] = "error"
		resp["status"] = "degraded"
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
