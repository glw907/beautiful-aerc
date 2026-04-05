package main

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/picker"
	"github.com/spf13/cobra"
)

func newPickLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pick-link",
		Short: "Interactive URL picker for email messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}

			// Run raw HTML through the filter to get clean footnoted output.
			var filtered bytes.Buffer
			cols := termCols()
			if err := filter.HTML(os.Stdin, &filtered, p, cols); err != nil {
				return err
			}

			colors := picker.ColorsFromPalette(p)
			url, err := picker.Run(&filtered, os.Stderr, colors)
			if err != nil {
				return err
			}
			if url != "" {
				return exec.Command("xdg-open", url).Start()
			}
			return nil
		},
	}
	return cmd
}
