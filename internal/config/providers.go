package config

// Provider is a built-in account preset filling in protocol,
// host/port (IMAP), or session URL (JMAP). Auth is still supplied
// per-account in config.toml.
//
// SMTP fields fill the [account.smtp] block at decode time when the
// user hasn't set them. JMAP presets ignore these because submission
// rides the JMAP session.
type Provider struct {
	Backend     string
	Host        string
	Port        int
	StartTLS    bool
	InsecureTLS bool // self-signed-cert presets only
	GmailQuirks bool // X-GM-EXT-1 + Trash-precondition-on-EXPUNGE
	URL         string

	SMTPHost        string
	SMTPPort        int
	SMTPStartTLS    bool
	SMTPInsecureTLS bool
}

// Providers maps preset name to Provider. Adding a new well-known
// service is one struct literal.
var Providers = map[string]Provider{
	"fastmail": {
		Backend:  "jmap",
		URL:      "https://api.fastmail.com/jmap/session",
		SMTPHost: "smtp.fastmail.com",
		SMTPPort: 465,
	},
	"yahoo": {
		Backend:  "imap",
		Host:     "imap.mail.yahoo.com",
		Port:     993,
		SMTPHost: "smtp.mail.yahoo.com",
		SMTPPort: 465,
	},
	"icloud": {
		Backend:      "imap",
		Host:         "imap.mail.me.com",
		Port:         993,
		SMTPHost:     "smtp.mail.me.com",
		SMTPPort:     587,
		SMTPStartTLS: true,
	},
	"zoho": {
		Backend:  "imap",
		Host:     "imap.zoho.com",
		Port:     993,
		SMTPHost: "smtp.zoho.com",
		SMTPPort: 465,
	},
	"outlook": {
		Backend:      "imap",
		Host:         "outlook.office365.com",
		Port:         993,
		SMTPHost:     "smtp.office365.com",
		SMTPPort:     587,
		SMTPStartTLS: true,
	},
	"mailbox-org": {
		Backend:  "imap",
		Host:     "imap.mailbox.org",
		Port:     993,
		SMTPHost: "smtp.mailbox.org",
		SMTPPort: 465,
	},
	"posteo": {
		Backend:  "imap",
		Host:     "posteo.de",
		Port:     993,
		SMTPHost: "posteo.de",
		SMTPPort: 465,
	},
	"runbox": {
		Backend:  "imap",
		Host:     "mail.runbox.com",
		Port:     993,
		SMTPHost: "mail.runbox.com",
		SMTPPort: 465,
	},
	"gmx": {
		Backend:  "imap",
		Host:     "imap.gmx.com",
		Port:     993,
		SMTPHost: "mail.gmx.com",
		SMTPPort: 465,
	},
	"gmail": {
		Backend:     "imap",
		Host:        "imap.gmail.com",
		Port:        993,
		GmailQuirks: true,
		SMTPHost:    "smtp.gmail.com",
		SMTPPort:    465,
	},
	"protonmail": {
		Backend:         "imap",
		Host:            "127.0.0.1",
		Port:            1143,
		StartTLS:        true,
		InsecureTLS:     true,
		SMTPHost:        "127.0.0.1",
		SMTPPort:        1025,
		SMTPStartTLS:    true,
		SMTPInsecureTLS: true,
	},
}
