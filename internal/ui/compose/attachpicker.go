package compose

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

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

type readDirMsg struct {
	id      int
	entries []attachEntry
	err     error
}

func (p AttachPicker) Open(dir string) (AttachPicker, tea.Cmd) {
	p.shell = p.shell.WithOpen(true)
	p.id++
	p.dir = dir
	p.entries = nil
	p.cursor = 0
	p.offset = 0
	p.selected = map[string]bool{}
	p.stack = nil
	p.err = ""
	return p, readDirCmd(p.id, dir, p.showHidden)
}

func readDirCmd(id int, dir string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		raw, err := os.ReadDir(dir)
		if err != nil {
			return readDirMsg{id: id, err: err}
		}
		out := make([]attachEntry, 0, len(raw))
		for _, e := range raw {
			name := e.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, attachEntry{
				name:  name,
				path:  filepath.Join(dir, name),
				isDir: e.IsDir(),
				size:  info.Size(),
			})
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].isDir != out[j].isDir {
				return out[i].isDir
			}
			return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
		})
		return readDirMsg{id: id, entries: out}
	}
}

func (p AttachPicker) Update(msg tea.Msg) (AttachPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	switch m := msg.(type) {
	case readDirMsg:
		if m.id != p.id {
			return p, nil
		}
		if m.err != nil {
			p.err = "cannot read " + p.dir + ": " + m.err.Error()
			return p, nil
		}
		p.entries = m.entries
		p.err = ""
		if p.cursor >= len(p.entries) {
			p.cursor = 0
		}
		return p, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(m, p.keys.Down):
			if p.cursor < len(p.entries)-1 {
				p.cursor++
			}
			return p.clampOffset(), nil
		case key.Matches(m, p.keys.Up):
			if p.cursor > 0 {
				p.cursor--
			}
			return p.clampOffset(), nil
		case key.Matches(m, p.keys.GoTop):
			p.cursor, p.offset = 0, 0
			return p, nil
		case key.Matches(m, p.keys.GoBot):
			if len(p.entries) > 0 {
				p.cursor = len(p.entries) - 1
			}
			return p.clampOffset(), nil
		case key.Matches(m, p.keys.PgDown):
			step := p.viewportRows()
			p.cursor += step
			if p.cursor >= len(p.entries) {
				p.cursor = len(p.entries) - 1
			}
			return p.clampOffset(), nil
		case key.Matches(m, p.keys.PgUp):
			step := p.viewportRows()
			p.cursor -= step
			if p.cursor < 0 {
				p.cursor = 0
			}
			return p.clampOffset(), nil
		case key.Matches(m, p.keys.Open):
			if len(p.entries) == 0 {
				return p, nil
			}
			e := p.entries[p.cursor]
			if e.isDir {
				return p.descend(e.path)
			}
			// file handling added in Task 7
			return p, nil
		case key.Matches(m, p.keys.Back):
			return p.ascend()
		case key.Matches(m, p.keys.ToggleHidden):
			p.showHidden = !p.showHidden
			p.id++
			p.entries = nil
			return p, readDirCmd(p.id, p.dir, p.showHidden)
		}
	}
	return p, nil
}

func (p AttachPicker) descend(path string) (AttachPicker, tea.Cmd) {
	p.stack = append(p.stack, attachViewState{cursor: p.cursor, offset: p.offset})
	p.id++
	p.dir = path
	p.entries = nil
	p.cursor, p.offset = 0, 0
	return p, readDirCmd(p.id, p.dir, p.showHidden)
}

func (p AttachPicker) ascend() (AttachPicker, tea.Cmd) {
	parent := filepath.Dir(p.dir)
	if parent == p.dir {
		return p, nil
	}
	var prev attachViewState
	if n := len(p.stack); n > 0 {
		prev = p.stack[n-1]
		p.stack = p.stack[:n-1]
	}
	p.id++
	p.dir = parent
	p.entries = nil
	p.cursor, p.offset = prev.cursor, prev.offset
	return p, readDirCmd(p.id, p.dir, p.showHidden)
}

// viewportRows is the body height available for entries inside the
// ModalShell box. shell.Height() includes border + title + footer
// rows; subtract a fixed budget of 5 (1 top border, 1 title, 1 sep,
// 2 footer rows, 1 bottom border) conservatively.
func (p AttachPicker) viewportRows() int {
	h := p.shell.Height() - 5
	if h < 1 {
		return 1
	}
	return h
}

func (p AttachPicker) clampOffset() AttachPicker {
	rows := p.viewportRows()
	p.offset = uicore.ClampScrollOffset(p.cursor, rows, p.offset)
	return p
}

// View is a no-op stub. Filled in by later tasks.
func (p AttachPicker) View() string { return "" }
