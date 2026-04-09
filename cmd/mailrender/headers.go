package main

import (
	"os"
	"strconv"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newHeadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headers",
		Short: "Format and colorize email headers",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Headers(os.Stdin, os.Stdout, colorsFromTheme(t), cols)
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

// colorsFromTheme builds a ColorSet from theme tokens.
func colorsFromTheme(t *theme.Theme) *filter.ColorSet {
	return &filter.ColorSet{
		HdrKey: t.ANSI("hdr_key"),
		HdrFG:  t.ANSI("hdr_value"),
		HdrDim: t.ANSI("hdr_dim"),
		Reset:  t.Reset(),
	}
}

// termCols returns the terminal column count from AERC_COLUMNS or a default of 80.
func termCols() int {
	s := os.Getenv("AERC_COLUMNS")
	if s == "" {
		return 80
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 80
	}
	return n
}
