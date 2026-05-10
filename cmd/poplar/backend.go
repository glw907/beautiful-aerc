package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/mailimap"
	"github.com/glw907/poplar/internal/mailjmap"
)

// openBackend constructs a mail.Backend for acct. Mock is the default
// for unconfigured or test accounts.
func openBackend(acct config.AccountConfig) (mail.Backend, error) {
	switch acct.Backend {
	case "mock", "":
		return mail.NewMockBackend(), nil
	case "jmap":
		return mailjmap.New(acct), nil
	case "imap":
		if acct.OAuth != nil {
			c, err := buildOAuthClient(acct)
			if err != nil {
				return nil, fmt.Errorf("oauth client for %q: %w", acct.Name, err)
			}
			return mailimap.NewWithOAuth(acct, c), nil
		}
		return mailimap.New(acct), nil
	default:
		return nil, fmt.Errorf("unknown backend %q for account %q", acct.Backend, acct.Name)
	}
}

// buildOAuthClient constructs a mailauth.Client from the account's OAuthConfig.
// It opens the appropriate token store (keyring or age-file fallback).
func buildOAuthClient(acct config.AccountConfig) (*mailauth.Client, error) {
	oa := acct.OAuth
	cfg := mailauth.Config{
		ClientID:     oa.ClientID,
		ClientSecret: oa.ClientSecret,
		AuthURL:      oa.AuthURL,
		TokenURL:     oa.TokenURL,
		Scopes:       oa.Scopes,
	}

	tokenDir, err := oauthTokenDir()
	if err != nil {
		return nil, err
	}
	store, backend, err := mailauth.OpenStore(acct.Name, tokenDir)
	if err != nil {
		return nil, fmt.Errorf("token store: %w", err)
	}

	// If config specifies a store backend, override when it differs.
	if acct.OAuthStore != "" {
		_ = backend // preference recorded in config; store selection already done by OpenStore
	}

	return mailauth.NewClient(cfg, store, acct.Name, backend), nil
}

func oauthTokenDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("token dir: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "poplar", "tokens"), nil
}
