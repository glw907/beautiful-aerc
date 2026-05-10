package mailauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

// memStore is a map-backed TokenStore for tests.
type memStore struct {
	m map[string]string
}

func newMemStore() *memStore { return &memStore{m: map[string]string{}} }

func (s *memStore) Set(account, refresh string) error {
	s.m[account] = refresh
	return nil
}

func (s *memStore) Get(account string) (string, error) {
	return s.m[account], nil
}

func (s *memStore) Delete(account string) error {
	delete(s.m, account)
	return nil
}

type fakeServer struct {
	srv       *httptest.Server
	requests  atomic.Int64
	respond   func(http.ResponseWriter)
	expiresIn int
}

func newFakeTokenServer(t *testing.T) *fakeServer {
	t.Helper()
	fs := &fakeServer{expiresIn: 3600}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.requests.Add(1)
		if fs.respond != nil {
			fs.respond(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		n := fs.requests.Load()
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"access_token":  "acc-" + string(rune('0'+n)),
			"expires_in":    fs.expiresIn,
			"refresh_token": "refresh-" + string(rune('0'+n)),
		})
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func newTestClient(cfg Config, store TokenStore) *Client {
	return NewClient(cfg, store, "test-account")
}

func TestTokenReturnsCachedWhenFresh(t *testing.T) {
	fs := newFakeTokenServer(t)
	store := newMemStore()
	store.m["test-account"] = "refresh-initial"

	cfg := Config{
		ClientID: "cid",
		TokenURL: fs.srv.URL + "/token",
	}
	c := newTestClient(cfg, store)

	tok1, err := c.Token(t.Context())
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	if tok1 == "" {
		t.Fatal("expected non-empty token")
	}

	// Reset counter; fresh token should come from cache.
	fs.requests.Store(0)

	tok2, err := c.Token(t.Context())
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if tok2 == "" {
		t.Fatal("expected non-empty token")
	}
	if got := fs.requests.Load(); got != 0 {
		t.Errorf("expected 0 network hits, got %d", got)
	}
}

func TestTokenRefreshesNearExpiry(t *testing.T) {
	fs := newFakeTokenServer(t)
	fs.expiresIn = 60 // 60-second tokens, below 5-min threshold

	store := newMemStore()
	store.m["test-account"] = "refresh-initial"

	cfg := Config{
		ClientID: "cid",
		TokenURL: fs.srv.URL + "/token",
	}
	c := newTestClient(cfg, store)

	if _, err := c.Token(t.Context()); err != nil {
		t.Fatalf("first Token: %v", err)
	}

	// Cached token expires in 60s, under the 5-min threshold → must refresh.
	hits := fs.requests.Load()
	if _, err := c.Token(t.Context()); err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if got := fs.requests.Load(); got != hits+1 {
		t.Errorf("expected 1 additional network hit, got %d total", fs.requests.Load())
	}
}

func TestTokenSurfacesAuthErrorOnInvalidGrant(t *testing.T) {
	fs := newFakeTokenServer(t)
	fs.respond = func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"error":             "invalid_grant",
			"error_description": "token revoked",
		})
	}

	store := newMemStore()
	store.m["test-account"] = "refresh-bad"

	cfg := Config{
		ClientID: "cid",
		TokenURL: fs.srv.URL + "/token",
	}
	c := newTestClient(cfg, store)

	_, err := c.Token(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected error wrapping mail.ErrAuth, got: %v", err)
	}
}

func TestTokenWithoutStoredRefreshErrAuth(t *testing.T) {
	fs := newFakeTokenServer(t)

	store := newMemStore() // empty, no refresh token stored

	cfg := Config{
		ClientID: "cid",
		TokenURL: fs.srv.URL + "/token",
	}
	c := newTestClient(cfg, store)

	_, err := c.Token(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected error wrapping mail.ErrAuth, got: %v", err)
	}
	if got := fs.requests.Load(); got != 0 {
		t.Errorf("expected 0 network hits, got %d", got)
	}
}

// isErrAuth unwraps the error chain looking for mail.ErrAuth.
func isErrAuth(err error) bool {
	return errors.Is(err, mail.ErrAuth)
}
