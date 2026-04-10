package main

import (
	"fmt"
	"os"

	"github.com/glw907/beautiful-aerc/internal/content"
	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newPreviewCmd() *cobra.Command {
	var themeName string
	var width int

	cmd := &cobra.Command{
		Use:   "preview <file>",
		Short: "Preview email rendering from a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			t := resolveTheme(themeName)
			if width <= 0 {
				width = 78
			}
			md := filter.CleanHTML(string(raw))
			blocks := content.ParseBlocks(md)
			result := content.RenderBody(blocks, t, width)
			fmt.Print(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&themeName, "theme", "nord", "theme name (nord, solarized-dark, gruvbox-dark)")
	cmd.Flags().IntVar(&width, "width", 78, "rendering width in columns")
	return cmd
}

func resolveTheme(name string) *theme.CompiledTheme {
	switch name {
	case "solarized-dark":
		return theme.SolarizedDark
	case "gruvbox-dark":
		return theme.GruvboxDark
	default:
		return theme.Nord
	}
}
