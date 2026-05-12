package main

import (
	"context"
	"fmt"
	"os"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/spf13/cobra"
)

func newConfigDiscoverFoldersCmd() *cobra.Command {
	var write bool
	cmd := &cobra.Command{
		Use:          "discover-folders",
		Short:        "Connect to each account, discover server folders, merge defaults into [ui.folders]",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			path, err := config.Resolve(flagPath)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %v", path, err)
			}

			accounts, err := config.ParseAccountsFromBytes(data)
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fmt.Errorf("no accounts in %s", path)
			}

			backend, err := openBackend(accounts[0])
			if err != nil {
				return fmt.Errorf("backend for %q: %v", accounts[0].Name, err)
			}
			if err := backend.Connect(context.Background()); err != nil {
				return fmt.Errorf("connect %q: %v", accounts[0].Name, err)
			}
			defer backend.Disconnect()

			folders, err := backend.ListFolders()
			if err != nil {
				return fmt.Errorf("list server folders: %v", err)
			}
			classified := mail.Classify(folders)

			existing, err := config.FolderKeys(data)
			if err != nil {
				return fmt.Errorf("scan existing [ui.folders.*] keys: %v", err)
			}

			rendered := config.RenderFolderSubsections(classified, existing)
			merged := config.MergeFolderSubsections(data, rendered)

			if !write {
				fmt.Fprint(cmd.Root().OutOrStdout(), merged)
				return nil
			}
			return config.AtomicWrite(path, []byte(merged))
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "write merged output to the config file (default: dry-run to stdout)")
	return cmd
}
