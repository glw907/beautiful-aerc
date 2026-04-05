package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

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
			url, err := picker.Run(links, cols, colors)
			if err != nil {
				return err
			}
			if url != "" {
				name := "xdg-open"
				if strings.HasPrefix(url, "mailto:") {
					name = "aerc"
				}
				open := exec.Command(name, url)
				open.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
				return open.Start()
			}
			return nil
		},
	}
	return cmd
}
