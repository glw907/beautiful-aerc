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
	return &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a compiled theme",
		Long:  "Available themes: " + strings.Join(theme.ThemeNames(), ", ") + ". Generates all if no name given.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := findConfigDir()
			if err != nil {
				return err
			}
			stylesetsDir := filepath.Join(configDir, "stylesets")
			if err := os.MkdirAll(stylesetsDir, 0755); err != nil {
				return fmt.Errorf("create stylesets dir: %w", err)
			}

			if len(args) > 0 {
				name := strings.ToLower(args[0])
				t, ok := theme.Themes[name]
				if !ok {
					return fmt.Errorf("unknown theme %q (available: %s)",
						args[0], strings.Join(theme.ThemeNames(), ", "))
				}
				return generateOne(t, stylesetsDir)
			}

			for _, name := range theme.ThemeNames() {
				if err := generateOne(theme.Themes[name], stylesetsDir); err != nil {
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
