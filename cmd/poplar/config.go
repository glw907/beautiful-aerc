package main

import (
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "config",
		Short:        "Manage poplar configuration",
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigDiscoverFoldersCmd())
	cmd.AddCommand(newConfigInitTemplateCmd())
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigCheckCmd())
	return cmd
}
