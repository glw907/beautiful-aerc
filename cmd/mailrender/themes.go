package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	return &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a compiled theme",
		Long:  "Available themes: nord, solarized-dark, gruvbox-dark. Generates all if no name given.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := findConfigDir()
			if err != nil {
				return err
			}
			stylesetsDir := filepath.Join(configDir, "stylesets")
			if err := os.MkdirAll(stylesetsDir, 0755); err != nil {
				return fmt.Errorf("create stylesets dir: %w", err)
			}

			themes := map[string]*theme.CompiledTheme{
				"nord":           theme.Nord,
				"solarized-dark": theme.SolarizedDark,
				"gruvbox-dark":   theme.GruvboxDark,
			}

			if len(args) > 0 {
				t, ok := themes[args[0]]
				if !ok {
					return fmt.Errorf("unknown theme %q (available: nord, solarized-dark, gruvbox-dark)", args[0])
				}
				return generateOne(t, stylesetsDir)
			}

			for _, t := range []*theme.CompiledTheme{theme.Nord, theme.SolarizedDark, theme.GruvboxDark} {
				if err := generateOne(t, stylesetsDir); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func generateOne(t *theme.CompiledTheme, dir string) error {
	outPath := filepath.Join(dir, t.Name)
	if err := theme.WriteStyleset(t, outPath); err != nil {
		return fmt.Errorf("generate %s: %w", t.Name, err)
	}
	fmt.Fprintf(os.Stderr, "Theme: %s\nStyleset: %s\n", t.Name, outPath)
	return nil
}

func findConfigDir() (string, error) {
	if dir := os.Getenv("AERC_CONFIG"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home dir: %w", err)
	}
	return filepath.Join(home, ".config", "aerc"), nil
}
