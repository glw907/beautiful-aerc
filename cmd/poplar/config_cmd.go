package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/mailimap"
	"github.com/glw907/poplar/internal/theme"
	uiwizard "github.com/glw907/poplar/internal/ui/wizard"
	"github.com/spf13/cobra"
)

// configFlagPath returns the root command's --config flag value, or
// "" when the flag is not registered (subcommand-isolated tests).
func configFlagPath(cmd *cobra.Command) string {
	root := cmd.Root()
	if root == nil {
		return ""
	}
	f := root.PersistentFlags().Lookup("config")
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func newConfigInitTemplateCmd() *cobra.Command {
	var (
		force       bool
		interactive bool
		sections    string
	)
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Write a fresh config (template by default; --interactive runs the wizard)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			path, err := config.Resolve(flagPath)
			if err != nil {
				return err
			}
			if interactive {
				return runWizard(cmd, path, sections)
			}
			if !force {
				if _, statErr := os.Stat(path); statErr == nil {
					return fmt.Errorf("%s already exists; use --force to overwrite", path)
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(config.Template()), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "run the interactive setup wizard")
	cmd.Flags().StringVar(&sections, "section", "", "comma-separated section names to run (account,theme,confirm)")
	return cmd
}

func runWizard(cmd *cobra.Command, path, sections string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; remove it before running the wizard", path)
	}
	model := uiwizard.NewModel(theme.OneDark)
	if sections != "" {
		model = model.WithSections(strings.Split(sections, ","))
	}
	prog := tea.NewProgram(model,
		tea.WithContext(cmd.Context()),
		tea.WithInput(cmd.InOrStdin()),
		tea.WithOutput(cmd.OutOrStdout()),
	)
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}
	return nil
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "path",
		Short:        "Print the resolved config-file path",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			path, err := config.Resolve(flagPath)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
}

func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "check",
		Short:        "Validate config and test each account's connection",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagPath := configFlagPath(cmd)
			accounts, _, err := config.Load(flagPath)
			if err != nil {
				return err
			}
			anyFail := false
			for _, a := range accounts {
				b, err := openBackend(a)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s error: %v\n", a.Name, err)
					anyFail = true
					continue
				}
				if err := b.Connect(cmd.Context()); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s error: %v\n", a.Name, err)
					anyFail = true
					continue
				}
				_ = b.Disconnect()
				if a.Backend == "imap" {
					var oauthCli *mailauth.Client
					if a.OAuth != nil {
						c, err := buildOAuthClient(a)
						if err != nil {
							fmt.Fprintf(cmd.OutOrStdout(), "%-20s oauth error: %v\n", a.Name, err)
							anyFail = true
							continue
						}
						oauthCli = c
					}
					if err := mailimap.ProbeSMTP(a, oauthCli); err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "%-20s smtp error: %v\n", a.Name, err)
						anyFail = true
						continue
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s OK\n", a.Name)
			}
			if anyFail {
				return fmt.Errorf("one or more accounts failed")
			}
			return nil
		},
	}
}
