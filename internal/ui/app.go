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
	banner Banner

	sync           SyncStateMsg
	backfill       BackfillProgressMsg
	outbox         int
	spinnerFrame   int
	spinnerGen     int
	spinnerTicking bool

	toastOffer     UndoOffer
	toastActive    bool
	toastRemaining int
	toastGen       int

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
		a.layout = ComputeLayout(msg.Width, msg.Height, a.banner.Active)
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
	case SyncStateMsg:
		a.sync = msg
		return a.reconcileSpinner()
	case BackfillProgressMsg:
		a.backfill = msg
		return a.reconcileSpinner()
	case OutboxCountMsg:
		a.outbox = msg.Queued
		return a, nil
	case statusSpinnerTickMsg:
		return a.tickSpinner(msg)
	case ToastMsg:
		return a.showToast(msg)
	case toastTickMsg:
		return a.tickToast(msg)
	case BannerMsg:
		a.banner = Banner{Active: true, Message: msg.Message}
		a = a.recomputeLayout()
		return a.updateChildren(LayoutMsg{Layout: a.layout})
	case ConfirmAnsweredMsg:
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, msg.Next
	}
	return a.updateChildren(msg)
}

// View implements tea.Model. A StateModal stack top (Confirm, the 5a
// ruling) renders itself directly: a plain stack-top render, no dimmed
// backdrop, since a modal owns the whole terminal itself rather than
// landing in a named LayoutMode pane. Every other front, the active
// surface with an empty stack or a non-modal screen pushed onto it
// (the help overlay, first of its kind, task 9), runs through Render,
// the same seam the gallery renders through, so the product never
// drifts from what the gallery pins: a pushed screen's own content
// fills the whole Main band (RenderInput.FullRegion) rather than the
// narrower Content pane a surface's own sidebar reservation would
// otherwise squeeze it against, since it owns no sidebar of its own,
// and its own Entry, not the surface underneath it, is what the
// footer follows (task 9's carry fix from task 7's review).
func (a App) View() tea.View {
	screen := a.activeScreen()
	entry := screen.Entry()
	fullRegion := false
	if len(a.stack) > 0 {
		top := a.stack[len(a.stack)-1]
		topEntry := top.Entry()
		if topEntry.SwitchState == StateModal {
			return top.View()
		}
		screen, entry, fullRegion = top, topEntry, true
	}
	frame := Render(RenderInput{Screen: screen, Entry: entry, FullRegion: fullRegion, Layout: a.layout, Theme: a.theme, Status: a.statusLine(), Banner: a.banner})
	view := tea.NewView(frame.Content)
	view.Cursor = frame.Cursor
	return view
}

// statusLine builds the top band's own render state from a's current
// model state, App's whole answer to "StatusLine consumed by
// App.View": every field View hands to Render.
func (a App) statusLine() StatusLine {
	return StatusLine{
		Active:   a.activeSurface(),
		Sync:     a.sync,
		Backfill: a.backfill,
		Outbox:   a.outbox,
		Spinner:  a.spinnerFrame,
		Toast:    a.toast(),
	}
}

// toast builds the status line's own Toast value from a's undo-offer
// state: a zero Toast (Active false) once no offer is open, otherwise
// its label and the countdown's own current remaining seconds.
// Undoable reports whether the open offer actually has an Undo Cmd to
// run (task-8-findings-r1.md F8): a toast with none renders its label
// alone, never a hint advertising a dead `u`.
func (a App) toast() Toast {
	if !a.toastActive {
		return Toast{}
	}
	return Toast{
		Active:    true,
		Label:     a.toastOffer.Label,
		Remaining: a.toastRemaining,
		Undoable:  a.toastOffer.Undo != nil,
	}
}

// recomputeLayout rebuilds a.layout from its own current Width/Height
// with a.banner's current Active state (ComputeLayout's bannerRow
// input): the route back to a correct BannerRow after a banner shows
// or dismisses between tea.WindowSizeMsg events, since ComputeLayout
// is a pure function of all three inputs together, never incrementally
// patched.
func (a App) recomputeLayout() App {
	a.layout = ComputeLayout(a.layout.Width, a.layout.Height, a.banner.Active)
	return a
}

// statusSpinnerInterval is the sync segment's own spinner cadence:
// bubbles/spinner's braille "Dot" preset FPS, the same frame set
// theme.Theme.Spinner returns.
const statusSpinnerInterval = 100 * time.Millisecond

// statusSpinnerTickMsg is the sync segment's spinner tick, armed once
// per active run (mirroring wheelFlushMsg's gen convention): gen
// names which run it belongs to, so a tick from a run reconcileSpinner
// already ended is recognizable and ignored, and QA-8's idle posture
// holds (no timer wakeup once the state is synced or offline).
type statusSpinnerTickMsg struct {
	gen int
}

// reconcileSpinner arms or stops the spinner tick on a transition into
// or out of an active sync or backfill state: it only ever starts a
// new tick chain on the false-to-true edge, never on every progress
// message a run in progress delivers, so a burst of SyncStateMsg
// updates cannot starve the spinner by repeatedly replacing its own
// pending tick.
func (a App) reconcileSpinner() (App, tea.Cmd) {
	active := a.sync.State == SyncStateSyncing || a.backfill.Active
	if active == a.spinnerTicking {
		return a, nil
	}
	a.spinnerTicking = active
	if !active {
		return a, nil
	}
	a.spinnerGen++
	return a, armSpinnerTick(a.spinnerGen)
}

// tickSpinner absorbs one statusSpinnerTickMsg: a stale tick (msg.gen
// no longer matches a.spinnerGen) or one that arrived after
// reconcileSpinner already stopped ticking is silently ignored;
// otherwise the frame advances and the next tick is armed.
func (a App) tickSpinner(msg statusSpinnerTickMsg) (tea.Model, tea.Cmd) {
	if !a.spinnerTicking || msg.gen != a.spinnerGen {
		return a, nil
	}
	a.spinnerFrame++
	return a, armSpinnerTick(a.spinnerGen)
}

// armSpinnerTick returns the Cmd that ticks gen's spinner run after
// statusSpinnerInterval.
func armSpinnerTick(gen int) tea.Cmd {
	return tea.Tick(statusSpinnerInterval, func(time.Time) tea.Msg {
		return statusSpinnerTickMsg{gen: gen}
	})
}

// toastTickMsg is the toast countdown's own tick (UX-9), armed once
// per open window and mirroring statusSpinnerTickMsg's own gen
// convention: gen names which window it belongs to, so a tick from a
// window a newer toast already replaced, or quit already discarded,
// is recognizable and ignored.
type toastTickMsg struct {
	gen int
}

// showToast absorbs a ToastMsg: newest wins over a toast already
// showing (design decision 1), and every toast logs exactly one ER-1
// line through the same seam app.go's own background-color timeout
// line reaches (a plain slog call, routed to uerr's destination once
// cmd/poplar's startup path installs it as slog's default). The
// countdown starts at undoWindowSeconds-1, the pinned exemplar's own
// dim "9s".
func (a App) showToast(msg ToastMsg) (App, tea.Cmd) {
	slog.Info("toast shown", "label", msg.Offer.Label)
	a.toastGen++
	a.toastOffer = msg.Offer
	a.toastActive = true
	a.toastRemaining = undoWindowSeconds - 1
	return a, armToastTick(a.toastGen)
}

// tickToast absorbs one toastTickMsg: a stale tick (msg.gen no longer
// matches a.toastGen) is silently ignored; otherwise the countdown
// steps down one second, closing the window at zero (UX-9's own 10s
// visible countdown) rather than arming a further tick.
func (a App) tickToast(msg toastTickMsg) (tea.Model, tea.Cmd) {
	if !a.toastActive || msg.gen != a.toastGen {
		return a, nil
	}
	if a.toastRemaining == 0 {
		a.toastActive = false
		return a, nil
	}
	a.toastRemaining--
	return a, armToastTick(a.toastGen)
}

// undoEligible reports whether handleKey may treat `u` as answering
// the open undo window (task-8-findings-r1.md F2, CRITICAL): only at
// a surface root (an empty stack, so a modal or any other stacked
// screen never sees a stray `u` reinterpreted as an answer), with
// front in StateDigitsSwitch (the same gate the surface digits
// themselves already hold to, matchDigit), and an undo window
// actually open. Extracted as its own predicate so both of F2's probe
// cases test it directly, without needing a StatePrintableEntry root
// screen this pass registers none of.
func (a App) undoEligible(front ScreenEntry) bool {
	return len(a.stack) == 0 && front.SwitchState == StateDigitsSwitch && a.toastActive
}

// armToastTick returns the Cmd that ticks gen's countdown after one
// second.
func armToastTick(gen int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return toastTickMsg{gen: gen}
	})
}

// handleKey applies the interaction grammar's back/help/undo/quit/
// surface-switch precedence (design language section 2) before
// anything else sees the key: a StateModal front (a modal confirm,
// most notably) owns Esc itself as one of its own y/n/Esc answers, so
// the generic Back branch below skips it entirely and lets the final
// fallback forward the key to the modal's own Update (Confirm.Update
// emits ConfirmAnsweredMsg, App's own Update case pops the stack and
// runs the answer's Cmd: task-8-findings-r1.md's conventions ruling,
// the template every future modal copies); every other front's Esc
// pops the stack (or no-ops at a surface root, this pass), dismissing
// a showing banner first only when the stack is empty (F3, CRITICAL:
// the banner is invisible under a StateModal stack front, so
// dismissing it there would be a silent dead keypress; a non-modal
// front, the help overlay included, composites the banner alongside
// its own content instead, task 9) and the front context is not text
// entry (design decision 2: a banner never steals focus); `?` pushes
// the help overlay over whichever front is showing, unless that front
// is already the overlay itself or a modal (UX-5: help never covers
// or is covered by a StateModal front); `u` inside an open UX-9 undo
// window emits the offer's Cmd, gated to a surface root in
// StateDigitsSwitch (F2, CRITICAL: the same gate the surface digits
// themselves already hold to, so a text-entry or modal front never
// treats a stray `u` as an answer); a digit switches surfaces only
// when the state currently in front (the stack's top, or the active
// surface's own root state when the stack is empty) is
// StateDigitsSwitch, so a modal on the stack eats a digit instead
// (UX-4's acceptance criterion), and pops a non-modal stack front
// along with the switch (task 9: digits switch surfaces from help,
// and pop it); q quits only at a surface root and discards any open
// undo window (UX-9: the window does not survive quit; BACKLOG #71
// tracks q's own missing StateDigitsSwitch gate); anything else at a
// surface root reaches the active screen's own Update.
func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	front := a.activeScreen().Entry()
	if len(a.stack) > 0 {
		front = a.stack[len(a.stack)-1].Entry()
	}

	if key.Matches(msg, GrammarKeys.Back) && front.SwitchState != StateModal {
		if a.banner.Active && len(a.stack) == 0 && front.SwitchState != StatePrintableEntry {
			a.banner.Active = false
			a = a.recomputeLayout()
			return a.updateChildren(LayoutMsg{Layout: a.layout})
		}
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	}

	if key.Matches(msg, GrammarKeys.Help) && front.SwitchState != StateModal && front.Name != helpScreenName {
		a.stack = append(a.stack, HelpScreen{theme: a.theme, layout: a.layout, Covered: front})
		return a, nil
	}

	if a.undoEligible(front) && key.Matches(msg, GrammarKeys.Undo) {
		undo := a.toastOffer.Undo
		a.toastActive = false
		return a, undo
	}

	if s, ok := matchDigit(msg, front.SwitchState); ok {
		a.active.Set(a.account, s)
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	}

	if len(a.stack) == 0 {
		if key.Matches(msg, GrammarKeys.Quit) {
			a.toastActive = false
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
// It also forwards to every a.stack entry (task-8-findings-r1.md
// ruling F4 promoted): a pushed screen must keep fitting the viewport
// across a resize and keep matching the live theme across a repaint,
// the same contract every surface screen already holds to. Task 9
// starts pushing screens routinely, so this cannot wait on task 9's
// own dispatch.
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

	for i, screen := range a.stack {
		var updated tea.Model
		updated, cmd = screen.Update(msg)
		if s, ok := updated.(Screen); ok {
			a.stack[i] = s
		}
		cmds = append(cmds, cmd)
	}

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
