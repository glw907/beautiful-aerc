package main

import (
	"github.com/spf13/cobra"
)

var version = "0.2.0"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fastmail-cli",
		Short: "CLI for Fastmail JMAP operations",
		Long: `fastmail-cli provides command-line access to Fastmail via JMAP.

Commands:
  rules    Manage mail filter rules
  masked   Manage masked email addresses
  folders  List custom mailboxes

Setup:
  1. Generate a Fastmail API token at:
     Settings > Privacy & Security > API tokens
  2. Export it in your shell profile:
     export FASTMAIL_API_TOKEN="fmu1-..."
  3. Add keybindings to ~/.config/aerc/binds.conf:
     ff = :pipe -m fastmail-cli rules interactive from<Enter>
     fs = :pipe -m fastmail-cli rules interactive subject<Enter>
     ft = :pipe -m fastmail-cli rules interactive to<Enter>
     md = :pipe -m fastmail-cli masked delete<Enter>:delete<Enter>
  4. Optionally add a shutdown hook to ~/.config/aerc/aerc.conf:
     [hooks]
     aerc-shutdown=fastmail-cli rules export-check && fastmail-cli rules export

Environment Variables:
  FASTMAIL_API_TOKEN       Fastmail API token (required for JMAP commands)
  AERC_RULES_FILE          Path to rules file (default: ~/.config/aerc/mailrules.json)
  AERC_RULES_EXPORT_DEST   Export destination (default: ~/Documents/mailrules.json)`,
		SilenceUsage: true,
	}

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newMaskedCmd())
	cmd.AddCommand(newFoldersCmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("fastmail-cli " + version)
		},
	}
}
