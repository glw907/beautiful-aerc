// SPDX-License-Identifier: MIT

// Package ui implements poplar's bubbletea terminal UI.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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
	MsgListThreadPrefix  lipgloss.Style

	// Viewer surfaces. ViewerBg is BgBase (body region); ViewerHeader
	// is the BgSubtle panel above it (a slight elevation off the body,
	// keeping the header in the message track rather than the sidebar's
	// BgElevated chrome track). FgDim BorderBottom on BgBase marks the
	// panel/body divider.
	ViewerBg     lipgloss.Style
	ViewerHeader lipgloss.Style

	// Help popover (modal overlay, `?`)
	HelpTitle       lipgloss.Style
	HelpGroupHeader lipgloss.Style
	HelpKey         lipgloss.Style

	// Placeholder text
	Dim lipgloss.Style

	// Search shelf and search-related placeholder
	SearchIcon         lipgloss.Style
	SearchHint         lipgloss.Style
	SearchPrompt       lipgloss.Style
	SearchModeBadge    lipgloss.Style
	SearchResultCount  lipgloss.Style
	SearchNoResults    lipgloss.Style
	MsgListPlaceholder lipgloss.Style

	// Top line frame edge
	TopLine   lipgloss.Style
	ToastText lipgloss.Style

	// ErrorBanner is the one-line surface above the status bar that
	// renders the most recent ErrorMsg. Foreground only; no fill.
	ErrorBanner lipgloss.Style
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

// fillRowToWidth fits a fully-rendered row of ANSI segments to
// exactly width display cells. Short rows are right-padded with
// bgStyle so the row's background extends to the panel edge; over-
// wide rows are truncated to width. Shared by sidebar and message
// list row renderers.
//
// Width is measured with displayCells so Nerd Font SPUA-A icons are
// counted at their true 2-cell width.
func fillRowToWidth(row string, width int, bgStyle lipgloss.Style) string {
	rw := displayCells(row)
	if rw < width {
		return row + bgStyle.Render(strings.Repeat(" ", width-rw))
	}
	if rw > width {
		return displayTruncate(row, width)
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
			Foreground(t.BgBorder).Background(t.BgBase),
		PanelDivider: lipgloss.NewStyle().
			Foreground(t.BgBorder).Background(t.BgBase),

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
			Foreground(t.ColorError).
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
			Background(t.BgSubtle),
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
		MsgListThreadPrefix: lipgloss.NewStyle().
			Foreground(t.FgDim),

		ViewerBg: lipgloss.NewStyle().
			Background(t.BgBase),
		ViewerHeader: lipgloss.NewStyle().
			Background(t.BgSubtle).
			Padding(1, 0, 1, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(t.FgDim).
			BorderBackground(t.BgBase),

		HelpTitle: lipgloss.NewStyle().
			Foreground(t.AccentPrimary).Bold(true),
		HelpGroupHeader: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		HelpKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),

		Dim: lipgloss.NewStyle().
			Foreground(t.FgDim),

		SearchIcon: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchHint: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchPrompt: lipgloss.NewStyle().
			Foreground(t.FgBase),
		SearchModeBadge: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchResultCount: lipgloss.NewStyle().
			Foreground(t.AccentTertiary),
		SearchNoResults: lipgloss.NewStyle().
			Foreground(t.ColorWarning),
		MsgListPlaceholder: lipgloss.NewStyle().
			Foreground(t.FgDim),

		TopLine: lipgloss.NewStyle().
			Foreground(t.BgBorder).Background(t.BgBase),
		ToastText: lipgloss.NewStyle().
			Foreground(t.ColorSuccess),

		ErrorBanner: lipgloss.NewStyle().
			Foreground(t.ColorError),
	}
}

// NewSpinner returns a configured bubbles/spinner.Model with poplar's
// shared style: Dot variant, FgDim foreground. Centralized so future
// folder-load and send-progress placeholders inherit the same look.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
	return sp
}
