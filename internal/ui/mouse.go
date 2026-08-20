package ui

import (
	"image"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"
)

// dispatchClick resolves one left-button mouse click against
// ADR-0017's two priced grains, in resolution order: character grain
// first (the chrome's registered hit spans: status digits, footer
// hints, a showing banner's dismiss glyph, a modal's y/n answers),
// then pane grain (LayoutMode's rectangles). Every other button
// (middle, right, a side button) is a no-op (F1, task-10-findings-r2.md):
// ADR-0017 accelerates the keyboard's verbs, and only the left click
// carries one. A floor rung (width or height) paints no chrome at all
// (render.go's early return), so dispatch resolves nothing there
// either (F2). A StateModal front resolves only through the stack
// top's HitSpans(), the anonymous interface F5 rules for, never the
// status/footer/banner spans a full-terminal modal never painted
// over. A resolved span's action is its Binding, dispatched
// through the same key path the keyboard uses (fireVerb), never a
// parallel action path: probing a span fires exactly what pressing
// its key fires. Anything msg's coordinates do not land on at all is
// a no-op too, the same as an illegal key.
func (a App) dispatchClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return a, nil
	}
	if a.layout.Class == WidthFloor || a.layout.HeightClass == HeightFloor {
		return a, nil
	}

	front := a.frontEntry()
	pt := image.Pt(msg.X, msg.Y)

	if front.SwitchState == StateModal {
		top, ok := a.stack[len(a.stack)-1].(interface{ HitSpans() []HitSpan })
		if !ok {
			return a, nil
		}
		if span, ok := hitSpanAt(top.HitSpans(), pt); ok {
			return a.fireVerb(span.Verb)
		}
		return a, nil
	}

	if k, ok := statusDigitKeyAt(a.activeSurface(), front.SwitchState, a.layout.StatusRow.Rect, pt); ok {
		return a.handleKey(keyPressForString(k))
	}
	if span, ok := hitSpanAt(FooterHitSpans(front, front.SwitchState, a.layout.Footer.Rect), pt); ok {
		return a.fireVerb(span.Verb)
	}
	if a.layout.BannerRow {
		if span, ok := hitSpanAt(BannerHitSpans(a.banner, front.SwitchState, a.theme, a.layout.Banner.Rect), pt); ok {
			return a.fireVerb(span.Verb)
		}
	}
	paneAt(a.layout, pt) // pane focus: wide-rung behavior, no consumer this pass
	return a, nil
}

// dispatchWheel routes a coalesced wheel gesture to whichever
// scrollable owns msg's coordinates (ADR-0017's mechanics): a gesture
// over the Main band reaches the stack's top (the help overlay, this
// pass) when one is pushed, or the active surface otherwise. A
// gesture over a chrome band, a front state pointerLegalStates does
// not permit PointerWheel to fire in (a modal, most notably), or a
// front whose registry entry names no PointerWheel binding at all
// (F4, task-10-findings-r2.md: the registry is the authority, not
// merely the state table), is a no-op, the same rule the j/k keys
// themselves already obey.
func (a App) dispatchWheel(msg WheelMsg) (tea.Model, tea.Cmd) {
	front := a.frontEntry()
	if !slices.Contains(pointerLegalStates[PointerWheel], front.SwitchState) {
		return a, nil
	}
	if !registersPointerTarget(front, PointerWheel) {
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

// registersPointerTarget reports whether entry's Pointer bindings
// name target: the registry is the authority for which fronts accept
// a pointer action (ADR-0017's vocabulary), not merely whether
// pointerLegalStates permits the current state (F4).
func registersPointerTarget(entry ScreenEntry, target PointerTarget) bool {
	for _, pb := range entry.Pointer {
		if pb.Target == target {
			return true
		}
	}
	return false
}

// frontEntry returns whichever ScreenEntry is currently in front: the
// stack's top when one is pushed, otherwise the active surface's
// root. handleKey, dispatchClick, and dispatchWheel all gate their
// precedence on it, so a click or a wheel gesture is legal in exactly
// the states the keyboard equivalent already is.
func (a App) frontEntry() ScreenEntry {
	if len(a.stack) > 0 {
		return a.stack[len(a.stack)-1].Entry()
	}
	return a.activeScreen().Entry()
}

// updateStackTop forwards msg to whichever screen sits on top of a's
// stack, the shared tail handleKey and dispatchWheel both reach once
// their precedence has decided msg belongs there.
func (a App) updateStackTop(msg tea.Msg) (tea.Model, tea.Cmd) {
	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	if screen, ok := updated.(Screen); ok {
		a.stack[len(a.stack)-1] = screen
	}
	return a, cmd
}

// hitSpanAt returns the first of spans containing pt, ADR-0017's
// character-grain resolution: a chrome component's registered spans
// never overlap, so "first" is also "only."
func hitSpanAt(spans []HitSpan, pt image.Point) (HitSpan, bool) {
	for _, span := range spans {
		if pt.In(span.Rect) {
			return span, true
		}
	}
	return HitSpan{}, false
}

// paneAt reports which of lm's named panes contains pt, and whether
// one does: ADR-0017's pane grain, resolved against LayoutMode's
// rectangles rather than a zone library (ADR-0011 revision 3).
func paneAt(lm LayoutMode, pt image.Point) (PaneID, bool) {
	for id, pane := range lm.Panes {
		if pt.In(pane.Rect) {
			return id, true
		}
	}
	return 0, false
}

// statusDigitKeyAt reports the physical key StatusLineHitSpans's
// digit-ordered spans name at pt, and whether pt lands on one at all.
// Every span StatusLineHitSpans returns shares one Verb, the whole
// SurfaceSwitch binding, since it stands for four physical keys at
// once (StatusLineHitSpans's doc); resolving which one a click
// actually named needs the span's position within that ordered list,
// matched against GrammarKeys.SurfaceSwitch.Keys() index for index
// (clusterDigitX's precedent), not the shared Verb alone. The two
// lists are always the same length by construction, both driven by
// surfaceNames, so no bounds check guards the index (F8).
func statusDigitKeyAt(active Surface, state StateClass, statusRow image.Rectangle, pt image.Point) (string, bool) {
	keys := GrammarKeys.SurfaceSwitch.Keys()
	for i, span := range StatusLineHitSpans(active, state, statusRow) {
		if pt.In(span.Rect) {
			return keys[i], true
		}
	}
	return "", false
}

// specialKeyPresses maps a GrammarKeys key string to the tea.Key code
// it names: every non-printable key any binding's first key currently
// resolves to, plus the arrow/edit keys ultraviolet itself carries
// (up/down/left/right/insert/delete/backspace), so a future binding
// whose first key is one of those also resolves through the map
// rather than keyPressForString's multi-rune fallback.
var specialKeyPresses = map[string]rune{
	"esc":       tea.KeyEscape,
	"enter":     tea.KeyEnter,
	"tab":       tea.KeyTab,
	"space":     tea.KeySpace,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"insert":    tea.KeyInsert,
	"delete":    tea.KeyDelete,
	"backspace": tea.KeyBackspace,
}

// keyPressForString builds the tea.KeyPressMsg whose String() equals
// s (key.Matches's comparison, bubbles/v2/key), ultraviolet's Key
// shape all the way down rather than a Text-only stand-in (F3,
// task-10-findings-r2.md): specialKeyPresses' map lookup first; a
// single-rune printable literal next, Code and Text both set, the
// digitKey convention already uses; and tea.KeyExtended, uv's
// multi-rune code, last for a multi-character string the map does not
// name (no GrammarKeys binding's first key reaches this branch today,
// but a future one could, and it must not synthesize the string's
// first rune alone).
func keyPressForString(s string) tea.KeyPressMsg {
	if code, ok := specialKeyPresses[s]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	runes := []rune(s)
	if len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: s}
	}
	return tea.KeyPressMsg{Code: tea.KeyExtended, Text: s}
}

// fireVerb dispatches b's accelerated verb through the same Update
// path the keyboard uses (ADR-0017's grammar rule): b's primary key,
// Keys()[0] (RULING, task-10-findings-r2.md F6: a multi-key hint's
// click always synthesizes the forward direction, one affordance per
// span, never a second click path for the reverse), since key.Matches
// only needs one of a binding's keys to recognize it.
func (a App) fireVerb(b key.Binding) (tea.Model, tea.Cmd) {
	return a.handleKey(keyPressForString(b.Keys()[0]))
}

// doubleClickWindow is ADR-0017's double-click threshold, a compiled
// constant rather than a config value: two clicks landing on the same
// target inside this window upgrade to the open path; two clicks
// apart, or on different targets, are just two selects.
const doubleClickWindow = 400 * time.Millisecond

// pendingClick is the double-click state machine's open window
// (ADR-0017): the target the first click landed on, and the deadline
// a second click on the same target must beat to upgrade into a
// double-click. T is comparable rather than any (V1,
// task-10-findings-r2.md: `==` on an any holding a non-comparable
// dynamic type panics at runtime, a hole this closes before pass 3
// ever wires a real target). No screen registers PointerRowOpen yet,
// so nothing in the running product owns one this pass;
// resolveDoubleClick and its tests exercise the machine directly
// against a synthetic target, ready for pass 3's rows.
type pendingClick[T comparable] struct {
	open     bool
	target   T
	deadline time.Time
}

// resolveDoubleClick folds one click on target, arriving at now, into
// prev's open window: the same target within the window upgrades and
// closes it; anything else (no window open, a different target, or
// the window already elapsed) opens a fresh window over target
// instead. Single click acts immediately, always, so this only ever
// decides whether the click also upgrades to the open path.
func resolveDoubleClick[T comparable](prev pendingClick[T], target T, now time.Time) (pendingClick[T], bool) {
	if prev.open && prev.target == target && now.Before(prev.deadline) {
		return pendingClick[T]{}, true
	}
	return pendingClick[T]{open: true, target: target, deadline: now.Add(doubleClickWindow)}, false
}
