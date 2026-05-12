package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/glw907/poplar/internal/config"
)

// errUnknownReauthAccount and errReauthAccountNotOAuth are exit-78
// (config error) sentinels.
var (
	errUnknownReauthAccount  = errors.New("unknown account")
	errReauthAccountNotOAuth = errors.New("account is not OAuth")
)

func runReauth(f rootFlags) error {
	accts, configPath, err := config.Load(f.config)
	if err != nil {
		return fmt.Errorf("load config: %v", err)
	}

	var acct *config.AccountConfig
	for i := range accts {
		if accts[i].Name == f.reauth {
			acct = &accts[i]
			break
		}
	}
	if acct == nil {
		return fmt.Errorf("%w: %q in %s", errUnknownReauthAccount, f.reauth, configPath)
	}
	if acct.OAuth == nil {
		return fmt.Errorf("%w: %q", errReauthAccountNotOAuth, f.reauth)
	}

	cli, err := buildOAuthClient(*acct)
	if err != nil {
		return fmt.Errorf("oauth client for %q: %v", f.reauth, err)
	}

	ctx := context.Background()
	fmt.Fprintf(os.Stderr, "Opening browser for %s OAuth consent...\n", f.reauth)
	if err := cli.Authorize(ctx); err != nil {
		return fmt.Errorf("reauth %q: %v", f.reauth, err)
	}
	fmt.Fprintf(os.Stderr, "reauth %s: refresh token stored\n", f.reauth)
	return nil
}
