package mailauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

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
	return NewClient(cfg, store, "test-account", BackendKeyring)
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
	if tok1 != tok2 {
		t.Errorf("cached token mismatch: tok1=%q tok2=%q", tok1, tok2)
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

// simulateConsent parses redirect_uri and state from authURL, then sends the
// authorization code to the redirect URI in a goroutine.
func simulateConsent(authURL, code, stateOverride string) {
	go func() {
		u, err := url.Parse(authURL)
		if err != nil {
			return
		}
		q := u.Query()
		redirect := q.Get("redirect_uri")
		state := q.Get("state")
		if stateOverride != "" {
			state = stateOverride
		}
		target := redirect + "?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
		http.Get(target) //nolint:errcheck,noctx
	}()
}

func TestAuthorizeHappyPath(t *testing.T) {
	fs := newFakeTokenServer(t)
	store := newMemStore()

	cfg := Config{
		ClientID:          "cid",
		AuthURL:           "http://example.com/auth",
		TokenURL:          fs.srv.URL + "/token",
		Scopes:            []string{"email"},
		RedirectPortRange: [2]int{0, 0},
	}
	c := NewClient(cfg, store, "a", BackendKeyring)
	SetOpenBrowser(c, func(authURL string) error {
		simulateConsent(authURL, "ok", "")
		return nil
	})

	if err := c.Authorize(context.Background()); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	got, err := store.Get("a")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if got == "" {
		t.Error("expected refresh token in store, got empty")
	}
}

func TestAuthorizeStateMismatchRejected(t *testing.T) {
	fs := newFakeTokenServer(t)
	store := newMemStore()

	cfg := Config{
		ClientID:          "cid",
		AuthURL:           "http://example.com/auth",
		TokenURL:          fs.srv.URL + "/token",
		RedirectPortRange: [2]int{0, 0},
	}
	c := NewClient(cfg, store, "a", BackendKeyring)
	SetOpenBrowser(c, func(authURL string) error {
		simulateConsent(authURL, "ok", "wrong")
		return nil
	})

	err := c.Authorize(context.Background())
	if !errors.Is(err, ErrStateMismatch) {
		t.Errorf("expected ErrStateMismatch, got: %v", err)
	}
}

func TestAuthorizeTimeout(t *testing.T) {
	fs := newFakeTokenServer(t)
	_ = fs
	store := newMemStore()

	cfg := Config{
		ClientID:          "cid",
		AuthURL:           "http://example.com/auth",
		TokenURL:          "http://unused/token",
		RedirectPortRange: [2]int{0, 0},
	}
	c := NewClient(cfg, store, "a", BackendKeyring)
	SetOpenBrowser(c, func(string) error { return nil }) // no-op
	SetConsentTimeout(c, 50*time.Millisecond)

	err := c.Authorize(context.Background())
	if !errors.Is(err, ErrConsentTimeout) {
		t.Errorf("expected ErrConsentTimeout, got: %v", err)
	}
}

func TestForceRefreshDropsCache(t *testing.T) {
	fs := newFakeTokenServer(t)
	store := newMemStore()
	store.m["test-account"] = "refresh-1"

	cfg := Config{
		ClientID: "id",
		TokenURL: fs.srv.URL + "/token",
	}
	c := newTestClient(cfg, store)

	if _, err := c.Token(t.Context()); err != nil {
		t.Fatalf("Token: %v", err)
	}
	fs.requests.Store(0)

	tok, err := c.ForceRefresh(t.Context())
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token from ForceRefresh")
	}
	if got := fs.requests.Load(); got != 1 {
		t.Fatalf("requests after ForceRefresh: %d, want 1", got)
	}
}

// isErrAuth unwraps the error chain looking for mail.ErrAuth.
func isErrAuth(err error) bool {
	return errors.Is(err, mail.ErrAuth)
}

func TestBuildAuthURL_ScopeOmittedWhenEmpty(t *testing.T) {
	c := NewClient(Config{
		ClientID: "cid",
		AuthURL:  "https://example.test/auth",
	}, newMemStore(), "u", BackendKeyring)
	got := c.buildAuthURL("st", "ch", "http://127.0.0.1:1234/callback")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if u.Query().Has("scope") {
		t.Errorf("auth URL contains scope= with empty Scopes: %s", got)
	}

	c = NewClient(Config{
		ClientID: "cid",
		AuthURL:  "https://example.test/auth",
		Scopes:   []string{"a", "b"},
	}, newMemStore(), "u", BackendKeyring)
	got = c.buildAuthURL("st", "ch", "http://127.0.0.1:1234/callback")
	u, _ = url.Parse(got)
	if u.Query().Get("scope") != "a b" {
		t.Errorf("scope = %q, want %q", u.Query().Get("scope"), "a b")
	}
}

func TestGeneratePKCEVerifierAndStateAreNonEmpty(t *testing.T) {
	v, err := generatePKCEVerifier()
	if err != nil {
		t.Fatalf("generatePKCEVerifier: %v", err)
	}
	if v == "" {
		t.Error("verifier is empty")
	}
	s, err := generateState()
	if err != nil {
		t.Fatalf("generateState: %v", err)
	}
	if s == "" {
		t.Error("state is empty")
	}
}

type countingStore struct {
	memStore
	sets int
}

func (s *countingStore) Set(account, refresh string) error {
	s.sets++
	return s.memStore.Set(account, refresh)
}

func TestTokenPersistsRefreshOnlyWhenRotated(t *testing.T) {
	cases := []struct {
		name       string
		serverRT   string
		wantSets   int
		wantStored string
	}{
		{"unchanged", "rt-original", 0, "rt-original"},
		{"omitted", "", 0, "rt-original"},
		{"rotated", "rt-new", 1, "rt-new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeTokenServer(t)
			fs.respond = func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]any{"access_token": "at", "expires_in": 3600}
				if tc.serverRT != "" {
					resp["refresh_token"] = tc.serverRT
				}
				_ = json.NewEncoder(w).Encode(resp)
			}
			store := &countingStore{memStore: memStore{m: map[string]string{"test-account": "rt-original"}}}
			c := newTestClient(Config{ClientID: "cid", TokenURL: fs.srv.URL + "/token"}, store)
			if _, err := c.Token(t.Context()); err != nil {
				t.Fatalf("Token: %v", err)
			}
			if store.sets != tc.wantSets {
				t.Errorf("Set calls = %d, want %d", store.sets, tc.wantSets)
			}
			if got := store.m["test-account"]; got != tc.wantStored {
				t.Errorf("stored refresh = %q, want %q", got, tc.wantStored)
			}
		})
	}
}

func TestClassifyOAuthErr_StatusGuard(t *testing.T) {
	// The `sc == 400 || sc == 401` guard means an invalid_grant body at
	// 500 must not classify as ErrAuth. Either condition could drop
	// silently without changing 200/400/401 behavior, so the 500 case
	// is what pins both halves of the OR.
	fs := newFakeTokenServer(t)
	fs.respond = func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid_grant",
		})
	}
	store := newMemStore()
	store.m["test-account"] = "rt"
	c := newTestClient(Config{ClientID: "cid", TokenURL: fs.srv.URL + "/token"}, store)

	_, err := c.Token(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, mail.ErrAuth) {
		t.Errorf("err wraps mail.ErrAuth for 500-status response: %v", err)
	}
}
