package main

import (
	"context"
	"fmt"
	"os"

	"github.com/glw907/poplar/internal/config"
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
		fmt.Fprintf(os.Stderr, "poplar: unknown account %q in %s\n", f.reauth, configPath)
		os.Exit(78)
	}
	if acct.OAuth == nil {
		fmt.Fprintf(os.Stderr, "poplar: account %q is not OAuth\n", f.reauth)
		os.Exit(78)
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
