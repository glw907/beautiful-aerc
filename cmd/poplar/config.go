package main

import (
	"github.com/spf13/cobra"
)

// newConfigCmd creates the parent `poplar config` command.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "config",
		Short:        "Manage poplar configuration",
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigInitCmd())
	return cmd
}
