package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newMarkdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "markdown",
		Short: "Convert HTML email to clean markdown (for reply templates)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := termCols()
			return filter.Markdown(os.Stdin, os.Stdout, cols)
		},
	}
	return cmd
}
