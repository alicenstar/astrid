package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicenstar/astrid/internal/handlers"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping() error { return m.err }

func TestHealthz_AllHealthy(t *testing.T) {
	h := handlers.NewHealthHandler(&mockPinger{}, &mockPinger{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", resp["status"])
	}
	if resp["postgres"] != "ok" {
		t.Fatalf("expected postgres ok, got %s", resp["postgres"])
	}
	if resp["redis"] != "ok" {
		t.Fatalf("expected redis ok, got %s", resp["redis"])
	}
}

func TestHealthz_PostgresDown(t *testing.T) {
	h := handlers.NewHealthHandler(
		&mockPinger{err: fmt.Errorf("connection refused")},
		&mockPinger{},
	)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["postgres"] != "error" {
		t.Fatalf("expected postgres error, got %s", resp["postgres"])
	}
}
