package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/picker"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pick-link",
		Short:        "Interactive URL picker for aerc email messages",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}

			cols := termCols()
			links, err := filter.HTMLLinks(os.Stdin, cols)
			if err != nil {
				return err
			}

			colors := picker.ColorsFromTheme(t)
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

// loadTheme finds and loads the active theme via aerc.conf.
func loadTheme() (*theme.Theme, error) {
	path, err := theme.FindPath()
	if err != nil {
		return nil, err
	}
	return theme.Load(path)
}

// termCols returns the terminal column count from AERC_COLUMNS or a default of 80.
func termCols() int {
	if s := os.Getenv("AERC_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
