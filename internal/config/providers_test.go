// SPDX-License-Identifier: MIT

package config

import "testing"

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
