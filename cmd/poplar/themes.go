package main

import (
	"fmt"

	"github.com/glw907/poplar/internal/theme"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "themes",
		Short: "List available color themes",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range theme.ThemeNames() {
				if name == theme.DefaultThemeName {
					fmt.Printf("* %s (default)\n", name)
				} else {
					fmt.Printf("  %s\n", name)
				}
			}
		},
	}
}
