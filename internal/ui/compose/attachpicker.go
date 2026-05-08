package compose

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// AttachPicker is the compose-side multi-select file browser overlay.
// Vim-style nav (j/k/g/G), async readDir with an id guard so stale
// results don't clobber a fresh listing, and a view-state stack so
// ascend lands the cursor back on the dir you came from.
type AttachPicker struct {
	shell      uicore.ModalShell
	id         int
	dir        string
	entries    []attachEntry
	cursor     int
	offset     int
	selected   map[string]bool
	showHidden bool
	stack      []attachViewState
	err        string
	styles     Styles
	icons      uicore.IconSet
	keys       attachPickerKeys
}

type attachEntry struct {
	name  string
	path  string
	isDir bool
	size  int64
}

type attachViewState struct{ cursor, offset int }

type attachPickerKeys struct {
	Up, Down     key.Binding
	PgUp, PgDown key.Binding
	GoTop, GoBot key.Binding
	Open         key.Binding // l, right, enter
	Back         key.Binding // h, left, backspace
	Toggle       key.Binding // space
	Accept       key.Binding // a
	ToggleHidden key.Binding // .
	Close        key.Binding // esc
}

func defaultAttachPickerKeys() attachPickerKeys {
	return attachPickerKeys{
		Up:           key.NewBinding(key.WithKeys("k", "up")),
		Down:         key.NewBinding(key.WithKeys("j", "down")),
		PgUp:         key.NewBinding(key.WithKeys("K", "pgup")),
		PgDown:       key.NewBinding(key.WithKeys("J", "pgdown")),
		GoTop:        key.NewBinding(key.WithKeys("g", "home")),
		GoBot:        key.NewBinding(key.WithKeys("G", "end")),
		Open:         key.NewBinding(key.WithKeys("l", "right", "enter")),
		Back:         key.NewBinding(key.WithKeys("h", "left", "backspace")),
		Toggle:       key.NewBinding(key.WithKeys(" ")),
		Accept:       key.NewBinding(key.WithKeys("a")),
		ToggleHidden: key.NewBinding(key.WithKeys(".")),
		Close:        key.NewBinding(key.WithKeys("esc")),
	}
}

// NewAttachPicker returns a closed picker. Open(dir) bumps the id
// and returns the readDir cmd that populates entries.
func NewAttachPicker(styles Styles, icons uicore.IconSet) AttachPicker {
	return AttachPicker{
		styles:   styles,
		icons:    icons,
		keys:     defaultAttachPickerKeys(),
		selected: map[string]bool{},
	}
}

func (p AttachPicker) IsOpen() bool { return p.shell.IsOpen() }

func (p AttachPicker) Close() AttachPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p AttachPicker) SetSize(w, h int) AttachPicker {
	p.shell = p.shell.SetSize(w, h)
	return p
}

// Update is a no-op stub. Filled in by later tasks.
func (p AttachPicker) Update(msg tea.Msg) (AttachPicker, tea.Cmd) {
	return p, nil
}

// View is a no-op stub. Filled in by later tasks.
func (p AttachPicker) View() string { return "" }
