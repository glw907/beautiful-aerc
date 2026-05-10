package main

import (
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
	accts, _ := loadReauthTestConfig(t)

	var found *config.AccountConfig
	for i := range accts {
		if accts[i].Name == "nonexistent" {
			found = &accts[i]
		}
	}
	if found != nil {
		t.Fatal("expected nil for unknown account")
	}
}

func TestRunReauth_NonOAuthAccount(t *testing.T) {
	accts, _ := loadReauthTestConfig(t)

	var found *config.AccountConfig
	for i := range accts {
		if accts[i].Name == "work" {
			found = &accts[i]
		}
	}
	if found == nil {
		t.Fatal("account 'work' not found")
	}
	if found.OAuth != nil {
		t.Fatal("expected nil OAuth for plain-IMAP account")
	}
}
