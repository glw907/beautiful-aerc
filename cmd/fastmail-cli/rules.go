package main

import "github.com/spf13/cobra"

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage mail filter rules",
		Long: `Manage mail filter rules for Fastmail.

Commands for creating, testing, and exporting filter rules that control
how incoming mail is sorted into folders.`,
	}

	cmd.AddCommand(newInteractiveCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newSweepCmd())
	cmd.AddCommand(newCountCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newExportCheckCmd())
	cmd.AddCommand(newExtractCmd())

	return cmd
}
