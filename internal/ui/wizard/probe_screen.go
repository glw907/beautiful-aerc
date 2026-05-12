package wizard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/ui/uicore"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

// probeScreen is the wizard's only non-huh form: a live spinner +
// step transcript that runs wizard.Probe in a tea.Cmd. On success it
// auto-emits AdvanceMsg after a short pause so the user sees the
// final ✓ before the screen swaps. On failure it offers retry / edit /
// save-and-quit hotkeys.
type probeScreen struct {
	parent  *Model
	cfg     config.AccountConfig
	spinner spinner.Model
	result  mail.ProbeResult
	done    bool
	failure bool
}

func newProbeScreen(parent *Model) *probeScreen {
	cfg, _ := wizdomain.Apply(parent.State)
	return &probeScreen{
		parent:  parent,
		cfg:     cfg,
		spinner: uicore.NewSpinner(parent.Theme),
	}
}

func (p *probeScreen) Init() tea.Cmd {
	return tea.Batch(p.spinner.Tick, p.runProbe())
}

func (p *probeScreen) oauthClient() *mailauth.Client {
	state := p.parent.State
	if !state.OAuthDone {
		return nil
	}
	preset := config.Providers[state.Preset]
	if preset.OAuth == nil {
		return nil
	}
	slug := cache.Slugify(state.Email)
	store, backend, err := mailauth.OpenStore(slug, oauthFallbackDir())
	if err != nil {
		return nil
	}
	cfg := mailauth.Config{
		ClientID:     state.OAuthCID,
		ClientSecret: state.OAuthSecret,
		AuthURL:      preset.OAuth.AuthURL,
		TokenURL:     preset.OAuth.TokenURL,
		Scopes:       preset.OAuth.Scopes,
	}
	return mailauth.NewClient(cfg, store, slug, backend)
}

func (p *probeScreen) runProbe() tea.Cmd {
	cfg := p.cfg
	oauthCli := p.oauthClient()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return ProbeDoneMsg{Result: wizdomain.Probe(ctx, cfg, oauthCli)}
	}
}

func (p *probeScreen) Update(msg tea.Msg) (*probeScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case ProbeDoneMsg:
		p.result = msg.Result
		p.done = true
		p.failure = !msg.Result.OK()
		if !p.failure {
			return p, tea.Tick(800*time.Millisecond, func(time.Time) tea.Msg { return AdvanceMsg{} })
		}
		return p, nil
	case tea.KeyPressMsg:
		if !p.done || !p.failure {
			break
		}
		switch msg.Code {
		case 'r':
			p.done, p.failure = false, false
			p.result = mail.ProbeResult{}
			return p, p.runProbe()
		case 'e':
			return p, func() tea.Msg { return BackMsg{} }
		case 's':
			return p, func() tea.Msg { return CancelMsg{} }
		}
	}
	var cmd tea.Cmd
	p.spinner, cmd = p.spinner.Update(msg)
	return p, cmd
}

func (p *probeScreen) View() string {
	st := p.parent.Styles
	var b strings.Builder
	switch p.cfg.Backend {
	case "jmap":
		b.WriteString("Testing JMAP connection")
	default:
		fmt.Fprintf(&b, "Testing connection to %s:%d", p.cfg.Host, p.cfg.Port)
	}
	b.WriteString("\n\n")

	for _, step := range p.result.Steps {
		var marker string
		switch step.Status {
		case mail.ProbeOK:
			marker = st.ProbeStepOK.Render("✓")
		case mail.ProbeFail:
			marker = st.ProbeStepFail.Render("✗")
		default:
			marker = st.ProbeStepPending.Render(p.spinner.View())
		}
		fmt.Fprintf(&b, "  %s  %s", marker, step.Label)
		if step.Detail != "" {
			b.WriteString(st.Help.Render("  " + step.Detail))
		}
		b.WriteString("\n")
	}

	if !p.done {
		fmt.Fprintf(&b, "  %s  %s\n", st.ProbeStepPending.Render(p.spinner.View()), st.Help.Render("running…"))
	}

	if p.done && p.failure {
		b.WriteString("\n")
		b.WriteString(st.Help.Render("[r] retry   [e] edit credentials   [s] save and quit"))
	}
	if p.done && !p.failure {
		b.WriteString("\n")
		b.WriteString(st.ProbeStepOK.Render("Connected. Continuing…"))
	}

	return lipgloss.NewStyle().PaddingLeft(2).Render(b.String())
}
