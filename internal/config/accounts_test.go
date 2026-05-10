package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAccountContactsDecode(t *testing.T) {
	const src = `
[[account]]
name        = "personal"
provider    = "fastmail"
password    = "stub"

[account.contacts]
url                 = "https://carddav.fastmail.com/dav/addressbooks/user/me/"
username            = "me@example.com"
password-cmd        = "cmd"
default-addressbook = "Default"
refresh-interval    = "10m"
`
	got, err := ParseAccountsFromBytes([]byte(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c := got[0].Contacts
	if c == nil {
		t.Fatal("Contacts nil")
	}
	if c.URL != "https://carddav.fastmail.com/dav/addressbooks/user/me/" {
		t.Errorf("URL = %q", c.URL)
	}
	if c.DefaultAddressbook != "Default" {
		t.Errorf("DefaultAddressbook = %q", c.DefaultAddressbook)
	}
	if c.RefreshInterval != 10*time.Minute {
		t.Errorf("RefreshInterval = %v", c.RefreshInterval)
	}
}

func TestContactsConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		c    ContactsConfig
		want string // substring of error; "" = ok
	}{
		{"empty url", ContactsConfig{}, "url"},
		{"non-https", ContactsConfig{URL: "ftp://x"}, "url"},
		{"http allowed when insecure-tls", ContactsConfig{URL: "http://x.local/", InsecureTLS: true}, ""},
		{"refresh too small", ContactsConfig{URL: "https://x/", RefreshInterval: 30 * time.Second}, "refresh-interval"},
		{"both password and cmd", ContactsConfig{URL: "https://x/", Password: "a", PasswordCmd: "b"}, "password"},
		{"defaults", ContactsConfig{URL: "https://x/"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.validate()
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q; got %v", tc.want, err)
			}
		})
	}
}

func TestContactsCredentialFallback(t *testing.T) {
	const src = `
[[account]]
name         = "fm"
provider     = "fastmail"
email        = "me@example.com"
password-cmd = "echo parent-secret"

[account.contacts]
url = "https://carddav.fastmail.com/"
`
	got, err := ParseAccountsFromBytes([]byte(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c := got[0].Contacts
	if c == nil {
		t.Fatal("Contacts nil")
	}
	if c.Username != "me@example.com" {
		t.Errorf("Username fallback = %q, want parent email", c.Username)
	}
	if c.PasswordCmd != "echo parent-secret" {
		t.Errorf("PasswordCmd fallback = %q, want parent value", c.PasswordCmd)
	}
}

func TestContactsNoBlockNil(t *testing.T) {
	const src = `
[[account]]
name     = "fm"
provider = "fastmail"
password = "tok"
`
	got, err := ParseAccountsFromBytes([]byte(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0].Contacts != nil {
		t.Errorf("Contacts = %+v, want nil", got[0].Contacts)
	}
}

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
provider = "jmap"
source = "jmap+oauthbearer://geoff@907.life@api.fastmail.com/.well-known/jmap"
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
provider = "jmap"
source = "jmap://user@work.com@jmap.work.com"

[[account]]
name = "Personal"
provider = "imap"
host = "imap.personal.com"
port = 993

[account.smtp]
host = "smtp.personal.com"
port = 465
`,
			wantN: 2,
		},
		{
			name:    "missing name and email",
			toml:    "[[account]]\nprovider = \"jmap\"\nsource = \"jmap://x@y\"\n",
			wantErr: "name is required when email is also blank",
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
			path := filepath.Join(dir, "config.toml")
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

func TestParseAccounts_NameDefaultsToEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	src := `[[account]]
provider = "fastmail"
email    = "you@example.com"
password = "x"
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	accounts, err := ParseAccounts(path)
	if err != nil {
		t.Fatalf("ParseAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(accounts))
	}
	if accounts[0].Name != "you@example.com" {
		t.Errorf("Name = %q, want %q (defaulted from email)", accounts[0].Name, "you@example.com")
	}
}

func TestParseAccountsFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	toml := `[[account]]
name = "Fastmail"
provider = "jmap"
source = "jmap://user@fm.com@api.fm.com"
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
	path := filepath.Join(dir, "config.toml")
	toml := `[[account]]
name = "Fastmail"
provider = "jmap"
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
	path := filepath.Join(dir, "config.toml")
	toml := `[[account]]
name = "Fastmail"
provider = "jmap"
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
provider  = "yahoo"
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
provider  = "imap"
email    = "u@example.com"
host     = "mail.example.com"
port     = 143
starttls = true
auth     = "plain"
password = "$IMAP_PASS"

[account.smtp]
host = "smtp.example.com"
port = 465
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
provider  = "fastmail"
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

func TestParseAccountsPresetSetsInsecureTLS(t *testing.T) {
	Providers["test-insecure"] = Provider{
		Backend:     "imap",
		Host:        "test.local",
		Port:        993,
		InsecureTLS: true,
		SMTPHost:    "smtp.test.local",
		SMTPPort:    465,
	}
	t.Cleanup(func() { delete(Providers, "test-insecure") })

	toml := `
[[account]]
name     = "t"
provider = "test-insecure"
email    = "u@test.local"
auth     = "plain"
password = "pw"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got[0].InsecureTLS {
		t.Errorf("InsecureTLS = false, want true (from preset)")
	}
}

func TestParseAccountsPasswordCmdField(t *testing.T) {
	toml := `
[[account]]
name         = "p"
provider     = "fastmail"
email        = "u@fm.com"
password-cmd = "echo secret"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].PasswordCmd != "echo secret" {
		t.Errorf("PasswordCmd = %q, want \"echo secret\"", got[0].PasswordCmd)
	}
	if got[0].Password != "" {
		t.Errorf("Password should be empty (resolution deferred)")
	}
}

func TestParseAccountsRejectsBothPasswordAndCmd(t *testing.T) {
	t.Setenv("PW", "x")
	toml := `
[[account]]
name         = "p"
provider     = "fastmail"
email        = "u@fm.com"
password     = "$PW"
password-cmd = "echo secret"
`
	_, err := ParseAccountsFromBytes([]byte(toml))
	if err == nil {
		t.Fatalf("expected error for both password and password-cmd set")
	}
	if !strings.Contains(err.Error(), "password and password-cmd") {
		t.Errorf("error %q should mention both fields", err)
	}
}

func TestParseAccountsErrorsAreFriendly(t *testing.T) {
	cases := []struct {
		name     string
		toml     string
		wantSubs []string
	}{
		{
			"unknown provider",
			`[[account]]
name = "p"
provider = "yahho"
email = "u@y.com"
password = "x"
`,
			[]string{`account "p"`, "unknown provider", `"yahho"`},
		},
		{
			"both password fields",
			`[[account]]
name = "p"
provider = "fastmail"
email = "u@fm.com"
password = "x"
password-cmd = "echo y"
`,
			[]string{`account "p"`, "password", "password-cmd"},
		},
		{
			"missing host for imap",
			`[[account]]
name = "p"
provider = "imap"
email = "u@x.com"
password = "x"
`,
			[]string{`account "p"`, "host", "imap"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseAccountsFromBytes([]byte(tc.toml))
			if err == nil {
				t.Fatal("expected error")
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q missing substring %q", err.Error(), sub)
				}
			}
		})
	}
}

func TestUnknownProviderIncludesSuggestion(t *testing.T) {
	toml := `[[account]]
name = "p"
provider = "yahho"
email = "u@y.com"
password = "x"
`
	_, err := ParseAccountsFromBytes([]byte(toml))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `did you mean "yahoo"`) {
		t.Errorf("error %q missing 'did you mean yahoo' suggestion", err)
	}
}

func TestParseAccounts_GmailPresetCopiesQuirks(t *testing.T) {
	const cfg = `
[[account]]
name        = "g"
provider    = "gmail"
email       = "me@example.com"
password    = "tok"
`
	accts, err := ParseAccountsFromBytes([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	if len(accts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accts))
	}
	a := accts[0]
	if a.Backend != "imap" {
		t.Errorf("Backend = %q, want imap", a.Backend)
	}
	if a.Host != "imap.gmail.com" {
		t.Errorf("Host = %q, want imap.gmail.com", a.Host)
	}
	if !a.GmailQuirks {
		t.Errorf("GmailQuirks = false, want true")
	}
}

func TestParseAccountsIdentitiesDecode(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		want    []Identity
		wantErr string
	}{
		{
			name: "two identities ordered, with mixed text and file signatures",
			toml: `
[[account]]
name = "fastmail"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "Geoff Wright"
email = "geoff@907.life"

[[account.identity.signature]]
name = "default"
text = "Geoff Wright\nhttps://907.life"

[[account.identity]]
name = "Geoff @ ASC"
email = "geoff.wright@aksailingclub.org"

[[account.identity.signature]]
name = "casual"
text = "Geoff"
`,
			want: []Identity{
				{
					Name:  "Geoff Wright",
					Email: "geoff@907.life",
					Signatures: []Signature{
						{Name: "default", Text: "-- \nGeoff Wright\nhttps://907.life"},
					},
				},
				{
					Name:  "Geoff @ ASC",
					Email: "geoff.wright@aksailingclub.org",
					Signatures: []Signature{
						{Name: "casual", Text: "-- \nGeoff"},
					},
				},
			},
		},
		{
			name: "identity with zero signatures decodes",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"
`,
			want: []Identity{{Name: "G", Email: "g@x"}},
		},
		{
			name: "text and file mutually exclusive",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
text = "y"
file = "/tmp/z"
`,
			wantErr: `signature "x": text and file are mutually exclusive`,
		},
		{
			name: "signature missing both text and file",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
`,
			wantErr: `signature "x": text or file is required`,
		},
		{
			name: "duplicate signature name within identity",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "dup"
text = "a"

[[account.identity.signature]]
name = "dup"
text = "b"
`,
			wantErr: `duplicate signature name "dup"`,
		},
		{
			name: "preserves existing -- \\n sentinel",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
text = "-- \nalready sentineled"
`,
			want: []Identity{{
				Name:  "G",
				Email: "g@x",
				Signatures: []Signature{
					{Name: "x", Text: "-- \nalready sentineled"},
				},
			}},
		},
		{
			name: "invalid identity email",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "not-an-address"
`,
			wantErr: `identity "G": email`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAccountsFromBytes([]byte(tt.toml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("got nil error, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAccountsFromBytes: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d accounts, want 1", len(got))
			}
			if !reflect.DeepEqual(got[0].Identities, tt.want) {
				t.Errorf("identities mismatch\ngot:  %#v\nwant: %#v", got[0].Identities, tt.want)
			}
		})
	}
}

func TestParseAccountsSignatureFile(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "sig.md")
	if err := os.WriteFile(sigPath, []byte("Geoff\nhttps://907.life"), 0o600); err != nil {
		t.Fatalf("write sig: %v", err)
	}
	toml := fmt.Sprintf(`
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "default"
file = %q
`, sigPath)
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	want := "-- \nGeoff\nhttps://907.life"
	if got[0].Identities[0].Signatures[0].Text != want {
		t.Errorf("text = %q, want %q", got[0].Identities[0].Signatures[0].Text, want)
	}
}

func TestParseAccountsLegacyFromSynthesis(t *testing.T) {
	toml := `
[[account]]
name = "a"
provider = "fastmail"
password = "x"
from = "Geoff <geoff@907.life>"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	want := []Identity{{Name: "Geoff", Email: "geoff@907.life"}}
	if !reflect.DeepEqual(got[0].Identities, want) {
		t.Errorf("identities = %#v, want %#v", got[0].Identities, want)
	}
}

func TestExampleConfigParses(t *testing.T) {
	const example = `[[account]]
name = "Example"
provider = "jmap"
source = "jmap+oauthbearer://you@example.com@api.example.com/.well-known/jmap"

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
	path := filepath.Join(dir, "config.toml")
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
