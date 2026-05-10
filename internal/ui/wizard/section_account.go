package wizard

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/glw907/poplar/internal/config"
	mailpkg "github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

type accountStage int

const (
	stageProvider accountStage = iota
	stageEmail
	stageCredentials
	stageProbe
	stageIdentity
	stageLabel
	stageDone
)

// accountSection drives the wizard's account flow as a state machine
// over a chain of huh.Forms plus one custom probe screen. Each form
// binds directly to the parent Model.State fields; advance happens
// when huh reports StateCompleted.
type accountSection struct {
	parent   *Model
	stage    accountStage
	form     *huh.Form
	probe    *probeScreen
	oauthSub *oauthSection
}

func newAccountSection(parent *Model) *accountSection {
	s := &accountSection{parent: parent, stage: stageProvider}
	s.buildForm()
	return s
}

func (s *accountSection) Name() string { return "account" }

func (s *accountSection) buildForm() {
	state := &s.parent.State
	switch s.stage {
	case stageProvider:
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose your mail provider").
				Options(providerOptions()...).
				Value(&state.Preset),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageEmail:
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("What's your email address?").
				Validate(validateEmail).
				Value(&state.Email),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageCredentials:
		strategy, err := wizdomain.SelectStrategy(state.Preset)
		if err != nil {
			s.form = errorForm(err.Error())
			return
		}
		if strategy == config.StrategyOAuth {
			s.oauthSub = newOAuthSection(s.parent)
			s.form = nil
			return
		}
		s.oauthSub = nil
		s.form = credentialsForm(strategy, state, s.parent.Theme)

	case stageProbe:
		s.probe = newProbeScreen(s.parent)
		s.form = nil

	case stageIdentity:
		if state.IdentityName == "" {
			state.IdentityName = defaultIdentityName(state.Email)
		}
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Display name on outgoing mail").
				Description("Shown to recipients on the From: header.").
				Value(&state.IdentityName),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageLabel:
		if state.AccountLabel == "" {
			state.AccountLabel = defaultAccountLabel(state.Preset, state.Email)
		}
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Sidebar label for this account").
				Description("Shown in the sidebar account header.").
				Value(&state.AccountLabel),
		)).WithTheme(HuhTheme(s.parent.Theme))
	}
}

func (s *accountSection) Init() tea.Cmd {
	if s.form != nil {
		return s.form.Init()
	}
	if s.probe != nil {
		return s.probe.Init()
	}
	if s.oauthSub != nil {
		return s.oauthSub.Init()
	}
	return nil
}

func (s *accountSection) Update(msg tea.Msg) (section, tea.Cmd) {
	if _, ok := msg.(BackMsg); ok {
		s.stage = stageCredentials
		s.probe = nil
		s.oauthSub = nil
		s.buildForm()
		if s.form != nil {
			return s, s.form.Init()
		}
		if s.oauthSub != nil {
			return s, s.oauthSub.Init()
		}
		return s, nil
	}

	if s.oauthSub != nil {
		updated, cmd := s.oauthSub.Update(msg)
		if os, ok := updated.(*oauthSection); ok {
			s.oauthSub = os
		}
		if _, ok := msg.(oauthAdvanceMsg); ok {
			s.oauthSub = nil
			s.advance()
			if s.stage == stageDone {
				return s, tea.Batch(cmd, func() tea.Msg { return AdvanceMsg{} })
			}
			return s, tea.Batch(cmd, s.Init())
		}
		return s, cmd
	}

	if s.probe != nil {
		_, cmd := s.probe.Update(msg)
		if done, ok := msg.(ProbeDoneMsg); ok && done.Result.OK() {
			s.advance()
			if s.stage == stageDone {
				return s, tea.Batch(cmd, func() tea.Msg { return AdvanceMsg{} })
			}
			return s, tea.Batch(cmd, s.Init())
		}
		return s, cmd
	}

	if s.form == nil {
		return s, nil
	}
	updated, cmd := s.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		s.form = f
	}
	if s.form.State == huh.StateCompleted {
		s.advance()
		if s.stage == stageDone {
			return s, tea.Batch(cmd, func() tea.Msg { return AdvanceMsg{} })
		}
		return s, tea.Batch(cmd, s.Init())
	}
	return s, cmd
}

func (s *accountSection) advance() {
	if s.stage < stageDone {
		s.stage++
	}
	if s.stage == stageDone {
		return
	}
	s.buildForm()
}

func (s *accountSection) View() string {
	if s.stage == stageDone {
		return ""
	}
	if s.probe != nil {
		return s.probe.View()
	}
	if s.oauthSub != nil {
		return s.oauthSub.View()
	}
	if s.form == nil {
		return ""
	}
	return s.form.View()
}

func providerOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Fastmail              JMAP, paste an API token", "fastmail"),
		huh.NewOption("Gmail                 OAuth, app approval required", "gmail"),
		huh.NewOption("Outlook / Microsoft   OAuth, app approval required", "outlook"),
		huh.NewOption("iCloud                IMAP, app password", "icloud"),
		huh.NewOption("Yahoo                 IMAP, app password", "yahoo"),
		huh.NewOption("Zoho                  IMAP, app password", "zoho"),
		huh.NewOption("Mailbox.org           IMAP, app password", "mailbox-org"),
		huh.NewOption("Posteo                IMAP, app password", "posteo"),
		huh.NewOption("Runbox                IMAP, app password", "runbox"),
		huh.NewOption("GMX                   IMAP, app password", "gmx"),
		huh.NewOption("ProtonMail (Bridge)   local IMAP, Bridge required", "protonmail"),
		huh.NewOption("Other / self-hosted IMAP", "imap"),
		huh.NewOption("Other / self-hosted JMAP", "jmap"),
	}
}

func validateEmail(s string) error {
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("invalid email: %v", err)
	}
	return nil
}

func defaultIdentityName(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return ""
	}
	local := email[:at]
	return strings.ToUpper(local[:1]) + local[1:]
}

func defaultAccountLabel(preset, email string) string {
	if p, ok := config.Providers[preset]; ok && p.Backend != "" {
		return strings.ToUpper(preset[:1]) + preset[1:]
	}
	if at := strings.IndexByte(email, '@'); at >= 0 {
		return email[at+1:]
	}
	return email
}

func credentialsForm(strategy config.CredentialStrategy, state *wizdomain.Model, t *theme.CompiledTheme) *huh.Form {
	switch strategy {
	case config.StrategyAppPassword:
		preset := config.Providers[state.Preset]
		return tokenForm(t,
			"App password required",
			fmt.Sprintf("Generate one at:\n  %s", preset.HelpURL),
			"App password",
			&state.Password,
		)

	case config.StrategyAPIToken:
		preset := config.Providers[state.Preset]
		return tokenForm(t,
			"API token",
			fmt.Sprintf("Generate one at:\n  %s\n\nTokens don't expire unless revoked.", preset.HelpURL),
			"API token",
			&state.Token,
		)

	case config.StrategyPlainIMAP:
		if state.Port == "" {
			state.Port = "993"
		}
		return huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Host").Value(&state.Host),
			huh.NewInput().Title("Port").Validate(validatePort).Value(&state.Port),
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&state.Password),
		), huh.NewGroup(
			huh.NewConfirm().
				Title("Skip TLS verify? (only for self-signed certs)").
				Value(&state.InsecureTLS),
		).WithHideFunc(func() bool { return !mailpkg.IsSelfHosted(state.Host) }),
		).WithTheme(HuhTheme(t))

	case config.StrategyPlainJMAP:
		return huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Session URL").Value(&state.SessionURL),
			huh.NewInput().Title("Bearer token").EchoMode(huh.EchoModePassword).Value(&state.Token),
		), huh.NewGroup(
			huh.NewConfirm().
				Title("Skip TLS verify? (only for self-signed certs)").
				Value(&state.InsecureTLS),
		).WithHideFunc(func() bool { return !urlIsSelfHosted(state.SessionURL) }),
		).WithTheme(HuhTheme(t))
	}
	return errorForm("no credential strategy")
}

// tokenForm is the shared shape of every "paste this single secret"
// credential page (app password, API token).
func tokenForm(t *theme.CompiledTheme, noteTitle, noteDesc, inputTitle string, value *string) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(noteTitle).Description(noteDesc),
		huh.NewInput().
			Title(inputTitle).
			EchoMode(huh.EchoModePassword).
			Value(value),
	)).WithTheme(HuhTheme(t))
}

func errorForm(msg string) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewNote().Title("Wizard error").Description(msg),
	))
}

func validatePort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port out of range")
	}
	return nil
}

func urlIsSelfHosted(rawURL string) bool {
	for _, prefix := range []string{"https://", "http://"} {
		if !strings.HasPrefix(rawURL, prefix) {
			continue
		}
		rest := rawURL[len(prefix):]
		if i := strings.IndexAny(rest, "/:"); i >= 0 {
			rest = rest[:i]
		}
		return mailpkg.IsSelfHosted(rest)
	}
	return false
}
