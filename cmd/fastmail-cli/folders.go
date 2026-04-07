package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/jmap"
)

func newFoldersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "folders",
		Short: "List available destination folders via JMAP",
		Long: `List custom mailbox folders from the mail server via JMAP.
Excludes standard folders (Inbox, Sent, Drafts, Trash, Spam, Archive, Outbox).
Prints one folder name per line, sorted alphabetically.`,
		Example: "  fastmail-cli folders",
		RunE:    runFolders,
	}
}

func runFolders(cmd *cobra.Command, args []string) error {
	s, err := newJMAPSession()
	if err != nil {
		return err
	}

	folders, err := jmap.ListFolders(s)
	if err != nil {
		return err
	}

	for _, f := range folders {
		fmt.Println(f.Name)
	}
	return nil
}
