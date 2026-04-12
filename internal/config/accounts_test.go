package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAccounts(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantN   int
		wantErr string
	}{
		{
			name: "single jmap account",
			toml: `[[account]]
name = "Fastmail"
backend = "jmap"
source = "jmap+oauthbearer://geoff@907.life@api.fastmail.com/.well-known/jmap"
credential-cmd = "echo test-token"
copy-to = "Sent"
folders-sort = ["Inbox", "Sent", "Archive"]
params = {cache-state = "true", cache-blobs = "true"}
`,
			wantN: 1,
		},
		{
			name: "multiple accounts",
			toml: `[[account]]
name = "Work"
backend = "jmap"
source = "jmap://user@work.com@jmap.work.com"
credential-cmd = "echo work-pass"

[[account]]
name = "Personal"
backend = "imap"
source = "imaps://user@personal.com@imap.personal.com:993"
credential-cmd = "echo personal-pass"
`,
			wantN: 2,
		},
		{
			name:    "missing name",
			toml:    "[[account]]\nbackend = \"jmap\"\nsource = \"jmap://x@y\"\n",
			wantErr: "account 0: name is required",
		},
		{
			name:    "missing source",
			toml:    "[[account]]\nname = \"Test\"\nbackend = \"jmap\"\n",
			wantErr: "account \"Test\": source is required",
		},
		{
			name:    "empty file",
			toml:    "",
			wantErr: "no accounts defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "accounts.toml")
			if err := os.WriteFile(path, []byte(tt.toml), 0644); err != nil {
				t.Fatal(err)
			}
			accounts, err := ParseAccounts(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(accounts) != tt.wantN {
				t.Fatalf("expected %d accounts, got %d", tt.wantN, len(accounts))
			}
		})
	}
}

func TestParseAccountsCredentialInjection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	toml := `[[account]]
name = "Test"
backend = "jmap"
source = "jmap+oauthbearer://user@example.com@api.example.com/.well-known/jmap"
credential-cmd = "echo secret-token"
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	accounts, err := ParseAccounts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	// Source URL should now contain the credential
	if !strings.Contains(accounts[0].Source, "secret-token") {
		t.Errorf("expected source to contain credential, got %q", accounts[0].Source)
	}
}

func TestParseAccountsFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	toml := `[[account]]
name = "Fastmail"
backend = "jmap"
source = "jmap://user@fm.com@api.fm.com"
credential-cmd = "echo pass"
copy-to = "Sent"
folders-sort = ["Inbox", "Sent"]
from = "Test User <test@fm.com>"
params = {cache-state = "true"}
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	accounts, err := ParseAccounts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := accounts[0]
	if a.Name != "Fastmail" {
		t.Errorf("name = %q, want %q", a.Name, "Fastmail")
	}
	if a.Backend != "jmap" {
		t.Errorf("backend = %q, want %q", a.Backend, "jmap")
	}
	if len(a.CopyTo) != 1 || a.CopyTo[0] != "Sent" {
		t.Errorf("copy-to = %v, want [Sent]", a.CopyTo)
	}
	if len(a.Folders) != 2 || a.Folders[0] != "Inbox" {
		t.Errorf("folders = %v, want [Inbox Sent]", a.Folders)
	}
	if a.Params["cache-state"] != "true" {
		t.Errorf("params[cache-state] = %q, want %q", a.Params["cache-state"], "true")
	}
	if a.From == nil || a.From.Address != "test@fm.com" {
		t.Errorf("from = %v, want test@fm.com", a.From)
	}
}
