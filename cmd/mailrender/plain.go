package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newPlainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plain",
		Short: "Format plain text email (reflow and colorize)",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Plain(os.Stdin, os.Stdout, p, cols)
		},
	}
	return cmd
}
