// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-message/mail"
)

type configFile struct {
	Account []accountEntry `toml:"account"`
}

type accountEntry struct {
	Name              string            `toml:"name"`
	Display           string            `toml:"display"`
	Provider          string            `toml:"provider"`
	Source            string            `toml:"source"`
	Email             string            `toml:"email"`
	Host              string            `toml:"host"`
	Port              int               `toml:"port"`
	StartTLS          bool              `toml:"starttls"`
	InsecureTLS       bool              `toml:"insecure-tls"`
	Auth              string            `toml:"auth"`
	Password          string            `toml:"password"`
	PasswordCmd       string            `toml:"password-cmd"`
	OAuthClientID     string            `toml:"oauth-client-id"`
	OAuthClientSecret string            `toml:"oauth-client-secret"`
	OAuthRefreshToken string            `toml:"oauth-refresh-token"`
	CopyTo            string            `toml:"copy-to"`
	FoldersSort       []string          `toml:"folders-sort"`
	FoldersExclude    []string          `toml:"folders-exclude"`
	From              string            `toml:"from"`
	Params            map[string]string `toml:"params"`
}

// ParseAccounts reads a poplar config.toml file and returns
// configured accounts with credentials resolved.
func ParseAccounts(path string) ([]AccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading accounts config: %w", err)
	}
	return ParseAccountsFromBytes(data)
}

// ParseAccountsFromBytes parses config.toml contents. Callers that
// have already read the file should pass its bytes here to avoid a
// second read.
func ParseAccountsFromBytes(data []byte) ([]AccountConfig, error) {
	var cf configFile
	if err := toml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing accounts config: %w", err)
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
	source := e.Source

	if preset, ok := LookupProvider(e.Provider); ok {
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
		if source == "" {
			source = preset.URL
		}
	}

	password, err := resolveEnv(e.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q password: %w", e.Name, err)
	}
	if password != "" && e.PasswordCmd != "" {
		return nil, fmt.Errorf("account %q: both password and password-cmd set; use one", e.Name)
	}
	clientID, err := resolveEnv(e.OAuthClientID)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-client-id: %w", e.Name, err)
	}
	clientSecret, err := resolveEnv(e.OAuthClientSecret)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-client-secret: %w", e.Name, err)
	}
	refresh, err := resolveEnv(e.OAuthRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-refresh-token: %w", e.Name, err)
	}

	// Validate provider against the registry + fallbacks.
	// "mock" is permitted for testing; it short-circuits to
	// mail.NewMockBackend in cmd/poplar/backend.go.
	if e.Provider != "imap" && e.Provider != "jmap" && e.Provider != "mock" {
		if _, ok := LookupProvider(e.Provider); !ok {
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
		Name:              e.Name,
		Display:           e.Display,
		Backend:           backend,
		Source:            source,
		Email:             e.Email,
		Host:              host,
		Port:              port,
		StartTLS:          startTLS,
		InsecureTLS:       insecureTLS,
		Auth:              e.Auth,
		Password:          password,
		PasswordCmd:       e.PasswordCmd,
		OAuthClientID:     clientID,
		OAuthClientSecret: clientSecret,
		OAuthRefreshToken: refresh,
		Folders:           e.FoldersSort,
		FoldersExclude:    e.FoldersExclude,
		Params:            e.Params,
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
// only supported form is the bare $VAR token; anything else is
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

// knownProvidersList returns a sorted, comma-separated list of all
// recognized provider names (presets + bare "imap"/"jmap").
func knownProvidersList() string {
	names := make([]string, 0, len(Providers))
	for k := range Providers {
		names = append(names, k)
	}
	sort.Strings(names)
	names = append(names, "imap", "jmap")
	return strings.Join(names, ", ")
}

// isShellName reports whether s is a valid shell variable name:
// starts with a letter or underscore, followed by letters, digits,
// or underscores only.
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
