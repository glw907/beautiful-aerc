package ui

import "github.com/glw907/poplar/internal/theme"

// ComposeView is the one composition path App.View and cmd/sketch
// both render through, so cmd/sketch's fixtures are painted evidence
// for exactly what the running product shows (design decision 10):
// below the floor (width or height, design language section 9) the
// centered notice renderFloorNotice composes is the whole frame,
// selected before anything else runs, since a StateModal stack top
// (Confirm, most notably) would otherwise render its box at a size
// confirmBoxWidth never promised to tolerate, and the floor rung's
// chrome-free premise (mouse.go's dispatchClick) would be a lie.
// Above the floor, a StateModal stack top renders itself directly: a
// plain stack-top render, no dimmed backdrop, since a modal owns the
// whole terminal itself rather than landing in a named LayoutMode
// pane. Every other front, active with an empty stack or a non-modal
// screen pushed onto stack (the help overlay, first of its kind),
// runs through Render, the same seam the gallery renders through, so
// the product never drifts from what the gallery pins: a pushed
// screen's content fills the whole Main band (RenderInput.FullRegion),
// the same treatment pass 2's four surface placeholders get
// (isPlaceholderScreen, F1/F8's ruling: each owns no sidebar and no
// split), rather than the narrower Content pane a surface's sidebar
// reservation would otherwise squeeze either against. active is
// unused once stack is non-empty, so a caller with nothing to offer
// there (cmd/sketch's modal-confirm and help fixtures, neither of
// which is also the active surface) may pass nil.
func ComposeView(layout LayoutMode, th theme.Theme, status StatusLine, banner Banner, active Screen, stack []Screen) Frame {
	if layout.Class == WidthFloor || layout.HeightClass == HeightFloor {
		return Frame{Content: renderFloorNotice(th, layout.Width, layout.Height)}
	}

	screen := active
	fullRegion := isPlaceholderScreen(screen)
	if len(stack) > 0 {
		top := stack[len(stack)-1]
		if top.Entry().SwitchState == StateModal {
			view := top.View()
			return Frame{Content: view.Content, Cursor: view.Cursor}
		}
		screen, fullRegion = top, true
	}
	return Render(RenderInput{Screen: screen, FullRegion: fullRegion, Layout: layout, Theme: th, Status: status, Banner: banner})
}

// isPlaceholderScreen reports whether screen is one of pass 2's four
// surface placeholders (F1/F8's ruling): each owns no sidebar and no
// split, so ComposeView renders it FullRegion the same way a pushed
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
