package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
)

func TestContextRoundTrip(t *testing.T) {
	uid := uuid.New()
	ctx := auth.ContextWithUserID(context.Background(), uid)

	got, ok := auth.UserIDFromContext(ctx)
	if !ok {
		t.Fatal("expected user ID in context")
	}
	if got != uid {
		t.Fatalf("expected %s, got %s", uid, got)
	}
}

func TestContextMissing(t *testing.T) {
	_, ok := auth.UserIDFromContext(context.Background())
	if ok {
		t.Fatal("expected no user ID in empty context")
	}
}

func TestContextWithUser(t *testing.T) {
	uid := uuid.New()
	ctx := auth.ContextWithUser(context.Background(), uid, "test@example.com")

	got, ok := auth.UserIDFromContext(ctx)
	if !ok || got != uid {
		t.Fatal("expected user ID")
	}
	email := auth.EmailFromContext(ctx)
	if email != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", email)
	}
}

func TestEmailFromContextEmpty(t *testing.T) {
	email := auth.EmailFromContext(context.Background())
	if email != "" {
		t.Fatalf("expected empty, got %s", email)
	}
}

func getTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	return rdb
}

func TestSessionCreateAndGet(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	uid := uuid.New()
	sessionID, err := auth.CreateSession(rdb, uid, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	session, err := auth.GetSession(rdb, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if session.UserID != uid {
		t.Fatalf("expected %s, got %s", uid, session.UserID)
	}
	if session.Email != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", session.Email)
	}

	// Cleanup
	auth.DeleteSession(rdb, sessionID)
}

func TestSessionGetMissing(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	session, err := auth.GetSession(rdb, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if session != nil {
		t.Fatal("expected nil for missing session")
	}
}

func TestSessionDelete(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	uid := uuid.New()
	sessionID, _ := auth.CreateSession(rdb, uid, "del@example.com")

	err := auth.DeleteSession(rdb, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	session, _ := auth.GetSession(rdb, sessionID)
	if session != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMiddlewareRedirectsWithoutSession(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	mw := auth.NewAuthMiddleware(rdb)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to /login, got %s", w.Header().Get("Location"))
	}
}

func TestMiddlewareAllowsValidSession(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	uid := uuid.New()
	sessionID, _ := auth.CreateSession(rdb, uid, "test@example.com")
	defer auth.DeleteSession(rdb, sessionID)

	var gotUID uuid.UUID
	var gotEmail string
	mw := auth.NewAuthMiddleware(rdb)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, _ = auth.UserIDFromContext(r.Context())
		gotEmail = auth.EmailFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "astrid_session", Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUID != uid {
		t.Fatalf("expected uid %s, got %s", uid, gotUID)
	}
	if gotEmail != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", gotEmail)
	}
}

func TestMiddlewareRedirectsWithInvalidSession(t *testing.T) {
	rdb := getTestRedis(t)
	defer rdb.Close()

	mw := auth.NewAuthMiddleware(rdb)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "astrid_session", Value: "invalid-session-id"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}
