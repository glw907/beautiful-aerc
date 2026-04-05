package main

import (
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

			cols := termCols()
			links, err := filter.HTMLLinks(os.Stdin, cols)
			if err != nil {
				return err
			}

			colors := picker.ColorsFromPalette(p)
			url, err := picker.Run(links, os.Stderr, cols, colors)
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
