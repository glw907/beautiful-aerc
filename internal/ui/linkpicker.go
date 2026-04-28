// SPDX-License-Identifier: MIT

package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
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
	theme  *theme.CompiledTheme
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Close key.Binding
}

// NewLinkPicker returns a closed picker.
func NewLinkPicker(styles Styles, t *theme.CompiledTheme) LinkPicker {
	return LinkPicker{
		styles: styles,
		theme:  t,
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
	if s := keyMsg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[idx]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	}
	return p, nil
}
