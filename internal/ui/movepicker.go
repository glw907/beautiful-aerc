// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
