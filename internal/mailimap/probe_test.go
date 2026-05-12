package mailimap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

// memStore is a no-op TokenStore that always returns ("", nil) from Get,
// simulating a store with no token written.
type memStore struct{}

func (m *memStore) Set(_, _ string) error        { return nil }
func (m *memStore) Get(_ string) (string, error) { return "", nil }
func (m *memStore) Delete(_ string) error        { return nil }

func withProbeDial(t *testing.T, fn probeDialFn) {
	t.Helper()
	prev := probeDial
	probeDial = fn
	t.Cleanup(func() { probeDial = prev })
}

func okDial(cli imapClient) probeDialFn {
	return func(cfg config.AccountConfig, _ string) (imapClient, []mail.ProbeStep, error) {
		return cli, []mail.ProbeStep{
			{Label: "Connecting", Status: mail.ProbeOK, Detail: "imap.example:993"},
			{Label: "TLS handshake", Status: mail.ProbeOK},
			{Label: "AUTHENTICATE", Status: mail.ProbeOK, Detail: "plain"},
		}, nil
	}
}

func TestProbeOAuthTokenStepFirst(t *testing.T) {
	store := &memStore{}
	cli := mailauth.NewClient(mailauth.Config{TokenURL: "http://example.invalid/token"}, store, "a", mailauth.BackendKeyring)
	cfg := config.AccountConfig{Host: "h", Port: 993, Email: "u@x"}
	r := Probe(context.Background(), cfg, cli)
	if len(r.Steps) == 0 || r.Steps[0].Label != "oauth-token" {
		t.Fatalf("first step label: %+v", r.Steps)
	}
	if !errors.Is(r.Err, mail.ErrAuth) {
		t.Fatalf("err: %v", r.Err)
	}
}

func TestProbeNoOAuthStepWhenNoClient(t *testing.T) {
	withProbeDial(t, func(cfg config.AccountConfig, _ string) (imapClient, []mail.ProbeStep, error) {
		return nil, []mail.ProbeStep{{Label: "Connecting", Status: mail.ProbeFail, Detail: "refused"}},
			errors.New("dial: refused")
	})
	cfg := config.AccountConfig{Host: "h", Port: 993, Email: "u@x", Password: "p"}
	r := Probe(context.Background(), cfg, nil)
	for _, s := range r.Steps {
		if s.Label == "oauth-token" {
			t.Fatalf("unexpected oauth-token step: %+v", r.Steps)
		}
	}
}

func TestProbe_HappyPath(t *testing.T) {
	cli := newFakeClient()
	cli.caps["UIDPLUS"] = true
	cli.folderSummary["INBOX"] = mail.Folder{Name: "INBOX", Exists: 1247}
	withProbeDial(t, okDial(cli))

	r := Probe(context.Background(), config.AccountConfig{
		Name: "t", Backend: "imap", Email: "u@x", Host: "imap.example", Port: 993,
	}, nil)

	wantLabels := []string{"Connecting", "TLS handshake", "AUTHENTICATE", "CAPABILITY (UIDPLUS)", "STATUS INBOX"}
	if len(r.Steps) != len(wantLabels) {
		t.Fatalf("len(Steps) = %d, want %d (%+v)", len(r.Steps), len(wantLabels), r.Steps)
	}
	for i, want := range wantLabels {
		if r.Steps[i].Label != want {
			t.Errorf("Steps[%d].Label = %q, want %q", i, r.Steps[i].Label, want)
		}
		if r.Steps[i].Status != mail.ProbeOK {
			t.Errorf("Steps[%d].Status = %v, want ProbeOK", i, r.Steps[i].Status)
		}
	}
	if got := r.Steps[4].Detail; got != "1247 messages" {
		t.Errorf("STATUS INBOX detail = %q, want %q", got, "1247 messages")
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
	if !r.OK() {
		t.Errorf("OK() = false, want true")
	}
}

func TestProbe_DialFailureStopsTranscript(t *testing.T) {
	withProbeDial(t, func(cfg config.AccountConfig, _ string) (imapClient, []mail.ProbeStep, error) {
		return nil, []mail.ProbeStep{
			{Label: "Connecting", Status: mail.ProbeFail, Detail: "i/o timeout"},
		}, errors.New("dial: i/o timeout")
	})

	r := Probe(context.Background(), config.AccountConfig{Host: "x", Port: 993}, nil)
	if len(r.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
	if r.OK() {
		t.Error("OK() = true, want false")
	}
}

func TestProbe_MissingUIDPLUSFails(t *testing.T) {
	cli := newFakeClient() // caps map empty
	withProbeDial(t, okDial(cli))

	r := Probe(context.Background(), config.AccountConfig{Host: "x", Port: 993}, nil)
	if len(r.Steps) != 4 {
		t.Fatalf("len(Steps) = %d, want 4 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Steps[3].Status != mail.ProbeFail {
		t.Errorf("CAPABILITY step status = %v, want ProbeFail", r.Steps[3].Status)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
}

// memTokenStore is a map-backed TokenStore for probe tests.
type memTokenStore struct {
	tok string
}

func (s *memTokenStore) Set(_, v string) error        { s.tok = v; return nil }
func (s *memTokenStore) Get(_ string) (string, error) { return s.tok, nil }
func (s *memTokenStore) Delete(_ string) error        { return nil }

// tokenServerStub starts an httptest server that returns a fresh access token
// using the stored refresh token, and returns the server URL.
func tokenServerStub(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"access_token":  "stub-access-token",
			"expires_in":    3600,
			"refresh_token": "stub-refresh-token",
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestProbeUsesOAuthAccessToken(t *testing.T) {
	prev := probeDial
	t.Cleanup(func() { probeDial = prev })

	var gotToken string
	probeDial = func(cfg config.AccountConfig, accessToken string) (imapClient, []mail.ProbeStep, error) {
		gotToken = accessToken
		fc := newFakeClient()
		fc.caps["UIDPLUS"] = true
		return fc, nil, nil
	}

	store := &memTokenStore{tok: "rt"}
	cli := mailauth.NewClient(mailauth.Config{
		ClientID: "id", AuthURL: "https://example/auth", TokenURL: tokenServerStub(t),
	}, store, "slug", mailauth.BackendKeyring)

	r := Probe(context.Background(), config.AccountConfig{Auth: "xoauth2", Host: "h"}, cli)
	if !r.OK() {
		t.Fatalf("probe failed: %+v", r)
	}
	if gotToken == "" {
		t.Fatal("expected access token threaded into probeDial")
	}
}
