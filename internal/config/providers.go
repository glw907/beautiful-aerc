// SPDX-License-Identifier: MIT

package config

// Provider is a built-in account preset that fills in protocol,
// host/port (IMAP), or session URL (JMAP) so users don't have to
// look those up. Auth is still supplied per-account in accounts.toml.
type Provider struct {
	Name        string
	Backend     string // "imap" or "jmap"
	Host        string // IMAP presets only
	Port        int
	StartTLS    bool
	URL         string // JMAP presets only
	AuthHint    string // "app-password" | "bearer" | "xoauth2"
	HelpURL     string
	GmailQuirks bool // Pass 8.1 — assert-not-negotiate + Trash precondition
}

// Providers maps preset name → Provider. Adding a new well-known
// service is one struct literal.
var Providers = map[string]Provider{
	"fastmail": {
		Name:     "fastmail",
		Backend:  "jmap",
		URL:      "https://api.fastmail.com/jmap/session",
		AuthHint: "bearer",
		HelpURL:  "https://app.fastmail.com/settings/security/tokens",
	},
	"yahoo": {
		Name:     "yahoo",
		Backend:  "imap",
		Host:     "imap.mail.yahoo.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://login.yahoo.com/account/security",
	},
	"icloud": {
		Name:     "icloud",
		Backend:  "imap",
		Host:     "imap.mail.me.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://appleid.apple.com",
	},
	"zoho": {
		Name:     "zoho",
		Backend:  "imap",
		Host:     "imap.zoho.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://accounts.zoho.com/home#security/app_password",
	},
}

// LookupProvider returns the Provider for key and true if known.
func LookupProvider(key string) (Provider, bool) {
	p, ok := Providers[key]
	return p, ok
}
