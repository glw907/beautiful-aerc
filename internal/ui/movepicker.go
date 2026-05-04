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
)

// OpenMovePickerMsg asks App to open the move-to-folder picker.
type OpenMovePickerMsg struct {
	UIDs    []mail.UID
	Src     string
	Folders []FolderEntry
}

// MovePickerPickedMsg is emitted when the user selects a destination folder.
type MovePickerPickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string
}

// MovePickerClosedMsg is emitted when the picker is dismissed without a pick.
type MovePickerClosedMsg struct{}

// MovePicker is the modal overlay launched by `m` from the account view.
// App owns open state and overlay composition (mirrors LinkPicker, ADR-0087).
type MovePicker struct {
	shell   ModalShell
	uids    []mail.UID
	src     string
	all     []FolderEntry
	filter  string
	matches []int
	cursor  int
	offset  int
	styles  Styles
	keys    movePickerKeys
}

type movePickerKeys struct {
	Up        key.Binding
	Down      key.Binding
	Pick      key.Binding
	Close     key.Binding
	Backspace key.Binding
	// Swallow consumes 'q' so the picker doesn't quit the app while
	// open — consistent with the other overlay surfaces.
	Swallow key.Binding
}

func NewMovePicker(styles Styles) MovePicker {
	return MovePicker{
		styles: styles,
		keys: movePickerKeys{
			Up:        key.NewBinding(key.WithKeys("up")),
			Down:      key.NewBinding(key.WithKeys("down")),
			Pick:      key.NewBinding(key.WithKeys("enter")),
			Close:     key.NewBinding(key.WithKeys("esc")),
			Backspace: key.NewBinding(key.WithKeys("backspace")),
			Swallow:   key.NewBinding(key.WithKeys("q")),
		},
	}
}

func (p MovePicker) IsOpen() bool { return p.shell.IsOpen() }

// Open snapshots the targets and folder list. Source folder is
// excluded so the picker never offers a no-op move-to-self.
func (p MovePicker) Open(uids []mail.UID, src string, folders []FolderEntry) MovePicker {
	p.shell = p.shell.WithOpen(true)
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
	return p.recompute()
}

func (p MovePicker) Close() MovePicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p MovePicker) SetSize(width, height int) MovePicker {
	p.shell = p.shell.SetSize(width, height)
	return p
}

// movePickerVisibleRows is the list-row capacity at the given total
// box height. Reserves rows for top + bottom border + filter line +
// preview lines + slack.
func movePickerVisibleRows(height int) int {
	rows := height - 7
	if rows < 1 {
		rows = 1
	}
	return rows
}

// clampOffset adjusts p.offset so p.cursor lies within the visible
// window. Called after every cursor move.
func (p MovePicker) clampOffset() MovePicker {
	p.offset = clampScrollOffset(p.cursor, movePickerVisibleRows(p.shell.Height()), p.offset)
	return p
}

func (p MovePicker) recompute() MovePicker {
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
	return p
}

func (p MovePicker) Update(msg tea.Msg) (MovePicker, tea.Cmd) {
	if !p.shell.IsOpen() {
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
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p.clampOffset(), nil
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
		return p.recompute(), nil
	}
	if key.Matches(keyMsg, p.keys.Swallow) {
		return p, nil
	}
	if r, ok := singlePrintableRune(keyMsg); ok {
		p.filter += string(r)
		return p.recompute(), nil
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
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p MovePicker) Box(w, h int) string {
	boxW := movePickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < movePickerMinWidth {
		boxW = movePickerMinWidth
	}
	contentW := boxW - 2

	maxListRows := movePickerVisibleRows(h)

	rows := p.buildListRows(contentW)
	if len(rows) > maxListRows {
		end := p.offset + maxListRows
		if end > len(rows) {
			end = len(rows)
		}
		rows = rows[p.offset:end]
	}

	bodyRows := make([]string, maxListRows)
	for i := 0; i < maxListRows; i++ {
		if i < len(rows) {
			bodyRows[i] = padOrTruncate(rows[i], contentW)
		} else {
			bodyRows[i] = strings.Repeat(" ", contentW)
		}
	}

	hint := ""
	if p.filter != "" {
		hint = "filter: " + p.filter
	}
	footerRows := []string{
		p.styles.Dim.Render(padOrTruncate(hint, contentW)),
		p.styles.Dim.Render(padOrTruncate("↑↓ select · enter pick · esc cancel", contentW)),
	}

	title := "Move to (" + strconv.Itoa(len(p.matches)) + ")"
	return p.shell.Box(title, bodyRows, footerRows, contentW)
}

func (p MovePicker) buildListRows(contentW int) []string {
	if len(p.matches) == 0 && p.filter != "" {
		return []string{"  no folders match \"" + truncateToWidth(p.filter, contentW-22) + "\""}
	}
	rows := make([]string, 0, len(p.matches)+2)
	prevGroup := mail.Group(-1)
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

func (p MovePicker) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
