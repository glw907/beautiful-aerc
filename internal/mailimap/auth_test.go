package mailimap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mailauth"
)

// mapStore is a minimal mailauth.TokenStore for tests.
type mapStore struct{ m map[string]string }

func newMapStore(slug, refresh string) *mapStore {
	return &mapStore{m: map[string]string{slug: refresh}}
}
func (s *mapStore) Set(a, r string) error { s.m[a] = r; return nil }
func (s *mapStore) Get(a string) (string, error) {
	return s.m[a], nil
}
func (s *mapStore) Delete(a string) error { delete(s.m, a); return nil }

// newFakeTokenSrv starts a fake OAuth token server. Each request
// increments the counter and returns a fresh access token.
func newFakeTokenSrv(t *testing.T) (url string, reqs *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"access_token":  "tok-" + string(rune('0'+n.Load())),
			"expires_in":    3600,
			"refresh_token": "ref-" + string(rune('0'+n.Load())),
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &n
}

// TestResolveXOAUTH2Token_WithOAuthClient verifies that when b.oauth is
// set, resolveXOAUTH2Token fetches the token from the mailauth.Client
// rather than from password-cmd.
func TestResolveXOAUTH2Token_WithOAuthClient(t *testing.T) {
	tokenURL, reqs := newFakeTokenSrv(t)
	store := newMapStore("acct", "refresh-1")
	c := mailauth.NewClient(
		mailauth.Config{ClientID: "id", TokenURL: tokenURL},
		store, "acct", mailauth.BackendKeyring,
	)

	b := &Backend{
		cfg:   config.AccountConfig{Name: "acct", Auth: "xoauth2"},
		oauth: c,
	}

	tok, err := resolveXOAUTH2Token(context.Background(), b)
	if err != nil {
		t.Fatalf("resolveXOAUTH2Token: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	if got := reqs.Load(); got != 1 {
		t.Errorf("expected 1 token request, got %d", got)
	}
}

// TestResolveXOAUTH2Token_NoOAuth verifies the password-cmd fallback
// when b.oauth is nil.
func TestResolveXOAUTH2Token_NoOAuth(t *testing.T) {
	b := &Backend{
		cfg: config.AccountConfig{
			Name:     "acct",
			Auth:     "xoauth2",
			Password: "static-token",
		},
	}

	tok, err := resolveXOAUTH2Token(context.Background(), b)
	if err != nil {
		t.Fatalf("resolveXOAUTH2Token: %v", err)
	}
	if tok != "static-token" {
		t.Errorf("expected static-token, got %q", tok)
	}
}

// TestNewWithOAuth verifies the constructor sets the oauth field.
func TestNewWithOAuth(t *testing.T) {
	tokenURL, _ := newFakeTokenSrv(t)
	store := newMapStore("acct", "refresh-1")
	c := mailauth.NewClient(
		mailauth.Config{ClientID: "id", TokenURL: tokenURL},
		store, "acct", mailauth.BackendKeyring,
	)
	b := NewWithOAuth(config.AccountConfig{Name: "acct"}, c, nil, false)
	if b.oauth == nil {
		t.Fatal("expected b.oauth to be set")
	}
}

func TestLooksSelfHosted(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"192.168.1.10", true},
		{"10.0.0.5", true},
		{"172.16.4.7", true},
		{"127.0.0.1", true},
		{"mail.local", true},
		{"imap.fastmail.com", false},
		{"outlook.office365.com", false},
		{"8.8.8.8", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := looksSelfHosted(tc.host); got != tc.want {
				t.Errorf("looksSelfHosted(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
