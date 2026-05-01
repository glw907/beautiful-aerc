// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpContext selects which binding layout the popover renders.
type HelpContext int

const (
	HelpAccount HelpContext = iota
	HelpViewer
)

// HelpPopover is the modal help overlay. App owns key routing;
// this model only renders.
type HelpPopover struct {
	styles  Styles
	context HelpContext
}

// NewHelpPopover constructs a popover for the given context.
func NewHelpPopover(styles Styles, context HelpContext) HelpPopover {
	return HelpPopover{styles: styles, context: context}
}

// bindingRow is a single key/description entry in the popover.
// Unwired rows render dim per the future-binding policy.
type bindingRow struct {
	key   string
	desc  string
	wired bool
}

// bindingGroup is a labeled cluster of bindingRow entries
// (e.g., "Navigate", "Triage").
type bindingGroup struct {
	title string
	rows  []bindingRow
}

// accountGroups is the binding map shown when the popover opens
// from the account view. Order is the visual layout order.
var accountGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "up/down", true},
			{"g/G", "top/bot", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", true},
			{"a", "archive", true},
			{"s", "star", true},
			{".", "read/unrd", true},
			{"m", "move", true},
			{"E", "empty", true},
			{"u", "undo", true},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
	{
		title: "Search",
		rows: []bindingRow{
			{"/", "search", true},
			{"n", "next", false},
			{"N", "prev", false},
		},
	},
	{
		title: "Select",
		rows: []bindingRow{
			{"v", "select", true},
			{"␣", "toggle", true},
		},
	},
	{
		title: "Threads",
		rows: []bindingRow{
			{"␣", "fold", true},
			{"F", "fold all", true},
		},
	},
	{
		title: "Go To",
		rows: []bindingRow{
			{"I", "inbox", true},
			{"D", "drafts", true},
			{"S", "sent", true},
			{"A", "archive", true},
			{"X", "spam", true},
			{"T", "trash", true},
		},
	},
}

// accountBottomHints is the trailing line under the groups in the
// account context: "Enter open    ?  close".
var accountBottomHints = []bindingRow{
	{"Enter", "open", true},
	{"?", "close", true},
}

// viewerGroups is the binding map shown when the popover opens
// from the message viewer.
var viewerGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "scroll", true},
			{"g/G", "top/bot", true},
			{"␣/b", "page d/u", true},
			{"1-9", "open link", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", true},
			{"a", "archive", true},
			{"s", "star", true},
			{"u", "undo", true},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
}

// viewerBottomHints is the trailing line in the viewer context:
// "Tab link picker    q  close    ?  close".
var viewerBottomHints = []bindingRow{
	{"Tab", "link picker", true},
	{"q", "close", true},
	{"?", "close", true},
}

// Box returns the popover box string sized from its content. The returned
// string does NOT include full-screen padding — it is the raw box ready
// for overlay compositing. The second return value is a "too narrow"
// fallback string; it is non-empty when the box does not fit within
// (width, height) and the caller should display it instead.
func (h HelpPopover) Box(width, height int) (box string, tooNarrow string) {
	var title, body string
	var bottomHints []bindingRow
	switch h.context {
	case HelpViewer:
		title = "Message Viewer"
		body = renderViewerLayout(h.styles, viewerGroups)
		bottomHints = viewerBottomHints
	default:
		title = "Message List"
		body = renderAccountLayout(h.styles, accountGroups)
		bottomHints = accountBottomHints
	}
	inner := body + "\n\n" + renderHintLine(h.styles, bottomHints)

	// Wrap inner in a rounded box, with top border drawn manually
	// so the title can be embedded. Border(style, top, right, bottom, left).
	b := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(h.styles.FrameBorder.GetForeground()).
		Padding(1, 2).
		Render(inner)

	boxWidth := lipgloss.Width(b)
	topEdge := h.renderTopEdge(title, boxWidth)
	popover := topEdge + "\n" + b

	if boxWidth > width || lipgloss.Height(popover) > height {
		return "", h.styles.Dim.Render("Terminal too narrow for help popover")
	}
	return popover, ""
}

// Position returns the top-left (x, y) cell coordinates at which the
// popover box should be placed to appear centered on (width, height).
func (h HelpPopover) Position(box string, width, height int) (x, y int) {
	return centerOverlay(box, width, height)
}

// View renders the popover centered on a width × height area.
// When the underlying account frame is available the caller should use
// Box + Position + PlaceOverlay instead; View is retained as a fallback
// for callers that need a standalone full-screen string (e.g. tests).
func (h HelpPopover) View(width, height int) string {
	box, tooNarrow := h.Box(width, height)
	if tooNarrow != "" {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			tooNarrow,
		)
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

// renderTopEdge builds "╭─ <title> ───╮" at the box's natural width.
func (h HelpPopover) renderTopEdge(title string, boxWidth int) string {
	titleSeg := h.styles.HelpTitle.Render(title)
	border := h.styles.FrameBorder
	prefix := border.Render("╭─ ") + titleSeg + border.Render(" ")
	visible := lipgloss.Width(prefix) + 1 // +1 for the closing ╮
	pad := boxWidth - visible
	if pad < 0 {
		pad = 0
	}
	return prefix + border.Render(strings.Repeat("─", pad)+"╮")
}

// renderAccountLayout builds the four-section layout for the
// account context: three rows (Nav/Triage/Reply, then
// Search/Select/Threads, then Go To grid). Bottom hint line is
// added by View.
func renderAccountLayout(styles Styles, groups []bindingGroup) string {
	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[0]),
		renderGap(),
		renderGroup(styles, groups[1]),
		renderGap(),
		renderGroup(styles, groups[2]),
	)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[3]),
		renderGap(),
		renderGroup(styles, groups[4]),
		renderGap(),
		renderGroup(styles, groups[5]),
	)
	gotoBlock := renderGotoGrid(styles, groups[6])
	return lipgloss.JoinVertical(lipgloss.Left,
		row1, "", row2, "", gotoBlock)
}

// renderViewerLayout builds the single-row layout for the viewer
// context: Nav/Triage/Reply side-by-side.
func renderViewerLayout(styles Styles, groups []bindingGroup) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[0]),
		renderGap(),
		renderGroup(styles, groups[1]),
		renderGap(),
		renderGroup(styles, groups[2]),
	)
}

// renderGroup builds a single labeled column: heading on top,
// then key/desc rows.
func renderGroup(styles Styles, g bindingGroup) string {
	lines := []string{styles.HelpGroupHeader.Render(g.title)}
	for _, r := range g.rows {
		lines = append(lines, renderRow(styles, r))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderRow builds "<key>  <desc>" for a single row, padding the key
// column to a fixed width.
func renderRow(styles Styles, r bindingRow) string {
	const keyWidth = 5
	keyPadded := r.key
	for lipgloss.Width(keyPadded) < keyWidth {
		keyPadded += " "
	}
	return renderKeyDesc(styles, keyPadded, r.desc, r.wired)
}

// renderKeyDesc applies the wired-vs-unwired styling to a key+desc
// pair. Wired: bright-bold key, dim desc. Unwired: entire pair dim
// (no bold) — the contrast is the future-binding signal.
func renderKeyDesc(styles Styles, key, desc string, wired bool) string {
	if wired {
		return styles.HelpKey.Render(key) + "  " + styles.Dim.Render(desc)
	}
	return styles.Dim.Render(key + "  " + desc)
}

// renderGap returns the inter-column spacer used between groups
// on a layout row.
func renderGap() string {
	return "    "
}

// renderGotoGrid builds the Go To group as a 3×2 grid:
// "I inbox    D drafts    S sent" / "A archive  X spam  T trash".
// The group's heading is rendered above. Falls back to a flat
// column if the row count drifts from 6 — defensive against
// careless edits to the binding tables.
func renderGotoGrid(styles Styles, g bindingGroup) string {
	heading := styles.HelpGroupHeader.Render(g.title)
	if len(g.rows) != 6 {
		return renderGroup(styles, g)
	}
	gap := renderGap()
	row1 := renderRow(styles, g.rows[0]) + gap +
		renderRow(styles, g.rows[1]) + gap +
		renderRow(styles, g.rows[2])
	row2 := renderRow(styles, g.rows[3]) + gap +
		renderRow(styles, g.rows[4]) + gap +
		renderRow(styles, g.rows[5])
	return lipgloss.JoinVertical(lipgloss.Left, heading, row1, row2)
}

// renderHintLine builds the bottom hint line: "Enter  open    ?  close".
func renderHintLine(styles Styles, hints []bindingRow) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, renderKeyDesc(styles, h.key, h.desc, h.wired))
	}
	return strings.Join(parts, "    ")
}
