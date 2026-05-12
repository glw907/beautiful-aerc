package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

const reauthTestTOML = `[[account]]
name = "work"
email = "u@example.com"
provider = "imap"
host = "imap.example.com"
port = 993
password = "p"

[account.smtp]
host = "smtp.example.com"
port = 587
`

func loadReauthTestConfig(t *testing.T) ([]config.AccountConfig, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(reauthTestTOML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	accts, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return accts, cfgPath
}

func TestRunReauth_UnknownAccountLookup(t *testing.T) {
	_, cfgPath := loadReauthTestConfig(t)
	err := runReauth(rootFlags{config: cfgPath, reauth: "nonexistent"})
	if !errors.Is(err, errUnknownReauthAccount) {
		t.Fatalf("err = %v, want errUnknownReauthAccount", err)
	}
}

func TestRunReauth_NonOAuthAccount(t *testing.T) {
	_, cfgPath := loadReauthTestConfig(t)
	err := runReauth(rootFlags{config: cfgPath, reauth: "work"})
	if !errors.Is(err, errReauthAccountNotOAuth) {
		t.Fatalf("err = %v, want errReauthAccountNotOAuth", err)
	}
}
