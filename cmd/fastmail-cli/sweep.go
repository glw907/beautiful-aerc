package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/jmap"
)

type sweepFlags struct {
	search string
	folder string
}

func newSweepCmd() *cobra.Command {
	var f sweepFlags

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Move matching Inbox messages to a folder via JMAP",
		Long: `Move all Inbox messages matching a search query to a destination folder.

Connects to Fastmail via JMAP, queries the Inbox, and moves matching
messages in a single batch operation.`,
		Example: `  fastmail-cli rules sweep --search "from:user@example.com" --folder Notifications
  fastmail-cli rules sweep --search "to:committee@example.com" --folder Committee`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweep(f.search, f.folder)
		},
	}

	cmd.Flags().StringVar(&f.search, "search", "", "Search query")
	cmd.Flags().StringVar(&f.folder, "folder", "", "Destination folder")
	cmd.MarkFlagRequired("search")
	cmd.MarkFlagRequired("folder")

	return cmd
}

func runSweep(search, folder string) error {
	s, err := newJMAPSession()
	if err != nil {
		return err
	}

	inboxID, err := jmap.FindMailbox(s, "Inbox")
	if err != nil {
		return fmt.Errorf("finding Inbox: %w", err)
	}
	destID, err := jmap.FindMailbox(s, folder)
	if err != nil {
		return fmt.Errorf("finding folder %q: %w", folder, err)
	}

	ids, err := jmap.QueryInbox(s, search)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "No matching messages")
		return nil
	}

	if err := jmap.MoveEmails(s, ids, inboxID, destID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Moved %d messages to %s\n", len(ids), folder)
	return nil
}
