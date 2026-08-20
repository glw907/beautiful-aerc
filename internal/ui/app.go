package ui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
)

// Deps is the read-only wiring ui.NewApp needs from the rest of the
// process. The UI layer never writes the store (the writer/intent
// rule), so Store is a read pool handle, never a *store.Writer.
type Deps struct {
	Store   *store.ReadPool
	Theme   theme.Theme
	Profile theme.Profile
	Account string
}

// App is poplar's root bubbletea model (technical design section 12):
// the active surface, the screen stack help and modals push onto, and
// the one LayoutMode every child consumes, recomputed once per
// tea.WindowSizeMsg. It performs no store write and no network I/O;
// every user action beyond surface switching and the screen stack
// enqueues an intent for a later task to carry.
type App struct {
	theme   theme.Theme
	profile theme.Profile
	account string

	active AccountScoped[Surface]
	stack  []Screen
	layout LayoutMode

	mail     MailPlaceholder
	calendar CalendarPlaceholder
	contacts ContactsPlaceholder
	config   ConfigPlaceholder
}

// NewApp returns poplar's root model, wired against deps.
func NewApp(deps Deps) App {
	return App{
		theme:    deps.Theme,
		profile:  deps.Profile,
		account:  deps.Account,
		mail:     newMailPlaceholder(deps.Store, deps.Theme),
		calendar: newCalendarPlaceholder(deps.Store, deps.Theme),
		contacts: newContactsPlaceholder(deps.Theme),
		config:   newConfigPlaceholder(deps.Theme),
	}
}

// NewProgram returns a *tea.Program running app, with the wheel
// coalescer (ADR-0017, machine design section 8) installed as the
// program-construction tea.WithFilter option.
func NewProgram(app App, opts ...tea.ProgramOption) *tea.Program {
	opts = append([]tea.ProgramOption{tea.WithFilter(newWheelFilter(time.Now))}, opts...)
	return tea.NewProgram(app, opts...)
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return tea.Batch(a.mail.Init(), a.calendar.Init(), a.contacts.Init(), a.config.Init(), QueryBackgroundColor())
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.layout = ComputeLayout(msg.Width, msg.Height, false)
		return a.updateChildren(LayoutMsg{Layout: a.layout})
	case BackgroundColorTimeoutMsg:
		slog.Debug("background-color query unanswered; staying on the default dark theme")
		return a, nil
	case tea.BackgroundColorMsg:
		a.theme = theme.New(msg.IsDark(), a.profile)
		return a.updateChildren(ThemeMsg{Theme: a.theme})
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}
	return a.updateChildren(msg)
}

// View implements tea.Model.
func (a App) View() tea.View {
	return a.activeScreen().View()
}

// handleKey applies the interaction grammar's back/quit/surface-switch
// precedence (design language section 2) before anything on the
// screen stack sees the key: Esc always pops the stack first (or
// no-ops at a surface root, this pass); a digit switches surfaces
// only when the state currently in front (the stack's top, or the
// active surface's own root state when the stack is empty) is
// StateDigitsSwitch, so a modal on the stack answers or eats a digit
// instead (UX-4's acceptance criterion); q quits only at a surface
// root.
func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, GrammarKeys.Back) {
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	}

	front := a.activeScreen().Entry()
	if len(a.stack) > 0 {
		front = a.stack[len(a.stack)-1].Entry()
	}
	if s, ok := matchDigit(msg, front.SwitchState); ok {
		a.active.Set(a.account, s)
		return a, nil
	}

	if len(a.stack) == 0 {
		if key.Matches(msg, GrammarKeys.Quit) {
			return a, tea.Quit
		}
		return a, nil
	}

	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	if screen, ok := updated.(Screen); ok {
		a.stack[len(a.stack)-1] = screen
	}
	return a, cmd
}

// updateChildren delegates msg to every surface screen, regardless of
// which is active, so a screen off-screen this frame still absorbs a
// layout or theme change and a load result addressed to it (UX-4's
// round-trip: switching away and back must not have missed anything).
func (a App) updateChildren(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.mail = a.mail.update(msg)
	a.calendar = a.calendar.update(msg)
	a.contacts = a.contacts.update(msg)
	a.config = a.config.update(msg)
	return a, nil
}

// activeSurface returns the surface currently in front for a.account.
func (a App) activeSurface() Surface {
	return a.active.Get(a.account)
}

// activeScreen returns whichever surface screen a.activeSurface
// names.
func (a App) activeScreen() Screen {
	switch a.activeSurface() {
	case SurfaceCalendar:
		return a.calendar
	case SurfaceContacts:
		return a.contacts
	case SurfaceConfig:
		return a.config
	default:
		return a.mail
	}
}

// digitSurfaceKeys is the surface-switch digit keymap, one
// key.Binding per digit so a match also names the Surface it
// switches to; GrammarKeys.SurfaceSwitch bundles the same four keys
// under one Binding for advertisement, which cannot report which
// digit fired.
type digitSurfaceKeys struct {
	Mail, Calendar, Contacts, Config key.Binding
}

var digitKeys = digitSurfaceKeys{
	Mail:     key.NewBinding(key.WithKeys("1")),
	Calendar: key.NewBinding(key.WithKeys("2")),
	Contacts: key.NewBinding(key.WithKeys("3")),
	Config:   key.NewBinding(key.WithKeys("4")),
}

// matchDigit reports the Surface msg names among digitKeys. It gates
// legality through SetEnabled rather than an inline SwitchState
// check, mirroring GrammarKeys.Back and GrammarKeys.Quit's own
// key.Matches dispatch above: state != StateDigitsSwitch disables the
// whole set before key.Matches ever runs, so key.Matches itself is
// what turns a digit into a no-op in a StateModal front (UX-4's
// acceptance criterion) rather than a second, hand-checked condition.
func matchDigit(msg tea.KeyPressMsg, state StateClass) (Surface, bool) {
	keys := digitKeys
	enabled := state == StateDigitsSwitch
	keys.Mail.SetEnabled(enabled)
	keys.Calendar.SetEnabled(enabled)
	keys.Contacts.SetEnabled(enabled)
	keys.Config.SetEnabled(enabled)

	switch {
	case key.Matches(msg, keys.Mail):
		return SurfaceMail, true
	case key.Matches(msg, keys.Calendar):
		return SurfaceCalendar, true
	case key.Matches(msg, keys.Contacts):
		return SurfaceContacts, true
	case key.Matches(msg, keys.Config):
		return SurfaceConfig, true
	default:
		return 0, false
	}
}
