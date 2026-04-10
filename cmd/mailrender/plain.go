package main

import (
	"fmt"
	"io"
	"os"

	"github.com/glw907/beautiful-aerc/internal/content"
	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newPlainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plain",
		Short: "Render plain text email to styled terminal output",
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := termCols()
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			md := filter.CleanPlain(string(raw))
			blocks := content.ParseBlocks(md)
			result := content.RenderBody(blocks, selectedTheme(), cols)
			fmt.Fprintln(os.Stdout)
			fmt.Fprint(os.Stdout, result)
			return nil
		},
	}
}
