package ui

import (
	"image"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"
)

// dispatchClick resolves one mouse click against ADR-0017's two priced
// grains, in resolution order: character grain first (the chrome's
// own registered hit spans: status digits, footer hints, a showing
// banner's dismiss glyph, a modal's own y/n answers), then pane grain
// (LayoutMode's own rectangles). A resolved span's action is its own
// Binding, dispatched through the same key path the keyboard uses
// (fireVerb), never a parallel action path: probing a span fires
// exactly what pressing its key fires, no-op included wherever the
// front state does not offer it. A pane hit resolves which PaneID msg
// landed on but takes no action this pass: focusing a pane is
// wide-rung behavior with no consumer yet, so paneAt's own resolution
// is the seam, left ready rather than acted on. Anything msg's
// coordinates do not land on at all is a no-op too, the same as an
// illegal key.
func (a App) dispatchClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	front := a.frontEntry()
	pt := image.Pt(msg.X, msg.Y)

	if k, ok := statusDigitKeyAt(a.activeSurface(), front.SwitchState, a.layout.StatusRow.Rect, pt); ok {
		return a.handleKey(keyPressForString(k))
	}
	if span, ok := hitSpanAt(FooterHitSpans(front, front.SwitchState, a.layout.Width, a.layout.Footer.Rect), pt); ok {
		return a.fireVerb(span.Verb)
	}
	if a.layout.BannerRow {
		if span, ok := hitSpanAt(BannerHitSpans(a.banner, front.SwitchState, a.theme, a.layout.Banner.Rect), pt); ok {
			return a.fireVerb(span.Verb)
		}
	}
	if len(a.stack) > 0 {
		if confirm, ok := a.stack[len(a.stack)-1].(Confirm); ok {
			if span, ok := hitSpanAt(ConfirmHitSpans(confirm), pt); ok {
				return a.fireVerb(span.Verb)
			}
		}
	}
	if _, ok := paneAt(a.layout, pt); ok {
		return a, nil // pane focus: wide-rung behavior, no consumer this pass
	}
	return a, nil // outside every span and every pane: a plain miss
}

// dispatchWheel routes a coalesced wheel gesture to whichever
// scrollable owns msg's own coordinates (ADR-0017's mechanics): a
// gesture over the Main band reaches the stack's own top (the help
// overlay, this pass) when one is pushed, or the active surface
// otherwise. A gesture over a chrome band (the status row, the
// banner, the footer), or over a front the pointer table does not
// permit PointerWheel to fire in (a modal, most notably), is a
// no-op, the same rule the j/k keys themselves already obey.
func (a App) dispatchWheel(msg WheelMsg) (tea.Model, tea.Cmd) {
	front := a.frontEntry()
	if !slices.Contains(pointerLegalStates[PointerWheel], front.SwitchState) {
		return a, nil
	}
	if !image.Pt(msg.X, msg.Y).In(a.layout.Main.Rect) {
		return a, nil
	}
	if len(a.stack) > 0 {
		return a.updateStackTop(msg)
	}
	return a.updateActive(msg)
}

// frontEntry returns whichever ScreenEntry is currently in front: the
// stack's own top when one is pushed, otherwise the active surface's
// own root. handleKey, dispatchClick, and dispatchWheel all gate their
// own precedence on it, so a click or a wheel gesture is legal in
// exactly the states the keyboard equivalent already is.
func (a App) frontEntry() ScreenEntry {
	if len(a.stack) > 0 {
		return a.stack[len(a.stack)-1].Entry()
	}
	return a.activeScreen().Entry()
}

// updateStackTop forwards msg to whichever screen sits on top of a's
// own stack, the shared tail handleKey and dispatchWheel both reach
// once their own precedence has decided msg belongs there.
func (a App) updateStackTop(msg tea.Msg) (tea.Model, tea.Cmd) {
	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	if screen, ok := updated.(Screen); ok {
		a.stack[len(a.stack)-1] = screen
	}
	return a, cmd
}

// hitSpanAt returns the first of spans containing pt, ADR-0017's
// character-grain resolution: a chrome component's own registered
// spans never overlap, so "first" is also "only."
func hitSpanAt(spans []HitSpan, pt image.Point) (HitSpan, bool) {
	for _, span := range spans {
		if pt.In(span.Rect) {
			return span, true
		}
	}
	return HitSpan{}, false
}

// paneAt reports which of lm's own named panes contains pt, and
// whether one does: ADR-0017's pane grain, resolved against
// LayoutMode's own rectangles rather than a zone library (ADR-0011
// revision 3).
func paneAt(lm LayoutMode, pt image.Point) (PaneID, bool) {
	for id, pane := range lm.Panes {
		if pt.In(pane.Rect) {
			return id, true
		}
	}
	return 0, false
}

// statusDigitKeyAt reports the physical key StatusLineHitSpans's own
// digit-ordered spans name at pt, and whether pt lands on one at all.
// Every span StatusLineHitSpans returns shares one Verb, the whole
// SurfaceSwitch binding, since it stands for four physical keys at
// once (StatusLineHitSpans's own doc); resolving which one a click
// actually named needs the span's own position within that ordered
// list, matched against GrammarKeys.SurfaceSwitch.Keys() index for
// index (clusterDigitX's own precedent), not the shared Verb alone.
func statusDigitKeyAt(active Surface, state StateClass, statusRow image.Rectangle, pt image.Point) (string, bool) {
	keys := GrammarKeys.SurfaceSwitch.Keys()
	for i, span := range StatusLineHitSpans(active, state, statusRow) {
		if pt.In(span.Rect) && i < len(keys) {
			return keys[i], true
		}
	}
	return "", false
}

// specialKeyPresses maps a GrammarKeys key string to the tea.Key code
// it names, for every non-printable key any binding's own first key
// currently resolves to (GrammarKeys.fields()'s own closed set):
// keyPressForString's fallback, a printable literal, covers everything
// else a pointer dispatch this pass ever needs to fire.
var specialKeyPresses = map[string]rune{
	"esc":    tea.KeyEscape,
	"enter":  tea.KeyEnter,
	"tab":    tea.KeyTab,
	"space":  tea.KeySpace,
	"home":   tea.KeyHome,
	"end":    tea.KeyEnd,
	"pgup":   tea.KeyPgUp,
	"pgdown": tea.KeyPgDown,
}

// keyPressForString builds the tea.KeyPressMsg whose own String()
// equals s (key.Matches's own comparison, bubbles/v2/key): a special
// key resolves through specialKeyPresses, and a printable one carries
// its own rune as both Code and Text, key.Matches's only requirement.
func keyPressForString(s string) tea.KeyPressMsg {
	if code, ok := specialKeyPresses[s]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// fireVerb dispatches b's own accelerated verb through the same
// Update path the keyboard uses (ADR-0017's grammar rule): b's own
// first bound key, since key.Matches only needs one of a binding's
// keys to recognize it, and every span this pass's chrome components
// register names a single-purpose binding, never one covering the
// direction-sensitive synonyms GrammarKeys.Navigate itself bundles.
func (a App) fireVerb(b key.Binding) (tea.Model, tea.Cmd) {
	return a.handleKey(keyPressForString(b.Keys()[0]))
}

// doubleClickWindow is ADR-0017's own double-click threshold, a
// compiled constant rather than a config value: two clicks landing on
// the same target inside this window upgrade to the open path; two
// clicks apart, or on different targets, are just two selects.
const doubleClickWindow = 400 * time.Millisecond

// pendingClick is the double-click state machine's own open window
// (ADR-0017): the target the first click landed on, and the deadline
// a second click on the same target must beat to upgrade into a
// double-click. No screen registers PointerRowOpen yet (rows land
// with pass 3), so nothing in the running product owns one this pass;
// resolveDoubleClick and its own tests exercise the machine directly
// against a synthetic target, ready for pass 3 to wire against a real
// one.
type pendingClick struct {
	open     bool
	target   any
	deadline time.Time
}

// resolveDoubleClick folds one click on target, arriving at now, into
// prev's own open window. Single click acts immediately, always (the
// caller's own select action already ran by the time this is
// consulted), so this only ever decides whether the click also
// upgrades to the open path: the same target within the window
// upgrades and closes the window; anything else (no window open, a
// different target, or the window already elapsed) opens a fresh
// window over target and reports no upgrade.
func resolveDoubleClick(prev pendingClick, target any, now time.Time) (pendingClick, bool) {
	if prev.open && prev.target == target && now.Before(prev.deadline) {
		return pendingClick{}, true
	}
	return pendingClick{open: true, target: target, deadline: now.Add(doubleClickWindow)}, false
}
