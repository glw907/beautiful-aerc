// SPDX-License-Identifier: MIT

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

func TestResolveEnv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		envKey  string
		envVal  string
		want    string
		wantErr string
	}{
		{
			name:  "literal password unchanged",
			input: "s3cr3t",
			want:  "s3cr3t",
		},
		{
			name:   "dollar-var resolves when set",
			input:  "$MY_TOKEN",
			envKey: "MY_TOKEN",
			envVal: "tok-abc123",
			want:   "tok-abc123",
		},
		{
			name:    "unset dollar-var errors",
			input:   "$MISSING_VAR",
			wantErr: "env var MISSING_VAR is empty or unset",
		},
		{
			name:  "bare dollar unchanged",
			input: "$",
			want:  "$",
		},
		{
			name:  "digit-leading name unchanged",
			input: "$1abc",
			want:  "$1abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}
			got, err := resolveEnv(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAccountsEnvPassword(t *testing.T) {
	t.Setenv("TEST_PASS_TOKEN", "live-token-xyz")

	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	toml := `[[account]]
name = "Fastmail"
backend = "jmap"
source = "jmap+oauthbearer://user@example.com@api.example.com/.well-known/jmap"
password = "$TEST_PASS_TOKEN"
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
	if accounts[0].Password != "live-token-xyz" {
		t.Errorf("Password = %q, want %q", accounts[0].Password, "live-token-xyz")
	}
}

func TestParseAccountsEnvPasswordUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	toml := `[[account]]
name = "Fastmail"
backend = "jmap"
source = "jmap+oauthbearer://user@example.com@api.example.com/.well-known/jmap"
password = "$DEFINITELY_UNSET_VAR_XYZ"
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseAccounts(path)
	if err == nil {
		t.Fatal("expected error for unset env var, got nil")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_UNSET_VAR_XYZ") {
		t.Errorf("expected error to mention var name, got %q", err.Error())
	}
}

func TestParseAccountsResolvesYahooPreset(t *testing.T) {
	t.Setenv("YAHOO_APP_PASSWORD", "secret-app-pw")
	toml := `
[[account]]
name     = "personal"
backend  = "yahoo"
email    = "user@yahoo.com"
auth     = "plain"
password = "$YAHOO_APP_PASSWORD"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	a := got[0]
	if a.Backend != "imap" {
		t.Errorf("Backend = %q, want imap (preset should resolve)", a.Backend)
	}
	if a.Host != "imap.mail.yahoo.com" {
		t.Errorf("Host = %q, want imap.mail.yahoo.com", a.Host)
	}
	if a.Port != 993 {
		t.Errorf("Port = %d, want 993", a.Port)
	}
	if a.Email != "user@yahoo.com" {
		t.Errorf("Email = %q, want user@yahoo.com", a.Email)
	}
	if a.Auth != "plain" {
		t.Errorf("Auth = %q, want plain", a.Auth)
	}
	if a.Password != "secret-app-pw" {
		t.Errorf("Password = %q, want resolved env value", a.Password)
	}
}

func TestParseAccountsExplicitImap(t *testing.T) {
	t.Setenv("IMAP_PASS", "raw-pw")
	toml := `
[[account]]
name     = "self"
backend  = "imap"
email    = "u@example.com"
host     = "mail.example.com"
port     = 143
starttls = true
auth     = "plain"
password = "$IMAP_PASS"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.Host != "mail.example.com" || a.Port != 143 || !a.StartTLS {
		t.Errorf("transport mis-set: %+v", a)
	}
}

func TestParseAccountsResolvesFastmailPreset(t *testing.T) {
	t.Setenv("FASTMAIL_TOKEN", "tok")
	toml := `
[[account]]
name     = "fm"
backend  = "fastmail"
email    = "u@fastmail.com"
auth     = "bearer"
password = "$FASTMAIL_TOKEN"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.Backend != "jmap" {
		t.Errorf("Backend = %q, want jmap", a.Backend)
	}
	if a.Source != "https://api.fastmail.com/jmap/session" {
		t.Errorf("Source = %q, want preset URL", a.Source)
	}
}

func TestParseAccountsOAuthFieldsResolved(t *testing.T) {
	t.Setenv("OA_CID", "the-client-id")
	t.Setenv("OA_CS", "the-client-secret")
	t.Setenv("OA_RT", "the-refresh-token")
	toml := `
[[account]]
name                = "wk"
backend             = "imap"
email               = "u@example.com"
host                = "imap.example.com"
port                = 993
auth                = "xoauth2"
oauth-client-id     = "$OA_CID"
oauth-client-secret = "$OA_CS"
oauth-refresh-token = "$OA_RT"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.OAuthClientID != "the-client-id" {
		t.Errorf("OAuthClientID = %q", a.OAuthClientID)
	}
	if a.OAuthClientSecret != "the-client-secret" {
		t.Errorf("OAuthClientSecret = %q", a.OAuthClientSecret)
	}
	if a.OAuthRefreshToken != "the-refresh-token" {
		t.Errorf("OAuthRefreshToken = %q", a.OAuthRefreshToken)
	}
}

func TestExampleConfigParses(t *testing.T) {
	const example = `[[account]]
name = "Example"
backend = "jmap"
source = "jmap+oauthbearer://you@example.com@api.example.com/.well-known/jmap"
credential-cmd = "echo token"

[ui]
threading = true

[ui.folders.Inbox]
# rank = 0

[ui.folders.Drafts]
[ui.folders.Sent]
[ui.folders.Archive]
[ui.folders.Spam]
[ui.folders.Trash]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	if err := os.WriteFile(path, []byte(example), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccounts(path); err != nil {
		t.Fatalf("ParseAccounts: %v", err)
	}
	if _, err := LoadUI(path); err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
}
