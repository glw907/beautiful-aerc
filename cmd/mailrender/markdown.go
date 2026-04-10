package main

import (
	"fmt"
	"io"
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newMarkdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "markdown",
		Short: "Convert HTML email to clean markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			md := filter.CleanHTML(string(raw))
			fmt.Fprintln(os.Stdout, md)
			return nil
		},
	}
}
