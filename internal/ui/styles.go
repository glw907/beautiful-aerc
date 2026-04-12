// Package ui implements poplar's bubbletea terminal UI.
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds composed lipgloss styles derived from a CompiledTheme.
// Created once at startup and passed read-only to all child components.
type Styles struct {
	// Tab bar
	TabActiveBorder lipgloss.Style
	TabActiveText   lipgloss.Style
	TabInactiveText lipgloss.Style
	TabConnectLine  lipgloss.Style

	// Content frame
	FrameBorder  lipgloss.Style
	PanelDivider lipgloss.Style

	// Status bar
	StatusBar       lipgloss.Style
	StatusConnected lipgloss.Style
	StatusReconnect lipgloss.Style
	StatusOffline   lipgloss.Style

	// Footer
	FooterKey  lipgloss.Style
	FooterHint lipgloss.Style
	FooterSep  lipgloss.Style

	// Selection is a generic selected-row highlight (reserved for the
	// message list and other scrolling panels).
	Selection lipgloss.Style

	// Sidebar. All sidebar rows use SidebarBg as their background.
	// Selected rows override with SidebarSelected (BgSelection).
	SidebarBg        lipgloss.Style
	SidebarAccount   lipgloss.Style
	SidebarSelected  lipgloss.Style
	SidebarFolder    lipgloss.Style
	SidebarUnread    lipgloss.Style
	SidebarIndicator lipgloss.Style

	// Message list. Rows use MsgListBg as their base; selected rows
	// override with MsgListSelected (BgSelection). Read state is
	// encoded by brightness (FgBright/FgDim), not hue. The cursor ▐
	// and the unread+flagged row are the only places hue is used.
	MsgListBg            lipgloss.Style
	MsgListSelected      lipgloss.Style
	MsgListCursor        lipgloss.Style
	MsgListUnreadSender  lipgloss.Style
	MsgListUnreadSubject lipgloss.Style
	MsgListReadSender    lipgloss.Style
	MsgListReadSubject   lipgloss.Style
	MsgListDate          lipgloss.Style
	MsgListIconUnread    lipgloss.Style
	MsgListIconRead      lipgloss.Style
	MsgListFlagFlagged   lipgloss.Style

	// Placeholder text
	Dim lipgloss.Style

	// Top line frame edge
	TopLine   lipgloss.Style
	ToastText lipgloss.Style
}

// applyBg layers the background of bgStyle onto base. Used by row
// renderers (sidebar, message list) to compose a foreground style
// with the row's background color without clobbering already-rendered
// ANSI segments.
func applyBg(base, bgStyle lipgloss.Style) lipgloss.Style {
	if bg, ok := bgStyle.GetBackground().(lipgloss.Color); ok {
		return base.Background(bg)
	}
	return base
}

// fillRowToWidth right-pads a fully-rendered row of ANSI segments to
// exactly width display cells, using bgStyle for the trailing fill so
// the row's background extends to the panel edge. Shared by sidebar
// and message list row renderers.
func fillRowToWidth(row string, width int, bgStyle lipgloss.Style) string {
	if rw := lipgloss.Width(row); rw < width {
		return row + bgStyle.Render(strings.Repeat(" ", width-rw))
	}
	return row
}

// NewStyles creates a Styles from a CompiledTheme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		TabActiveBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		TabActiveText: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).
			Background(t.BgBase),
		TabInactiveText: lipgloss.NewStyle().
			Foreground(t.FgDim),
		TabConnectLine: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		FrameBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		PanelDivider: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.FgBright).
			Background(t.BgBorder),
		StatusConnected: lipgloss.NewStyle().
			Foreground(t.ColorSuccess).
			Background(t.BgBorder),
		StatusReconnect: lipgloss.NewStyle().
			Foreground(t.ColorWarning).
			Background(t.BgBorder),
		StatusOffline: lipgloss.NewStyle().
			Foreground(t.FgDim).
			Background(t.BgBorder),

		FooterKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		FooterHint: lipgloss.NewStyle().
			Foreground(t.FgDim),
		FooterSep: lipgloss.NewStyle().
			Foreground(t.FgDim),

		Selection: lipgloss.NewStyle().
			Background(t.BgSelection),

		SidebarBg: lipgloss.NewStyle().
			Background(t.BgElevated),
		SidebarAccount: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).Bold(true).
			Background(t.BgElevated),
		SidebarSelected: lipgloss.NewStyle().
			Background(t.BgSelection),
		SidebarFolder: lipgloss.NewStyle().
			Foreground(t.FgBase),
		SidebarUnread: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		SidebarIndicator: lipgloss.NewStyle().
			Foreground(t.AccentSecondary),

		MsgListBg: lipgloss.NewStyle().
			Background(t.BgBase),
		MsgListSelected: lipgloss.NewStyle().
			Background(t.BgSelection),
		MsgListCursor: lipgloss.NewStyle().
			Foreground(t.AccentPrimary),
		MsgListUnreadSender: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		MsgListUnreadSubject: lipgloss.NewStyle().
			Foreground(t.FgBright),
		MsgListReadSender: lipgloss.NewStyle().
			Foreground(t.FgDim),
		MsgListReadSubject: lipgloss.NewStyle().
			Foreground(t.FgDim),
		MsgListDate: lipgloss.NewStyle().
			Foreground(t.FgDim),
		MsgListIconUnread: lipgloss.NewStyle().
			Foreground(t.FgBright),
		MsgListIconRead: lipgloss.NewStyle().
			Foreground(t.FgDim),
		MsgListFlagFlagged: lipgloss.NewStyle().
			Foreground(t.ColorWarning),

		Dim: lipgloss.NewStyle().
			Foreground(t.FgDim),

		TopLine: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		ToastText: lipgloss.NewStyle().
			Foreground(t.ColorSuccess),
	}
}
