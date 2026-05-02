// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/spf13/cobra"
)

type configInitFlags struct {
	config string
	write  bool
}

func newConfigInitCmd() *cobra.Command {
	f := configInitFlags{}
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Discover folders and merge [ui.folders] defaults into config.toml",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.config, "config", "", "path to config.toml (default: $XDG_CONFIG_HOME/poplar/config.toml)")
	cmd.Flags().BoolVar(&f.write, "write", false, "write merged output to the config file (default: dry-run to stdout)")
	return cmd
}

func runConfigInit(cmd *cobra.Command, f configInitFlags) error {
	path := f.config
	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	accounts, err := config.ParseAccountsFromBytes(data)
	if err != nil {
		return fmt.Errorf("loading accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts in %s", path)
	}

	// v1 is single-account. Connect to the first account's backend.
	backend, err := openBackendForInit(accounts[0])
	if err != nil {
		return fmt.Errorf("opening backend for account %q: %w", accounts[0].Name, err)
	}
	defer backend.Disconnect()

	folders, err := backend.ListFolders()
	if err != nil {
		return fmt.Errorf("listing folders: %w", err)
	}
	classified := mail.Classify(folders)

	existing, err := config.ExistingFolderKeys(data)
	if err != nil {
		return fmt.Errorf("reading existing folder keys: %w", err)
	}

	rendered := config.RenderFolderSubsections(classified, existing)
	merged := config.MergeFolderSubsections(data, rendered)

	if !f.write {
		fmt.Fprint(cmd.OutOrStdout(), merged)
		return nil
	}
	return writeAtomically(path, merged)
}

// openBackendForInit returns a connected backend for the given account,
// ready to call ListFolders. Delegates to openBackend for construction,
// then calls Connect with a background context.
func openBackendForInit(acct config.AccountConfig) (mail.Backend, error) {
	b, err := openBackend(acct)
	if err != nil {
		return nil, err
	}
	if err := b.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return b, nil
}

// writeAtomically writes content to path via a temp file + rename.
func writeAtomically(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config.toml.tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op on success after Rename

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
