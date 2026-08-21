package ui

import (
	"fmt"
	"log/slog"
	"strings"
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

	// Account is cmd/poplar's first successful connect result: pass 2
	// wires exactly one account for the process's whole life, so
	// nothing here refreshes it once NewApp has been called. A live
	// account switch, or a second account added mid-session, is
	// pass 3's carry.
	Account string
}

// wheelGesture is App's open wheel-coalescing gesture (ADR-0017
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

// NewProgram returns a *tea.Program running app, its color profile set
// from app's theme.Profile (CARRY 1): without it, bubbletea's
// terminal auto-detection re-downsamples the theme's already-resolved
// values against whatever it independently guesses, discarding
// ResolveProfile's NO_COLOR/TERM/COLORTERM precedence and the config
// override seam layered on top of it. opts apply after, so a caller
// that also passes tea.WithColorProfile overrides this default rather
// than losing to it.
func NewProgram(app App, opts ...tea.ProgramOption) *tea.Program {
	all := append([]tea.ProgramOption{tea.WithColorProfile(mapColorProfile(app.profile))}, opts...)
	return tea.NewProgram(app, all...)
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return tea.Batch(a.mail.Init(), a.calendar.Init(), a.contacts.Init(), a.config.Init(), QueryBackgroundColor())
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var demoteCmd tea.Cmd
		if a.banner.Active && classifyHeight(msg.Height) != HeightFull {
			a, demoteCmd = a.demoteBanner(a.banner.Message)
		}
		a.layout = ComputeLayout(msg.Width, msg.Height, a.banner.Active)
		model, cmd := a.updateChildren(LayoutMsg{Layout: a.layout})
		return model, tea.Batch(demoteCmd, cmd)
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
	case WheelMsg:
		return a.dispatchWheel(msg)
	case tea.MouseClickMsg:
		return a.dispatchClick(msg)
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
		if a.layout.HeightClass != HeightFull {
			return a.demoteBanner(msg.Message)
		}
		slog.Warn("banner shown", "message", msg.Message)
		a.banner = Banner{Active: true, Message: msg.Message}
		a = a.recomputeLayout()
		return a.updateChildren(LayoutMsg{Layout: a.layout})
	case ConfirmAnsweredMsg:
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, msg.Next
	case quitYesMsg:
		if a.undoWindowOpen() {
			slog.Debug("quit discarded the open undo window")
		}
		a.toastActive = false
		return a, tea.Quit
	}
	return a.updateChildren(msg)
}

// View implements tea.Model. Below the floor (width or height,
// design language section 9) the centered notice renderFloorNotice
// composes is the whole frame, selected before anything else runs:
// a StateModal stack top (Confirm, most notably) would otherwise
// render its box at a size confirmBoxWidth never promised to
// tolerate, and the floor rung's chrome-free premise (mouse.go's
// dispatchClick) would be a lie. Above the floor, a StateModal stack
// top renders itself directly: a plain stack-top render, no dimmed
// backdrop, since a modal owns the whole terminal itself rather than
// landing in a named LayoutMode pane. Every other front, the active
// surface with an empty stack or a non-modal screen pushed onto it
// (the help overlay, first of its kind, task 9), runs through Render,
// the same seam the gallery renders through, so the product never
// drifts from what the gallery pins: a pushed screen's content fills
// the whole Main band (RenderInput.FullRegion), the same treatment
// this pass's four surface placeholders get (isPlaceholderScreen,
// F1/F8's ruling: each owns no sidebar and no split), rather than the
// narrower Content pane a surface's sidebar reservation would
// otherwise squeeze either against. The footer follows whichever
// Screen Render composes, the stack top's Entry included, since
// Render always reaches for Screen.Entry() itself. Every returned
// tea.View carries MouseMode (a per-frame declaration in bubbletea
// v2, not a program-construction option, so no return path can
// silently toggle reporting off) and AltScreen (bubbletea v2 removed
// WithAltScreen; without this every path renders inline, scrollback
// destroyed and a stale frame left behind after quit).
func (a App) View() tea.View {
	if a.layout.Class == WidthFloor || a.layout.HeightClass == HeightFloor {
		view := tea.NewView(renderFloorNotice(a.theme, a.layout.Width, a.layout.Height))
		view.MouseMode = tea.MouseModeCellMotion
		view.AltScreen = true
		return view
	}

	screen := a.activeScreen()
	fullRegion := isPlaceholderScreen(screen)
	if len(a.stack) > 0 {
		top := a.stack[len(a.stack)-1]
		if top.Entry().SwitchState == StateModal {
			view := top.View()
			view.MouseMode = tea.MouseModeCellMotion
			view.AltScreen = true
			return view
		}
		screen, fullRegion = top, true
	}
	frame := Render(RenderInput{Screen: screen, FullRegion: fullRegion, Layout: a.layout, Theme: a.theme, Status: a.statusLine(), Banner: a.banner})
	view := tea.NewView(frame.Content)
	view.Cursor = frame.Cursor
	view.MouseMode = tea.MouseModeCellMotion
	view.AltScreen = true
	return view
}

// isPlaceholderScreen reports whether screen is one of pass 2's four
// surface placeholders (F1/F8's ruling): each owns no sidebar and no
// split, so App.View renders it FullRegion the same way a pushed
// non-modal screen already is, rather than landing it in the
// narrower Content pane a surface's sidebar reservation would
// otherwise squeeze it against. The durable fix, a pane set each
// ScreenEntry declares so ComputeLayout allocates only what a screen
// actually asked for, is pass 3's carry (BACKLOG).
func isPlaceholderScreen(screen Screen) bool {
	switch screen.(type) {
	case MailPlaceholder, CalendarPlaceholder, ContactsPlaceholder, ConfigPlaceholder:
		return true
	default:
		return false
	}
}

// statusLine builds the top band's render state from a's current
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

// toast builds the status line's Toast value from a's undo-offer
// state: a zero Toast (Active false) once no offer is open, otherwise
// its label and the countdown's current remaining seconds.
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

// recomputeLayout rebuilds a.layout from its current Width/Height
// with a.banner's current Active state (ComputeLayout's bannerRow
// input): the route back to a correct BannerRow after a banner shows
// or dismisses between tea.WindowSizeMsg events, since ComputeLayout
// is a pure function of all three inputs together, never incrementally
// patched.
func (a App) recomputeLayout() App {
	a.layout = ComputeLayout(a.layout.Width, a.layout.Height, a.banner.Active)
	return a
}

// demoteBanner routes message through showToast instead of a.banner
// (design language section 9: under HeightFull a banner demotes to a
// toast, ER-3's window included, since a short rung grants no row for
// it): a.banner clears first, so a later resize back above HeightFull
// never resurrects a banner that was never actually shown. showToast
// already logs, which is the demoted path's whole answer to spec
// m11's logging seam.
func (a App) demoteBanner(message string) (App, tea.Cmd) {
	a.banner = Banner{}
	return a.showToast(ToastMsg{Offer: UndoOffer{Label: message}})
}

// statusSpinnerInterval is the sync segment's spinner cadence:
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
// updates cannot starve the spinner by repeatedly replacing its
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

// toastTickMsg is the toast countdown's tick (UX-9), armed once
// per open window and mirroring statusSpinnerTickMsg's gen
// convention: gen names which window it belongs to, so a tick from a
// window a newer toast already replaced, or quit already discarded,
// is recognizable and ignored.
type toastTickMsg struct {
	gen int
}

// showToast absorbs a ToastMsg: newest wins over a toast already
// showing (design decision 1), and every toast logs exactly one ER-1
// line through the same seam app.go's background-color timeout
// line reaches (a plain slog call, routed to uerr's destination once
// cmd/poplar's startup path installs it as slog's default). The
// countdown starts at undoWindowSeconds-1, the pinned exemplar's
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
// steps down one second, closing the window at zero (UX-9's 10s
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
// actually open. Extracted as its predicate so both of F2's probe
// cases test it directly, without needing a StatePrintableEntry root
// screen this pass registers none of.
func (a App) undoEligible(front ScreenEntry) bool {
	return len(a.stack) == 0 && front.SwitchState == StateDigitsSwitch && a.undoWindowOpen()
}

// undoWindowOpen reports whether a's open toast is actually an undo
// window rather than a plain notification (task-8 F8, Toast.Undoable's
// split): a.toastActive alone answers "is a toast showing", which
// a notification toast with no Undo Cmd also satisfies, and both q's
// quit gate and u's undo answer must agree with what
// Toast.Undoable already renders.
func (a App) undoWindowOpen() bool {
	return a.toastActive && a.toastOffer.Undo != nil
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
// most notably) owns Esc itself as one of its y/n/Esc answers, so
// the generic Back branch below skips it entirely and lets the final
// fallback forward the key to the modal's Update (Confirm.Update
// emits ConfirmAnsweredMsg, App's Update case pops the stack and
// runs the answer's Cmd: task-8-findings-r1.md's conventions ruling,
// the template every future modal copies); every other front's Esc
// dismisses a showing banner first (C4, amending task 8's F3 ruling:
// the banner renders under any non-modal front now, FullRegion
// included, so visibility, not stack emptiness, is what gates the
// dismiss) whenever the front context is not text entry either
// (design decision 2: a banner never steals focus), and otherwise
// pops the stack (or no-ops at a surface root, this pass); `?` toggles
// the help overlay, gated to StateDigitsSwitch exactly like
// undoEligible below (C2, subsuming the old modal check: a
// StatePrintableEntry front, a search bar most notably, keeps its
// `?` character rather than surrendering it to a global shortcut),
// opening it over whichever front is showing, or closing it again
// when help is already that front (C3, the mutt/aerc/less toggle
// idiom); `u` inside an open UX-9 undo window emits the offer's Cmd,
// gated to a surface root in StateDigitsSwitch (F2, CRITICAL: the same
// gate the surface digits themselves already hold to, so a text-entry
// or modal front never treats a stray `u` as an answer); a digit
// switches surfaces only when the state currently in front (the
// stack's top, or the active surface's root state when the stack
// is empty) is StateDigitsSwitch, so a modal on the stack eats a digit
// instead (UX-4's acceptance criterion), and pops a non-modal stack
// front along with the switch (task 9: digits switch surfaces from
// help, and pop it); q quits only at a surface root, straight through
// when the outbox is empty and no undo window is open, otherwise
// through F7's modal confirm naming what quitting costs (UX-9: the
// window does not survive quit, and the toast says so; BACKLOG #71
// tracks q's missing StateDigitsSwitch gate; task 11 carried this
// from task 8's review); anything else at a surface root reaches the
// active screen's Update.
func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	front := a.frontEntry()

	if key.Matches(msg, GrammarKeys.Back) && front.SwitchState != StateModal {
		if a.banner.Active && bannerDismissEligible(front) {
			a.banner.Active = false
			a = a.recomputeLayout()
			return a.updateChildren(LayoutMsg{Layout: a.layout})
		}
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	}

	if key.Matches(msg, GrammarKeys.Help) && helpOpenEligible(front) {
		if front.Name == helpScreenName {
			if len(a.stack) > 0 {
				a.stack = a.stack[:len(a.stack)-1]
			}
			return a, nil
		}
		return a.push(HelpScreen{Covered: front, Title: a.helpTitle()}), nil
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
			return a.handleQuit()
		}
		return a.updateActive(msg)
	}

	return a.updateStackTop(msg)
}

// handleQuit answers q at a surface root: an empty outbox and no open
// undo window quits straight through, and otherwise pushes F7's modal
// confirm naming whichever of the queued outbox and the open undo
// window are in play, so quitting never discards either in silence
// (UX-9's "the window does not survive quit, and the toast says so").
func (a App) handleQuit() (tea.Model, tea.Cmd) {
	if a.outbox == 0 && !a.undoWindowOpen() {
		return a, tea.Quit
	}
	return a.push(a.quitConfirm()), nil
}

// quitConfirm returns F7's modal confirm for a quit handleQuit did not
// answer directly. YesCmd emits quitYesMsg rather than baking a's
// undoWindowOpen() into the answer here: an undo window can expire
// while this very modal sits open, and quitYesMsg's App.Update
// case re-reads it fresh instead.
func (a App) quitConfirm() Confirm {
	return Confirm{
		Question:    quitQuestion(a.outbox),
		Consequence: quitConsequence(a.outbox, a.undoWindowOpen(), a.toastOffer.Label),
		YesLabel:    "quit",
		NoLabel:     "stay",
		YesCmd:      quitYesCmd,
	}
}

// helpTitle returns the display name a help push names in its header
// (HelpScreen.Title, wireframe F5's "Help · Mail"): the active
// surface's display name (surfaceNames) when help opens over a
// surface root (an empty stack), or "" otherwise, so
// HelpScreen.displayTitle falls back to the covered entry's own
// registered name for a future non-surface StateDigitsSwitch screen
// this pass never pushes help over.
func (a App) helpTitle() string {
	if len(a.stack) == 0 {
		return surfaceNames[a.activeSurface()]
	}
	return ""
}

// push appends s onto a's screen stack, feeding it a's current
// LayoutMsg and ThemeMsg first, the same pair updateChildren forwards
// to every stacked screen on a later resize or theme change: the one
// route onto the stack, so a push site can never construct a screen
// missing either field the way a bare append once let it. Every
// Screen's LayoutMsg/ThemeMsg case returns a nil Cmd, so push
// discards both return Cmds rather than threading them nowhere useful.
func (a App) push(s Screen) App {
	updated, _ := s.Update(LayoutMsg{Layout: a.layout})
	if scr, ok := updated.(Screen); ok {
		s = scr
	}
	updated, _ = s.Update(ThemeMsg{Theme: a.theme})
	if scr, ok := updated.(Screen); ok {
		s = scr
	}
	a.stack = append(a.stack, s)
	return a
}

// quitQuestion names what quitting would leave unsent (wireframe F7's
// "Quit with 2 unsent messages?"), or a bare "Quit now?" when the
// outbox holds nothing and the confirm exists only for the open undo
// window.
func quitQuestion(outbox int) string {
	switch outbox {
	case 0:
		return "Quit now?"
	case 1:
		return "Quit with 1 unsent message?"
	default:
		return fmt.Sprintf("Quit with %d unsent messages?", outbox)
	}
}

// quitConsequence names each cost quitting carries: the outbox's
// "they'll send the next time you open poplar" (wireframe F7) when it
// holds work, and the open undo window's label when one is open,
// so an operator who only has one of the two never reads a sentence
// about the other.
func quitConsequence(outbox int, undoOpen bool, undoLabel string) string {
	var parts []string
	if outbox > 0 {
		parts = append(parts, "They'll send the next time you open poplar.")
	}
	if undoOpen {
		parts = append(parts, fmt.Sprintf("The undo for %s will be lost.", undoLabel))
	}
	return strings.Join(parts, " ")
}

// quitYesMsg is F7's Yes answer signal: App.Update's case for
// it, not Confirm's YesCmd itself, decides whether an open undo window
// is what quitting discards. A background toastTickMsg keeps ticking
// regardless of what sits on the stack, so the window can expire while
// this very modal is showing; evaluating undoWindowOpen() here, when
// the answer is actually handled, rather than baking a bool into the
// Cmd back when q first pushed the modal, is what keeps the discard
// log line and the confirm's consequence truthful.
type quitYesMsg struct{}

// quitYesCmd is Confirm's YesCmd for the quit confirm: it names no
// state, so it never goes stale between push and answer.
func quitYesCmd() tea.Msg { return quitYesMsg{} }

// handleWheel folds msg into the open wheel gesture, or opens a new
// one (ADR-0017 revision 3): the coalescing decision lives on the
// root model rather than a program-construction filter, so a flush
// is never stranded waiting on a tick that may not arrive. Opening a
// gesture arms a tea.Tick(wheelWindow) flush timer (armWheelFlush); a
// same-direction tick within the window folds into the running sum;
// an opposite-direction tick flushes the open gesture immediately as
// a WheelMsg and opens a fresh one carrying the flipping tick,
// batched with that gesture's flush timer.
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
// flushes as one WheelMsg carrying its opening tick's
// coordinates.
func (a App) flushWheelTimer(msg wheelFlushMsg) (tea.Model, tea.Cmd) {
	if !a.wheel.open || msg.gen != a.wheel.gen {
		return a, nil
	}
	flush := flushWheelCmd(a.wheel)
	a.wheel = wheelGesture{gen: a.wheel.gen}
	return a, flush
}

// wheelFlushMsg is the flush timer's tick, armed once per gesture
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
// coordinates of g's opening tick.
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
// dispatch.
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
// could drift from the grammar's binding: key.Matches gates
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
