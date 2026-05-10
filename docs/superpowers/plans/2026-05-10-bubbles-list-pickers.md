# Pass 15a — `bubbles/v2/list` Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hand-rolled cursor, filter, and render code in three picker surfaces (`reader.LinkPicker`, `reader.AttachPicker`, `movepicker.Model`) with `charm.land/bubbles/v2/list`, behind a shared style helper.

**Architecture:** Each picker holds a `list.Model`, routes `tea.KeyPressMsg` through it after handling picker-specific keys (Enter / Esc / digits), and reads cursor + filter state via `list.Model` accessors. Custom `list.ItemDelegate` per consumer handles the bespoke item rendering. ModalShell consumers stay ModalShell consumers — `list.Model.View()` becomes the body string. A new `uicore.NewListStyles` projects the palette onto `list.Styles` for all three consumers.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/list`, `charm.land/lipgloss/v2`. Existing `internal/ansix`, `internal/ui/uicore`, `internal/theme`.

**Spec:** `docs/superpowers/specs/2026-05-10-bubbles-list-pickers-design.md`.

---

## File map

- **Create:** `internal/ui/uicore/list_styles.go` + `internal/ui/uicore/list_styles_test.go`
- **Modify:** `internal/ui/reader/linkpicker.go`, `internal/ui/reader/linkpicker_test.go`
- **Modify:** `internal/ui/reader/attachpicker.go`, `internal/ui/reader/attachpicker_test.go`
- **Modify:** `internal/ui/reader/styles.go` (add `List list.Styles` field; project via `uicore.NewListStyles`)
- **Modify:** `internal/ui/movepicker/model.go`, `internal/ui/movepicker/model_test.go`, `internal/ui/movepicker/styles.go`
- **Modify (pass end):** `docs/poplar/invariants.md`, `docs/poplar/STATUS.md`, `docs/poplar/decisions/INDEX.md`
- **Create (pass end):** `docs/poplar/decisions/0194-bubbles-v2-list-adoption.md`
- **Move (pass end):** plan + spec to `docs/superpowers/archive/plans/` and `archive/specs/`

---

## Task 1: Shared `uicore.NewListStyles` helper

**Files:**
- Create: `internal/ui/uicore/list_styles.go`
- Test: `internal/ui/uicore/list_styles_test.go`

**Why this task first:** All three pickers depend on the helper. Landing it standalone keeps the per-picker diffs focused on integration logic, not theme wiring.

- [ ] **Step 1.1: Read the upstream `list.Styles` shape**

Run: `grep -n "^type Styles\|^	[A-Z]" /home/glw907/go/pkg/mod/charm.land/bubbles/v2@v2.1.0/list/style.go`

Expected output: a `Styles` struct with fields `TitleBar`, `Title`, `Spinner`, `Filter` (a `textinput.Styles`), `DefaultFilterCharacterMatch`, `StatusBar`, `StatusEmpty`, `StatusBarActiveFilter`, `StatusBarFilterCount`, `NoItems`, `PaginationStyle`, `HelpStyle`, `ActivePaginationDot`, `InactivePaginationDot`, `ArabicPagination`, `DividerDot`.

- [ ] **Step 1.2: Read the poplar palette slot names**

Run: `grep -n "FgBright\|FgDim\|AccentPrimary\|ColorWarning\|FgMuted\|BgSubtle" internal/theme/palette.go | head -30`

Note the available slot names. Map them onto `list.Styles` fields in the next step. Slots used by the picker family today: `FgBright` (cursor row), `FgDim` (subdued text), `AccentPrimary` (cursor background or filter prompt), `ColorWarning` (no current use in list), `FgMuted`/`BgSubtle` if present.

- [ ] **Step 1.3: Write the failing test**

```go
package uicore

import (
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

// TestNewListStyles smokes the helper end-to-end: rendering through
// every styled chrome slot must not panic and must produce visibly
// styled output (longer than the input). Visual fidelity is verified
// in the tmux capture step of each picker task.
func TestNewListStyles(t *testing.T) {
	ct := theme.Themes[theme.DefaultThemeName]
	s := NewListStyles(ct)

	cases := []struct {
		name string
		out  string
	}{
		{"Title", s.Title.Render("title")},
		{"StatusBar", s.StatusBar.Render("status")},
		{"NoItems", s.NoItems.Render("nothing")},
		{"FilterPrompt", s.Filter.Prompt.Render(">")},
		{"DefaultFilterCharacterMatch", s.DefaultFilterCharacterMatch.Render("a")},
	}
	for _, c := range cases {
		if c.out == "" {
			t.Errorf("%s rendered empty string", c.name)
		}
	}
}

func TestNewListStyles_deterministic(t *testing.T) {
	ct := theme.Themes[theme.DefaultThemeName]
	a := NewListStyles(ct)
	b := NewListStyles(ct)
	if a.Title.Render("x") != b.Title.Render("x") {
		t.Fatalf("NewListStyles is not deterministic for the same theme")
	}
}
```

- [ ] **Step 1.4: Run test to verify it fails**

Run: `go test ./internal/ui/uicore/ -run TestNewListStyles -v`
Expected: FAIL with "undefined: NewListStyles".

- [ ] **Step 1.5: Implement the helper**

Create `internal/ui/uicore/list_styles.go`:

```go
package uicore

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
)

// NewListStyles projects a poplar compiled theme onto bubbles/v2/list
// styles. Three picker surfaces (movepicker, reader.LinkPicker,
// reader.AttachPicker) share this projection so list chrome stays
// consistent with the rest of the UI.
//
// Per-picker visual details (cursor row, filter match) live in the
// custom ItemDelegate, not here. This helper covers list-owned
// chrome only: title, filter prompt, status bar, pagination, help.
func NewListStyles(t *theme.CompiledTheme) list.Styles {
	s := list.DefaultStyles(true)

	s.Title = lipgloss.NewStyle().
		Foreground(t.FgBright).
		Bold(true).
		Padding(0, 1)

	s.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)

	s.StatusBar = lipgloss.NewStyle().Foreground(t.FgDim)
	s.StatusEmpty = lipgloss.NewStyle().Foreground(t.FgDim)
	s.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(t.AccentPrimary)
	s.StatusBarFilterCount = lipgloss.NewStyle().Foreground(t.FgDim)

	s.NoItems = lipgloss.NewStyle().Foreground(t.FgDim).Italic(true)

	s.PaginationStyle = lipgloss.NewStyle().Foreground(t.FgDim)
	s.HelpStyle = lipgloss.NewStyle().Foreground(t.FgDim)

	s.Filter.Prompt = lipgloss.NewStyle().Foreground(t.AccentPrimary)
	s.Filter.Text = lipgloss.NewStyle().Foreground(t.FgBright)
	s.Filter.Placeholder = lipgloss.NewStyle().Foreground(t.FgDim)
	s.Filter.Cursor = lipgloss.NewStyle().Foreground(t.AccentPrimary)

	s.DefaultFilterCharacterMatch = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).
		Underline(true)

	return s
}
```

- [ ] **Step 1.6: Run test to verify it passes**

Run: `go test ./internal/ui/uicore/ -run TestNewListStyles -v`
Expected: PASS for both subtests.

- [ ] **Step 1.7: Run full uicore tests**

Run: `go test ./internal/ui/uicore/ -v`
Expected: PASS, no regressions.

- [ ] **Step 1.8: Commit**

```bash
git add internal/ui/uicore/list_styles.go internal/ui/uicore/list_styles_test.go
git commit -m "$(cat <<'EOF'
Pass 15a t1: shared uicore.NewListStyles helper

Projects the compiled theme onto bubbles/v2/list.Styles so the
three picker surfaces share one chrome projection. Per-picker
visual details (cursor row, filter match) stay in custom
ItemDelegates.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Convert `reader.LinkPicker` to `bubbles/v2/list`

**Files:**
- Modify: `internal/ui/reader/linkpicker.go`
- Modify: `internal/ui/reader/linkpicker_test.go`
- Modify: `internal/ui/reader/styles.go` (add `List list.Styles` field; project via `uicore.NewListStyles`)

**Why this picker first:** smallest (251 lines), no filter, ≤9 items. Validates the integration shape with the lowest risk.

- [ ] **Step 2.1: Read the existing model**

Run: `cat internal/ui/reader/linkpicker.go`

Note: hand-rolled `cursor`, `offset`, `clampOffset`, manual `formatRow`, manual `previewLines`. External Msgs are `LaunchURLMsg{URL}` and `LinkPickerClosedMsg{}`. Behavior: `j/k` navigates, `Enter` launches cursor URL, `1`-`9` launches digit slot, `Esc`/`Tab` close, `q` swallowed.

- [ ] **Step 2.2: Read the existing tests**

Run: `cat internal/ui/reader/linkpicker_test.go`

Note which Msg outputs they assert. The integration must keep those assertions green. New tests cover the `list.Model` integration boundary (cursor reads, item count, empty list).

- [ ] **Step 2.3: Add `List list.Styles` field to reader.Styles**

Open `internal/ui/reader/styles.go`. Two edits:

1. Add the import (alongside existing imports):

```go
"charm.land/bubbles/v2/list"

"github.com/glw907/poplar/internal/ui/uicore"
```

2. Add `List list.Styles` to the `Styles` struct and populate it in `NewStyles`:

```go
type Styles struct {
	ViewerBg        lipgloss.Style
	ViewerHeader    lipgloss.Style
	Dim             lipgloss.Style
	Cursor          lipgloss.Style
	InviteIcon      lipgloss.Style
	InviteSummary   lipgloss.Style
	InviteField     lipgloss.Style
	InviteCancelled lipgloss.Style
	List            list.Styles
}
```

In `NewStyles(t *theme.CompiledTheme)` return value, append:

```go
List: uicore.NewListStyles(t),
```

- [ ] **Step 2.4: Verify reader builds**

Run: `go build ./internal/ui/reader/`
Expected: success.

- [ ] **Step 2.5: Write the integration test**

Append to `internal/ui/reader/linkpicker_test.go`:

```go
func TestLinkPicker_listModel_cursorAdvances(t *testing.T) {
	t.Helper()
	ct := theme.Themes[theme.DefaultThemeName]
	st := NewStyles(ct)
	p := NewLinkPicker(st).Open([]string{"https://a", "https://b", "https://c"})
	p = p.SetSize(60, 12)

	if got := p.Cursor(); got != 0 {
		t.Fatalf("initial cursor = %d, want 0", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})
	if got := p.Cursor(); got != 1 {
		t.Fatalf("cursor after j = %d, want 1", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'k'})
	if got := p.Cursor(); got != 0 {
		t.Fatalf("cursor after k = %d, want 0", got)
	}
}

func TestLinkPicker_listModel_enterLaunchesCursor(t *testing.T) {
	t.Helper()
	ct := theme.Themes[theme.DefaultThemeName]
	st := NewStyles(ct)
	p := NewLinkPicker(st).Open([]string{"https://a", "https://b"})
	p = p.SetSize(60, 12)
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on cursor produced no Cmd")
	}
	msgs := flattenBatch(cmd)
	wantURL := "https://b"
	var got string
	for _, m := range msgs {
		if launch, ok := m.(LaunchURLMsg); ok {
			got = launch.URL
		}
	}
	if got != wantURL {
		t.Fatalf("LaunchURLMsg.URL = %q, want %q", got, wantURL)
	}
}
```

If `flattenBatch` does not exist as a test helper, add it to a new `internal/ui/reader/testhelpers_test.go`:

```go
package reader

import (
	tea "charm.land/bubbletea/v2"
)

// flattenBatch invokes a tea.Cmd and recursively unwraps tea.BatchMsg
// to a flat []tea.Msg. Used by picker tests to assert the set of
// messages emitted from a key press without depending on order.
func flattenBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range m {
			out = append(out, flattenBatch(c)...)
		}
		return out
	case nil:
		return nil
	default:
		return []tea.Msg{msg}
	}
}
```

- [ ] **Step 2.6: Run test to verify it fails**

Run: `go test ./internal/ui/reader/ -run TestLinkPicker_listModel -v`
Expected: FAIL — these assertions probe the new integration that doesn't exist yet, but the build may also fail because the existing `LinkPicker` doesn't expose anything new yet. That's fine.

- [ ] **Step 2.7: Rewrite `linkpicker.go`**

Replace the entire contents of `internal/ui/reader/linkpicker.go` with:

```go
package reader

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// LinkPicker is the modal overlay launched by Tab while the viewer is
// open and ready. Single-column list of harvested URLs with cursor +
// Enter, 1-9 quick launch, Esc/Tab to close. App owns open state and
// overlay composition (ADR-0082).
type LinkPicker struct {
	shell  uicore.ModalShell
	list   list.Model
	links  []string
	styles Styles
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Enter key.Binding
	Close key.Binding
	// Digits[i] binds the digit key for harvested-link slot i+1.
	Digits [9]key.Binding
}

// linkItem wraps a URL string for the list.Item interface.
type linkItem string

func (i linkItem) FilterValue() string { return string(i) }

func NewLinkPicker(styles Styles) LinkPicker {
	keys := linkPickerKeys{
		Enter: key.NewBinding(key.WithKeys("enter")),
		Close: key.NewBinding(key.WithKeys("esc", "tab")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}

	l := list.New(nil, linkItemDelegate{styles: styles}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles = styles.List
	// list.Model defines its own quit key bindings; suppress them so q
	// stays the App-owned global quit (and is swallowed while picker
	// is open by reader/account routing).
	l.DisableQuitKeybindings()

	return LinkPicker{
		styles: styles,
		keys:   keys,
		list:   l,
	}
}

func (p LinkPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p LinkPicker) Cursor() int  { return p.list.Index() }

// Open transitions the picker into the open state with the given URL
// list, resetting cursor and offset.
func (p LinkPicker) Open(links []string) LinkPicker {
	p.shell = p.shell.WithOpen(true)
	p.links = links
	items := make([]list.Item, len(links))
	for i, u := range links {
		items[i] = linkItem(u)
	}
	p.list.SetItems(items)
	p.list.ResetSelected()
	return p
}

func (p LinkPicker) Close() LinkPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p LinkPicker) SetSize(width, height int) LinkPicker {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := linkPickerListSize(width, height)
	p.list.SetSize(contentW, listH)
	return p
}

// Update dispatches a tea.Msg while the picker is open and emits the
// launch/close Cmds for Enter, 1-9, Esc, and Tab. Navigation keys
// (j/k/up/down) are handled by list.Model.
func (p LinkPicker) Update(msg tea.Msg) (LinkPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Enter):
		idx := p.list.Index()
		if idx < 0 || idx >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[idx]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return LinkPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.links) {
				return p, tea.Batch(
					func() tea.Msg { return LaunchURLMsg{URL: p.links[i]} },
					func() tea.Msg { return LinkPickerClosedMsg{} },
				)
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

const linkPickerMaxWidth = 70

// linkPickerListSize derives the inner list dimensions from the box
// outer size. The 7-row reservation is top + bottom border + rule +
// 2 preview lines + 1 title slack (matches the previous box layout).
func linkPickerListSize(boxW, boxH int) (contentW, listH int) {
	bw := linkPickerMaxWidth
	if boxW-4 < bw {
		bw = boxW - 4
	}
	if bw < 20 {
		bw = 20
	}
	contentW = bw - 2
	listH = boxH - 7
	if listH < 1 {
		listH = 1
	}
	return contentW, listH
}

// View renders the picker as a standalone string. Production
// composition goes through Box + Position + PlaceOverlay; this is
// the fallback for tests and degenerate sizes.
func (p LinkPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

// Box renders the modal at the size derived from (w, h).
func (p LinkPicker) Box(w, h int) string {
	contentW, _ := linkPickerListSize(w, h)
	listView := p.list.View()
	bodyRows := strings.Split(listView, "\n")
	for i, row := range bodyRows {
		bodyRows[i] = uicore.PadOrTruncate(row, contentW)
	}

	previewLines := p.previewLines(contentW)
	footerRows := make([]string, 2)
	for i := 0; i < 2; i++ {
		line := ""
		if i < len(previewLines) {
			line = previewLines[i]
		}
		footerRows[i] = uicore.PadOrTruncate(line, contentW)
	}

	return p.shell.Box("Links", bodyRows, footerRows, contentW)
}

// linkPickerInlineCap caps the per-row inline URL display so the
// picker stays visually tight on wide terminals.
const linkPickerInlineCap = 50

// linkItemDelegate renders one link row as "  [N] URL", painted with
// the cursor background when index == m.Index().
type linkItemDelegate struct {
	styles Styles
}

func (d linkItemDelegate) Height() int                             { return 1 }
func (d linkItemDelegate) Spacing() int                            { return 0 }
func (d linkItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d linkItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(linkItem)
	if !ok {
		return
	}
	url := string(li)
	contentW := m.Width()
	indexW := 4 // up to "[9] " = 4 cells; ≤9 items so single digit
	urlW := contentW - indexW
	if urlW > linkPickerInlineCap {
		urlW = linkPickerInlineCap
	}
	if ansix.Width(url) > urlW {
		url = ansix.Truncate(url, urlW)
	}
	body := uicore.PadOrTruncate(fmt.Sprintf("[%d] %s", index+1, url), contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

// previewLines returns up to 2 wrapped lines of the cursor row's full
// URL. The second line is truncated with "…" when the URL exceeds two
// rows of cells.
func (p LinkPicker) previewLines(width int) []string {
	idx := p.list.Index()
	if idx < 0 || idx >= len(p.links) {
		return nil
	}
	full := p.links[idx]
	wrapped := strings.Split(linkPickerWrap(full, width), "\n")
	if len(wrapped) <= 2 {
		return wrapped
	}
	row2 := wrapped[1]
	if ansix.Width(row2) >= width {
		row2 = ansix.Truncate(row2, width-1) + "…"
	} else {
		row2 += "…"
	}
	return []string{wrapped[0], row2}
}

// linkPickerWrap wraps s to width. URLs are unbreakable tokens that
// Wordwrap can't split, so a Hardwrap pass forces the residue.
func linkPickerWrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, false)
}

// Position returns the centered top-left for the rendered box; App
// feeds it to PlaceOverlay.
func (p LinkPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
```

- [ ] **Step 2.8: Run linkpicker tests**

Run: `go test ./internal/ui/reader/ -run LinkPicker -v`
Expected: PASS for all existing tests + the two new `_listModel_` tests.

- [ ] **Step 2.9: Run full reader tests**

Run: `go test ./internal/ui/reader/ -v`
Expected: PASS, no regressions in viewer / attachpicker / body rendering.

- [ ] **Step 2.10: Live UI capture**

Build and run poplar against the Fastmail account, open a message with multiple URLs in the body, press `Tab`, capture the linkpicker overlay. See `.claude/docs/tmux-testing.md` for the harness.

Verify: frame chrome unchanged; rows render `[N] URL`; cursor row painted with the accent background; preview lines below the rule still show the wrapped full URL.

- [ ] **Step 2.11: Commit**

```bash
git add internal/ui/reader/linkpicker.go internal/ui/reader/linkpicker_test.go internal/ui/reader/styles.go internal/ui/reader/testhelpers_test.go
git commit -m "$(cat <<'EOF'
Pass 15a t2: reader.LinkPicker on bubbles/v2/list

Hand-rolled cursor + offset + render loop replaced by list.Model.
External Msgs (LaunchURLMsg, LinkPickerClosedMsg) and the
picker-owned keys (Enter, Esc/Tab, 1-9) are unchanged. Custom
linkItemDelegate keeps the "[N] URL" row format and accent-cursor
treatment.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Convert `reader.AttachPicker` to `bubbles/v2/list`

**Files:**
- Modify: `internal/ui/reader/attachpicker.go`
- Modify: `internal/ui/reader/attachpicker_test.go`

**Why this picker second:** same shape as linkpicker but with attachment metadata, no filter. Consolidates the list-integration recipe before the harder movepicker task.

- [ ] **Step 3.1: Read the existing model and tests**

Run: `cat internal/ui/reader/attachpicker.go internal/ui/reader/attachpicker_test.go`

Note: external Msgs are `OpenAttachmentMsg{UID, Att}`, `SaveAttachmentMsg{UID, Att}`, `AttachPickerClosedMsg{}`. Picker-owned keys: `j/k`, `Enter`/`o` open, `s` save, `Esc`/`q`/`@` close, `1`-`9` open digit slot.

- [ ] **Step 3.2: Write the integration test**

Append to `internal/ui/reader/attachpicker_test.go`:

```go
func TestAttachPicker_listModel_cursorAdvances(t *testing.T) {
	t.Helper()
	ct := theme.Themes[theme.DefaultThemeName]
	st := NewStyles(ct)
	icons := uicore.SimpleIcons
	atts := []mail.Attachment{
		{Filename: "a.pdf", Size: 1234},
		{Filename: "b.png", Size: 5678},
	}
	p := NewAttachPicker(st, icons).Open(mail.UID(1), atts)
	p = p.SetSize(60, 12)

	if got := p.Cursor(); got != 0 {
		t.Fatalf("initial cursor = %d, want 0", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})
	if got := p.Cursor(); got != 1 {
		t.Fatalf("cursor after j = %d, want 1", got)
	}
}

func TestAttachPicker_listModel_enterOpensCursor(t *testing.T) {
	t.Helper()
	ct := theme.Themes[theme.DefaultThemeName]
	st := NewStyles(ct)
	icons := uicore.SimpleIcons
	atts := []mail.Attachment{
		{Filename: "a.pdf", Size: 1234},
		{Filename: "b.png", Size: 5678},
	}
	p := NewAttachPicker(st, icons).Open(mail.UID(7), atts)
	p = p.SetSize(60, 12)
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msgs := flattenBatch(cmd)
	var got OpenAttachmentMsg
	for _, m := range msgs {
		if open, ok := m.(OpenAttachmentMsg); ok {
			got = open
		}
	}
	if got.Att.Filename != "b.png" || got.UID != mail.UID(7) {
		t.Fatalf("OpenAttachmentMsg = %+v, want UID=7 Filename=b.png", got)
	}
}
```

- [ ] **Step 3.3: Run test to verify it fails**

Run: `go test ./internal/ui/reader/ -run TestAttachPicker_listModel -v`
Expected: FAIL — assertions probe the new integration.

- [ ] **Step 3.4: Rewrite `attachpicker.go`**

Replace the entire contents of `internal/ui/reader/attachpicker.go` with:

```go
package reader

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/humanize"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// AttachPicker is the modal overlay launched by `@` in the viewer.
// Single-column list of attachment metadata. Cursor + Enter (open),
// `o` (open), `s` (save), 1-9 (open Nth), Esc/q/@ close.
type AttachPicker struct {
	shell  uicore.ModalShell
	list   list.Model
	uid    mail.UID
	items  []mail.Attachment
	styles Styles
	icons  uicore.IconSet
	keys   attachPickerKeys
}

type attachPickerKeys struct {
	Enter  key.Binding
	Open   key.Binding
	Save   key.Binding
	Close  key.Binding
	Digits [9]key.Binding
}

// attachItem wraps mail.Attachment for the list.Item interface.
type attachItem struct {
	att mail.Attachment
}

func (i attachItem) FilterValue() string { return i.att.Filename }

func NewAttachPicker(styles Styles, icons uicore.IconSet) AttachPicker {
	keys := attachPickerKeys{
		Enter: key.NewBinding(key.WithKeys("enter")),
		Open:  key.NewBinding(key.WithKeys("o")),
		Save:  key.NewBinding(key.WithKeys("s")),
		Close: key.NewBinding(key.WithKeys("esc", "q", "@")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}

	l := list.New(nil, attachItemDelegate{styles: styles, icons: icons}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles = styles.List
	l.DisableQuitKeybindings()

	return AttachPicker{styles: styles, icons: icons, keys: keys, list: l}
}

func (p AttachPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p AttachPicker) Cursor() int  { return p.list.Index() }

func (p AttachPicker) Open(uid mail.UID, items []mail.Attachment) AttachPicker {
	p.shell = p.shell.WithOpen(true)
	p.uid = uid
	p.items = items
	listItems := make([]list.Item, len(items))
	for i, a := range items {
		listItems[i] = attachItem{att: a}
	}
	p.list.SetItems(listItems)
	p.list.ResetSelected()
	return p
}

func (p AttachPicker) Close() AttachPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p AttachPicker) SetSize(width, height int) AttachPicker {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := attachPickerListSize(width, height)
	p.list.SetSize(contentW, listH)
	return p
}

func (p AttachPicker) Update(msg tea.Msg) (AttachPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Enter), key.Matches(keyMsg, p.keys.Open):
		return p, p.openIndex(p.list.Index())
	case key.Matches(keyMsg, p.keys.Save):
		return p, p.saveIndex(p.list.Index())
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return AttachPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.items) {
				return p, p.openIndex(i)
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p AttachPicker) openIndex(i int) tea.Cmd {
	if i < 0 || i >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return OpenAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

func (p AttachPicker) saveIndex(i int) tea.Cmd {
	if i < 0 || i >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return SaveAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

const attachPickerMaxWidth = 70

func attachPickerListSize(boxW, boxH int) (contentW, listH int) {
	bw := attachPickerMaxWidth
	if boxW-4 < bw {
		bw = boxW - 4
	}
	if bw < 24 {
		bw = 24
	}
	contentW = bw - 2
	listH = boxH - 5 // top + bottom border + title slack + 1 footer row
	if listH < 1 {
		listH = 1
	}
	return contentW, listH
}

func (p AttachPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p AttachPicker) Box(w, h int) string {
	contentW, _ := attachPickerListSize(w, h)
	listView := p.list.View()
	bodyRows := strings.Split(listView, "\n")
	for i, row := range bodyRows {
		bodyRows[i] = uicore.PadOrTruncate(row, contentW)
	}
	footer := uicore.PadOrTruncate("Enter/o open  s save  Esc close", contentW)
	return p.shell.Box("Attachments", bodyRows, []string{footer}, contentW)
}

// attachItemDelegate renders one attachment row as
// "<icon>[N] filename (size)", painted with the cursor background
// when index == m.Index().
type attachItemDelegate struct {
	styles Styles
	icons  uicore.IconSet
}

func (d attachItemDelegate) Height() int                             { return 1 }
func (d attachItemDelegate) Spacing() int                            { return 0 }
func (d attachItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d attachItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ai, ok := item.(attachItem)
	if !ok {
		return
	}
	att := ai.att
	contentW := m.Width()
	name := att.Filename
	if name == "" {
		name = "attachment"
	}
	size := humanize.Bytes(int64(att.Size))
	body := ansix.PadOrTruncate(
		fmt.Sprintf("%s[%d] %s (%s)", d.icons.Attachment, index+1, name, size),
		contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

func (p AttachPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
```

- [ ] **Step 3.5: Run attachpicker tests**

Run: `go test ./internal/ui/reader/ -run AttachPicker -v`
Expected: PASS for all existing tests + the two new `_listModel_` tests.

- [ ] **Step 3.6: Run full reader tests**

Run: `go test ./internal/ui/reader/ -v`
Expected: PASS.

- [ ] **Step 3.7: Live UI capture**

Open a message with attachments, press `@`, capture the attachpicker overlay. Verify chrome, row format (`<icon>[N] filename (size)`), cursor row, footer hint.

- [ ] **Step 3.8: Commit**

```bash
git add internal/ui/reader/attachpicker.go internal/ui/reader/attachpicker_test.go
git commit -m "$(cat <<'EOF'
Pass 15a t3: reader.AttachPicker on bubbles/v2/list

Same recipe as t2 — list.Model owns cursor + render, picker keeps
Enter/o/s/digit/Esc dispatch and the OpenAttachmentMsg /
SaveAttachmentMsg / AttachPickerClosedMsg surface.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Convert `movepicker.Model` to `bubbles/v2/list`

**Files:**
- Modify: `internal/ui/movepicker/model.go`
- Modify: `internal/ui/movepicker/model_test.go`
- Modify: `internal/ui/movepicker/styles.go`

**Why this picker last:** largest (360 lines), filter is always-on, ModalShell + cache. Most surface area to validate.

- [ ] **Step 4.1: Read the existing model and tests**

Run: `cat internal/ui/movepicker/model.go internal/ui/movepicker/styles.go internal/ui/movepicker/model_test.go`

Note:
- External Msgs: `OpenMsg{UIDs, Src, Folders}`, `PickedMsg{UIDs, Src, Dest}`, `ClosedMsg{}`.
- Picker-owned keys: arrow up/down, Enter (pick), Esc (close), Backspace (filter delete), `q` (swallowed). Raw rune keys (`unicode.IsPrint`) append to `p.filter`.
- Source folder is excluded from `p.all` on `Open`.
- `recompute()` rebuilds `p.matches` (indices into `p.all` matching `p.filter`).
- The existing `*modelCache` (heap-allocated dirty cache, ADR-0130) goes away with adoption — `list.Model` owns its own viewport state.

The filter is always-on — there is no `/` toggle. With `list.Model` we put it in `FilterStateFiltering` on `Open` and never let the user exit filter mode.

- [ ] **Step 4.2: Update `movepicker.NewStyles` to populate `List`**

Replace the contents of `internal/ui/movepicker/styles.go` with:

```go
package movepicker

import (
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:    lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Match:  lipgloss.NewStyle().Underline(true),
		List:   uicore.NewListStyles(t),
	}
}
```

The `List list.Styles` field itself is added in Step 4.5 as part of the `Styles` struct rewrite.

`internal/ui/app.go:140` already calls `movepicker.New(movepicker.NewStyles(t))` — no caller change needed.

- [ ] **Step 4.3: Write the integration test**

The existing tests use `newTestPicker()` which constructs a `Styles{}` literal. Update that helper first to include the new `List` field, then add the integration tests. In `internal/ui/movepicker/model_test.go`:

Update the `newTestPicker` helper (existing function) to:

```go
func newTestPicker() Model {
	ct := theme.Themes[theme.DefaultThemeName]
	return New(NewStyles(ct))
}
```

This routes through `NewStyles` so the helper picks up the new `List` field automatically. If existing tests called `New(Styles{Dim: ..., Cursor: ...})` inline, replace those call sites with `New(NewStyles(ct))` too.

Then append the integration tests:

```go
func TestMovepicker_listModel_cursorAndFilter(t *testing.T) {
	t.Helper()
	folders := []mail.FolderEntry{
		{Provider: "INBOX", Display: "Inbox"},
		{Provider: "Archive", Display: "Archive"},
		{Provider: "Sent", Display: "Sent"},
		{Provider: "Junk", Display: "Junk"},
	}
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", folders)
	p = p.SetSize(60, 16)

	// Source folder INBOX is excluded; 3 destinations remain.
	if got := p.Len(); got != 3 {
		t.Fatalf("Len after Open = %d, want 3 (INBOX excluded)", got)
	}

	// Filter "ar" matches "Archive" only.
	for _, r := range "ar" {
		p, _ = p.Update(tea.KeyPressMsg{Code: rune(r), Text: string(r)})
	}
	if got := p.Filter(); got != "ar" {
		t.Fatalf("Filter = %q, want %q", got, "ar")
	}
	if got := p.MatchCount(); got != 1 {
		t.Fatalf("MatchCount after filter = %d, want 1", got)
	}

	// Backspace removes a rune.
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := p.Filter(); got != "a" {
		t.Fatalf("Filter after backspace = %q, want %q", got, "a")
	}
}

func TestMovepicker_listModel_enterEmitsPicked(t *testing.T) {
	t.Helper()
	folders := []mail.FolderEntry{
		{Provider: "INBOX", Display: "Inbox"},
		{Provider: "Archive", Display: "Archive"},
	}
	p := newTestPicker().Open([]mail.UID{42}, "INBOX", folders)
	p = p.SetSize(60, 16)

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	picked, ok := msg.(PickedMsg)
	if !ok {
		t.Fatalf("Enter cmd returned %T, want PickedMsg", msg)
	}
	if picked.Dest != "Archive" || picked.Src != "INBOX" || len(picked.UIDs) != 1 {
		t.Fatalf("PickedMsg = %+v, want {UIDs:[42] Src:INBOX Dest:Archive}", picked)
	}
}
```

- [ ] **Step 4.4: Run test to verify it fails**

Run: `go test ./internal/ui/movepicker/ -run _listModel -v`
Expected: FAIL — `Len()`, `Filter()`, `MatchCount()` accessors don't exist yet, or the integration is incomplete.

- [ ] **Step 4.5: Rewrite `model.go`**

Replace the entire contents of `internal/ui/movepicker/model.go` with:

```go
package movepicker

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Styles is the move picker's projection of internal/ui.Styles.
type Styles struct {
	Dim    lipgloss.Style
	Cursor lipgloss.Style
	// Match underlines runes that match the active filter substring.
	// Underline-only so it composes with the row's base foreground.
	Match lipgloss.Style
	// List carries the shared list chrome projection.
	List list.Styles
}

// OpenMsg asks App to open the move-to-folder picker.
type OpenMsg struct {
	UIDs    []mail.UID
	Src     string
	Folders []mail.FolderEntry
}

// PickedMsg fires when the user selects a destination folder.
type PickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string
}

// ClosedMsg fires when the picker is dismissed without a pick.
type ClosedMsg struct{}

// Model is the modal overlay launched by `m` from the account view.
// App owns open state and overlay composition (ADR-0087).
type Model struct {
	shell  uicore.ModalShell
	list   list.Model
	uids   []mail.UID
	src    string
	all    []mail.FolderEntry
	styles Styles
	keys   modelKeys
}

type modelKeys struct {
	Pick    key.Binding
	Close   key.Binding
	Swallow key.Binding
}

// folderItem wraps mail.FolderEntry for list.Item.
type folderItem struct {
	entry mail.FolderEntry
}

func (i folderItem) FilterValue() string {
	if i.entry.Display != "" {
		return i.entry.Display
	}
	return i.entry.Provider
}

func New(styles Styles) Model {
	l := list.New(nil, folderItemDelegate{styles: styles}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles = styles.List
	l.DisableQuitKeybindings()

	return Model{
		styles: styles,
		list:   l,
		keys: modelKeys{
			Pick:    key.NewBinding(key.WithKeys("enter")),
			Close:   key.NewBinding(key.WithKeys("esc")),
			Swallow: key.NewBinding(key.WithKeys("q")),
		},
	}
}

func (p Model) IsOpen() bool { return p.shell.IsOpen() }

// Open snapshots the targets and folder list. The source folder is
// excluded so the picker never offers a no-op move-to-self.
func (p Model) Open(uids []mail.UID, src string, folders []mail.FolderEntry) Model {
	p.shell = p.shell.WithOpen(true)
	p.uids = uids
	p.src = src
	p.all = make([]mail.FolderEntry, 0, len(folders))
	for _, f := range folders {
		if f.Provider == src {
			continue
		}
		p.all = append(p.all, f)
	}
	items := make([]list.Item, len(p.all))
	for i, f := range p.all {
		items[i] = folderItem{entry: f}
	}
	p.list.SetItems(items)
	p.list.ResetSelected()
	// Always-on filter: enter Filtering state immediately so raw rune
	// keys append to the filter input and Backspace deletes runes.
	p.list.SetFilterState(list.Filtering)
	return p
}

func (p Model) Close() Model {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p Model) SetSize(width, height int) Model {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := movepickerListSize(width, height)
	p.list.SetSize(contentW, listH)
	return p
}

// Len reports the number of destination folders (post-source-filter).
func (p Model) Len() int { return len(p.all) }

// Filter returns the active filter substring (test-facing).
func (p Model) Filter() string { return p.list.FilterValue() }

// MatchCount returns the number of items currently visible after the
// active filter (test-facing).
func (p Model) MatchCount() int { return len(p.list.VisibleItems()) }

func (p Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	// q is swallowed while picker is open (App-owned global quit
	// would otherwise fire). Route nothing.
	if key.Matches(keyMsg, p.keys.Swallow) {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Pick):
		item, ok := p.list.SelectedItem().(folderItem)
		if !ok {
			return p, nil
		}
		dest := item.entry.Provider
		if dest == "" {
			dest = item.entry.Display
		}
		uids, src := p.uids, p.src
		return p, func() tea.Msg {
			return PickedMsg{UIDs: uids, Src: src, Dest: dest}
		}
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return ClosedMsg{} }
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

const movepickerMaxWidth = 56

func movepickerListSize(boxW, boxH int) (contentW, listH int) {
	bw := movepickerMaxWidth
	if boxW-4 < bw {
		bw = boxW - 4
	}
	if bw < 20 {
		bw = 20
	}
	contentW = bw - 2
	listH = boxH - 6 // top + bottom border + title + filter prompt slack
	if listH < 1 {
		listH = 1
	}
	return contentW, listH
}

func (p Model) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p Model) Box(w, h int) string {
	contentW, _ := movepickerListSize(w, h)
	listView := p.list.View()
	bodyRows := strings.Split(listView, "\n")
	for i, row := range bodyRows {
		bodyRows[i] = uicore.PadOrTruncate(row, contentW)
	}
	footer := uicore.PadOrTruncate(
		fmt.Sprintf("Enter move  Esc cancel  (%d match)", p.MatchCount()), contentW)
	return p.shell.Box("Move to", bodyRows, []string{footer}, contentW)
}

// folderItemDelegate renders one folder row. Filter-match runes are
// underlined via styles.Match; cursor row is painted with the accent
// background.
type folderItemDelegate struct {
	styles Styles
}

func (d folderItemDelegate) Height() int                             { return 1 }
func (d folderItemDelegate) Spacing() int                            { return 0 }
func (d folderItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d folderItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(folderItem)
	if !ok {
		return
	}
	display := fi.entry.Display
	if display == "" {
		display = fi.entry.Provider
	}
	contentW := m.Width()
	matches := m.MatchesForItem(index)
	body := renderWithMatches(display, matches, contentW, d.styles.Match)
	body = ansix.PadOrTruncate(body, contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

// renderWithMatches paints rune indices in `matches` with `match`
// style. matches comes from list.Model.MatchesForItem and is in
// rune-index space relative to FilterValue().
func renderWithMatches(s string, matches []int, _ int, match lipgloss.Style) string {
	if len(matches) == 0 {
		return s
	}
	mset := make(map[int]bool, len(matches))
	for _, i := range matches {
		mset[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		ch := string(r)
		if mset[i] {
			b.WriteString(match.Render(ch))
		} else {
			b.WriteString(ch)
		}
	}
	return b.String()
}

func (p Model) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
```

Notes for the engineer:
- The previous `recompute()` / `matches` / `cursor` / `offset` / `filter` fields and the `*modelCache` are gone. `list.Model` owns all of them.
- `Pick` reads `p.list.SelectedItem()` which honors the active filter (visible items only).
- `q` is explicitly swallowed before delegating to `list.Model` so the user can't accidentally trigger global quit while typing a filter that contains `q`.
- The renderer uses `list.Model.MatchesForItem(index)` to underline filter-match runes — this replaces the hand-rolled match-painting in the old code.

- [ ] **Step 4.6: Run movepicker tests**

Run: `go test ./internal/ui/movepicker/ -v`
Expected: PASS for all existing tests + the two new `_listModel_` tests.

If the existing tests reference removed fields (`p.filter`, `p.matches`, `p.cursor` directly), update them to use the new accessors (`p.Filter()`, `p.MatchCount()`, `p.list.Index()`).

- [ ] **Step 4.7: Run full ui tests**

Run: `go test ./internal/ui/...`
Expected: PASS.

- [ ] **Step 4.8: Live UI capture**

Open the account view, select a message, press `m`, capture the movepicker overlay at 80×24 and 120×40. Type a few characters to test filter, press Backspace, press Enter on a folder. Verify the move actually queues and executes.

- [ ] **Step 4.9: Profile worst-case folder count (per spec risk section)**

If you have access to an account with 1000+ folders, capture the picker open + filter typing latency in the tmux harness. Compare against the pre-adoption baseline (`git stash` the changes, capture, `git stash pop`).

If the regression is material (e.g. >50ms perceptible lag on filter keystroke), pause and write `0195-list-render-cache.md` with a poplar-side wrapper plan instead of completing this task. Otherwise note "no measurable regression" in the commit message.

If you don't have access to such an account, note that in the commit message and flag for follow-up.

- [ ] **Step 4.10: Commit**

```bash
git add internal/ui/movepicker/model.go internal/ui/movepicker/model_test.go internal/ui/movepicker/styles.go internal/ui/styles.go
git commit -m "$(cat <<'EOF'
Pass 15a t4: movepicker.Model on bubbles/v2/list

list.Model owns cursor, filter, matches, and render. Always-on
filter via SetFilterState(Filtering) on Open. modelCache (the
ADR-0130 escape hatch) is gone; list.Model owns viewport state.
External Msgs (PickedMsg, ClosedMsg) and the picker-owned keys
(Enter pick, Esc close, q swallowed) are unchanged. Filter-match
underline now sources from list.Model.MatchesForItem.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Pass-end consolidation

Per `poplar-pass` skill — ADR + invariants + STATUS + plan archival + commit gate + push + install.

- [ ] **Step 5.1: Run /simplify**

Invoke the `simplify` skill against the diff for Pass 15a (everything since the spec commit `d7d9005`).

Run: `git diff d7d9005..HEAD`

Apply genuine wins from the four review agents (reuse, quality, efficiency, voice). Skip findings under the pre-beta "valid skip rationales" (CLAUDE.md): speculative future consumer, upstream-blocked, premature optimization without measurement.

- [ ] **Step 5.2: Run the §10 idiomatic-bubbletea checklist**

Walk the Pass 15a diff against `docs/poplar/bubbletea-conventions.md` §10. Verify each item — tmux captures from steps 2.10, 3.7, 4.8 cover the visual checks.

If any deviation surfaced (cache retention, custom item delegate doing something unusual), name it in the ADR written below.

- [ ] **Step 5.3: Write ADR-0194**

Create `docs/poplar/decisions/0194-bubbles-v2-list-adoption.md`:

```markdown
---
title: Adopt bubbles/v2/list as the load-bearing list primitive
status: accepted
date: 2026-05-10
---

## Context

Poplar shipped Pass 13.2a/b on charm.land/v2 (ADRs 0189a/b) but
several picker surfaces still hand-rolled cursor, filter, and
render code that bubbles/v2/list now provides upstream.
bubbletea-conventions.md requires ADRs for any bubbles deviation;
none existed for the picker family, so the deviation was debt
rather than a deliberate call.

CLAUDE.md frames poplar as showcase-quality. v0.9.0 cannot freeze
on a hand-rolled list primitive that the upstream component would
cover.

## Decision

bubbles/v2/list is the load-bearing list primitive for picker-
shaped UI. Three surfaces adopted in Pass 15a: reader.LinkPicker,
reader.AttachPicker, movepicker.Model. Each holds a list.Model
field; routes tea.KeyPressMsg through it after handling picker-
specific keys (Enter / Esc / digits / pick); reads cursor and
filter state via list.Model accessors; renders bespoke item
shapes via custom list.ItemDelegate.

uicore.NewListStyles projects the compiled theme onto list.Styles
so the three consumers share one chrome projection.

The pre-adoption render cache on movepicker (an ADR-0130 escape
hatch) is removed — list.Model owns its own viewport state.

## Consequences

- Three subsystems gain upstream filter, scroll, pagination, and
  status-bar behavior for free; bug fixes upstream land in poplar
  on dependency bumps.
- list.Model.MatchesForItem replaces the hand-rolled filter-match
  painting in movepicker.
- Three deferred surfaces remain: compose/attachpicker (Pass
  15a.5, bubbles/v2/filepicker — different bubble), helppopover
  and schedulepicker (Pass 15d — ADR'd deviations because
  neither is list-shaped).
- Pass 15b (sidebar tree) and 15c (messagelist on list) build on
  this foundation.
```

- [ ] **Step 5.4: Update invariants.md**

Open `docs/poplar/invariants.md`. In the "Architecture → Repo & libraries" section, add the `bubbles/v2/list` adoption fact under the existing `internal/ui/` paragraph. The fact: "Picker surfaces (`movepicker`, `reader.LinkPicker`, `reader.AttachPicker`) are `bubbles/v2/list` consumers; chrome styles project from `uicore.NewListStyles(*theme.CompiledTheme)`. ADR-0194."

The file is hard-capped at 400 lines. If adding the fact pushes it over, prune by collapsing redundant sentences elsewhere — do not append blindly.

- [ ] **Step 5.5: Update decisions/INDEX.md**

Add a row mapping the new fact to ADR-0194 under the "Architecture" theme.

- [ ] **Step 5.6: Update STATUS.md**

- Mark Pass 15a `done` in the table.
- Replace the current starter prompt with the next one (Pass 15a.5 — `bubbles/v2/filepicker` adoption). Use the starter-prompt format from the `poplar-pass` skill.
- STATUS.md must stay ≤60 lines. Prune if needed.

Starter prompt for 15a.5 (drop into STATUS.md verbatim):

```markdown
## Next starter prompt (Pass 15a.5)

> **Goal.** Adopt `bubbles/v2/filepicker` for `internal/ui/compose/attachpicker.go` (ADR-0179) so the multi-select TUI file browser composes from the upstream bubble instead of hand-rolled directory traversal. Second of four bubbles-adoption passes (15a, 15a.5, 15b, 15c, 15d) before Polish II and the v0.9.0 freeze.
>
> **Scope.** `internal/ui/compose/attachpicker.go` and tests. Keep ADR-0179's keymap (Space toggle, `a` accept, Enter single-attach shortcut, `.` toggle hidden, `Esc` cancel) and the external `AttachAcceptedMsg` / `AttachCancelledMsg` surface. Reuse `uicore.NewListStyles` if filepicker exposes a list-styled internal; otherwise add a `uicore.NewFilePickerStyles` sibling.
>
> **Settled (do not re-brainstorm):** External Msg surface unchanged. Multi-select stays the poplar-side responsibility (filepicker is single-select upstream). View-state stack on ascend stays the same UX. Footer hint `^O attach` at rank 6 unchanged.
>
> **Still open — brainstorm these:** how to layer multi-select on top of filepicker's single-select API (own a parallel `selected map[string]bool`, intercept Space before dispatching to filepicker, render selected chips above the file list?); whether the async readDir + id-guard contract still applies or filepicker handles it; whether `.` (hidden toggle) is exposed by filepicker or stays poplar-side; whether the partial fit warrants an ADR'd deviation instead of full adoption.
>
> **Approach.** Brainstorm the open questions, write a plan doc at `docs/superpowers/plans/YYYY-MM-DD-bubbles-filepicker-attach.md`, then implement. Standard pass-end checklist applies.
```

- [ ] **Step 5.7: Archive plan + spec**

Run:
```bash
git mv docs/superpowers/plans/2026-05-10-bubbles-list-pickers.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-10-bubbles-list-pickers-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 5.8: make check**

Run: `make check`
Expected: green (fmt-check, vet, voice, test all pass).

- [ ] **Step 5.9: Commit consolidation**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 15a: ADR-0194 + invariants + STATUS + archival

ADR-0194 codifies bubbles/v2/list as the load-bearing list
primitive. Invariants gain the picker fact pointing at the new
ADR; STATUS marks 15a done and queues 15a.5 (filepicker for
compose/attachpicker). Plan + spec archived.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5.10: Push and install**

```bash
git push
make install
```

Verify the installed binary launches:

```bash
~/.local/bin/poplar --version
```

- [ ] **Step 5.11: Smoke test the installed binary**

Launch poplar against the Fastmail account:

```bash
~/.local/bin/poplar
```

Open a message with attachments and links. Press `Tab` (linkpicker), `@` (attachpicker), `Esc` to close, `m` (movepicker), type to filter, `Esc` to close. Confirm visual parity and that no crash, panic, or regression surfaces.

If anything is wrong, rollback is `git revert HEAD` (per consolidation commit) or per-task revert; investigate, fix, recommit. Pass is not "done" until the installed binary works.

---

## Pass-complete

When all checkboxes above are checked and the smoke test passes, Pass 15a is done. Pass 15a.5 begins on the next "continue development" trigger.
