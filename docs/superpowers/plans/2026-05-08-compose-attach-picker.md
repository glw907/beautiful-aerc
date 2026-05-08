# Compose-side attach picker — Pass 9p Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a multi-select TUI file browser overlay to compose so the user can attach and remove files end-to-end through the existing outbound stack.

**Architecture:** New `compose.AttachPicker` value sub-model in `internal/ui/compose/attachpicker.go`, modeled on `bubbles/filepicker` design vocabulary (vim h/l/g/G nav, async `readDir` with id-guard, stack-based ascend restoration) but reimplemented to add multi-select. Mounted as a `uicore.ModalShell` overlay; emits `AttachAcceptedMsg{Paths}` / `AttachCancelledMsg{}` that compose `Update` translates into mutations of `Draft.Attachments`. Compose `View` grows an "Attach:" chip row between Subject and Body when ≥1 attachment exists, and a `focusAttach` enum value lets the user cursor across chips and remove them with `d`/`Backspace`/`Delete`.

**Tech Stack:** Go 1.26, bubbletea v1, `bubbles/key`, `lipgloss`, `internal/ui/uicore` (ModalShell, DisplayCells), `internal/humanize`, `internal/compose` (Draft.Attachments + AssembleMIME multipart/mixed).

**Spec:** `docs/superpowers/specs/2026-05-08-compose-attach-picker-design.md`

## File map

- **Create** `internal/ui/compose/attachpicker.go` — `AttachPicker` model, keys, readDir cmd, View.
- **Create** `internal/ui/compose/attachpicker_test.go` — table-driven coverage (nav, stack, multi-select, accept/cancel, footer variants).
- **Modify** `internal/ui/compose/model.go` — `focusAttach` enum, `attach` / `attachCursor` / `attachLastDir` fields, focus-skip, attach-row rendering, remove keys, `AttachAccepted/Cancelled` handling.
- **Modify** `internal/ui/compose/styles.go` — `AttachChip`, `AttachChipFocus`, `AttachLabel`, `PickerCursor`, `PickerDim`, `PickerError` style fields (project them from theme).
- **Modify** `internal/ui/compose/msgs.go` — export `AttachAcceptedMsg` / `AttachCancelledMsg`.
- **Modify** `internal/ui/compose/model_test.go` — focus-skip, dedupe, dirty bump, remove keys, attachLastDir persistence.
- **Modify** `internal/ui/keys.go` — add `Attach` to `ComposeKeys` (`ctrl+o`).
- **Modify** `internal/ui/footer.go` — `composeFooterGroups` learns `^O attach` (rank 6, same as tidy).
- **Modify** `internal/ui/app.go` — overlay-render the picker on top of compose; route `AttachAcceptedMsg` / `AttachCancelledMsg` to compose; forward `WindowSizeMsg` to picker.

ADR + invariants update happen at pass-end via the `poplar-pass` skill ritual, not as planned tasks.

---

## Task 1: Probe `Ctrl+O` keybinding

**Files:**
- Test: ad-hoc — no permanent file.

- [ ] **Step 1: Run a 10-line probe to confirm `Ctrl+O` is not consumed by `bubbles/textinput` or `bubbles/textarea`**

```bash
cd /tmp && cat > ctrlo_probe.go <<'EOF'
package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type m struct{ ti textinput.Model; ta textarea.Model; saw string }

func (x m) Init() tea.Cmd { return nil }
func (x m) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+o" {
		x.saw = "got ctrl+o; ti=" + x.ti.Value() + " ta=" + x.ta.Value()
		return x, tea.Quit
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+c" { return x, tea.Quit }
	var c1, c2 tea.Cmd
	x.ti, c1 = x.ti.Update(msg)
	x.ta, c2 = x.ta.Update(msg)
	return x, tea.Batch(c1, c2)
}
func (x m) View() string {
	return fmt.Sprintf("[ti] %s\n[ta] %s\n%s\n(Ctrl+O / Ctrl+C)", x.ti.View(), x.ta.View(), x.saw)
}

func main() {
	ti := textinput.New(); ti.Focus()
	ta := textarea.New()
	tea.NewProgram(m{ti: ti, ta: ta}).Run()
}
EOF
cd /home/glw907/Projects/poplar && go run /tmp/ctrlo_probe.go
```

Expected: pressing Ctrl+O quits and prints the captured `ctrl+o`. Pressing it does NOT insert any character into the textinput or textarea values. If the probe shows characters being inserted into `ti=` or `ta=`, fall back to `Ctrl+Y` for the spec and update the plan accordingly.

- [ ] **Step 2: Note the chosen binding**

Record `Ctrl+O` (or `Ctrl+Y` fallback) as the picker-open key for the rest of the plan. Delete `/tmp/ctrlo_probe.go`. No commit.

---

## Task 2: AttachPicker scaffold + message types

**Files:**
- Create: `internal/ui/compose/attachpicker.go`
- Modify: `internal/ui/compose/msgs.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Add the cross-boundary messages**

In `internal/ui/compose/msgs.go`, append:

```go
// AttachAcceptedMsg fires when the user accepts a selection in
// AttachPicker. Paths are absolute. Caller is responsible for
// dedupe against the current Draft.Attachments.
type AttachAcceptedMsg struct{ Paths []string }

// AttachCancelledMsg fires when the user dismisses AttachPicker
// without selecting.
type AttachCancelledMsg struct{}
```

- [ ] **Step 2: Write the failing scaffold test**

Create `internal/ui/compose/attachpicker_test.go`:

```go
package compose

import (
	"testing"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func newTestPicker(t *testing.T) AttachPicker {
	t.Helper()
	styles := NewStyles(theme.NewCompiledTheme(theme.OneDark))
	return NewAttachPicker(styles, uicore.SimpleIcons)
}

func TestAttachPicker_StartsClosed(t *testing.T) {
	p := newTestPicker(t)
	if p.IsOpen() {
		t.Fatal("new picker should not be open")
	}
}
```

- [ ] **Step 3: Run the test, expect compile failure**

```
go test ./internal/ui/compose/ -run TestAttachPicker_StartsClosed -count=1
```

Expected: build error — `NewAttachPicker` undefined.

- [ ] **Step 4: Add the scaffold**

Create `internal/ui/compose/attachpicker.go`:

```go
package compose

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// AttachPicker is the compose-side multi-select file browser
// overlay. Vim-style nav (h/l/g/G), async readDir with an id
// guard so stale results don't clobber a fresh listing, and a
// view-state stack so ascend lands the cursor back on the dir
// you came from.
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
	name string
	path string
	isDir bool
	size int64
}

type attachViewState struct{ cursor, offset int }

type attachPickerKeys struct {
	Up, Down       key.Binding
	PgUp, PgDown   key.Binding
	GoTop, GoBot   key.Binding
	Open           key.Binding // l, right, enter
	Back           key.Binding // h, left, backspace
	Toggle         key.Binding // space
	Accept         key.Binding // a
	ToggleHidden   key.Binding // .
	Close          key.Binding // esc
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
```

- [ ] **Step 5: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker_StartsClosed -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/compose/attachpicker.go internal/ui/compose/attachpicker_test.go internal/ui/compose/msgs.go
git commit -m "Pass 9p.1: compose AttachPicker scaffold + msgs"
```

---

## Task 3: Async readDir with id guard

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Write failing tests for Open + readDirMsg routing**

Append to `attachpicker_test.go`:

```go
import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func writeTree(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAttachPicker_OpenReadsDir(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "alpha.txt", "beta.txt", "sub/", ".hidden")

	p := newTestPicker(t)
	p, cmd := p.Open(dir)
	if cmd == nil {
		t.Fatal("Open should return a readDir cmd")
	}
	if !p.IsOpen() {
		t.Fatal("Open should mark picker open")
	}
	msg := cmd()
	rd, ok := msg.(readDirMsg)
	if !ok {
		t.Fatalf("expected readDirMsg, got %T", msg)
	}
	if rd.id != p.id {
		t.Fatalf("id mismatch: msg=%d picker=%d", rd.id, p.id)
	}
	p, _ = p.Update(rd)
	names := make([]string, len(p.entries))
	for i, e := range p.entries {
		names[i] = e.name
	}
	sort.Strings(names)
	want := []string{"alpha.txt", "beta.txt", "sub"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v (hidden excluded)", names, want)
	}
}

func TestAttachPicker_StaleReadDirDropped(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := newTestPicker(t)
	p, _ = p.Open(dir)
	staleID := p.id
	// reopen — bumps id
	p, _ = p.Open(dir)
	stale := readDirMsg{id: staleID, entries: []attachEntry{{name: "ghost", path: "/ghost"}}}
	p, _ = p.Update(stale)
	for _, e := range p.entries {
		if e.name == "ghost" {
			t.Fatal("stale readDirMsg should have been dropped")
		}
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

```
go test ./internal/ui/compose/ -run TestAttachPicker_Open -count=1
```

Expected: build errors — `Open`, `readDirMsg` undefined.

- [ ] **Step 3: Implement `Open` + `readDirCmd` + `readDirMsg` routing**

Replace the stub `Update` and add `Open` / `readDirCmd` to `attachpicker.go`:

```go
import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	}
	return p, nil
}
```

- [ ] **Step 4: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker_Open -count=1
go test ./internal/ui/compose/ -run TestAttachPicker_StaleReadDirDropped -count=1
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose/attachpicker.go internal/ui/compose/attachpicker_test.go
git commit -m "Pass 9p.2: AttachPicker async readDir with id guard"
```

---

## Task 4: Linear navigation (j/k, g/G, pgup/pgdn)

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Write failing nav tests**

Append to `attachpicker_test.go`:

```go
func feedKeys(p AttachPicker, keys ...string) AttachPicker {
	for _, k := range keys {
		p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(k)}))
	}
	return p
}

func loadDir(t *testing.T, p AttachPicker, dir string) AttachPicker {
	t.Helper()
	p, cmd := p.Open(dir)
	if cmd != nil {
		p, _ = p.Update(cmd())
	}
	p = p.SetSize(60, 10)
	return p
}

func TestAttachPicker_Nav(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a", "b", "c", "d", "e")
	p := loadDir(t, newTestPicker(t), dir)

	if p.cursor != 0 {
		t.Fatalf("initial cursor = %d", p.cursor)
	}
	p = feedKeys(p, "j", "j")
	if p.cursor != 2 {
		t.Errorf("after jj: cursor = %d, want 2", p.cursor)
	}
	p = feedKeys(p, "k")
	if p.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", p.cursor)
	}
	p = feedKeys(p, "G")
	if p.cursor != len(p.entries)-1 {
		t.Errorf("after G: cursor = %d, want %d", p.cursor, len(p.entries)-1)
	}
	p = feedKeys(p, "g")
	if p.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", p.cursor)
	}

	// bounds
	p = feedKeys(p, "k", "k")
	if p.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", p.cursor)
	}
	p = feedKeys(p, "G", "j")
	if p.cursor != len(p.entries)-1 {
		t.Errorf("j at bottom: cursor = %d, want %d", p.cursor, len(p.entries)-1)
	}
}
```

- [ ] **Step 2: Run, expect failure**

```
go test ./internal/ui/compose/ -run TestAttachPicker_Nav -count=1
```

Expected: FAIL — cursor doesn't move (Update has no nav cases yet).

- [ ] **Step 3: Wire nav keys**

In `attachpicker.go`, extend the `Update` switch with a `tea.KeyMsg` arm, just before the trailing `return p, nil` of the existing switch:

```go
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
		}
```

Add helpers at the bottom of the file:

```go
// viewportRows is the body height available for entries inside the
// ModalShell box. shell.Height() includes border + title + footer
// rows; subtract a fixed budget of 4 (1 top border, 1 title, 2
// footer rows, 1 bottom border = 5; we keep 4 conservative).
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
```

- [ ] **Step 4: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker_Nav -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose/attachpicker.go internal/ui/compose/attachpicker_test.go
git commit -m "Pass 9p.3: AttachPicker linear navigation"
```

---

## Task 5: Descend / ascend with view-state stack

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Write failing tests**

Append:

```go
func TestAttachPicker_DescendAndAscendRestoresCursor(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a", "b", "child/", "child/inner.txt")
	p := loadDir(t, newTestPicker(t), dir)

	// Find the "child" dir entry; cursor onto it.
	idx := -1
	for i, e := range p.entries {
		if e.name == "child" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("child not in entries")
	}
	for p.cursor < idx {
		p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	}

	// Enter — descend.
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("descend should issue readDirCmd")
	}
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd2 := readCmd(t, p, "enter"); cmd2 != nil {
		p, _ = p.Update(cmd2())
	}
	if !strings.HasSuffix(p.dir, "/child") {
		t.Fatalf("after descend dir = %q, want suffix /child", p.dir)
	}
	if p.cursor != 0 {
		t.Errorf("after descend cursor = %d, want 0", p.cursor)
	}

	// Backspace — ascend.
	p, cmd = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyBackspace}))
	if cmd != nil {
		p, _ = p.Update(cmd())
	}
	if p.dir != dir {
		t.Errorf("after ascend dir = %q, want %q", p.dir, dir)
	}
	if p.cursor != idx {
		t.Errorf("after ascend cursor = %d, want %d (restored to child entry)", p.cursor, idx)
	}
}

// readCmd is a helper that issues the latest cmd if descend produced one
// in a separate tea.Tick — placeholder if the descend pattern needs it.
func readCmd(t *testing.T, p AttachPicker, _ string) tea.Cmd {
	t.Helper()
	return nil // descend handled inline by Update returning the cmd directly
}
```

(Drop the `readCmd` helper if the test compiles cleanly without the staged second-call pattern. Adjust the test to single-step descend if simpler — i.e., one `Update(EnterKey)` returns the cmd, run it, single readDirMsg.)

- [ ] **Step 2: Run, expect failure**

```
go test ./internal/ui/compose/ -run TestAttachPicker_DescendAndAscend -count=1
```

Expected: FAIL — Enter / Backspace are no-ops in current Update.

- [ ] **Step 3: Implement descend / ascend**

Add cases to the `tea.KeyMsg` switch in `Update`:

```go
		case key.Matches(m, p.keys.Open):
			if len(p.entries) == 0 {
				return p, nil
			}
			e := p.entries[p.cursor]
			if e.isDir {
				return p.descend(e.path)
			}
			// File: handled in Task 7 (multi-select). For now,
			// fall through to a no-op; later tasks layer on Toggle
			// + Enter-shortcut behavior.
			return p, nil
		case key.Matches(m, p.keys.Back):
			return p.ascend()
```

Add the helpers at the bottom of `attachpicker.go`:

```go
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
```

- [ ] **Step 4: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker_DescendAndAscend -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose/attachpicker.go internal/ui/compose/attachpicker_test.go
git commit -m "Pass 9p.4: AttachPicker descend/ascend with view stack"
```

---

## Task 6: Hidden-file toggle

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestAttachPicker_HiddenToggle(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "visible.txt", ".secret")
	p := loadDir(t, newTestPicker(t), dir)
	if has(p.entries, ".secret") {
		t.Fatal("hidden should be excluded by default")
	}
	p, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(".")}))
	if cmd == nil {
		t.Fatal("toggle should re-issue readDirCmd")
	}
	p, _ = p.Update(cmd())
	if !has(p.entries, ".secret") {
		t.Errorf("after toggle: %v should include .secret", entryNames(p.entries))
	}
}

func has(es []attachEntry, name string) bool {
	for _, e := range es {
		if e.name == name {
			return true
		}
	}
	return false
}

func entryNames(es []attachEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.name
	}
	return out
}
```

- [ ] **Step 2: Run, expect failure**

- [ ] **Step 3: Add toggle case to `Update`**

```go
		case key.Matches(m, p.keys.ToggleHidden):
			p.showHidden = !p.showHidden
			p.id++
			p.entries = nil
			return p, readDirCmd(p.id, p.dir, p.showHidden)
```

- [ ] **Step 4: Run tests, expect pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "Pass 9p.5: AttachPicker hidden-file toggle"
```

---

## Task 7: Multi-select (Space, `a`, Enter shortcut, Esc)

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestAttachPicker_SelectAndAccept(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt", "b.txt", "c.txt")
	p := loadDir(t, newTestPicker(t), dir)

	// Toggle a and c.
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(" ")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(" ")}))

	if c := selectedCount(p); c != 2 {
		t.Fatalf("selected = %d, want 2", c)
	}

	// Accept.
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("a")}))
	if cmd == nil {
		t.Fatal("accept should emit cmd")
	}
	msg := cmd()
	acc, ok := msg.(AttachAcceptedMsg)
	if !ok {
		t.Fatalf("expected AttachAcceptedMsg, got %T", msg)
	}
	if len(acc.Paths) != 2 {
		t.Errorf("Paths len = %d, want 2 (%v)", len(acc.Paths), acc.Paths)
	}
}

func TestAttachPicker_EnterOnFileShortcut(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "only.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter on file with empty selection should accept")
	}
	acc, ok := cmd().(AttachAcceptedMsg)
	if !ok || len(acc.Paths) != 1 || !strings.HasSuffix(acc.Paths[0], "only.txt") {
		t.Fatalf("got %#v", cmd())
	}
}

func TestAttachPicker_AcceptZeroSelectedNoOp(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt", "b.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("a")}))
	if cmd != nil {
		t.Fatal("accept with 0 selected should be no-op")
	}
}

func TestAttachPicker_EscEmitsCancelled(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("Esc should emit cmd")
	}
	if _, ok := cmd().(AttachCancelledMsg); !ok {
		t.Fatalf("got %T, want AttachCancelledMsg", cmd())
	}
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
```

- [ ] **Step 2: Run, expect failure**

- [ ] **Step 3: Wire selection + accept + cancel**

Replace the file-fall-through arm of the `Open` case (added in Task 5) and add new cases:

```go
		case key.Matches(m, p.keys.Open):
			if len(p.entries) == 0 {
				return p, nil
			}
			e := p.entries[p.cursor]
			if e.isDir {
				return p.descend(e.path)
			}
			// Enter on file: shortcut single-attach when nothing
			// is yet selected. Otherwise toggle this file into the
			// running selection (matches l/right behavior).
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
```

Add helpers:

```go
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
```

Move `selectedCount` from the test file into `attachpicker.go` (lowercase, package-private) since the production code now uses it; delete the duplicate from the test file:

```go
func selectedCount(p AttachPicker) int {
	n := 0
	for _, v := range p.selected {
		if v {
			n++
		}
	}
	return n
}
```

- [ ] **Step 4: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker -count=1
```

Expected: all picker tests PASS.

- [ ] **Step 5: Commit**

```bash
git commit -am "Pass 9p.6: AttachPicker multi-select, accept, cancel"
```

---

## Task 8: View rendering with ModalShell + footer hints

**Files:**
- Modify: `internal/ui/compose/attachpicker.go`
- Modify: `internal/ui/compose/styles.go`
- Test: `internal/ui/compose/attachpicker_test.go`

- [ ] **Step 1: Add picker styles**

In `internal/ui/compose/styles.go`, extend the `Styles` struct (locate the `type Styles struct { ... }` block) by appending fields:

```go
	// Picker (AttachPicker) styles.
	PickerCursor lipgloss.Style
	PickerDim    lipgloss.Style
	PickerError  lipgloss.Style
```

In the same file's `NewStyles(*theme.CompiledTheme)` constructor, populate them. Mirror the equivalents in `internal/ui/reader/styles.go` for `Cursor` / `Dim` / `Error` — invert background for cursor, low-emphasis foreground for dim, accent-error for error. Example:

```go
	PickerCursor: lipgloss.NewStyle().
		Foreground(t.Palette.Background).
		Background(t.Palette.AccentPrimary),
	PickerDim:   lipgloss.NewStyle().Foreground(t.Palette.TextDim),
	PickerError: lipgloss.NewStyle().Foreground(t.Palette.AccentError),
```

(Verify the actual palette field names by reading `internal/theme/palette.go` — the spec calls out `docs/poplar/styling.md` as the source of truth before any color change. Update `docs/poplar/styling.md` too: add a row mapping these three style fields to their palette slots.)

- [ ] **Step 2: Write failing render test**

Append to `attachpicker_test.go`:

```go
func TestAttachPicker_FooterHintsZeroSelected(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := loadDir(t, newTestPicker(t), dir).SetSize(80, 20)
	p.shell = p.shell.WithOpen(true)
	out := p.View()
	if out == "" {
		t.Fatal("View should render when open")
	}
	if !strings.Contains(out, "j/k nav") || !strings.Contains(out, "a accept") {
		t.Errorf("footer missing default hint, got:\n%s", out)
	}
}

func TestAttachPicker_FooterHintsSelectedCount(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt", "b.txt")
	p := loadDir(t, newTestPicker(t), dir).SetSize(80, 20)
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(" ")}))
	out := p.View()
	if !strings.Contains(out, "a accept (1)") {
		t.Errorf("footer missing count, got:\n%s", out)
	}
}

func TestAttachPicker_FooterShowsPath(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := loadDir(t, newTestPicker(t), dir).SetSize(80, 20)
	out := p.View()
	if !strings.Contains(out, filepath.Base(dir)) {
		t.Errorf("footer should show current path basename, got:\n%s", out)
	}
}
```

- [ ] **Step 3: Run, expect failure**

- [ ] **Step 4: Implement `View`**

Replace the stub `View()` in `attachpicker.go` with:

```go
import (
	"fmt"
	"github.com/glw907/poplar/internal/humanize"
)

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
	for i := 0; i < rows; i++ {
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

func (p AttachPicker) formatEntry(idx, contentW int) string {
	e := p.entries[idx]
	mark := "  "
	if p.selected[e.path] {
		mark = "✓ "
	}
	icon := p.icons.Attachment
	if e.isDir {
		icon = p.icons.Folder
	}
	size := ""
	if !e.isDir {
		size = humanize.Bytes(e.size)
	}
	body := fmt.Sprintf("%s%s %s", mark, icon, e.name)
	rendered := uicore.DisplayPadOrTruncate(body, contentW-len(size)-1) + " " + p.styles.PickerDim.Render(size)
	rendered = uicore.DisplayPadOrTruncate(rendered, contentW)
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
	if uicore.DisplayCells(path) > contentW {
		// Truncate from the left, prefix with "…/".
		runes := []rune(path)
		for uicore.DisplayCells("…/"+string(runes)) > contentW && len(runes) > 1 {
			runes = runes[1:]
		}
		path = "…/" + string(runes)
	}
	return uicore.PadOrTruncate(p.styles.PickerDim.Render(path), contentW)
}
```

Verify `p.icons.Folder` exists on `uicore.IconSet`. If not — check `internal/ui/uicore/layout.go`. If absent, add `Folder` to both `SimpleIcons` (e.g., `"📁 "` SPUA-mapped or `"d "`) and `FancyIcons` (Nerd Font folder glyph). Pass-end ADR notes the icon addition if you do.

- [ ] **Step 5: Run tests, expect pass**

```
go test ./internal/ui/compose/ -run TestAttachPicker_Footer -count=1
go test ./internal/ui/compose/ -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/compose/attachpicker.go internal/ui/compose/styles.go docs/poplar/styling.md
git commit -m "Pass 9p.7: AttachPicker View with ModalShell + footer hints"
```

---

## Task 9: Compose model — `focusAttach`, `Ctrl+O`, message routing

**Files:**
- Modify: `internal/ui/compose/model.go`
- Modify: `internal/ui/keys.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 1: Extend `ComposeKeys` with `Attach`**

In `internal/ui/keys.go`:

```go
type ComposeKeys struct {
	Send       key.Binding
	Cancel     key.Binding
	NextField  key.Binding
	PrevField  key.Binding
	EscapeBody key.Binding
	Attach     key.Binding
}

func NewComposeKeys() ComposeKeys {
	return ComposeKeys{
		Send:       key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("^X", "send")),
		Cancel:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "cancel")),
		NextField:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "next")),
		PrevField:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧⇥", "prev")),
		EscapeBody: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "focus")),
		Attach:     key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("^O", "attach")),
	}
}
```

- [ ] **Step 2: Add fields and the `focusAttach` constant**

In `internal/ui/compose/model.go`, extend the focus enum (currently around line 78):

```go
const (
	focusTo = iota
	focusCc
	focusBcc
	focusSubject
	focusAttach
	focusBody
	focusFrom
)
```

Extend the `Model` struct (around line 32) with new fields:

```go
	attach        AttachPicker
	attachCursor  int
	attachLastDir string
```

In `newModel(...)`, after the existing initializations, add:

```go
	c.attach = NewAttachPicker(styles, uicore.SimpleIcons)
```

(Verify `uicore` is imported — it already is.) The icon set will be replaced with the App's resolved set in Task 12 via a setter; for now the Simple set keeps tests deterministic.

- [ ] **Step 3: Write failing tests**

In `internal/ui/compose/model_test.go`, append:

```go
func TestCompose_TabSkipsAttachWhenEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	// Tab from focusSubject should land on focusBody, skipping focusAttach
	// because Draft.Attachments is empty.
	c.setFocus(focusSubject)
	c.nextField()
	if c.focus != focusBody {
		t.Errorf("Tab from Subject with no attachments: focus = %d, want focusBody=%d", c.focus, focusBody)
	}
}

func TestCompose_AttachAcceptedAppends(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf", "/tmp/b.pdf"}})
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 {
		t.Fatalf("attachments len = %d, want 2", len(d.Attachments))
	}
	if c.attachLastDir != "/tmp" {
		t.Errorf("attachLastDir = %q, want /tmp", c.attachLastDir)
	}
	if !c.localDirty {
		t.Error("expected localDirty after AttachAcceptedMsg")
	}
}

func TestCompose_AttachAcceptedDedupes(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf"}})
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf", "/tmp/b.pdf"}})
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 {
		t.Errorf("dedupe failed: got %v", d.Attachments)
	}
}
```

(`newComposeForTest` should already exist or be straightforward to mirror from existing tests in `model_test.go`. If not, write a thin helper that constructs a Model with a fake `CacheStore` and an empty `SuggestFn`.)

- [ ] **Step 4: Run, expect failure**

```
go test ./internal/ui/compose/ -run TestCompose_Attach -count=1
go test ./internal/ui/compose/ -run TestCompose_Tab -count=1
```

Expected: FAIL (focus enum changed but routing not yet added) or compile errors.

- [ ] **Step 5: Wire focus-skip + AttachAcceptedMsg / AttachCancelledMsg**

In `model.go`, update `nextField` / `prevField` (find where they live — likely near `setFocus`) to skip `focusAttach` when `len(c.draft().Attachments) == 0`. Sketch:

```go
func (c *Model) nextField() {
	for {
		c.focus = (c.focus + 1) % focusFrom // wrap excluding focusFrom which is keyboard-only
		if c.focus == focusAttach && len(c.currentAttachments()) == 0 {
			continue
		}
		return
	}
}

func (c *Model) prevField() { /* mirror */ }

func (c *Model) currentAttachments() []string {
	return c.CurrentDraft().Attachments
}
```

(Adjust to match existing focus-cycle conventions in this file — do not introduce a new pattern. If `focusFrom` is not part of the Tab cycle today, keep the cycle as it stood and just add the skip case.)

In `Update`, add `AttachAcceptedMsg` / `AttachCancelledMsg` handling — locate the `tea.Msg` switch:

```go
	case AttachAcceptedMsg:
		d := c.CurrentDraft()
		existing := map[string]bool{}
		for _, p := range d.Attachments {
			existing[p] = true
		}
		for _, p := range msg.Paths {
			if !existing[p] {
				d.Attachments = append(d.Attachments, p)
				existing[p] = true
			}
		}
		c.applyDraft(d)            // existing helper or equivalent setter
		if len(msg.Paths) > 0 {
			c.attachLastDir = filepath.Dir(msg.Paths[0])
		}
		c.attach = c.attach.Close()
		c.localDirty = true
		c.lastEditAt = time.Now()
		return c, c.kickAutosave()  // existing autosave kicker
	case AttachCancelledMsg:
		c.attach = c.attach.Close()
		return c, nil
```

(Replace `applyDraft` / `kickAutosave` with the actual existing helpers — read the file to find them. If `Draft` is mutated through field-by-field setters, mutate `Draft.Attachments` directly the same way.)

In the key-routing block of `Update` (where `Send` / `Cancel` / `NextField` / `PrevField` / `EscapeBody` are matched), insert before any of those:

```go
		// Picker takes precedence when open.
		if c.attach.IsOpen() {
			var cmd tea.Cmd
			c.attach, cmd = c.attach.Update(msg)
			return c, cmd
		}
		if key.Matches(msg, c.keys.Attach) {
			start := c.attachLastDir
			if start == "" {
				if wd, err := os.Getwd(); err == nil {
					start = wd
				} else if home, err := os.UserHomeDir(); err == nil {
					start = home
				} else {
					start = "/"
				}
			}
			var cmd tea.Cmd
			c.attach, cmd = c.attach.Open(start)
			return c, cmd
		}
```

`c.keys` is the existing `ComposeKeys` field; if compose currently uses module-scoped `defaultComposeKeys()`, mirror that pattern.

- [ ] **Step 6: Run tests, expect pass**

```
go test ./internal/ui/compose/ -count=1
```

- [ ] **Step 7: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/model_test.go internal/ui/keys.go
git commit -m "Pass 9p.8: compose focusAttach, Ctrl+O, AttachAccepted/Cancelled routing"
```

---

## Task 10: Attach row rendering in compose `View`

**Files:**
- Modify: `internal/ui/compose/model.go`
- Modify: `internal/ui/compose/styles.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 1: Add chip styles**

In `styles.go`, add to `Styles` and the constructor:

```go
	AttachLabel      lipgloss.Style
	AttachChip       lipgloss.Style
	AttachChipFocus  lipgloss.Style
```

Populate from the same theme palette used for the existing `Label` style (`Foreground(t.Palette.TextDim)` for label; `Foreground(t.Palette.TextNormal)` for chips; chip-focus = invert with `AccentPrimary` background). Update `docs/poplar/styling.md` with the new rows.

- [ ] **Step 2: Write failing tests**

```go
func TestCompose_AttachRowHiddenWhenEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	out := c.View()
	if strings.Contains(out, "Attach:") {
		t.Errorf("attach row should not render when no attachments")
	}
}

func TestCompose_AttachRowVisibleWithAttachments(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/notes.pdf"}})
	out := c.View()
	if !strings.Contains(out, "Attach:") {
		t.Errorf("attach row should render: \n%s", out)
	}
	if !strings.Contains(out, "notes.pdf") {
		t.Errorf("chip should show basename: \n%s", out)
	}
}
```

- [ ] **Step 3: Run, expect failure**

- [ ] **Step 4: Implement chip row rendering**

In `model.go`, locate the `View` method (around line 247). After the Subject row append, before the divider, insert:

```go
	if atts := c.CurrentDraft().Attachments; len(atts) > 0 {
		rows = append(rows, c.attachRow(atts))
	}
```

Add the helper:

```go
func (c *Model) attachRow(atts []string) string {
	label := c.styles.AttachLabel.Render("Attach: ")
	avail := c.width - uicore.DisplayCells(label)
	if avail < 1 {
		return c.padRow(label)
	}
	chips := make([]string, 0, len(atts))
	for i, p := range atts {
		size := ""
		if info, err := os.Stat(p); err == nil {
			size = " (" + humanize.Bytes(info.Size()) + ")"
		}
		body := c.icons.Attachment + " " + filepath.Base(p) + size
		style := c.styles.AttachChip
		if c.focus == focusAttach && i == c.attachCursor {
			style = c.styles.AttachChipFocus
		}
		chips = append(chips, style.Render(body))
	}
	joined := strings.Join(chips, "  ")
	if uicore.DisplayCells(joined) > avail {
		joined = c.fitChips(chips, avail)
	}
	return c.padRow(label + joined)
}

// fitChips greedily includes leading chips until they no longer fit,
// then appends a "+N" chip describing the overflow.
func (c *Model) fitChips(chips []string, avail int) string {
	overflowFmt := "  " + c.styles.AttachChip.Render(c.icons.Attachment+" +%d")
	var b strings.Builder
	used := 0
	for i, ch := range chips {
		sep := ""
		if b.Len() > 0 {
			sep = "  "
		}
		piece := sep + ch
		// Reserve room for an overflow chip if more remain.
		reserve := 0
		if i < len(chips)-1 {
			reserve = uicore.DisplayCells(fmt.Sprintf(overflowFmt, len(chips)-i-1))
		}
		if used+uicore.DisplayCells(piece)+reserve > avail {
			b.WriteString(fmt.Sprintf(overflowFmt, len(chips)-i))
			return b.String()
		}
		b.WriteString(piece)
		used += uicore.DisplayCells(piece)
	}
	return b.String()
}
```

(Adjust import set: add `os`, `path/filepath`, `fmt`, `github.com/glw907/poplar/internal/humanize`.)

- [ ] **Step 5: Run tests, expect pass**

- [ ] **Step 6: Commit**

```bash
git commit -am "Pass 9p.9: compose attach-row chip rendering"
```

---

## Task 11: Remove via `d` / `Backspace` / `Delete` in `focusAttach`

**Files:**
- Modify: `internal/ui/compose/model.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCompose_FocusAttachRemoveCollapsesEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf"}})
	c.setFocus(focusAttach)
	_, _ = c.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("d")}))
	if d := c.CurrentDraft(); len(d.Attachments) != 0 {
		t.Fatalf("attachments not removed: %v", d.Attachments)
	}
	if c.focus != focusSubject {
		t.Errorf("focus did not collapse to Subject: %d", c.focus)
	}
}

func TestCompose_FocusAttachArrowsAndDeleteMid(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a", "/tmp/b", "/tmp/c"}})
	c.setFocus(focusAttach)
	_, _ = c.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRight}))
	_, _ = c.Update(tea.KeyMsg(tea.Key{Type: tea.KeyBackspace}))
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 || d.Attachments[1] != "/tmp/c" {
		t.Errorf("after middle delete: %v", d.Attachments)
	}
}
```

- [ ] **Step 2: Run, expect failure**

- [ ] **Step 3: Wire focusAttach key handling**

In `Update`, after the picker-precedence block from Task 9, add a focusAttach key-handler arm in the `tea.KeyMsg` switch (or wherever per-focus key dispatch lives — there is an existing per-focus block around line 436):

```go
	case focusAttach:
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.Type {
			case tea.KeyLeft:
				if c.attachCursor > 0 {
					c.attachCursor--
				}
			case tea.KeyRight:
				if c.attachCursor < len(c.CurrentDraft().Attachments)-1 {
					c.attachCursor++
				}
			case tea.KeyBackspace, tea.KeyDelete:
				return c, c.removeAttachAtCursor()
			default:
				if k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] == 'd' {
					return c, c.removeAttachAtCursor()
				}
			}
		}
		return c, nil
```

Add the helper:

```go
func (c *Model) removeAttachAtCursor() tea.Cmd {
	d := c.CurrentDraft()
	if c.attachCursor < 0 || c.attachCursor >= len(d.Attachments) {
		return nil
	}
	d.Attachments = append(d.Attachments[:c.attachCursor], d.Attachments[c.attachCursor+1:]...)
	c.applyDraft(d)
	c.localDirty = true
	c.lastEditAt = time.Now()
	if len(d.Attachments) == 0 {
		c.attachCursor = 0
		c.setFocus(focusSubject)
	} else if c.attachCursor >= len(d.Attachments) {
		c.attachCursor = len(d.Attachments) - 1
	}
	return c.kickAutosave()
}
```

- [ ] **Step 4: Run tests, expect pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "Pass 9p.10: compose focusAttach remove keys"
```

---

## Task 12: Footer hint + App-level overlay wiring

**Files:**
- Modify: `internal/ui/footer.go`
- Modify: `internal/ui/app.go`
- Test: existing app + footer tests

- [ ] **Step 1: Add `^O attach` to compose footer hints**

In `internal/ui/footer.go`, modify `composeFooterGroups` (around line 116):

```go
func composeFooterGroups(hasSig, isFocusFrom, tidyVisible bool) [][]footerHint {
	core := []footerHint{
		hint("Ctrl+X", "send", 0),
		hint("Ctrl+C", "cancel", 0),
		hint("Tab", "field", 4),
		hint("Ctrl+O", "attach", 6),
	}
	if hasSig {
		core = append(core, hint("Ctrl+G", "sig", 5))
	}
	if tidyVisible {
		core = append(core, hint("Ctrl+T", "tidy", 6))
	}
	groups := [][]footerHint{core}
	if isFocusFrom {
		groups = append(groups, []footerHint{
			hint("Space/←→", "identity", 6),
		})
	}
	return groups
}
```

- [ ] **Step 2: Forward picker overlay rendering in App**

In `internal/ui/app.go`, locate the compose-rendering branch in `View`. After compose's view is composited, layer the picker on top when `m.compose.AttachPickerIsOpen()`:

```go
if m.compose != nil {
	body := m.compose.View()
	if box := m.compose.AttachPickerView(); box != "" {
		body = uicore.PlaceOverlay(body, box, /* x,y centered */)
	}
	// existing return
}
```

To support this, expose two methods on `*compose.Model` in `model.go`:

```go
func (c *Model) AttachPickerIsOpen() bool { return c.attach.IsOpen() }
func (c *Model) AttachPickerView() string { return c.attach.View() }
```

Forward `WindowSizeMsg` into the picker. In `compose.Model.SetSize`:

```go
	c.attach = c.attach.SetSize(w, h)
```

Route `AttachAcceptedMsg` / `AttachCancelledMsg` from picker → compose. The simplest path: compose's `Update` already handles them (Task 9). The picker emits those via `tea.Cmd` so they hit the App's top-level `Update` first. Add a routing arm in App's `Update`:

```go
case uicompose.AttachAcceptedMsg, uicompose.AttachCancelledMsg:
	if m.compose != nil {
		var cmd tea.Cmd
		m.compose, cmd = m.compose.Update(msg)
		return m, cmd
	}
```

(If the project's idiom is Msg-types-private-to-package, expose them via `package uicompose` exports — they already are exported per Task 2.)

- [ ] **Step 3: Cascade order**

In the overlay cascade documented in `ui-invariants.md`, the compose picker is part of the compose surface — it does not interact with the global `confirm > conflict > outbox > help > linkpicker > attachpicker > movepicker > form > popover` cascade because compose is its own tab. Confirm by inspection that App's modal-cascade switch routes keys to global modals first; compose receives keys only when no global modal is open. The picker thus rides inside compose's allowed input window with no further plumbing.

- [ ] **Step 4: Build + run all tests**

```
make check
```

Expected: green (vet, fmt-check, voice, test all pass). Voice scan applies — keep the new code stdlib-formal, no AI-tells.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/footer.go internal/ui/app.go internal/ui/compose/model.go
git commit -m "Pass 9p.11: ^O attach footer hint + App overlay wiring"
```

---

## Task 13: Live tmux verification

**Files:**
- No code; capture artifacts only.

- [ ] **Step 1: Install + start poplar in tmux**

```bash
make install
.claude/docs/tmux-testing.md  # read for the harness
```

Follow the harness to launch poplar inside tmux at 80×24 and 120×40.

- [ ] **Step 2: Capture matrix**

For each terminal size (80×24, 120×40), capture a screenshot of:

1. Compose with 0 attachments (baseline — should match prior pass).
2. Compose with 1 attachment, focus on Body.
3. Compose with 3 attachments, focus on Attach row, cursor on middle chip.
4. Picker open on a populated dir (`~/Downloads` or `/etc`).
5. Picker open on the filesystem root after ascending past start.
6. Picker open on an empty `t.TempDir()`-equivalent.
7. Picker after a readDir error (point it at `/root` as unprivileged user).

- [ ] **Step 3: Verify against the spec acceptance list**

For each capture:
- Every line is exactly the assigned width.
- No ANSI bleed past the box borders.
- Cursor highlight is visible.
- Footer hints render correctly, with selected-count variant when applicable.
- Path row truncates with `…/` when long.

If any capture fails, fix the underlying issue inline and re-capture. No commit yet — captures are documentation.

- [ ] **Step 4: Commit captures (optional)**

If the project's convention is to keep a `docs/poplar/captures/` directory, add the captures and commit:

```bash
git add docs/poplar/captures/9p-*.txt
git commit -m "Pass 9p.12: tmux captures — compose attach picker"
```

Otherwise note the captures in the pass-end commit message.

---

## Pass-end consolidation (handled by `poplar-pass` skill)

After Task 13 completes:

1. `/simplify` over the diff. Apply genuine wins.
2. Idiomatic-bubbletea checklist (`docs/poplar/bubbletea-conventions.md` §10) — verify against `internal/ui/compose/attachpicker.go` and the compose `View` changes.
3. Write **ADR-0179** at `docs/poplar/decisions/0179-compose-attach-picker-and-modal-footer-hint-contract.md`. Two decisions:
   - Compose-side attach surface = TUI file browser overlay reusing `bubbles/filepicker` design vocabulary, reimplemented for multi-select.
   - Every `ModalShell` overlay carries a footer hint row enumerating active key bindings; yes/no confirms exempt by implicit-action-set rule.
4. Update `.claude/rules/ui-invariants.md`: add the footer-hint sentence to the modal cascade section (~line 244).
5. Update `docs/poplar/invariants.md` with one line about the compose attach picker, and add ADR-0179 to `docs/poplar/decisions/INDEX.md`.
6. Update `docs/poplar/keybindings.md` Compose table: `^O` row.
7. Move the spec to `docs/superpowers/archive/specs/`, this plan to `docs/superpowers/archive/plans/`, via `git mv`.
8. Update `docs/poplar/STATUS.md`: mark Pass 9p `done`, write the Pass 9q starter prompt (outbox delivery controls — undo + schedule send, #35).
9. `make check` green. `git push`. `make install`.

---

## Self-review notes

- **Spec coverage:** every section of the spec maps to a task. Picker scaffold (T2), readDir+id (T3), nav (T4), descend/ascend (T5), hidden (T6), multi-select (T7), View+footer (T8), focusAttach+Ctrl+O+routing (T9), attach row (T10), removal (T11), footer hint+overlay (T12), live verify (T13). ADR + invariants + keybindings live in pass-end.
- **Type consistency:** `AttachAcceptedMsg` / `AttachCancelledMsg` (with the `-Msg` suffix) used uniformly. `attachEntry` / `attachViewState` / `attachPickerKeys` lowercase package-private. `AttachPicker` exported. Field names `attach`, `attachCursor`, `attachLastDir` consistent across tasks.
- **Risk: `Ctrl+O` conflict** is gated by Task 1's probe; fallback `Ctrl+Y`.
- **Risk: `applyDraft` / `kickAutosave` / `setFocus` helper names** are placeholders — Task 9 step 5 explicitly says to read `model.go` and use the actual existing helpers. Worth verifying inline rather than blocking.
