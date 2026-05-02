// SPDX-License-Identifier: MIT

package config

// Provider is a built-in account preset that fills in protocol,
// host/port (IMAP), or session URL (JMAP) so users don't have to
// look those up. Auth is still supplied per-account in config.toml.
type Provider struct {
	Name        string
	Backend     string // "imap" or "jmap"
	Host        string // IMAP presets only
	Port        int
	StartTLS    bool
	InsecureTLS bool   // true only for self-signed-cert presets (self-hosted)
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
	"outlook": {
		Name:     "outlook",
		Backend:  "imap",
		Host:     "outlook.office365.com",
		Port:     993,
		AuthHint: "xoauth2",
		HelpURL:  "https://account.live.com/proofs/AppPassword",
	},
	"mailbox-org": {
		Name:     "mailbox-org",
		Backend:  "imap",
		Host:     "imap.mailbox.org",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://account.mailbox.org/",
	},
	"posteo": {
		Name:     "posteo",
		Backend:  "imap",
		Host:     "posteo.de",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://posteo.de/en/help/",
	},
	"runbox": {
		Name:     "runbox",
		Backend:  "imap",
		Host:     "mail.runbox.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://help.runbox.com/",
	},
	"gmx": {
		Name:     "gmx",
		Backend:  "imap",
		Host:     "imap.gmx.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://www.gmx.com/mail/",
	},
	"protonmail": {
		Name:        "protonmail",
		Backend:     "imap",
		Host:        "127.0.0.1",
		Port:        1143,
		StartTLS:    true,
		InsecureTLS: true,
		AuthHint:    "bridge-password",
		HelpURL:     "https://proton.me/mail/bridge",
	},
}

// LookupProvider returns the Provider for key and true if known.
func LookupProvider(key string) (Provider, bool) {
	p, ok := Providers[key]
	return p, ok
}
