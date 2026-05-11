package compose

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/contacts"
)

// SuggestFn is the autocomplete seam: pass a prefix, get back rows.
type SuggestFn func(prefix string) []contacts.Suggestion

const minPrefix = 2

type DropdownKeyMap struct {
	Up   key.Binding
	Down key.Binding
}

func DefaultDropdownKeyMap() DropdownKeyMap {
	return DropdownKeyMap{
		Up:   key.NewBinding(key.WithKeys("up")),
		Down: key.NewBinding(key.WithKeys("down")),
	}
}

// Dropdown owns the result set and cursor; compose.Model owns the
// field-rewrite path.
type Dropdown struct {
	fn     SuggestFn
	rows   []contacts.Suggestion
	cursor int
	styles Styles
	keys   DropdownKeyMap
}

func NewDropdown(fn SuggestFn) Dropdown {
	return Dropdown{fn: fn, keys: DefaultDropdownKeyMap()}
}

func (d Dropdown) WithStyles(s Styles) Dropdown {
	d.styles = s
	return d
}

// SetPrefix re-runs SuggestFn; selection always resets to the head row
// since carrying a cursor across edits is rarely what the user wants.
func (d Dropdown) SetPrefix(p string) Dropdown {
	d.cursor = 0
	if d.fn == nil || len(p) < minPrefix {
		d.rows = nil
		return d
	}
	d.rows = d.fn(p)
	return d
}

func (d Dropdown) Empty() bool { return len(d.rows) == 0 }

func (d Dropdown) Selected() (contacts.Suggestion, bool) {
	if len(d.rows) == 0 {
		return contacts.Suggestion{}, false
	}
	return d.rows[d.cursor], true
}

func (d Dropdown) Clear() Dropdown {
	d.rows = nil
	d.cursor = 0
	return d
}

// Update handles Up/Down cursor motion.
func (d Dropdown) Update(msg tea.Msg) (Dropdown, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || len(d.rows) == 0 {
		return d, nil
	}
	switch {
	case key.Matches(k, d.keys.Down):
		d.cursor = (d.cursor + 1) % len(d.rows)
	case key.Matches(k, d.keys.Up):
		d.cursor = (d.cursor - 1 + len(d.rows)) % len(d.rows)
	}
	return d, nil
}

func (d Dropdown) View() string {
	if len(d.rows) == 0 {
		return ""
	}
	lines := make([]string, len(d.rows))
	for i, r := range d.rows {
		head := fmt.Sprintf("%s <%s>", r.Name, r.Email)
		line := head
		if !r.IsOrg && r.Org != "" {
			line += d.styles.DropdownOrg.Render(" · " + r.Org)
		}
		if i == d.cursor {
			line = d.styles.DropdownRowSelected.Render(line)
		} else {
			line = d.styles.DropdownRow.Render(line)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
