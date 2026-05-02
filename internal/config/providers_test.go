// SPDX-License-Identifier: MIT

package config

import "testing"

func TestProtonmailPresetSetsInsecureTLS(t *testing.T) {
	p := Provider{Name: "test", InsecureTLS: true}
	if !p.InsecureTLS {
		t.Errorf("InsecureTLS field missing or unread")
	}
}

func TestProvidersV1Roster(t *testing.T) {
	expected := []string{
		"fastmail", "icloud", "yahoo", "zoho",
		"outlook", "mailbox-org", "posteo", "runbox", "gmx",
		"protonmail",
	}
	for _, name := range expected {
		if _, ok := Providers[name]; !ok {
			t.Errorf("Providers[%q] missing", name)
		}
	}
}

func TestProtonmailPresetShape(t *testing.T) {
	p, ok := Providers["protonmail"]
	if !ok {
		t.Fatal("protonmail preset missing")
	}
	if p.Backend != "imap" {
		t.Errorf("Backend = %q, want imap", p.Backend)
	}
	if p.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", p.Host)
	}
	if p.Port != 1143 {
		t.Errorf("Port = %d, want 1143", p.Port)
	}
	if !p.StartTLS {
		t.Errorf("StartTLS = false, want true")
	}
	if !p.InsecureTLS {
		t.Errorf("InsecureTLS = false, want true (Bridge uses self-signed)")
	}
}

func TestOutlookPresetShape(t *testing.T) {
	p, ok := Providers["outlook"]
	if !ok {
		t.Fatal("outlook preset missing")
	}
	if p.Backend != "imap" {
		t.Errorf("Backend = %q, want imap", p.Backend)
	}
	if p.Host != "outlook.office365.com" {
		t.Errorf("Host = %q, want outlook.office365.com", p.Host)
	}
	if p.Port != 993 {
		t.Errorf("Port = %d, want 993", p.Port)
	}
	if p.AuthHint != "xoauth2" {
		t.Errorf("AuthHint = %q, want xoauth2", p.AuthHint)
	}
}

func TestProviderRegistryLookup(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantOK   bool
		wantHost string
		wantURL  string
	}{
		{"fastmail preset", "fastmail", true, "", "https://api.fastmail.com/jmap/session"},
		{"yahoo preset", "yahoo", true, "imap.mail.yahoo.com", ""},
		{"icloud preset", "icloud", true, "imap.mail.me.com", ""},
		{"zoho preset", "zoho", true, "imap.zoho.com", ""},
		{"unknown preset", "nonesuch", false, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := LookupProvider(tc.key)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if p.Host != tc.wantHost {
				t.Errorf("host = %q, want %q", p.Host, tc.wantHost)
			}
			if p.URL != tc.wantURL {
				t.Errorf("url = %q, want %q", p.URL, tc.wantURL)
			}
		})
	}
}
