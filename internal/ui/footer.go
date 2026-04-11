package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterContext identifies which keybinding set to display.
type FooterContext int

const (
	MsgListContext FooterContext = iota
	SidebarContext
	ViewerContext
)

// Footer renders context-appropriate keybinding hints with group separators.
type Footer struct {
	styles      Styles
	context     FooterContext
	msgKeys     MsgListKeys
	sidebarKeys SidebarKeys
	viewerKeys  ViewerKeys
}

// NewFooter creates a Footer with the given styles.
func NewFooter(styles Styles) Footer {
	return Footer{
		styles:      styles,
		context:     MsgListContext,
		msgKeys:     NewMsgListKeys(),
		sidebarKeys: NewSidebarKeys(),
		viewerKeys:  NewViewerKeys(),
	}
}

// SetContext switches the displayed keybinding set.
func (f *Footer) SetContext(ctx FooterContext) {
	f.context = ctx
}

// View renders the footer at the given width.
func (f Footer) View(width int) string {
	var groups []keyGroup
	switch f.context {
	case SidebarContext:
		groups = f.sidebarKeys.Groups()
	case ViewerContext:
		groups = f.viewerKeys.Groups()
	default:
		groups = f.msgKeys.Groups()
	}

	sep := " " + f.styles.FooterSep.Render("┊") + "  "

	var parts []string
	for _, g := range groups {
		var bindings []string
		for _, b := range g {
			k := f.styles.FooterKey.Render(b.Help().Key)
			d := f.styles.FooterHint.Render(" " + b.Help().Desc)
			bindings = append(bindings, k+d)
		}
		parts = append(parts, strings.Join(bindings, "  "))
	}

	line := " " + strings.Join(parts, sep)
	return lipgloss.NewStyle().Width(width).Render(line)
}
