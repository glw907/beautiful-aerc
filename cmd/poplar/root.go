package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	theme string
}

func newRootCmd() *cobra.Command {
	f := rootFlags{}
	cmd := &cobra.Command{
		Use:          "poplar",
		Short:        "A bubbletea-based terminal email client",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(f)
		},
	}
	cmd.Flags().StringVarP(&f.theme, "theme", "t", theme.DefaultThemeName,
		"color theme ("+strings.Join(theme.ThemeNames(), ", ")+")")
	return cmd
}

// appModel wraps ui.App to satisfy tea.Model (returns tea.Model, not App).
type appModel struct {
	app ui.App
}

func (m appModel) Init() tea.Cmd { return m.app.Init() }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	app, cmd := m.app.Update(msg)
	m.app = app
	return m, cmd
}

func (m appModel) View() string { return m.app.View() }

func runRoot(f rootFlags) error {
	t, ok := theme.Themes[strings.ToLower(f.theme)]
	if !ok {
		return fmt.Errorf("unknown theme %q (available: %s)",
			f.theme, strings.Join(theme.ThemeNames(), ", "))
	}

	backend := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig()
	// Pass 3 swaps this for config.LoadUI(configPath).
	app := ui.NewApp(t, backend, uiCfg)

	p := tea.NewProgram(appModel{app: app}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running poplar: %w", err)
	}
	return nil
}
