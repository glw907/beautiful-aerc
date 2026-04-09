package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newHTMLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "html",
		Short: "Convert HTML email to styled markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.HTML(os.Stdin, os.Stdout, t, cols)
		},
	}
	return cmd
}
