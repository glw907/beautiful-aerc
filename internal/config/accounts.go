// Package config holds poplar's configuration types and loaders.
package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"

	"github.com/glw907/poplar/internal/strdist"
)

// ResolvePassword returns the cleartext password. Inline Password
// wins. Otherwise PasswordCmd runs via /bin/sh -c and trimmed stdout
// is the password.
func (c *AccountConfig) ResolvePassword() (string, error) {
	return resolvePasswordCmd(c.Password, c.PasswordCmd, "password-cmd")
}

// resolvePasswordCmd is shared by AccountConfig and SMTPConfig.
// errPrefix labels the command in the error chain.
func resolvePasswordCmd(password, cmd, errPrefix string) (string, error) {
	if password != "" {
		return password, nil
	}
	if cmd == "" {
		return "", fmt.Errorf("no password or %s", errPrefix)
	}
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if stderr := strings.TrimSpace(string(ee.Stderr)); stderr != "" {
				return "", fmt.Errorf("%s: %s", errPrefix, stderr)
			}
		}
		return "", fmt.Errorf("%s: %v", errPrefix, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// ContactsConfig configures CardDAV ingest for one account.
// Accounts without a [account.contacts] block skip contact sync.
type ContactsConfig struct {
	URL                string        `toml:"url"`
	Username           string        `toml:"username"`
	Password           string        `toml:"password"`
	PasswordCmd        string        `toml:"password-cmd"`
	DefaultAddressbook string        `toml:"default-addressbook"`
	RefreshInterval    time.Duration `toml:"refresh-interval"`
	InsecureTLS        bool          `toml:"insecure-tls"`
}

func (c *ContactsConfig) validate() error {
	if c == nil {
		return nil
	}
	if c.URL == "" {
		return fmt.Errorf("contacts: url: required")
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("contacts: url: not parseable")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !c.InsecureTLS {
			return fmt.Errorf("contacts: url: http requires insecure-tls = true")
		}
	default:
		return fmt.Errorf("contacts: url: scheme must be https (or http with insecure-tls)")
	}
	if c.Password != "" && c.PasswordCmd != "" {
		return fmt.Errorf("contacts: set password OR password-cmd, not both")
	}
	if c.RefreshInterval == 0 {
		c.RefreshInterval = 15 * time.Minute
	} else if c.RefreshInterval < time.Minute {
		return fmt.Errorf("contacts: refresh-interval must be >= 1m")
	}
	return nil
}

// SMTPConfig is the per-account submission transport. Filled from
// provider preset SMTP fields at decode, then overridden by an
// explicit [account.smtp] block. Auth, Password, and PasswordCmd
// default to the IMAP-side credentials when unset.
type SMTPConfig struct {
	Host        string
	Port        int
	StartTLS    bool
	InsecureTLS bool
	Auth        string
	Password    string
	PasswordCmd string
}

func (s *SMTPConfig) ResolvePassword() (string, error) {
	return resolvePasswordCmd(s.Password, s.PasswordCmd, "smtp password-cmd")
}

func (c *ContactsConfig) ResolvePassword() (string, error) {
	return resolvePasswordCmd(c.Password, c.PasswordCmd, "contacts password-cmd")
}

// Identity is one of an account's sending identities. Name is the
// display name (rendered in From), Email is the address (must match
// a server-side identity for JMAP submission), Signatures is the
// ordered list the user cycles through in compose.
type Identity struct {
	Name       string
	Email      string
	Signatures []Signature
}

// Signature is one of an identity's named signatures. Text is the
// resolved body (with RFC 3676 "-- \n" sentinel guaranteed). Name
// labels it in the compose chip and footer.
type Signature struct {
	Name string
	Text string
}

type identityEntry struct {
	Name       string           `toml:"name"`
	Email      string           `toml:"email"`
	Signatures []signatureEntry `toml:"signature"`
}

type signatureEntry struct {
	Name string `toml:"name"`
	Text string `toml:"text"`
	File string `toml:"file"`
}

// OAuthConfig carries the OAuth 2.0 client registration for one account.
// Endpoint URLs may be omitted when the provider preset supplies them.
type OAuthConfig struct {
	ClientID     string   `toml:"client-id"`
	ClientSecret string   `toml:"client-secret"`
	AuthURL      string   `toml:"auth-url,omitempty"`
	TokenURL     string   `toml:"token-url,omitempty"`
	Scopes       []string `toml:"scopes,omitempty"`
}

// AccountConfig holds the configuration for a single email account.
type AccountConfig struct {
	Name string
	// Preset is the provider preset key used in TOML ("fastmail",
	// "yahoo", …). Empty when the user wrote "imap" / "jmap" directly.
	// Renderers prefer Preset over Backend so the round-tripped TOML
	// keeps the preset's host/SMTP defaults wired on next load.
	Preset         string
	Display        string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string

	// Identity
	From   *mail.Address
	CopyTo []string

	// Identities is the ordered list of sending identities. Length is
	// always >= 1: a missing [[account.identity]] block synthesizes one
	// from the legacy top-level From.
	Identities []Identity

	// Password is the bearer token or password after env-var
	// substitution. Use "$VAR_NAME" in config.toml to pull from env.
	Password string

	// PasswordCmd's stdout becomes the password. Resolution defers to
	// first Connect so secret-manager prompts fire near the action.
	// Mutually exclusive with Password.
	PasswordCmd string

	// Auth is the SASL mechanism. Empty defers to the backend default.
	// Recognized: "plain", "login", "cram-md5", "xoauth2", "bearer".
	Auth string

	// Email may be empty when the backend auto-discovers it (JMAP).
	Email string

	Host     string
	Port     int
	StartTLS bool

	// InsecureTLS skips TLS verification. Self-hosted, self-signed
	// only. Never for hosted providers.
	InsecureTLS bool

	// GmailQuirks enables Gmail-specific IMAP behavior: X-GM-EXT-1
	// assertion at Connect, and Destroy routed via SELECT
	// [Gmail]/Trash before EXPUNGE. Set by the gmail preset.
	GmailQuirks bool

	// SMTP is the submission transport for IMAP-backed accounts. JMAP
	// accounts ignore it because submission rides the JMAP session.
	SMTP SMTPConfig

	// Contacts is the CardDAV sync config. Nil when no
	// [account.contacts] block is present.
	Contacts *ContactsConfig

	// OAuth carries the OAuth 2.0 client registration. Nil for
	// non-OAuth accounts. Endpoint URLs fill from the provider preset
	// when absent from the TOML block.
	OAuth *OAuthConfig

	// OAuthStore names the token-store backend ("keyring", "age-file").
	// Written by the wizard; empty means unset.
	OAuthStore string
}

// ResolvePreset fills empty backend/transport fields on c from the
// provider preset table keyed by c.Preset. Non-empty slots win, so
// explicit user overrides survive. Idempotent. Safe to call from
// the decoder and again from the wizard before the TOML round-trip.
// Unknown preset keys are a no-op; the decoder still surfaces the
// "unknown provider" error.
func ResolvePreset(c *AccountConfig) {
	preset, ok := Providers[c.Preset]
	if !ok {
		return
	}
	c.Backend = preset.Backend
	if c.Host == "" {
		c.Host = preset.Host
	}
	if c.Port == 0 {
		c.Port = preset.Port
	}
	if !c.StartTLS {
		c.StartTLS = preset.StartTLS
	}
	if !c.InsecureTLS {
		c.InsecureTLS = preset.InsecureTLS
	}
	c.GmailQuirks = preset.GmailQuirks
	if c.Source == "" {
		c.Source = preset.URL
	}
	if c.SMTP.Host == "" {
		c.SMTP.Host = preset.SMTPHost
	}
	if c.SMTP.Port == 0 {
		c.SMTP.Port = preset.SMTPPort
	}
	if !c.SMTP.StartTLS {
		c.SMTP.StartTLS = preset.SMTPStartTLS
	}
	if !c.SMTP.InsecureTLS {
		c.SMTP.InsecureTLS = preset.SMTPInsecureTLS
	}
}

func (e *accountEntry) accountLabel(index int) string {
	if e.Name != "" {
		return e.Name
	}
	if e.Email != "" {
		return e.Email
	}
	return fmt.Sprintf("[%d]", index)
}

// ExpandHome rewrites a leading "~" to the user's home directory.
// Other paths and empty strings pass through.
func ExpandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
}

// suggestProvider returns the closest known provider within edit
// distance 2, or "".
func suggestProvider(s string) string {
	if s == "" {
		return ""
	}
	candidates := []string{"imap", "jmap"}
	for k := range Providers {
		candidates = append(candidates, k)
	}
	bestName := ""
	bestDist := 3
	for _, c := range candidates {
		d := strdist.Levenshtein(s, c, 2)
		if d < bestDist {
			bestDist = d
			bestName = c
		}
	}
	return bestName
}

type configFile struct {
	Account []accountEntry `toml:"account"`
}

type accountEntry struct {
	Name           string            `toml:"name"`
	Display        string            `toml:"display"`
	Provider       string            `toml:"provider"`
	Source         string            `toml:"source"`
	Email          string            `toml:"email"`
	Host           string            `toml:"host"`
	Port           int               `toml:"port"`
	StartTLS       bool              `toml:"starttls"`
	InsecureTLS    bool              `toml:"insecure-tls"`
	Auth           string            `toml:"auth"`
	Password       string            `toml:"password"`
	PasswordCmd    string            `toml:"password-cmd"`
	CopyTo         string            `toml:"copy-to"`
	FoldersSort    []string          `toml:"folders-sort"`
	FoldersExclude []string          `toml:"folders-exclude"`
	From           string            `toml:"from"`
	Params         map[string]string `toml:"params"`
	SMTP           smtpEntry         `toml:"smtp"`
	Contacts       *ContactsConfig   `toml:"contacts"`
	Identities     []identityEntry   `toml:"identity"`
	OAuth          *OAuthConfig      `toml:"oauth"`
	OAuthStore     string            `toml:"oauth-store"`
}

type smtpEntry struct {
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	StartTLS    bool   `toml:"starttls"`
	InsecureTLS bool   `toml:"insecure-tls"`
	Auth        string `toml:"auth"`
	Password    string `toml:"password"`
	PasswordCmd string `toml:"password-cmd"`
}

// ParseAccounts reads config.toml and returns the configured accounts
// with credentials resolved.
func ParseAccounts(path string) ([]AccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v", path, err)
	}
	return ParseAccountsFromBytes(data)
}

// ParseAccountsFromBytes is the byte-slice form of ParseAccounts.
func ParseAccountsFromBytes(data []byte) ([]AccountConfig, error) {
	var cf configFile
	if err := strictDecode(data, &cf); err != nil {
		return nil, err
	}

	if len(cf.Account) == 0 {
		return nil, fmt.Errorf("no accounts defined")
	}

	var accounts []AccountConfig
	for i, entry := range cf.Account {
		acct, err := entry.toAccountConfig(i)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acct)
	}
	return accounts, nil
}

func (e *accountEntry) toAccountConfig(index int) (*AccountConfig, error) {
	if e.Provider == "" {
		return nil, &ConfigError{
			Account: e.accountLabel(index),
			Field:   "provider",
			Message: "provider is required",
			Suggest: fmt.Sprintf(`set "provider" on the [[account]] block; known: %s`, knownProvidersList()),
		}
	}

	if e.Name == "" {
		if e.Email == "" {
			return nil, &ConfigError{
				Account: fmt.Sprintf("[%d]", index),
				Field:   "name",
				Message: "name is required when email is also blank",
				Suggest: `set "name" or "email" on the [[account]] block`,
			}
		}
		e.Name = e.Email
	}

	presetKey := ""
	if _, ok := Providers[e.Provider]; ok {
		presetKey = e.Provider
	}
	acct := &AccountConfig{
		Name:        e.Name,
		Preset:      presetKey,
		Backend:     e.Provider,
		Source:      e.Source,
		Email:       e.Email,
		Host:        e.Host,
		Port:        e.Port,
		StartTLS:    e.StartTLS,
		InsecureTLS: e.InsecureTLS,
		Auth:        e.Auth,
		PasswordCmd: e.PasswordCmd,
		SMTP: SMTPConfig{
			Host:        e.SMTP.Host,
			Port:        e.SMTP.Port,
			StartTLS:    e.SMTP.StartTLS,
			InsecureTLS: e.SMTP.InsecureTLS,
			Auth:        e.SMTP.Auth,
			Password:    e.SMTP.Password,
			PasswordCmd: e.SMTP.PasswordCmd,
		},
	}
	ResolvePreset(acct)

	if p, ok := Providers[e.Provider]; ok && p.OAuth != nil {
		if e.OAuth == nil {
			return nil, &ConfigError{
				Account: e.Name,
				Field:   "oauth",
				Message: "OAuth provider requires [account.oauth] block",
				Suggest: `add an [account.oauth] block with client-id and client-secret`,
			}
		}
		if e.OAuth.AuthURL == "" {
			e.OAuth.AuthURL = p.OAuth.AuthURL
		}
		if e.OAuth.TokenURL == "" {
			e.OAuth.TokenURL = p.OAuth.TokenURL
		}
		if len(e.OAuth.Scopes) == 0 {
			e.OAuth.Scopes = p.OAuth.Scopes
		}
		if p.OAuthRequiresSecret && e.OAuth.ClientSecret == "" {
			return nil, &ConfigError{
				Account: e.Name,
				Field:   "oauth.client-secret",
				Message: "client-secret required for this provider",
			}
		}
	}

	if e.OAuth != nil && e.OAuth.ClientID == "" {
		return nil, &ConfigError{
			Account: e.Name,
			Field:   "oauth.client-id",
			Message: "client-id required",
		}
	}

	password, err := resolveEnv(e.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) password: %w", e.Name, e.Provider, err)
	}
	if password != "" && e.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q (provider = %q): both password and password-cmd set; use one", e.Name, e.Provider)
	}
	acct.Password = password
	if e.Provider != "imap" && e.Provider != "jmap" && e.Provider != "mock" {
		if _, ok := Providers[e.Provider]; !ok {
			suggest := fmt.Sprintf("known providers: %s", knownProvidersList())
			if s := suggestProvider(e.Provider); s != "" {
				suggest = fmt.Sprintf(`did you mean "%s"? known: %s`, s, knownProvidersList())
			}
			return nil, &ConfigError{
				Account: e.Name,
				Field:   "provider",
				Message: fmt.Sprintf("unknown provider %q", e.Provider),
				Suggest: suggest,
			}
		}
	}

	if acct.Backend == "imap" && acct.Host == "" {
		return nil, &ConfigError{
			Account: e.Name,
			Field:   "host",
			Message: fmt.Sprintf("host is required for imap accounts (provider = %q)", e.Provider),
			Suggest: `set "host" on the [[account]] block, or pick a hosted preset like "fastmail"`,
		}
	}

	if acct.Backend == "imap" && acct.Port == 0 {
		return nil, &ConfigError{
			Account: e.Name,
			Field:   "port",
			Message: fmt.Sprintf("port is required for imap accounts (provider = %q)", e.Provider),
			Suggest: `set "port" (993 for implicit TLS, 143 for STARTTLS) on the [[account]] block`,
		}
	}

	if acct.Backend == "jmap" && acct.Source == "" {
		return nil, &ConfigError{
			Account: e.Name,
			Field:   "source",
			Message: fmt.Sprintf("source URL is required for jmap accounts (provider = %q)", e.Provider),
			Suggest: `set "source" to the JMAP session URL, or pick the "fastmail" preset`,
		}
	}

	smtpPassword, err := resolveEnv(acct.SMTP.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) smtp password: %w", e.Name, e.Provider, err)
	}
	if smtpPassword != "" && acct.SMTP.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q (provider = %q): smtp.password and smtp.password-cmd both set; use one", e.Name, e.Provider)
	}
	acct.SMTP.Password = smtpPassword

	// Default SMTP credentials to account-level credentials when nothing
	// is explicit. Typical case is the same login for both transports.
	if acct.SMTP.Password == "" && acct.SMTP.PasswordCmd == "" {
		acct.SMTP.Password = password
		acct.SMTP.PasswordCmd = e.PasswordCmd
	}
	if acct.SMTP.Auth == "" {
		acct.SMTP.Auth = e.Auth
	}

	if acct.Backend == "imap" && acct.SMTP.Host == "" {
		return nil, &ConfigError{
			Account: e.Name,
			Field:   "smtp.host",
			Message: fmt.Sprintf("smtp.host is required for imap accounts (provider = %q)", e.Provider),
			Suggest: `set "host" on the [account.smtp] block, or pick a hosted preset that fills it in`,
		}
	}

	if err := validateAuth(e.Name, "auth", acct.Auth); err != nil {
		return nil, err
	}
	if err := validateAuth(e.Name, "smtp.auth", acct.SMTP.Auth); err != nil {
		return nil, err
	}
	if err := validateOAuthStore(e.Name, e.OAuthStore); err != nil {
		return nil, err
	}

	acct.Display = e.Display
	acct.Folders = e.FoldersSort
	acct.FoldersExclude = e.FoldersExclude
	acct.Params = e.Params
	acct.OAuth = e.OAuth
	acct.OAuthStore = e.OAuthStore

	if e.CopyTo != "" {
		acct.CopyTo = []string{e.CopyTo}
	}

	if e.From != "" {
		addrs, err := mail.ParseAddressList(e.From)
		if err != nil {
			return nil, fmt.Errorf("account %q: parsing from address: %w", e.Name, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("account %q: from address is empty", e.Name)
		}
		acct.From = addrs[0]
	}

	identities, err := decodeIdentities(e.Name, e.Identities)
	if err != nil {
		return nil, err
	}
	if len(identities) == 0 && acct.From != nil {
		identities = []Identity{{
			Name:  acct.From.Name,
			Email: acct.From.Address,
		}}
	}
	acct.Identities = identities
	if len(acct.Identities) > 0 {
		first := acct.Identities[0]
		acct.From = &mail.Address{Name: first.Name, Address: first.Email}
	}

	acct.Contacts = e.Contacts
	acct.finalizeContacts()
	if err := acct.Contacts.validate(); err != nil {
		return nil, fmt.Errorf("account %q: %w", e.Name, err)
	}
	if acct.Contacts != nil {
		if acct.Contacts.Username == "" {
			return nil, &ConfigError{
				Account: e.Name,
				Field:   "contacts.username",
				Message: "username required (set on [account.contacts] or inherit from [[account]] email)",
			}
		}
		if acct.Contacts.Password == "" && acct.Contacts.PasswordCmd == "" {
			return nil, &ConfigError{
				Account: e.Name,
				Field:   "contacts.password",
				Message: "password or password-cmd required (set on [account.contacts] or inherit from [[account]])",
			}
		}
	}

	return acct, nil
}

func (a *AccountConfig) finalizeContacts() {
	if a.Contacts == nil {
		return
	}
	if a.Contacts.Username == "" {
		a.Contacts.Username = a.Email
	}
	if a.Contacts.Password == "" && a.Contacts.PasswordCmd == "" {
		a.Contacts.Password = a.Password
		a.Contacts.PasswordCmd = a.PasswordCmd
	}
}

// resolveEnv replaces a leading bare "$VAR" with os.Getenv("VAR").
// Anything else passes through so passwords with a literal "$" still
// work. An empty env value returns an error.
func resolveEnv(s string) (string, error) {
	if !strings.HasPrefix(s, "$") || len(s) < 2 {
		return s, nil
	}
	name := s[1:]
	if !isShellName(name) {
		return s, nil
	}
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("env var %s is empty or unset", name)
	}
	return val, nil
}

func knownProvidersList() string {
	names := slices.Sorted(maps.Keys(Providers))
	names = append(names, "imap", "jmap")
	return strings.Join(names, ", ")
}

var validAuthMechanisms = []string{"plain", "login", "cram-md5", "xoauth2", "bearer"}

func validateAuth(account, field, v string) error {
	if v == "" {
		return nil
	}
	for _, m := range validAuthMechanisms {
		if v == m {
			return nil
		}
	}
	return &ConfigError{
		Account: account,
		Field:   field,
		Message: fmt.Sprintf("unknown auth mechanism %q", v),
		Suggest: fmt.Sprintf("known: %s", strings.Join(validAuthMechanisms, ", ")),
	}
}

func validateOAuthStore(account, v string) error {
	switch v {
	case "", "keyring", "age-file":
		return nil
	}
	return &ConfigError{
		Account: account,
		Field:   "oauth-store",
		Message: fmt.Sprintf("unknown oauth-store %q", v),
		Suggest: `known: "keyring", "age-file"`,
	}
}

func isShellName(s string) bool {
	for i, r := range s {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return s != ""
}

func decodeIdentities(account string, entries []identityEntry) ([]Identity, error) {
	var out []Identity
	for _, ie := range entries {
		if ie.Name == "" {
			return nil, fmt.Errorf("account %q: identity: name is required", account)
		}
		if _, err := mail.ParseAddress(ie.Email); err != nil {
			return nil, fmt.Errorf("account %q: identity %q: email: %w", account, ie.Name, err)
		}
		sigs, err := decodeSignatures(ie.Name, ie.Signatures)
		if err != nil {
			return nil, fmt.Errorf("account %q: %w", account, err)
		}
		out = append(out, Identity{
			Name:       ie.Name,
			Email:      ie.Email,
			Signatures: sigs,
		})
	}
	return out, nil
}

func decodeSignatures(identity string, entries []signatureEntry) ([]Signature, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(entries))
	var out []Signature
	for _, se := range entries {
		if se.Name == "" {
			return nil, fmt.Errorf("identity %q: signature: name is required", identity)
		}
		if seen[se.Name] {
			return nil, fmt.Errorf("identity %q: duplicate signature name %q", identity, se.Name)
		}
		seen[se.Name] = true
		if se.Text != "" && se.File != "" {
			return nil, fmt.Errorf("identity %q: signature %q: text and file are mutually exclusive", identity, se.Name)
		}
		if se.Text == "" && se.File == "" {
			return nil, fmt.Errorf("identity %q: signature %q: text or file is required", identity, se.Name)
		}
		text := se.Text
		if se.File != "" {
			path, err := ExpandHome(se.File)
			if err != nil {
				return nil, fmt.Errorf("identity %q: signature %q: file: %w", identity, se.Name, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("identity %q: signature %q: file: %w", identity, se.Name, err)
			}
			text = string(data)
		}
		text = injectSentinel(text)
		out = append(out, Signature{Name: se.Name, Text: text})
	}
	return out, nil
}

// injectSentinel prepends RFC 3676's "-- \n" if t doesn't already start
// with it. Idempotent.
func injectSentinel(t string) string {
	if strings.HasPrefix(t, "-- \n") {
		return t
	}
	return "-- \n" + t
}
