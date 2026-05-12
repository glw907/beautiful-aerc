package config

// OAuthDefaults holds the OAuth 2.0 authorization and token endpoint
// URLs for a built-in provider preset. Scopes is the recommended
// minimal set; callers may append provider-specific additions.
type OAuthDefaults struct {
	AuthURL       string
	TokenURL      string
	DeviceAuthURL string // RFC 8628 endpoint; empty when device-code is unsupported
	Scopes        []string
}

// CredentialStrategy names how a provider expects callers to obtain
// the credential: an app-specific password, an API token, an OAuth
// flow, or hand-entered IMAP/JMAP creds for self-hosted servers.
type CredentialStrategy string

const (
	StrategyAppPassword CredentialStrategy = "app-password"
	StrategyAPIToken    CredentialStrategy = "api-token"
	StrategyOAuth       CredentialStrategy = "oauth"
	StrategyPlainIMAP   CredentialStrategy = "plain-imap"
	StrategyPlainJMAP   CredentialStrategy = "plain-jmap"
)

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

	// CredentialStrategy controls which credential flow the setup step
	// uses. An empty value falls back to StrategyAppPassword.
	CredentialStrategy CredentialStrategy

	// HelpURL points at the provider's app-password or token-management
	// page. Empty when no canonical URL exists.
	HelpURL string

	// OAuth carries the provider's OIDC/OAuth 2.0 endpoint defaults.
	// Nil for non-OAuth presets. When non-nil, accounts using this
	// preset must supply an [account.oauth] block.
	OAuth *OAuthDefaults

	// OAuthRequiresSecret reports whether the provider's OAuth app
	// registration must include a client secret. Public-client flows
	// (PKCE-only) leave this false.
	OAuthRequiresSecret bool
}

// Providers maps preset name to Provider. Adding a new well-known
// service is one struct literal.
var Providers = map[string]Provider{
	"fastmail": {
		Backend:            "jmap",
		URL:                "https://api.fastmail.com/jmap/session",
		SMTPHost:           "smtp.fastmail.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyAPIToken,
		HelpURL:            "https://app.fastmail.com/settings/security/tokens",
	},
	"yahoo": {
		Backend:            "imap",
		Host:               "imap.mail.yahoo.com",
		Port:               993,
		SMTPHost:           "smtp.mail.yahoo.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://login.yahoo.com/account/security",
	},
	"icloud": {
		Backend:            "imap",
		Host:               "imap.mail.me.com",
		Port:               993,
		SMTPHost:           "smtp.mail.me.com",
		SMTPPort:           587,
		SMTPStartTLS:       true,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://appleid.apple.com/account/manage",
	},
	"zoho": {
		Backend:            "imap",
		Host:               "imap.zoho.com",
		Port:               993,
		SMTPHost:           "smtp.zoho.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://accounts.zoho.com/home#security/app_password",
	},
	"outlook": {
		Backend:            "imap",
		Host:               "outlook.office365.com",
		Port:               993,
		SMTPHost:           "smtp.office365.com",
		SMTPPort:           587,
		SMTPStartTLS:       true,
		CredentialStrategy: StrategyOAuth,
		HelpURL:            "https://account.microsoft.com/security",
		OAuth: &OAuthDefaults{
			AuthURL:       "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			DeviceAuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode",
			Scopes:        []string{"https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com/SMTP.Send", "offline_access"},
		},
		OAuthRequiresSecret: true,
	},
	"mailbox-org": {
		Backend:            "imap",
		Host:               "imap.mailbox.org",
		Port:               993,
		SMTPHost:           "smtp.mailbox.org",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://kb.mailbox.org/en/private/security/two-factor-authentication-and-app-passwords",
	},
	"posteo": {
		Backend:            "imap",
		Host:               "posteo.de",
		Port:               993,
		SMTPHost:           "posteo.de",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://posteo.de/en/help/setting-up-app-passwords",
	},
	"runbox": {
		Backend:            "imap",
		Host:               "mail.runbox.com",
		Port:               993,
		SMTPHost:           "mail.runbox.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://help.runbox.com/imap-and-smtp-configuration/",
	},
	"gmx": {
		Backend:            "imap",
		Host:               "imap.gmx.com",
		Port:               993,
		SMTPHost:           "mail.gmx.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://www.gmx.com/mail/imap-pop3-settings/",
	},
	"gmail": {
		Backend:            "imap",
		Host:               "imap.gmail.com",
		Port:               993,
		GmailQuirks:        true,
		SMTPHost:           "smtp.gmail.com",
		SMTPPort:           465,
		CredentialStrategy: StrategyOAuth,
		HelpURL:            "https://myaccount.google.com/apppasswords",
		OAuth: &OAuthDefaults{
			AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:      "https://oauth2.googleapis.com/token",
			DeviceAuthURL: "https://oauth2.googleapis.com/device/code",
			Scopes:        []string{"https://mail.google.com/"},
		},
		OAuthRequiresSecret: true,
	},
	"protonmail": {
		Backend:            "imap",
		Host:               "127.0.0.1",
		Port:               1143,
		StartTLS:           true,
		InsecureTLS:        true,
		SMTPHost:           "127.0.0.1",
		SMTPPort:           1025,
		SMTPStartTLS:       true,
		SMTPInsecureTLS:    true,
		CredentialStrategy: StrategyAppPassword,
		HelpURL:            "https://proton.me/support/protonmail-bridge",
	},
}
