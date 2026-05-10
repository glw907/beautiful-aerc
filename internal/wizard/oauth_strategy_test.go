package wizard_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/wizard"
)

type fakeOAuthClient struct {
	outcome error
	calls   int
}

func (f *fakeOAuthClient) Authorize(ctx context.Context) error { f.calls++; return f.outcome }
func (f *fakeOAuthClient) Backend() mailauth.Backend           { return mailauth.BackendKeyring }

func TestOAuthStrategyApplySuccess(t *testing.T) {
	cli := &fakeOAuthClient{outcome: nil}
	s := wizard.NewOAuthStrategy(cli, "client-id", "client-secret")
	cfg, err := s.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth != "xoauth2" {
		t.Fatalf("auth: %q", cfg.Auth)
	}
	if cfg.PasswordCmd != "" {
		t.Fatalf("PasswordCmd should be empty, got %q", cfg.PasswordCmd)
	}
	if cfg.OAuth == nil || cfg.OAuth.ClientID != "client-id" {
		t.Fatalf("OAuth: %+v", cfg.OAuth)
	}
	if cfg.OAuthStore != string(mailauth.BackendKeyring) {
		t.Fatalf("OAuthStore: %q", cfg.OAuthStore)
	}
	if cli.calls != 1 {
		t.Fatalf("calls: %d", cli.calls)
	}
}

func TestOAuthStrategyApplyConsentTimeout(t *testing.T) {
	cli := &fakeOAuthClient{outcome: mailauth.ErrConsentTimeout}
	s := wizard.NewOAuthStrategy(cli, "id", "sec")
	if _, err := s.Apply(context.Background()); !errors.Is(err, mailauth.ErrConsentTimeout) {
		t.Fatalf("want ErrConsentTimeout, got %v", err)
	}
}
