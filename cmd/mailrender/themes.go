package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "Theme management commands",
	}
	cmd.AddCommand(newThemesGenerateCmd())
	return cmd
}

func newThemesGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a TOML theme file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			confDir, err := theme.FindConfigDir()
			if err != nil {
				return err
			}

			var themePath string
			if len(args) == 1 {
				themePath = filepath.Join(confDir, "themes", args[0]+".toml")
			} else {
				themePath, err = theme.FindPath()
				if err != nil {
					return err
				}
			}

			th, err := theme.Load(themePath)
			if err != nil {
				return err
			}

			// Styleset name matches the TOML filename (e.g. "nord" from "nord.toml"),
			// not th.Name (display name like "Nord"), so it matches styleset-name in aerc.conf.
			stylesetName := strings.TrimSuffix(filepath.Base(themePath), ".toml")
			stylesetDir := filepath.Join(confDir, "stylesets")
			if err := os.MkdirAll(stylesetDir, 0755); err != nil {
				return fmt.Errorf("creating stylesets directory: %w", err)
			}

			outPath := filepath.Join(stylesetDir, stylesetName)
			if err := theme.WriteStyleset(th, outPath); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Theme:    %s\n", filepath.Base(themePath))
			fmt.Fprintf(os.Stderr, "Styleset: stylesets/%s\n", stylesetName)
			return nil
		},
	}
	return cmd
}
