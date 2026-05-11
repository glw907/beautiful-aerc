package config

import (
	"errors"
	"strings"
	"testing"
)

func TestStrictDecode_UnknownKey(t *testing.T) {
	data := []byte(`
[[account]]
name = "geoff"
provider = "fastmail"
email = "geoff@example.com"
passwrd = "hunter2"
`)
	_, err := ParseAccountsFromBytes(data)
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("err = %v, want ErrConfigInvalid family", err)
	}
	if !strings.Contains(err.Error(), "passwrd") {
		t.Errorf("err = %v, want it to mention 'passwrd'", err)
	}
	if !strings.Contains(err.Error(), `"password"`) {
		t.Errorf("err = %v, want Levenshtein suggestion of 'password'", err)
	}
}

func TestStrictDecode_AcceptsParamsMap(t *testing.T) {
	data := []byte(`
[[account]]
name = "geoff"
provider = "fastmail"
email = "geoff@example.com"
password = "hunter2"

[account.params]
arbitrary = "value"
another = "thing"
`)
	if _, err := ParseAccountsFromBytes(data); err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
}

func TestValidateAuth(t *testing.T) {
	tests := []struct {
		v     string
		valid bool
	}{
		{"", true},
		{"plain", true},
		{"xoauth2", true},
		{"bearer", true},
		{"cleartext", false},
		{"PLAIN", false},
	}
	for _, tt := range tests {
		err := validateAuth("a", "auth", tt.v)
		if (err == nil) != tt.valid {
			t.Errorf("validateAuth(%q) err = %v, want valid=%v", tt.v, err, tt.valid)
		}
	}
}

func TestValidateOAuthStore(t *testing.T) {
	for _, v := range []string{"", "keyring", "age-file"} {
		if err := validateOAuthStore("a", v); err != nil {
			t.Errorf("validateOAuthStore(%q) = %v, want nil", v, err)
		}
	}
	if err := validateOAuthStore("a", "nope"); err == nil {
		t.Errorf("validateOAuthStore(\"nope\") = nil, want error")
	}
}

func TestParseAccounts_BareImapRequiresPort(t *testing.T) {
	data := []byte(`
[[account]]
name = "self"
provider = "imap"
email = "u@example.com"
host = "mail.example.com"
password = "x"

[account.smtp]
host = "smtp.example.com"
port = 587
`)
	_, err := ParseAccountsFromBytes(data)
	if err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("want port-required error, got %v", err)
	}
}

func TestParseAccounts_ContactsRequiresCredentials(t *testing.T) {
	// Parent account has no password to inherit; contacts has only URL.
	data := []byte(`
[[account]]
name = "geoff"
provider = "fastmail"
email = "geoff@example.com"

[account.contacts]
url = "https://carddav.example.com/"
`)
	_, err := ParseAccountsFromBytes(data)
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("want password-required error, got %v", err)
	}
}

func TestContactsURLEmptyIsRequired(t *testing.T) {
	c := &ContactsConfig{URL: ""}
	err := c.validate()
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("validate(empty url) = %v, want 'required'", err)
	}
}
