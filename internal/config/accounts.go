// Package config holds poplar's configuration types and loaders.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-message/mail"
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

// AccountConfig holds the configuration for a single email account.
type AccountConfig struct {
	Name           string
	Display        string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string

	// Identity
	From   *mail.Address
	CopyTo []string

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
		d := levenshtein(s, c)
		if d < bestDist {
			bestDist = d
			bestName = c
		}
	}
	if bestDist > 2 {
		return ""
	}
	return bestName
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
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
	if err := toml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("decode accounts: %v", err)
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
	if e.Name == "" {
		return nil, fmt.Errorf("account %d: name is required", index)
	}

	backend := e.Provider
	host := e.Host
	port := e.Port
	startTLS := e.StartTLS
	insecureTLS := e.InsecureTLS
	gmailQuirks := false
	source := e.Source

	smtp := SMTPConfig{
		Host:        e.SMTP.Host,
		Port:        e.SMTP.Port,
		StartTLS:    e.SMTP.StartTLS,
		InsecureTLS: e.SMTP.InsecureTLS,
		Auth:        e.SMTP.Auth,
		Password:    e.SMTP.Password,
		PasswordCmd: e.SMTP.PasswordCmd,
	}

	if preset, ok := Providers[e.Provider]; ok {
		backend = preset.Backend
		if host == "" {
			host = preset.Host
		}
		if port == 0 {
			port = preset.Port
		}
		if !startTLS {
			startTLS = preset.StartTLS
		}
		if !insecureTLS {
			insecureTLS = preset.InsecureTLS
		}
		gmailQuirks = preset.GmailQuirks
		if source == "" {
			source = preset.URL
		}
		if smtp.Host == "" {
			smtp.Host = preset.SMTPHost
		}
		if smtp.Port == 0 {
			smtp.Port = preset.SMTPPort
		}
		if !smtp.StartTLS {
			smtp.StartTLS = preset.SMTPStartTLS
		}
		if !smtp.InsecureTLS {
			smtp.InsecureTLS = preset.SMTPInsecureTLS
		}
	}

	password, err := resolveEnv(e.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) password: %w", e.Name, e.Provider, err)
	}
	if password != "" && e.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q (provider = %q): both password and password-cmd set; use one", e.Name, e.Provider)
	}
	// "mock" short-circuits to mail.NewMockBackend in cmd/poplar/backend.go.
	if e.Provider != "imap" && e.Provider != "jmap" && e.Provider != "mock" {
		if _, ok := Providers[e.Provider]; !ok {
			hint := ""
			if s := suggestProvider(e.Provider); s != "" {
				hint = fmt.Sprintf("; did you mean %q?", s)
			}
			return nil, fmt.Errorf("account %q: unknown provider %q%s (known: %s)",
				e.Name, e.Provider, hint, knownProvidersList())
		}
	}

	if backend == "imap" && host == "" {
		return nil, fmt.Errorf("account %q (provider = %q): host is required for imap accounts",
			e.Name, e.Provider)
	}

	if backend == "jmap" && source == "" {
		return nil, fmt.Errorf("account %q (provider = %q): source URL is required for jmap accounts",
			e.Name, e.Provider)
	}

	smtpPassword, err := resolveEnv(smtp.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) smtp password: %w", e.Name, e.Provider, err)
	}
	if smtpPassword != "" && smtp.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q (provider = %q): smtp.password and smtp.password-cmd both set; use one", e.Name, e.Provider)
	}
	smtp.Password = smtpPassword

	// Default SMTP credentials to IMAP credentials when nothing is
	// explicit. Typical case is the same login for both transports.
	if smtp.Password == "" && smtp.PasswordCmd == "" {
		smtp.Password = password
		smtp.PasswordCmd = e.PasswordCmd
	}
	if smtp.Auth == "" {
		smtp.Auth = e.Auth
	}

	if backend == "imap" && smtp.Host == "" {
		return nil, fmt.Errorf("account %q (provider = %q): smtp.host is required for imap accounts (set [account.smtp].host)",
			e.Name, e.Provider)
	}

	acct := &AccountConfig{
		Name:           e.Name,
		Display:        e.Display,
		Backend:        backend,
		Source:         source,
		Email:          e.Email,
		Host:           host,
		Port:           port,
		StartTLS:       startTLS,
		InsecureTLS:    insecureTLS,
		GmailQuirks:    gmailQuirks,
		Auth:           e.Auth,
		Password:       password,
		PasswordCmd:    e.PasswordCmd,
		Folders:        e.FoldersSort,
		FoldersExclude: e.FoldersExclude,
		Params:         e.Params,
		SMTP:           smtp,
	}

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

	acct.Contacts = e.Contacts
	acct.finalizeContacts()
	if err := acct.Contacts.validate(); err != nil {
		return nil, fmt.Errorf("account %q: %w", e.Name, err)
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
	names := make([]string, 0, len(Providers))
	for k := range Providers {
		names = append(names, k)
	}
	sort.Strings(names)
	names = append(names, "imap", "jmap")
	return strings.Join(names, ", ")
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
