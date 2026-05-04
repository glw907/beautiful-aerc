// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/config"
	"github.com/spf13/cobra"
)

// configFlagPath returns the value of the root command's --config
// persistent flag, or "" when the flag is not registered (e.g. tests
// that exercise a subcommand in isolation).
func configFlagPath(cmd *cobra.Command) string {
	root := cmd.Root()
	if root == nil {
		return ""
	}
	f := root.PersistentFlags().Lookup("config")
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func newConfigInitTemplateCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Write a fresh self-documenting config template",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			path, err := config.Resolve(flagPath)
			if err != nil {
				return err
			}
			if !force {
				if _, statErr := os.Stat(path); statErr == nil {
					return fmt.Errorf("%s already exists; use --force to overwrite", path)
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(config.Template()), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")
	return cmd
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "path",
		Short:        "Print the resolved config-file path",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			path, err := config.Resolve(flagPath)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
}

func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "check",
		Short:        "Validate config and test each account's connection",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			accounts, _, err := config.Load(flagPath)
			if err != nil {
				return err
			}
			anyFail := false
			for _, a := range accounts {
				b, err := openBackend(a)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s error: %v\n", a.Name, err)
					anyFail = true
					continue
				}
				if err := b.Connect(cmd.Context()); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s error: %v\n", a.Name, err)
					anyFail = true
					continue
				}
				_ = b.Disconnect()
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s OK\n", a.Name)
			}
			if anyFail {
				return fmt.Errorf("one or more accounts failed")
			}
			return nil
		},
	}
}
