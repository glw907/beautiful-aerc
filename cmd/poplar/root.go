package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/poplar"
	"github.com/spf13/cobra"

	// Import forked workers for init() side effects (handler registration).
	_ "github.com/glw907/beautiful-aerc/internal/aercfork/worker"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "poplar",
		Short:        "A bubbletea-based terminal email client",
		SilenceUsage: true,
		RunE:         runRoot,
	}
	return cmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}

	configPath := filepath.Join(home, ".config", "poplar", "accounts.toml")
	accounts, err := poplar.ParseAccounts(configPath)
	if err != nil {
		return fmt.Errorf("loading accounts: %w", err)
	}

	acct := &accounts[0]
	adapter, err := mail.NewJMAPAdapter(acct)
	if err != nil {
		return fmt.Errorf("creating adapter: %w", err)
	}

	ctx := context.Background()
	if err := adapter.Connect(ctx); err != nil {
		return fmt.Errorf("connecting: %w", err)
	}

	folders, err := adapter.ListFolders()
	if err != nil {
		return fmt.Errorf("listing folders: %w", err)
	}

	for _, f := range folders {
		role := ""
		if f.Role != "" {
			role = " [" + f.Role + "]"
		}
		fmt.Fprintf(os.Stdout, "%-30s %d messages, %d unread%s\n",
			f.Name, f.Exists, f.Unseen, role)
	}

	return nil
}
