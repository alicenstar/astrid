package license

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newMockSDK(t *testing.T, fields map[string]LicenseField) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route: /api/v1/license/fields/{name}
		const prefix = "/api/v1/license/fields/"
		if len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix {
			name := r.URL.Path[len(prefix):]
			field, ok := fields[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(field)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestIsExpired_PastDate(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	srv := newMockSDK(t, map[string]LicenseField{
		"expires_at": {Name: "expires_at", Title: "Expiration", Value: past, ValueType: "String"},
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.IsExpired() {
		t.Fatal("expected IsExpired()=true for past date")
	}
}

func TestIsExpired_FutureDate(t *testing.T) {
	future := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	srv := newMockSDK(t, map[string]LicenseField{
		"expires_at": {Name: "expires_at", Title: "Expiration", Value: future, ValueType: "String"},
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.IsExpired() {
		t.Fatal("expected IsExpired()=false for future date")
	}
}

func TestIsExpired_MissingField(t *testing.T) {
	srv := newMockSDK(t, map[string]LicenseField{})
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.IsExpired() {
		t.Fatal("expected IsExpired()=false when expires_at field is missing")
	}
}

func TestIsExpired_EmptyValue(t *testing.T) {
	srv := newMockSDK(t, map[string]LicenseField{
		"expires_at": {Name: "expires_at", Title: "Expiration", Value: "", ValueType: "String"},
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.IsExpired() {
		t.Fatal("expected IsExpired()=false for empty value")
	}
}

func TestIsExpired_InvalidDateFormat(t *testing.T) {
	srv := newMockSDK(t, map[string]LicenseField{
		"expires_at": {Name: "expires_at", Title: "Expiration", Value: "not-a-date", ValueType: "String"},
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.IsExpired() {
		t.Fatal("expected IsExpired()=false for invalid date")
	}
}

func TestIsExpired_SDKUnreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // nothing listening
	if c.IsExpired() {
		t.Fatal("expected IsExpired()=false when SDK is unreachable")
	}
}
