// SPDX-License-Identifier: MIT

package ui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// MovePicker is the modal overlay launched by `m` from the account view.
// App owns open state and overlay composition (mirrors LinkPicker, ADR-0087).
type MovePicker struct {
	open    bool
	uids    []mail.UID
	src     string
	all     []FolderEntry
	filter  string
	matches []int
	cursor  int
	offset  int
	width   int
	height  int
	styles  Styles
	theme   *theme.CompiledTheme
	keys    movePickerKeys
}

type movePickerKeys struct {
	Up        key.Binding
	Down      key.Binding
	Pick      key.Binding
	Close     key.Binding
	Backspace key.Binding
}

// NewMovePicker returns a closed picker ready to be opened.
func NewMovePicker(styles Styles, t *theme.CompiledTheme) MovePicker {
	return MovePicker{
		styles: styles,
		theme:  t,
		keys: movePickerKeys{
			Up:        key.NewBinding(key.WithKeys("up")),
			Down:      key.NewBinding(key.WithKeys("down")),
			Pick:      key.NewBinding(key.WithKeys("enter")),
			Close:     key.NewBinding(key.WithKeys("esc")),
			Backspace: key.NewBinding(key.WithKeys("backspace")),
		},
	}
}

func (p MovePicker) IsOpen() bool { return p.open }

// Open transitions the picker into view with a fresh snapshot.
// The source folder is excluded; filter + cursor reset to zero.
func (p MovePicker) Open(uids []mail.UID, src string, folders []FolderEntry) MovePicker {
	p.open = true
	p.uids = uids
	p.src = src
	p.all = make([]FolderEntry, 0, len(folders))
	for _, f := range folders {
		if f.Provider == src {
			continue
		}
		p.all = append(p.all, f)
	}
	p.filter = ""
	p.cursor = 0
	p.offset = 0
	p.recompute()
	return p
}

func (p MovePicker) Close() MovePicker {
	p.open = false
	return p
}

func (p MovePicker) SetSize(width, height int) MovePicker {
	p.width = width
	p.height = height
	return p
}

func (p *MovePicker) recompute() {
	p.matches = p.matches[:0]
	if cap(p.matches) < len(p.all) {
		p.matches = make([]int, 0, len(p.all))
	}
	needle := strings.ToLower(p.filter)
	for i, f := range p.all {
		if needle == "" || strings.Contains(strings.ToLower(f.Display), needle) {
			p.matches = append(p.matches, i)
		}
	}
	p.cursor = 0
	p.offset = 0
}

// Update handles key events while the picker is open.
func (p MovePicker) Update(msg tea.Msg) (MovePicker, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Pick):
		if p.cursor < 0 || p.cursor >= len(p.matches) {
			return p, nil
		}
		dest := p.all[p.matches[p.cursor]].Provider
		picked := MovePickerPickedMsg{UIDs: p.uids, Src: p.src, Dest: dest}
		return p, tea.Batch(
			func() tea.Msg { return picked },
			func() tea.Msg { return MovePickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return MovePickerClosedMsg{} }
	case key.Matches(keyMsg, p.keys.Backspace):
		if p.filter == "" {
			return p, nil
		}
		_, size := utf8.DecodeLastRuneInString(p.filter)
		p.filter = p.filter[:len(p.filter)-size]
		p.recompute()
		return p, nil
	}
	// q is swallowed — consistent with help/link picker overlays.
	if keyMsg.String() == "q" {
		return p, nil
	}
	if r, ok := singlePrintableRune(keyMsg); ok {
		p.filter += string(r)
		p.recompute()
		return p, nil
	}
	return p, nil
}

func singlePrintableRune(k tea.KeyMsg) (rune, bool) {
	if len(k.Runes) != 1 {
		return 0, false
	}
	r := k.Runes[0]
	if !unicode.IsPrint(r) {
		return 0, false
	}
	return r, true
}

const (
	movePickerMaxWidth = 50
	movePickerMinWidth = 24
)

func (p MovePicker) View() string {
	if !p.open {
		return ""
	}
	return p.Box(p.width, p.height)
}

// Box renders the picker modal at the size derived from (w, h).
func (p MovePicker) Box(w, h int) string {
	boxW := movePickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < movePickerMinWidth {
		boxW = movePickerMinWidth
	}
	contentW := boxW - 2

	maxListRows := h - 7
	if maxListRows < 1 {
		maxListRows = 1
	}

	rows := p.buildListRows(contentW)
	if len(rows) > maxListRows {
		if p.cursor < p.offset {
			p.offset = p.cursor
		}
		if p.cursor >= p.offset+maxListRows {
			p.offset = p.cursor - maxListRows + 1
		}
		end := p.offset + maxListRows
		if end > len(rows) {
			end = len(rows)
		}
		rows = rows[p.offset:end]
	}

	var b strings.Builder
	title := " Move to (" + strconv.Itoa(len(p.matches)) + ") "
	rest := boxW - 2 - lipgloss.Width(title)
	if rest < 0 {
		rest = 0
	}
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")

	for _, row := range rows {
		padded := padOrTruncate(row, contentW)
		b.WriteString("│" + padded + "│\n")
	}
	for i := len(rows); i < maxListRows; i++ {
		b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
	}

	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")

	hint := ""
	if p.filter != "" {
		hint = "filter: " + p.filter
	}
	b.WriteString("│" + p.styles.Dim.Render(padOrTruncate(hint, contentW)) + "│\n")

	help := "↑↓ select · enter pick · esc cancel"
	b.WriteString("│" + p.styles.Dim.Render(padOrTruncate(help, contentW)) + "│\n")

	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}

func (p MovePicker) buildListRows(contentW int) []string {
	if len(p.matches) == 0 && p.filter != "" {
		return []string{"  no folders match \"" + truncateToWidth(p.filter, contentW-22) + "\""}
	}
	rows := make([]string, 0, len(p.matches)+2)
	prevGroup := FolderGroup(-1)
	for i, idx := range p.matches {
		entry := p.all[idx]
		if p.filter == "" && i > 0 && entry.Group != prevGroup {
			rows = append(rows, "")
		}
		prevGroup = entry.Group
		marker := "  "
		if i == p.cursor {
			marker = "> "
		}
		row := marker + entry.Display
		if i == p.cursor {
			row = p.styles.MsgListCursor.Render(padOrTruncate(row, contentW))
		}
		rows = append(rows, row)
	}
	return rows
}

// padOrTruncate pads s with spaces or truncates it to exactly width display cells.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return truncateToWidth(s, width)
}

// Position returns the centered top-left for the rendered box at (totalW, totalH).
func (p MovePicker) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
