package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/glw907/beautiful-aerc/internal/content"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newHeadersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "headers",
		Short: "Render email headers with styling",
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := termCols()
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			h := content.ParseHeaders(string(raw))
			result := content.RenderHeaders(h, selectedTheme(), cols)
			fmt.Fprint(os.Stdout, result)
			return nil
		},
	}
}

// termCols reads the terminal width from AERC_COLUMNS, defaulting to 80.
func termCols() int {
	if s := os.Getenv("AERC_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

// selectedTheme returns the active compiled theme.
// For now, always Nord. A --theme flag can be added later.
func selectedTheme() *theme.CompiledTheme {
	return theme.Nord
}
