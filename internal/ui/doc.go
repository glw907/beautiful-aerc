// Package ui is poplar's bubbletea v2 UI layer (technical design
// section 12): the root model App, the screens it composes, and the
// chrome that surrounds them. It performs no store write and no
// network I/O; a user action beyond surface switching and the screen
// stack enqueues an intent for a later pass to carry, and it reads
// engine state only through the typed messages this package declares
// (messages.go). Logging runs through plain log/slog calls against
// the uerr-installed process-wide default (cmd/poplar's startup path
// calls uerr.SetDefault before anything here can log); this package
// never imports internal/uerr itself.
//
// The registry and the grammar (registry.go). Every screen calls
// Register once, from its init, binding a ScreenEntry: its keymap
// (help.KeyMap), its pointer bindings, its UX-4 switch-table state
// (StateClass), and its footer priority. GrammarKeys is the
// interaction grammar's canonical key-to-verb table (design language
// section 2); checkGrammar and checkPointerGrammar check a registered
// screen's bindings against it, and the footer, the help overlay, and
// the switch-table and pointer-conformance tests all derive from the
// same registered entries rather than a hand-maintained second copy.
//
// LayoutMode (layout.go). ComputeLayout is the pure function of one
// terminal size, plus whether a banner wants a row, that every screen
// and every chrome component consumes: width and height class, the
// chrome bands' rectangles, and a ground-carrying rectangle for each
// pane the current rung composes. It is the one place a size boundary
// is decided; nothing downstream re-derives one from a raw width or
// height.
//
// The render seam (render.go). Render composes a Screen's
// already-rendered View into the frame LayoutMode describes: every
// band's ground painted first, then a named pane's content overlaid
// on top (LayoutMode.Main's doc comment), so every cell ComputeLayout
// allocates is accounted for. Below the floor it composes
// renderFloorNotice instead, the one frame no screen ever reaches.
// Render is pure and does no I/O, the same seam the gallery
// (internal/ui/testdata/gallery) renders every fixture through, so
// the product can never drift from what the gallery pins.
//
// The chrome inventory: statusline.go (the surface cluster and the
// sync/outbox/toast segment), footer.go (the width-maximal hint
// prefix plus the pinned help hint), banner.go (the persistent notice
// a short terminal demotes to a toast), toast.go (UX-9's undo
// countdown), confirm.go (the modal y/n/Esc component), and help.go
// (the registry-derived overlay). Each renders one band or takes over
// the screen stack; none reaches past RenderInput for its state.
//
// Pointer dispatch (mouse.go, ADR-0017). dispatchClick resolves a
// left click against the chrome's registered HitSpan set first,
// character grain, then LayoutMode's pane rectangles; a resolved span
// fires through fireVerb, the same key.Matches path the keyboard
// uses, so a click can never do anything its keyboard equivalent
// could not.
//
// Wheel coalescing (wheel.go, app.go's handleWheel/flushWheelTimer,
// ADR-0017 revision 3). A same-direction run of wheel ticks folds
// into one WheelMsg, flushed after a short idle window or immediately
// on a direction flip, so a scrollable pane sees one intentional
// gesture rather than a tick per detent.
//
// AccountScoped (accountscoped.go). A per-account UI-state value (the
// active surface, a future cursor or scroll position) lives in one of
// these rather than a bare field, so a second account never needs an
// audit to find every place state should have been keyed by account
// from the start.
package ui
