package main

import "github.com/spf13/cobra"

func newMaskedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "masked",
		Short: "Manage masked email addresses",
		Long: `Manage Fastmail masked email addresses via JMAP.

Masked emails are disposable addresses that forward to your real inbox.
Use these commands to manage them from the terminal or from within aerc.`,
	}

	cmd.AddCommand(newMaskedDeleteCmd())

	return cmd
}
