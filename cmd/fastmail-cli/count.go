package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/jmap"
)

type countFlags struct {
	search string
}

func newCountCmd() *cobra.Command {
	var f countFlags

	cmd := &cobra.Command{
		Use:   "count",
		Short: "Count matching Inbox messages via JMAP",
		Long: `Count Inbox messages matching a search query via JMAP.

Prints the number of matching messages to stdout. Useful for checking
how many messages a filter rule would affect before running sweep.`,
		Example: `  fastmail-cli rules count --search "from:user@example.com"
  fastmail-cli rules count --search "to:team@example.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCount(f.search)
		},
	}

	cmd.Flags().StringVar(&f.search, "search", "", "Search query")
	cmd.MarkFlagRequired("search")

	return cmd
}

func runCount(search string) error {
	s, err := newJMAPSession()
	if err != nil {
		return err
	}

	ids, err := jmap.QueryInbox(s, search)
	if err != nil {
		return err
	}

	fmt.Println(len(ids))
	return nil
}
