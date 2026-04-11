package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// FooterContext identifies which keybinding set to display.
type FooterContext int

const (
	MsgListContext FooterContext = iota
	SidebarContext
)

// Footer renders context-appropriate keybinding hints.
type Footer struct {
	styles      Styles
	help        help.Model
	context     FooterContext
	msgKeys     MsgListKeys
	sidebarKeys SidebarKeys
}

// NewFooter creates a Footer with the given styles.
func NewFooter(styles Styles) Footer {
	h := help.New()
	h.ShortSeparator = "  "
	h.Styles.ShortKey = styles.FooterKey
	h.Styles.ShortDesc = styles.FooterHint
	h.Styles.ShortSeparator = styles.FooterHint

	return Footer{
		styles:      styles,
		help:        h,
		context:     MsgListContext,
		msgKeys:     NewMsgListKeys(),
		sidebarKeys: NewSidebarKeys(),
	}
}

// SetContext switches the displayed keybinding set.
func (f *Footer) SetContext(ctx FooterContext) {
	f.context = ctx
}

// View renders the footer at the given width.
func (f Footer) View(width int) string {
	f.help.Width = width

	var line string
	switch f.context {
	case SidebarContext:
		line = f.help.ShortHelpView(f.sidebarKeys.ShortHelp())
	default:
		line = f.help.ShortHelpView(f.msgKeys.ShortHelp())
	}

	return lipgloss.NewStyle().Width(width).Render(line)
}
