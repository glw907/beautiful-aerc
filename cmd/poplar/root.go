package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/term"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/uicore"
	uiwiz "github.com/glw907/poplar/internal/ui/wizard"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	config   string
	theme    string
	noWizard bool
	repair   string
	reauth   string
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
	cmd.Flags().BoolVar(&f.noWizard, "no-wizard", false,
		"don't auto-launch the first-run wizard; exit 78 instead")
	cmd.Flags().StringVar(&f.repair, "repair", "",
		"repair the named account interactively")
	cmd.Flags().StringVar(&f.reauth, "reauth", "",
		"re-run OAuth consent for the named account and store the new refresh token")
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

func (m appModel) View() tea.View { return m.app.View() }

func runRoot(f rootFlags) error {
	t, ok := theme.Themes[strings.ToLower(f.theme)]
	if !ok {
		return fmt.Errorf("unknown theme %q (available: %s)",
			f.theme, strings.Join(theme.ThemeNames(), ", "))
	}

	if f.repair != "" && f.reauth != "" {
		return fmt.Errorf("--repair and --reauth are mutually exclusive")
	}
	if f.repair != "" {
		return runRepair(f)
	}
	if f.reauth != "" {
		return runReauth(f)
	}

	accts, configPath, err := config.Load(f.config)
	if errors.Is(err, config.ErrFirstRun) {
		if f.noWizard || os.Getenv("POPLAR_NO_WIZARD") != "" {
			fmt.Fprintln(os.Stderr, err.Error())
			fmt.Fprintln(os.Stderr, "Edit the file and run poplar again.")
			os.Exit(78)
		}
		// ErrFirstRun wrote a template; clear it so the wizard's confirm
		// step (which refuses to overwrite a non-empty file) can write
		// the user's choices instead.
		_ = os.Remove(configPath)
		if err := runWizardAutoLaunch(t, configPath); err != nil {
			return err
		}
		accts, configPath, err = config.Load(f.config)
		if err != nil {
			return fmt.Errorf("post-wizard config load: %v", err)
		}
	}
	if errors.Is(err, config.ErrOldAccountsToml) {
		fmt.Fprintln(os.Stderr, "poplar: "+err.Error())
		fmt.Fprintln(os.Stderr, "  poplar 1.0 reads config.toml; rename your accounts.toml file.")
		os.Exit(78)
	}
	var ce *config.ConfigError
	if errors.As(err, &ce) {
		fmt.Fprintln(os.Stderr, "poplar:", ce.Error())
		if ce.Account != "" {
			fmt.Fprintf(os.Stderr,
				"Run `poplar --repair=%s` to fix this account interactively.\n", ce.Account)
		}
		fmt.Fprintln(os.Stderr, "Or edit the file by hand and rerun poplar.")
		os.Exit(78)
	}
	if err != nil {
		return fmt.Errorf("load accounts: %v", err)
	}
	if len(accts) == 0 {
		return fmt.Errorf("no accounts configured; see %s", configPath)
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
	measurer := ansix.NewMeasurer(cellWidth)

	cacheCfg, err := config.LoadCache(configPath)
	if err != nil {
		return fmt.Errorf("cache config: %v", err)
	}

	ct, ok := backend.(mail.ChangeTracker)
	if !ok {
		return fmt.Errorf("backend does not implement mail.ChangeTracker")
	}
	acct, err := cache.Open(accts[0].Name, backend, ct, "", cache.Config{MaxSize: cacheCfg.MaxSize}, nil)
	if err != nil {
		return fmt.Errorf("open cache for %s: %v", accts[0].Name, err)
	}
	defer acct.Close()
	if err := acct.StartDrainer(ctx); err != nil {
		return fmt.Errorf("start drainer: %v", err)
	}

	app := ui.NewApp(t, acct, uiCfg, iconSet, measurer, accts[0].Contacts, accts[0].Identities)

	p := tea.NewProgram(appModel{app: app})
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// runWizardAutoLaunch runs the first-run wizard with the default
// section registry. The wizard writes the final config itself.
func runWizardAutoLaunch(t *theme.CompiledTheme, configPath string) error {
	model := uiwiz.NewModel(t)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("wizard: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("wizard exited without writing %s", configPath)
	}
	return nil
}

// runRepair launches the wizard's account section pre-populated from
// the named account, then splices the result back into the existing
// config and atomically rewrites it.
func runRepair(f rootFlags) error {
	t := theme.Themes[strings.ToLower(f.theme)]
	if t == nil {
		t = theme.OneDark
	}

	accts, configPath, err := config.Load(f.config)
	// A ConfigError on load is exactly the case --repair exists to fix:
	// re-collect the broken account's fields.
	var ce *config.ConfigError
	if err != nil && !errors.As(err, &ce) {
		return err
	}
	idx := -1
	for i, a := range accts {
		if a.Name == f.repair {
			idx = i
			break
		}
	}
	var seed config.AccountConfig
	if idx >= 0 {
		seed = accts[idx]
	} else if ce != nil && ce.Account == f.repair {
		seed = config.AccountConfig{Name: f.repair}
	} else {
		return fmt.Errorf("account %q not found in %s", f.repair, configPath)
	}

	model := uiwiz.NewModel(t).WithRepair(f.repair, seed)
	res, err := tea.NewProgram(model).Run()
	if err != nil {
		return fmt.Errorf("repair: %v", err)
	}
	final, ok := res.(uiwiz.Model)
	if !ok || final.RepairResult == nil {
		// User cancelled.
		return nil
	}
	if idx >= 0 {
		accts[idx] = *final.RepairResult
	} else {
		accts = append(accts, *final.RepairResult)
	}
	uiCfg, _ := config.LoadUI(configPath)
	cacheCfg, _ := config.LoadCache(configPath)
	body := config.Render(accts, uiCfg, cacheCfg)
	if err := writeConfigAtomic(configPath, body); err != nil {
		return fmt.Errorf("write %s: %v", configPath, err)
	}
	fmt.Fprintf(os.Stderr, "repaired %s in %s\n", f.repair, configPath)
	return nil
}

func writeConfigAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config.toml.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
