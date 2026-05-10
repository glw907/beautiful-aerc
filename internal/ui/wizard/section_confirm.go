package wizard

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/config"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

type confirmSection struct {
	parent  *Model
	form    *huh.Form
	confirm bool
	written string
	err     error
}

func newConfirmSection(parent *Model) *confirmSection {
	s := &confirmSection{parent: parent}
	s.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Write this config?").
			Affirmative("Write").
			Negative("Cancel").
			Value(&s.confirm),
	)).WithTheme(HuhTheme(parent.Theme))
	return s
}

func (s *confirmSection) Name() string { return "confirm" }

func (s *confirmSection) Init() tea.Cmd { return s.form.Init() }

func (s *confirmSection) Update(msg tea.Msg) (section, tea.Cmd) {
	updated, cmd := s.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		s.form = f
	}
	if s.form.State != huh.StateCompleted {
		return s, cmd
	}
	if !s.confirm {
		return s, func() tea.Msg { return CancelMsg{} }
	}
	if s.parent.Repair {
		cfg, err := wizdomain.Apply(s.parent.State)
		if err != nil {
			s.err = err
			return s, nil
		}
		s.parent.RepairResult = &cfg
		s.written = "(repair: caller will splice and write)"
		return s, func() tea.Msg { return FinishMsg{} }
	}
	path, err := writeConfig(s.parent.State)
	if err != nil {
		s.err = err
		return s, nil
	}
	s.written = path
	return s, func() tea.Msg { return FinishMsg{Path: path} }
}

func (s *confirmSection) View() string {
	st := s.parent.Styles
	if s.err != nil {
		return st.ProbeStepFail.Render("write failed: " + s.err.Error())
	}
	if s.written != "" {
		return st.ProbeStepOK.Render("Wrote " + s.written)
	}
	cfg, err := wizdomain.Apply(s.parent.State)
	if err != nil {
		return st.ProbeStepFail.Render("apply: " + err.Error())
	}
	body := string(config.Render([]config.AccountConfig{cfg}, config.DefaultUIConfig(), config.DefaultCacheConfig()))
	preview := st.Frame.Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, preview, "", s.form.View())
}

// writeConfig assembles the config and writes it atomically (tmp +
// rename in the same directory) so a partial write on crash doesn't
// leave a truncated file blocking the next launch.
func writeConfig(state wizdomain.Model) (string, error) {
	cfg, err := wizdomain.Apply(state)
	if err != nil {
		return "", fmt.Errorf("assemble config: %w", err)
	}
	path, err := config.Resolve("")
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if existing, err := os.Stat(path); err == nil && existing.Size() > 0 {
		return "", fmt.Errorf("%s already has content; refusing to overwrite", path)
	}
	body := config.Render([]config.AccountConfig{cfg}, config.DefaultUIConfig(), config.DefaultCacheConfig())
	tmp, err := os.CreateTemp(dir, ".config.toml.*")
	if err != nil {
		return "", fmt.Errorf("create tmp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return "", fmt.Errorf("chmod tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("rename to %s: %w", path, err)
	}
	return path, nil
}
