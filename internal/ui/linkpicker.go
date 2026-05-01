// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// LinkPicker is the modal overlay launched by Tab while the viewer is
// open and ready. Single-column list of harvested URLs, cursor +
// Enter, 1-9 quick launch, Esc/Tab close. App owns the open state and
// the overlay composition (mirrors help popover, ADR-0082).
type LinkPicker struct {
	open   bool
	links  []string
	cursor int
	offset int
	width  int
	height int
	styles Styles
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Close key.Binding
}

// NewLinkPicker returns a closed picker.
func NewLinkPicker(styles Styles) LinkPicker {
	return LinkPicker{
		styles: styles,
		keys: linkPickerKeys{
			Up:    key.NewBinding(key.WithKeys("k", "up")),
			Down:  key.NewBinding(key.WithKeys("j", "down")),
			Enter: key.NewBinding(key.WithKeys("enter")),
			Close: key.NewBinding(key.WithKeys("esc", "tab")),
		},
	}
}

// IsOpen reports whether the picker is visible.
func (p LinkPicker) IsOpen() bool { return p.open }

// Cursor returns the highlighted row index. Exposed for tests.
func (p LinkPicker) Cursor() int { return p.cursor }

// Open transitions the picker into the open state with the given URL
// list. Cursor and offset reset to 0.
func (p LinkPicker) Open(links []string) LinkPicker {
	p.open = true
	p.links = links
	p.cursor = 0
	p.offset = 0
	return p
}

// Close transitions the picker out of view. Caller is responsible for
// any chrome-revert side effects (App handles this via Msg flow).
func (p LinkPicker) Close() LinkPicker {
	p.open = false
	return p
}

// SetSize updates the picker's box dimensions. App threads
// WindowSizeMsg here.
func (p LinkPicker) SetSize(width, height int) LinkPicker {
	p.width = width
	p.height = height
	return p
}

// Update dispatches a tea.Msg while the picker is open. Returns the
// updated picker and any Cmds (launch + close on Enter / numeric;
// close on Esc/Tab; nil otherwise).
func (p LinkPicker) Update(msg tea.Msg) (LinkPicker, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.links)-1 {
			p.cursor++
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Enter):
		if p.cursor < 0 || p.cursor >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[p.cursor]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return LinkPickerClosedMsg{} }
	}
	if idx, ok := parseLinkKey(keyMsg.String(), len(p.links)); ok {
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[idx]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	}
	return p, nil
}

// linkPickerMaxWidth caps the picker's natural width.
const linkPickerMaxWidth = 70

// linkPickerInlineCap caps the inline URL display length per row,
// independent of box width — keeps the visual tight even on very wide
// terminals.
const linkPickerInlineCap = 50

// View renders the picker as a standalone string. App composes via
// Box + Position + PlaceOverlay; this method is the fallback used by
// tests and when the box doesn't fit.
func (p LinkPicker) View() string {
	if !p.open {
		return ""
	}
	return p.Box(p.width, p.height)
}

// Box returns the rendered modal at the size derived from (w, h).
func (p LinkPicker) Box(w, h int) string {
	boxW := linkPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 20 {
		boxW = 20
	}
	contentW := boxW - 2 // left/right border
	maxIndexDigits := len(strconv.Itoa(len(p.links)))
	indexW := 2 + maxIndexDigits
	urlW := contentW - indexW - 1 // 1 space between index and URL
	if urlW > linkPickerInlineCap {
		urlW = linkPickerInlineCap
	}

	visibleRows := len(p.links)
	maxListRows := h - 7 // top + bottom border + rule + 2 preview + 1 title slack
	if maxListRows < 1 {
		maxListRows = 1
	}
	if visibleRows > maxListRows {
		visibleRows = maxListRows
	}

	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visibleRows {
		p.offset = p.cursor - visibleRows + 1
	}

	var b strings.Builder
	title := " Links "
	rest := boxW - 2 - len(title)
	if rest < 0 {
		rest = 0
	}
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")

	for i := 0; i < visibleRows; i++ {
		row := p.offset + i
		if row >= len(p.links) {
			b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
			continue
		}
		b.WriteString("│")
		b.WriteString(p.formatRow(row, maxIndexDigits, urlW, contentW))
		b.WriteString("│\n")
	}

	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")

	previewLines := p.previewLines(contentW)
	for i := 0; i < 2; i++ {
		line := ""
		if i < len(previewLines) {
			line = previewLines[i]
		}
		lw := lipgloss.Width(line)
		if lw < contentW {
			line += strings.Repeat(" ", contentW-lw)
		}
		b.WriteString("│" + line + "│\n")
	}

	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}

// formatRow renders one list row: leading-space-pad + [N] + space + URL.
// Painted with cursor background when row == p.cursor.
func (p LinkPicker) formatRow(row, maxIndexDigits, urlW, contentW int) string {
	idxStr := strconv.Itoa(row + 1)
	pad := strings.Repeat(" ", maxIndexDigits-len(idxStr))
	url := p.links[row]
	if displayCells(url) > urlW {
		url = displayTruncate(url, urlW)
	}
	body := fmt.Sprintf("%s[%d] %s", pad, row+1, url)
	bw := lipgloss.Width(body)
	if bw < contentW {
		body += strings.Repeat(" ", contentW-bw)
	}
	if row == p.cursor {
		return p.styles.MsgListCursor.Render(body)
	}
	return body
}

// previewLines returns up to 2 wrapped lines of the cursor row's full
// URL. The 2nd line is truncated with "…" when the URL exceeds 2
// rows worth of cells.
func (p LinkPicker) previewLines(width int) []string {
	if p.cursor < 0 || p.cursor >= len(p.links) {
		return nil
	}
	full := p.links[p.cursor]
	wrapped := strings.Split(linkPickerWrap(full, width), "\n")
	if len(wrapped) <= 2 {
		return wrapped
	}
	row2 := wrapped[1]
	if displayCells(row2) >= width {
		row2 = displayTruncate(row2, width-1) + "…"
	} else {
		row2 += "…"
	}
	return []string{wrapped[0], row2}
}

// linkPickerWrap honors width for the preview footer. URLs are
// unbreakable tokens, so Wordwrap can't split them — Hardwrap forces
// the residue to honor width. Mirrors content.wrap; cross-package
// duplication is acceptable at two callers.
func linkPickerWrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, false)
}

// Position returns the centered top-left for the rendered box at
// (totalW, totalH). Used by App to feed PlaceOverlay.
func (p LinkPicker) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}

// parseLinkKey decodes a 1-9 keypress into a link index. Returns
// (idx, true) when the key is a digit in [1, count]; (0, false)
// otherwise. Shared by the viewer's quick-launch and the link picker.
func parseLinkKey(s string, count int) (int, bool) {
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0, false
	}
	idx := int(s[0] - '1')
	if idx >= count {
		return 0, false
	}
	return idx, true
}

// centerOverlay returns the top-left (x, y) cell coordinates that
// center box on (totalW, totalH). Shared by the help popover and the
// link picker; both compose via PlaceOverlay.
func centerOverlay(box string, totalW, totalH int) (int, int) {
	x := (totalW - lipgloss.Width(box)) / 2
	y := (totalH - lipgloss.Height(box)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}
