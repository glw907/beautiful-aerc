// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/config"
	"github.com/spf13/cobra"
)

// newConfigInitTemplateCmd creates the `poplar config init` subcommand,
// which writes a fresh self-documenting config template to disk.
func newConfigInitTemplateCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Write a fresh self-documenting config template",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _, err := config.Resolve("")
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

// newConfigPathCmd creates the `poplar config path` subcommand,
// which prints the resolved config-file path without reading it.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "path",
		Short:        "Print the resolved config-file path",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _, err := config.Resolve("")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
}

// newConfigCheckCmd creates the `poplar config check` subcommand,
// which validates the config file and tests each account's connection.
func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "check",
		Short:        "Validate config and test each account's connection",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			accounts, _, err := config.Load("")
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
