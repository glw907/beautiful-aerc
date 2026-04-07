package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/rules"
)

type addFlags struct {
	search    string
	folder    string
	rulesFile string
}

func newAddCmd() *cobra.Command {
	var f addFlags

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a filter rule to the local rules file",
		Long: `Add a filter rule to the local rules JSON file.

The rule is appended with the given search query and destination folder.
Use 'fastmail-cli rules export' to upload changes to Fastmail.`,
		Example: `  fastmail-cli rules add --search "from:user@example.com" --folder Notifications
  fastmail-cli rules add --search "to:team@example.com" --folder Team --rules-file /path/to/rules.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(f.search, f.folder, resolveRulesFile(f.rulesFile))
		},
	}

	cmd.Flags().StringVar(&f.search, "search", "", "Search query (e.g. from:user@example.com)")
	cmd.Flags().StringVar(&f.folder, "folder", "", "Destination folder")
	cmd.Flags().StringVar(&f.rulesFile, "rules-file", "", "Path to rules file (default: $AERC_RULES_FILE or ~/.config/aerc/mailrules.json)")
	cmd.MarkFlagRequired("search")
	cmd.MarkFlagRequired("folder")

	return cmd
}

func runAdd(search, folder, path string) error {
	if err := rules.Add(path, search, folder); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Rule added: %s -> %s\n", search, folder)
	return nil
}
