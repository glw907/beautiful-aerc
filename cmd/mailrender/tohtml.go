package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newToHTMLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "to-html",
		Short:        "Convert markdown to HTML (for multipart-converters)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return filter.ToHTML(os.Stdin, os.Stdout)
		},
	}
	return cmd
}
