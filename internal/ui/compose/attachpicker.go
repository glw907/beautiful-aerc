package compose

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/humanize"
	"github.com/glw907/poplar/internal/ui/uicore"
)

var nextAttachID atomic.Int64

func nextID() int { return int(nextAttachID.Add(1)) }

// AttachPicker is the compose-side multi-select file browser overlay.
// Vim-style nav (j/k/g/G), async readDir with an id guard so stale
// results don't clobber a fresh listing, and a view-state stack so
// ascend lands the cursor back on the dir you came from.
type AttachPicker struct {
	shell      uicore.ModalShell
	currentID  int
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
	name   string
	path   string
	isDir  bool
	size   int64
	target string // non-empty when entry is a symlink that resolved successfully
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
		Toggle:       key.NewBinding(key.WithKeys("space")),
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
	p.currentID = nextID()
	p.dir = dir
	p.entries = nil
	p.cursor = 0
	p.offset = 0
	p.selected = map[string]bool{}
	p.stack = nil
	p.err = ""
	return p, readDirCmd(p.currentID, dir, p.showHidden)
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
			full := filepath.Join(dir, name)
			ent := attachEntry{name: name, path: full}
			if e.Type()&os.ModeSymlink != 0 {
				if resolved, rerr := filepath.EvalSymlinks(full); rerr == nil {
					if info, serr := os.Stat(resolved); serr == nil {
						ent.isDir = info.IsDir()
						ent.target = resolved
					}
				}
				if !ent.isDir {
					if info, serr := os.Stat(full); serr == nil {
						ent.size = info.Size()
					}
				}
			} else {
				info, ierr := e.Info()
				if ierr != nil {
					continue
				}
				ent.isDir = info.IsDir()
				ent.size = info.Size()
			}
			out = append(out, ent)
		}
		slices.SortStableFunc(out, func(a, b attachEntry) int {
			if a.isDir != b.isDir {
				if a.isDir {
					return -1
				}
				return 1
			}
			return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
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
		if m.id != p.currentID {
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
	case tea.KeyPressMsg:
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
			// Enter on file: shortcut single-attach when nothing is yet
			// selected. Otherwise toggle this file into the running selection.
			if selectedCount(p) == 0 {
				return p, func() tea.Msg {
					return AttachAcceptedMsg{Paths: []string{e.path}}
				}
			}
			p.selected[e.path] = !p.selected[e.path]
			return p, nil
		case key.Matches(m, p.keys.Toggle):
			if len(p.entries) == 0 || p.entries[p.cursor].isDir {
				return p, nil
			}
			path := p.entries[p.cursor].path
			p.selected[path] = !p.selected[path]
			return p, nil
		case key.Matches(m, p.keys.Accept):
			paths := p.acceptedPaths()
			if len(paths) == 0 {
				return p, nil
			}
			return p, func() tea.Msg { return AttachAcceptedMsg{Paths: paths} }
		case key.Matches(m, p.keys.Close):
			return p, func() tea.Msg { return AttachCancelledMsg{} }
		case key.Matches(m, p.keys.Back):
			return p.ascend()
		case key.Matches(m, p.keys.ToggleHidden):
			p.showHidden = !p.showHidden
			p.currentID = nextID()
			p.entries = nil
			return p, readDirCmd(p.currentID, p.dir, p.showHidden)
		}
	}
	return p, nil
}

// acceptedPaths returns selected paths in the entry order of the
// current directory (stable, predictable).
func (p AttachPicker) acceptedPaths() []string {
	out := make([]string, 0, len(p.selected))
	for _, e := range p.entries {
		if p.selected[e.path] {
			out = append(out, e.path)
		}
	}
	return out
}

func selectedCount(p AttachPicker) int {
	n := 0
	for _, v := range p.selected {
		if v {
			n++
		}
	}
	return n
}

func (p AttachPicker) descend(path string) (AttachPicker, tea.Cmd) {
	p.stack = append(p.stack, attachViewState{cursor: p.cursor, offset: p.offset})
	p.currentID = nextID()
	p.dir = path
	p.entries = nil
	p.cursor, p.offset = 0, 0
	return p, readDirCmd(p.currentID, p.dir, p.showHidden)
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
	p.currentID = nextID()
	p.dir = parent
	p.entries = nil
	p.cursor, p.offset = prev.cursor, prev.offset
	return p, readDirCmd(p.currentID, p.dir, p.showHidden)
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

const attachPickerMaxWidth = 70

func (p AttachPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	w := p.shell.Width()
	boxW := attachPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 32 {
		boxW = 32
	}
	contentW := boxW - 2

	rows := p.viewportRows()
	bodyRows := make([]string, rows)
	for i := range rows {
		idx := p.offset + i
		if idx >= len(p.entries) {
			bodyRows[i] = uicore.PadOrTruncate("", contentW)
			continue
		}
		bodyRows[i] = p.formatEntry(idx, contentW)
	}
	if len(p.entries) == 0 && p.err == "" {
		bodyRows[0] = uicore.PadOrTruncate(p.styles.PickerDim.Render("(empty)"), contentW)
	}

	footerRows := []string{
		p.formatHintRow(contentW),
		p.formatPathRow(contentW),
	}
	return p.shell.Box("Attach files", bodyRows, footerRows, contentW)
}

const attachSizeWidth = 7 // fits "999.9MB"; matches filepicker.fileSizeWidth

func (p AttachPicker) formatEntry(idx, contentW int) string {
	e := p.entries[idx]
	mark := "  "
	if p.selected[e.path] {
		mark = "✓ "
	}
	icon := p.icons.Attachment
	if e.isDir {
		icon = p.icons.CustomFolder
	}
	size := ""
	if !e.isDir {
		size = humanize.Bytes(e.size)
	}
	bodyW := contentW - attachSizeWidth - 1
	body := fmt.Sprintf("%s%s %s", mark, icon, e.name)
	if e.target != "" {
		arrow := " → " + e.target
		if ansix.Width(body+arrow) > bodyW {
			maxTarget := bodyW - ansix.Width(body) - 3 // 3 for " → "
			if maxTarget > 0 {
				arrow = " → " + ansix.Truncate(e.target, maxTarget)
			} else {
				arrow = ""
			}
		}
		body += arrow
	}
	sizeCol := p.styles.PickerDim.Width(attachSizeWidth).Align(lipgloss.Right).Render(size)
	rendered := ansix.PadOrTruncate(body, bodyW) + " " + sizeCol
	if idx == p.cursor {
		return p.styles.PickerCursor.Render(rendered)
	}
	return rendered
}

func (p AttachPicker) formatHintRow(contentW int) string {
	if p.err != "" {
		return uicore.PadOrTruncate(p.styles.PickerError.Render(p.err), contentW)
	}
	n := selectedCount(p)
	var hint string
	if n == 0 {
		hint = "j/k nav · l/Enter open · Space select · a accept · . hidden · Esc cancel"
	} else {
		hint = fmt.Sprintf("j/k nav · l/Enter open · Space toggle · a accept (%d) · Esc cancel", n)
	}
	return uicore.PadOrTruncate(p.styles.PickerDim.Render(hint), contentW)
}

func (p AttachPicker) formatPathRow(contentW int) string {
	path := p.dir
	if ansix.Width(path) > contentW {
		// truncate from the left, prefix with "…/"
		runes := []rune(path)
		for ansix.Width("…/"+string(runes)) > contentW && len(runes) > 1 {
			runes = runes[1:]
		}
		path = "…/" + string(runes)
	}
	return uicore.PadOrTruncate(p.styles.PickerDim.Render(path), contentW)
}
