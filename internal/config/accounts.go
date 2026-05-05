// SPDX-License-Identifier: MIT

// Package config holds poplar's configuration types and loaders.
package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-message/mail"
)

// ResolvePassword returns the cleartext password for c. Inline
// Password wins. Otherwise PasswordCmd is run via /bin/sh -c and
// stdout (trimmed of trailing newlines) is the password.
func (c *AccountConfig) ResolvePassword() (string, error) {
	if c.Password != "" {
		return c.Password, nil
	}
	if c.PasswordCmd == "" {
		return "", errors.New("account has no password or password-cmd")
	}
	out, err := exec.Command("/bin/sh", "-c", c.PasswordCmd).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if stderr := strings.TrimSpace(string(ee.Stderr)); stderr != "" {
				return "", fmt.Errorf("password-cmd: %s", stderr)
			}
		}
		return "", fmt.Errorf("password-cmd: %v", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
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

	// Password is the bearer token or password after env-var substitution.
	// In config.toml use "$VAR_NAME" to pull from the environment.
	Password string

	// PasswordCmd is a shell command whose stdout becomes the
	// password. Deferred to first Connect so secret-manager prompts
	// fire near the action. Mutually exclusive with Password.
	PasswordCmd string

	// Auth holds the SASL mechanism. Recognized values: "plain",
	// "login", "cram-md5", "xoauth2", "bearer". Empty defers to
	// the backend default.
	Auth string

	// Email is the user's address. May be empty when the backend
	// auto-discovers (e.g. JMAP session).
	Email string

	// IMAP transport (set directly or via a provider preset).
	Host     string
	Port     int
	StartTLS bool

	// InsecureTLS skips TLS verification. Use only for self-hosted
	// servers with self-signed certs. Never set for hosted providers.
	InsecureTLS bool

	// GmailQuirks enables Gmail-specific IMAP behavior in mailimap:
	// X-GM-EXT-1 assertion at Connect, and Destroy routed via
	// SELECT [Gmail]/Trash before EXPUNGE. Set by the gmail preset.
	GmailQuirks bool
}

// ExpandHome turns a leading "~" into the user's home directory.
// "~" expands to $HOME. "~/x" expands to $HOME/x. Other paths and empty strings pass through.
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

// suggestProvider returns the closest known-provider name to s, or ""
// when nothing's within edit distance 2.
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
}

// ParseAccounts reads a poplar config.toml file and returns
// configured accounts with credentials resolved.
func ParseAccounts(path string) ([]AccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v", path, err)
	}
	return ParseAccountsFromBytes(data)
}

// ParseAccountsFromBytes is the byte-slice form of ParseAccounts.
// Use it when the file has already been read.
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
	}

	password, err := resolveEnv(e.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) password: %w", e.Name, e.Provider, err)
	}
	if password != "" && e.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q (provider = %q): both password and password-cmd set; use one", e.Name, e.Provider)
	}
	// Validate provider against the registry + fallbacks.
	// "mock" is permitted for testing. It short-circuits to
	// mail.NewMockBackend in cmd/poplar/backend.go.
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

	// IMAP requires a host (after preset resolution).
	if backend == "imap" && host == "" {
		return nil, fmt.Errorf("account %q (provider = %q): host is required for imap accounts",
			e.Name, e.Provider)
	}

	// JMAP requires a session URL (after preset resolution).
	if backend == "jmap" && source == "" {
		return nil, fmt.Errorf("account %q (provider = %q): source URL is required for jmap accounts",
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

	return acct, nil
}

// resolveEnv replaces a leading "$VAR" with os.Getenv("VAR"). The
// only supported form is the bare $VAR token. Anything else is
// returned unchanged so passwords containing a literal "$" still
// work. Empty env returns an error so the user gets a clear
// failure on misconfiguration.
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

// isShellName reports whether s is a valid POSIX shell variable name.
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
