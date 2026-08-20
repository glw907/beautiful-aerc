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

// wheelGesture is App's own open wheel-coalescing gesture (ADR-0017
// revision 3): the running signed sum, the coordinates of its opening
// tick, and a generation counter that tells a stale flush timer (one
// whose gesture a direction flip already closed) from the live one.
type wheelGesture struct {
	open bool
	gen  int
	x, y int
	sum  int
}

// App is poplar's root bubbletea model (technical design section 12):
// the active surface, the screen stack help and modals push onto, and
// the one LayoutMode every child consumes, recomputed once per
// tea.WindowSizeMsg. It performs no store write and no network I/O;
// every user action beyond surface switching and the screen stack
// enqueues an intent for a later task to carry.
type App struct {
	theme      theme.Theme
	profile    theme.Profile
	account    string
	bgAnswered bool

	active AccountScoped[Surface]
	stack  []Screen
	layout LayoutMode
	wheel  wheelGesture

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

// NewProgram returns a *tea.Program running app.
func NewProgram(app App, opts ...tea.ProgramOption) *tea.Program {
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
		// DefaultDark already governs every frame rendered so far; an
		// answer that arrived first (a.bgAnswered) means this timeout
		// fired after the fact and carries nothing worth logging.
		if !a.bgAnswered {
			slog.Debug("background-color query unanswered; staying on the default dark theme")
		}
		return a, nil
	case tea.BackgroundColorMsg:
		a.bgAnswered = true
		a.theme = theme.New(msg.IsDark(), a.profile)
		return a.updateChildren(ThemeMsg{Theme: a.theme})
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.MouseWheelMsg:
		return a.handleWheel(msg)
	case wheelFlushMsg:
		return a.flushWheelTimer(msg)
	}
	return a.updateChildren(msg)
}

// View implements tea.Model. It renders the top of the screen stack
// when one is pushed, else the active surface: the minimal correct
// behavior until task 8 adds compositing the dimmed surface behind a
// stacked screen.
func (a App) View() tea.View {
	if len(a.stack) > 0 {
		return a.stack[len(a.stack)-1].View()
	}
	return a.activeScreen().View()
}

// handleKey applies the interaction grammar's back/quit/surface-switch
// precedence (design language section 2) before anything else sees
// the key: Esc always pops the stack first (or no-ops at a surface
// root, this pass); a digit switches surfaces only when the state
// currently in front (the stack's top, or the active surface's own
// root state when the stack is empty) is StateDigitsSwitch, so a
// modal on the stack answers or eats a digit instead (UX-4's
// acceptance criterion); q quits only at a surface root; anything
// else at a surface root reaches the active screen's own Update.
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
		return a.updateActive(msg)
	}

	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	if screen, ok := updated.(Screen); ok {
		a.stack[len(a.stack)-1] = screen
	}
	return a, cmd
}

// handleWheel folds msg into the open wheel gesture, or opens a new
// one (ADR-0017 revision 3): the coalescing decision lives on the
// root model rather than a program-construction filter, so a flush
// is never stranded waiting on a tick that may not arrive. Opening a
// gesture arms a tea.Tick(wheelWindow) flush timer (armWheelFlush); a
// same-direction tick within the window folds into the running sum;
// an opposite-direction tick flushes the open gesture immediately as
// a WheelMsg and opens a fresh one carrying the flipping tick,
// batched with that gesture's own flush timer.
func (a App) handleWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	delta := wheelDelta(msg.Button)
	if delta == 0 {
		return a, nil
	}

	if !a.wheel.open {
		a.wheel = openWheelGesture(a.wheel.gen, msg, delta)
		return a, armWheelFlush(a.wheel.gen)
	}

	if wheelSign(delta) == wheelSign(a.wheel.sum) {
		a.wheel.sum += delta
		return a, nil
	}

	flush := flushWheelCmd(a.wheel)
	a.wheel = openWheelGesture(a.wheel.gen, msg, delta)
	return a, tea.Batch(flush, armWheelFlush(a.wheel.gen))
}

// flushWheelTimer absorbs a wheelFlushMsg: the flush timer armed when
// a.wheel's gesture opened. A stale timer, one whose gen no longer
// matches a.wheel.gen because a direction flip already flushed and
// replaced that gesture, is silently ignored; otherwise the gesture
// flushes as one WheelMsg carrying its own opening tick's
// coordinates.
func (a App) flushWheelTimer(msg wheelFlushMsg) (tea.Model, tea.Cmd) {
	if !a.wheel.open || msg.gen != a.wheel.gen {
		return a, nil
	}
	flush := flushWheelCmd(a.wheel)
	a.wheel = wheelGesture{}
	return a, flush
}

// wheelFlushMsg is the flush timer's own tick, armed once per gesture
// at open time (ADR-0017 revision 3). gen names which gesture it
// closes, so a stale timer from a gesture a direction flip already
// flushed is recognizable and ignored.
type wheelFlushMsg struct {
	gen int
}

// openWheelGesture starts a new gesture from msg's first tick,
// generation prevGen+1 so a still-pending flush timer from whatever
// gesture prevGen named reads as stale against it.
func openWheelGesture(prevGen int, msg tea.MouseWheelMsg, delta int) wheelGesture {
	return wheelGesture{open: true, gen: prevGen + 1, x: msg.X, y: msg.Y, sum: delta}
}

// armWheelFlush returns the Cmd that closes gen's gesture after
// wheelWindow, unconditionally: the mechanism that guarantees no
// gesture, including a single isolated detent, is ever stranded
// waiting for a tick that never arrives.
func armWheelFlush(gen int) tea.Cmd {
	return tea.Tick(wheelWindow, func(time.Time) tea.Msg {
		return wheelFlushMsg{gen: gen}
	})
}

// flushWheelCmd returns g's accumulated sum as one WheelMsg, at the
// coordinates of g's own opening tick.
func flushWheelCmd(g wheelGesture) tea.Cmd {
	return func() tea.Msg {
		return WheelMsg{X: g.x, Y: g.y, Delta: g.sum}
	}
}

// updateActive delegates msg to whichever surface screen
// a.activeSurface names (an ordinary key at a surface root previously
// had nowhere to go once back/digit/quit handling found nothing to
// do with it).
func (a App) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.activeSurface() {
	case SurfaceCalendar:
		a.calendar, cmd = a.calendar.update(msg)
	case SurfaceContacts:
		a.contacts, cmd = a.contacts.update(msg)
	case SurfaceConfig:
		a.config, cmd = a.config.update(msg)
	default:
		a.mail, cmd = a.mail.update(msg)
	}
	return a, cmd
}

// updateChildren delegates msg to every surface screen, regardless of
// which is active, so a screen off-screen this frame still absorbs a
// layout or theme change and a load result addressed to it (UX-4's
// round-trip: switching away and back must not have missed anything).
func (a App) updateChildren(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	a.mail, cmd = a.mail.update(msg)
	cmds = append(cmds, cmd)
	a.calendar, cmd = a.calendar.update(msg)
	cmds = append(cmds, cmd)
	a.contacts, cmd = a.contacts.update(msg)
	cmds = append(cmds, cmd)
	a.config, cmd = a.config.update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
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

// matchDigit reports the Surface msg names, derived from
// GrammarKeys.SurfaceSwitch itself rather than a parallel keymap that
// could drift from the grammar's own binding: key.Matches gates
// legality (state other than StateDigitsSwitch disables the whole
// binding before matching runs), and surfaceForDigit resolves which
// of the bundled keys actually matched.
func matchDigit(msg tea.KeyPressMsg, state StateClass) (Surface, bool) {
	binding := GrammarKeys.SurfaceSwitch
	binding.SetEnabled(state == StateDigitsSwitch)
	if !key.Matches(msg, binding) {
		return 0, false
	}
	return surfaceForDigit(msg.String())
}

// surfaceForDigit reports the Surface a surface-switch digit names.
func surfaceForDigit(s string) (Surface, bool) {
	switch s {
	case "1":
		return SurfaceMail, true
	case "2":
		return SurfaceCalendar, true
	case "3":
		return SurfaceContacts, true
	case "4":
		return SurfaceConfig, true
	default:
		return 0, false
	}
}
