package wizard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/ui/uicore"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

type oauthStage int

const (
	oauthStageCredentials oauthStage = iota
	oauthStageFlow
	oauthStageDone
)

type oauthDoneMsg struct {
	backend mailauth.Backend
	err     error
}

// oauthAdvanceMsg signals the account section that the OAuth sub-flow
// completed and the stage machine should advance to probe.
type oauthAdvanceMsg struct{}

// oauthSection drives the OAuth consent flow: a credentials form
// (client_id + client_secret), then a live spinner while Authorize runs,
// then success or failure state with retry.
type oauthSection struct {
	parent       *Model
	stage        oauthStage
	form         *huh.Form
	spinner      spinner.Model
	clientID     string
	clientSecret string
	authErr      string
}

func newOAuthSection(parent *Model) *oauthSection {
	s := &oauthSection{
		parent:  parent,
		stage:   oauthStageCredentials,
		spinner: uicore.NewSpinner(parent.Theme),
	}
	s.buildForm()
	return s
}

func (s *oauthSection) Name() string { return "oauth" }

func (s *oauthSection) buildForm() {
	preset := config.Providers[s.parent.State.Preset]
	helpURL := preset.HelpURL
	if helpURL == "" {
		helpURL = "(no URL — see your provider's developer console)"
	}
	note := fmt.Sprintf("Register an OAuth application at:\n  %s\n\nThen paste the client ID and secret below.", helpURL)
	s.form = huh.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("OAuth credentials").
			Description(note),
		huh.NewInput().
			Title("Client ID").
			Value(&s.clientID),
		huh.NewInput().
			Title("Client secret").
			EchoMode(huh.EchoModePassword).
			Value(&s.clientSecret),
	)).WithTheme(HuhTheme(s.parent.Theme))
}

func (s *oauthSection) Init() tea.Cmd {
	if s.stage == oauthStageCredentials {
		return s.form.Init()
	}
	return nil
}

func (s *oauthSection) Update(msg tea.Msg) (section, tea.Cmd) {
	switch s.stage {
	case oauthStageCredentials:
		return s.updateCredentials(msg)
	case oauthStageFlow:
		return s.updateFlow(msg)
	}
	return s, nil
}

func (s *oauthSection) updateCredentials(msg tea.Msg) (section, tea.Cmd) {
	updated, cmd := s.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		s.form = f
	}
	if s.form.State == huh.StateCompleted {
		s.stage = oauthStageFlow
		s.authErr = ""
		return s, tea.Batch(s.spinner.Tick, s.runAuthorize())
	}
	return s, cmd
}

func (s *oauthSection) updateFlow(msg tea.Msg) (section, tea.Cmd) {
	switch msg := msg.(type) {
	case oauthDoneMsg:
		if msg.err != nil {
			s.authErr = msg.err.Error()
			return s, nil
		}
		s.parent.State.OAuthDone = true
		s.parent.State.OAuthStore = string(msg.backend)
		s.parent.State.OAuthCID = s.clientID
		s.parent.State.OAuthSecret = s.clientSecret
		s.stage = oauthStageDone
		return s, func() tea.Msg { return oauthAdvanceMsg{} }

	case tea.KeyPressMsg:
		if s.authErr != "" {
			switch msg.Code {
			case 'r':
				s.authErr = ""
				s.stage = oauthStageCredentials
				s.buildForm()
				return s, s.form.Init()
			case 's':
				return s, func() tea.Msg { return CancelMsg{} }
			}
		}
	}
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s *oauthSection) runAuthorize() tea.Cmd {
	preset := s.parent.State.Preset
	email := s.parent.State.Email
	cid := s.clientID
	secret := s.clientSecret
	p, _ := config.Providers[preset]

	return func() tea.Msg {
		slug := cache.Slugify(email)
		fallbackDir := oauthFallbackDir()
		store, backend, err := mailauth.OpenStore(slug, fallbackDir)
		if err != nil {
			return oauthDoneMsg{err: fmt.Errorf("open token store: %w", err)}
		}

		cfg := mailauth.Config{
			ClientID:     cid,
			ClientSecret: secret,
		}
		if p.OAuth != nil {
			cfg.AuthURL = p.OAuth.AuthURL
			cfg.TokenURL = p.OAuth.TokenURL
			cfg.Scopes = p.OAuth.Scopes
		}

		cli := mailauth.NewClient(cfg, store, slug, backend)
		strategy := wizdomain.NewOAuthStrategy(cli, preset, email, cid, secret)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, err := strategy.Apply(ctx); err != nil {
			return oauthDoneMsg{err: err}
		}
		return oauthDoneMsg{backend: backend}
	}
}

func (s *oauthSection) View() string {
	st := s.parent.Styles
	switch s.stage {
	case oauthStageCredentials:
		return s.form.View()
	case oauthStageFlow:
		if s.authErr != "" {
			var b strings.Builder
			b.WriteString(st.ProbeStepFail.Render("OAuth failed: " + s.authErr))
			b.WriteString("\n\n")
			b.WriteString(st.Help.Render("[r] retry   [s] cancel"))
			return lipgloss.NewStyle().PaddingLeft(2).Render(b.String())
		}
		return lipgloss.NewStyle().PaddingLeft(2).Render(
			s.spinner.View() + "  " + st.Help.Render("Waiting for browser consent…"),
		)
	}
	return ""
}

// oauthFallbackDir returns the XDG-aware directory for age-encrypted
// token files, creating it if absent.
func oauthFallbackDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "poplar", "oauth")
}
