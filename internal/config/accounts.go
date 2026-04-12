package config

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-message/mail"
)

type configFile struct {
	Account []accountEntry `toml:"account"`
}

type accountEntry struct {
	Name            string            `toml:"name"`
	Backend         string            `toml:"backend"`
	Source          string            `toml:"source"`
	CredentialCmd   string            `toml:"credential-cmd"`
	CopyTo          string            `toml:"copy-to"`
	FoldersSort     []string          `toml:"folders-sort"`
	FoldersExclude  []string          `toml:"folders-exclude"`
	From            string            `toml:"from"`
	Outgoing        string            `toml:"outgoing"`
	OutgoingCredCmd string            `toml:"outgoing-credential-cmd"`
	Params          map[string]string `toml:"params"`
}

// ParseAccounts reads a poplar accounts.toml file and returns
// configured accounts with credentials resolved.
func ParseAccounts(path string) ([]AccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading accounts config: %w", err)
	}

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
	if e.Source == "" {
		return nil, fmt.Errorf("account %q: source is required", e.Name)
	}

	source := e.Source
	if e.CredentialCmd != "" {
		cred, err := runCredentialCmd(e.CredentialCmd)
		if err != nil {
			return nil, fmt.Errorf("account %q: credential command: %w", e.Name, err)
		}
		source, err = injectCredential(source, cred)
		if err != nil {
			return nil, fmt.Errorf("account %q: injecting credential: %w", e.Name, err)
		}
	}

	acct := &AccountConfig{
		Name:            e.Name,
		Backend:         e.Backend,
		Source:          source,
		Folders:         e.FoldersSort,
		FoldersExclude:  e.FoldersExclude,
		Params:          e.Params,
		Outgoing:        e.Outgoing,
		OutgoingCredCmd: e.OutgoingCredCmd,
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

func runCredentialCmd(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return "", fmt.Errorf("running %q: %w", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func injectCredential(source, credential string) (string, error) {
	u, err := url.Parse(source)
	if err != nil {
		return "", fmt.Errorf("parsing source URL: %w", err)
	}
	username := ""
	if u.User != nil {
		username = u.User.Username()
	}
	u.User = url.UserPassword(username, credential)
	return u.String(), nil
}
