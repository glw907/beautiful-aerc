package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/term"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	config string
	theme  string
}

func newRootCmd() *cobra.Command {
	f := rootFlags{}
	cmd := &cobra.Command{
		Use:           "poplar",
		Short:         "A bubbletea-based terminal email client",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(f)
		},
	}
	cmd.PersistentFlags().StringVar(&f.config, "config", "",
		"path to config file (default: $POPLAR_CONFIG or ~/.config/poplar/config.toml)")
	cmd.Flags().StringVarP(&f.theme, "theme", "t", theme.DefaultThemeName,
		"color theme ("+strings.Join(theme.ThemeNames(), ", ")+")")
	return cmd
}

// appModel wraps ui.App so its Update returns the tea.Model interface
// rather than ui.App.
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

	accts, configPath, err := config.Load(f.config)
	if errors.Is(err, config.ErrFirstRun) {
		fmt.Fprintln(os.Stderr, err.Error())
		fmt.Fprintln(os.Stderr, "Edit the file and run poplar again.")
		os.Exit(78)
	}
	if errors.Is(err, config.ErrOldAccountsToml) {
		fmt.Fprintln(os.Stderr, "poplar: "+err.Error())
		fmt.Fprintln(os.Stderr, "  poplar 1.0 reads config.toml; rename your accounts.toml file.")
		os.Exit(78)
	}
	if err != nil {
		return fmt.Errorf("load accounts: %v", err)
	}
	if len(accts) == 0 {
		return fmt.Errorf("no accounts configured; see ~/.config/poplar/config.toml")
	}
	backend, err := openBackend(accts[0])
	if err != nil {
		return fmt.Errorf("open backend %q: %v", accts[0].Name, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := backend.Connect(ctx); err != nil {
		return fmt.Errorf("connect %s: %v", accts[0].Name, err)
	}
	defer backend.Disconnect()

	uiCfg, err := config.LoadUI(configPath)
	if err != nil {
		return fmt.Errorf("ui config: %v", err)
	}

	hasNF := term.HasNerdFont()
	probe := term.MeasureSPUACells()
	mode, cellWidth := term.Resolve(uiCfg.Icons, hasNF, probe)

	iconSet := uicore.SimpleIcons
	if mode == term.IconModeFancy {
		iconSet = uicore.FancyIcons
	}
	uicore.SetSPUACellWidth(cellWidth)

	cacheCfg, err := config.LoadCache(configPath)
	if err != nil {
		return fmt.Errorf("cache config: %v", err)
	}

	ct, ok := backend.(mail.ChangeTracker)
	if !ok {
		return fmt.Errorf("backend does not implement mail.ChangeTracker")
	}
	acct, err := cache.Open(accts[0].Name, backend, ct, "", cache.Config{MaxSize: cacheCfg.MaxSize})
	if err != nil {
		return fmt.Errorf("open cache for %s: %v", accts[0].Name, err)
	}
	defer acct.Close()
	if err := acct.StartDrainer(ctx); err != nil {
		return fmt.Errorf("start drainer: %v", err)
	}

	app := ui.NewApp(t, acct, uiCfg, iconSet, accts[0].Contacts, accts[0].Identities)

	p := tea.NewProgram(appModel{app: app}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
