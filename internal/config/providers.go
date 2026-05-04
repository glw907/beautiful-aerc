// SPDX-License-Identifier: MIT

package config

// Provider is a built-in account preset that fills in protocol,
// host/port (IMAP), or session URL (JMAP) so users don't have to
// look those up. Auth is still supplied per-account in config.toml.
type Provider struct {
	Backend     string // "imap" or "jmap"
	Host        string // IMAP presets only
	Port        int
	StartTLS    bool
	InsecureTLS bool // true only for self-signed-cert presets (self-hosted)
	GmailQuirks bool // X-GM-EXT-1 + Trash-precondition-on-EXPUNGE
	URL         string // JMAP presets only
}

// Providers maps preset name → Provider. Adding a new well-known
// service is one struct literal.
var Providers = map[string]Provider{
	"fastmail": {
		Backend: "jmap",
		URL:     "https://api.fastmail.com/jmap/session",
	},
	"yahoo": {
		Backend: "imap",
		Host:    "imap.mail.yahoo.com",
		Port:    993,
	},
	"icloud": {
		Backend: "imap",
		Host:    "imap.mail.me.com",
		Port:    993,
	},
	"zoho": {
		Backend: "imap",
		Host:    "imap.zoho.com",
		Port:    993,
	},
	"outlook": {
		Backend: "imap",
		Host:    "outlook.office365.com",
		Port:    993,
	},
	"mailbox-org": {
		Backend: "imap",
		Host:    "imap.mailbox.org",
		Port:    993,
	},
	"posteo": {
		Backend: "imap",
		Host:    "posteo.de",
		Port:    993,
	},
	"runbox": {
		Backend: "imap",
		Host:    "mail.runbox.com",
		Port:    993,
	},
	"gmx": {
		Backend: "imap",
		Host:    "imap.gmx.com",
		Port:    993,
	},
	"gmail": {
		Backend:     "imap",
		Host:        "imap.gmail.com",
		Port:        993,
		GmailQuirks: true,
	},
	"protonmail": {
		Backend:     "imap",
		Host:        "127.0.0.1",
		Port:        1143,
		StartTLS:    true,
		InsecureTLS: true,
	},
}
