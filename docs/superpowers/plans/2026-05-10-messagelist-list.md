# Pass 17b — messagelist on bubbles/v2/list

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate `internal/ui/messagelist/` to `bubbles/v2/list` with a custom item delegate, exposing `KeyMap` + `Update` as the dispatch surface and landing the `iter.Seq2` thread walk that closes BACKLOG #46.

**Architecture:** `messagelist.Model` keeps its data block (source, derived rows, fold/sort/filter state, ActionTargets) but embeds `list.Model` for cursor + viewport + key dispatch. A `*rowDelegate` holds per-frame render context (styles, layout, icons, clock, results-mode origin map) and routes `Render()` through the existing row builder. Hidden rows are filtered out of the items the list sees; the full `m.rows` slice still backs `Rows()` / `ActionTargets`. The `appendThreadRows` walker becomes an `iter.Seq2` pull iterator.

**Tech Stack:** `charm.land/bubbles/v2/list@v2.1.0`, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, Go 1.26.1, `iter.Seq2` (stdlib).

**Spec:** `docs/superpowers/specs/2026-05-10-messagelist-list-design.md`

---

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `internal/ui/messagelist/model.go` | Modify | Embedded `list.Model`; new `Update`; removed imperative movement methods; `syncList` / `snapToUIDInList`; renderRow lifted onto delegate |
| `internal/ui/messagelist/keys.go` | Create | Exported `KeyMap` (Down, Up, Top, Bottom) + `DefaultKeyMap()` |
| `internal/ui/messagelist/delegate.go` | Create | `rowDelegate` type + `Render`/`Height`/`Spacing`/`Update` |
| `internal/ui/messagelist/walk.go` | Create | `walkThread` `iter.Seq2` pull iterator |
| `internal/ui/messagelist/walk_test.go` | Create | walkThread parity test |
| `internal/ui/messagelist/keys_test.go` | Create | KeyMap binding + modifier-rejection tests |
| `internal/ui/messagelist/model_test.go` | Modify | MoveDown/MoveUp/etc. → `Update(keyPress(...))`; new fold-cursor-preservation test |
| `internal/ui/account/model.go` | Modify | `handleKey` MsgList* cases delegate to `m.msglist.Update(msg)` |
| `internal/ui/account/keys.go` | Modify (minimal) | MsgList* bindings stay (no-op pass-through, removed in 17c) |
| `.claude/rules/ui-invariants.md` | Modify | Message-list section: drop "Hand-rolled" framing; add bubbles/v2/list + delegate fact |
| `docs/poplar/decisions/0199-pass-17b-messagelist-on-bubbles-list.md` | Create | ADR for the migration + iter.Seq2 walk + ADR-0130 scope extension |
| `docs/poplar/decisions/INDEX.md` | Modify | Index row for ADR-0199 |
| `docs/poplar/STATUS.md` | Modify | Pass 17b done; next prompt = 17c |
| `docs/superpowers/archive/specs/2026-05-10-messagelist-list-design.md` | Move | Archive spec at pass end |
| `docs/superpowers/archive/plans/2026-05-10-messagelist-list.md` | Move | Archive this plan at pass end |

---

## Task 1: Add `messagelist.KeyMap`

**Files:**
- Create: `internal/ui/messagelist/keys.go`
- Test: `internal/ui/messagelist/keys_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/messagelist/keys_test.go`:

```go
package messagelist

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestDefaultKeyMap_Bindings(t *testing.T) {
	km := DefaultKeyMap()
	cases := []struct {
		name    string
		binding key.Binding
		text    string
		want    bool
	}{
		{"down j", km.Down, "j", true},
		{"down arrow", km.Down, "down", true},
		{"up k", km.Up, "k", true},
		{"up arrow", km.Up, "up", true},
		{"top g", km.Top, "g", true},
		{"bottom G", km.Bottom, "G", true},
		{"down rejects K", km.Down, "K", false},
		{"top rejects ctrl-g", km.Top, "ctrl+g", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := keyPress(tc.text)
			if got := key.Matches(msg, tc.binding); got != tc.want {
				t.Errorf("key.Matches(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// keyPress returns a tea.KeyPressMsg for the given key text. Modifier
// chords use the "ctrl+x" / "alt+x" / "shift+x" form; the prefix is
// parsed onto KeyPressMsg.Mod and the trailing rune populates Code+Text.
func keyPress(s string) tea.KeyPressMsg {
	var mod tea.KeyMod
	for {
		switch {
		case len(s) > 5 && s[:5] == "ctrl+":
			mod |= tea.ModCtrl
			s = s[5:]
		case len(s) > 4 && s[:4] == "alt+":
			mod |= tea.ModAlt
			s = s[4:]
		case len(s) > 6 && s[:6] == "shift+":
			mod |= tea.ModShift
			s = s[6:]
		default:
			goto done
		}
	}
done:
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: mod}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp, Mod: mod}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: mod}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc, Mod: mod}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: mod}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s, Mod: mod}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/glw907/Projects/poplar
go test ./internal/ui/messagelist/ -run TestDefaultKeyMap_Bindings 2>&1 | head -10
```

Expected: FAIL — `undefined: DefaultKeyMap` and `undefined: KeyMap`.

- [ ] **Step 3: Create `internal/ui/messagelist/keys.go`**

```go
package messagelist

import "charm.land/bubbles/v2/key"

// KeyMap collects the bindings messagelist.Update dispatches on.
// Only navigation keys live here — fold, visual-mode, and triage
// bindings need account-level guards and stay in account.keys.
type KeyMap struct {
	Down   key.Binding
	Up     key.Binding
	Top    key.Binding
	Bottom key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/ui/messagelist/ -run TestDefaultKeyMap_Bindings -v 2>&1 | tail -15
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/messagelist/keys.go internal/ui/messagelist/keys_test.go
git commit -m "$(cat <<'EOF'
Pass 17b: add messagelist.KeyMap (#45 item 2)

Exported navigation bindings — Down (j/down), Up (k/up), Top (g),
Bottom (G). Fold/visual/triage stay in account.keys because they
need account-level guards.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `iter.Seq2` thread walker

**Files:**
- Create: `internal/ui/messagelist/walk.go`
- Test: `internal/ui/messagelist/walk_test.go`

- [ ] **Step 1: Write the failing parity test**

Create `internal/ui/messagelist/walk_test.go`:

```go
package messagelist

import (
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestWalkThread_PrefixParity(t *testing.T) {
	// Build a tree:
	//   root
	//   ├── a
	//   │   └── a1
	//   └── b
	root := &threadNode{msg: mail.MessageInfo{UID: "root"}}
	a := &threadNode{msg: mail.MessageInfo{UID: "a"}}
	a1 := &threadNode{msg: mail.MessageInfo{UID: "a1"}}
	b := &threadNode{msg: mail.MessageInfo{UID: "b"}}
	a.children = []*threadNode{a1}
	root.children = []*threadNode{a, b}

	type seen struct {
		uid    mail.UID
		depth  uint8
		prefix string
	}
	var got []seen
	for node, step := range walkThread(root) {
		got = append(got, seen{
			uid:    node.msg.UID,
			depth:  step.depth,
			prefix: buildPrefix(step.ancestorLastFlags, step.isLast),
		})
	}

	want := []seen{
		{uid: "a", depth: 1, prefix: "├─ "},
		{uid: "a1", depth: 2, prefix: "│  └─ "},
		{uid: "b", depth: 1, prefix: "└─ "},
	}
	if len(got) != len(want) {
		t.Fatalf("walk yielded %d nodes, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestWalkThread_EarlyStop(t *testing.T) {
	root := &threadNode{msg: mail.MessageInfo{UID: "root"}}
	a := &threadNode{msg: mail.MessageInfo{UID: "a"}}
	b := &threadNode{msg: mail.MessageInfo{UID: "b"}}
	root.children = []*threadNode{a, b}

	count := 0
	for range walkThread(root) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break consumed %d nodes, want 1", count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/ui/messagelist/ -run TestWalkThread 2>&1 | head -10
```

Expected: FAIL — `undefined: walkThread` and `undefined: walkStep`.

- [ ] **Step 3: Create `internal/ui/messagelist/walk.go`**

```go
package messagelist

import "iter"

// walkStep carries the per-node prefix inputs walkThread yields
// alongside each *threadNode. ancestorLastFlags is the trail of
// "is-last-sibling" flags from root to this node's parent;
// isLast is the flag for this node itself.
type walkStep struct {
	depth             uint8
	ancestorLastFlags []bool
	isLast            bool
}

// walkThread visits every non-root descendant of root in
// depth-first root-then-children order, yielding the node and the
// inputs buildPrefix needs to render its box-drawing prefix.
//
// The yield callback's bool return is honored — break in a range
// loop stops the walk before recursing into the current node's
// children.
func walkThread(root *threadNode) iter.Seq2[*threadNode, walkStep] {
	return func(yield func(*threadNode, walkStep) bool) {
		var rec func(n *threadNode, ancestors []bool) bool
		rec = func(n *threadNode, ancestors []bool) bool {
			for i, c := range n.children {
				isLast := i == len(n.children)-1
				step := walkStep{
					depth:             uint8(len(ancestors) + 1),
					ancestorLastFlags: ancestors,
					isLast:            isLast,
				}
				if !yield(c, step) {
					return false
				}
				if !rec(c, append(ancestors, isLast)) {
					return false
				}
			}
			return true
		}
		rec(root, nil)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/ui/messagelist/ -run TestWalkThread -v 2>&1 | tail -15
```

Expected: PASS for both subtests.

- [ ] **Step 5: Refactor `appendThreadRows` to use the iterator**

Edit `internal/ui/messagelist/model.go`. Replace the existing `walk` closure block (current lines 419–433) so the function reads:

```go
// appendThreadRows builds a transient tree from one thread bucket and
// emits displayRows in depth-first root-then-children order with the
// box-drawing prefix for each row's position.
func appendThreadRows(rows []displayRow, bucket []mail.MessageInfo) []displayRow {
	rootIdx := pickRoot(bucket)
	root := &threadNode{msg: bucket[rootIdx]}

	byUID := map[mail.UID]*threadNode{}
	for i, msg := range bucket {
		if i == rootIdx {
			byUID[msg.UID] = root
			continue
		}
		byUID[msg.UID] = &threadNode{msg: msg}
	}

	for i, msg := range bucket {
		if i == rootIdx {
			continue
		}
		node := byUID[msg.UID]
		parent, ok := byUID[msg.InReplyTo]
		if !ok {
			parent = root
		}
		parent.children = append(parent.children, node)
	}

	var sortChildren func(n *threadNode)
	sortChildren = func(n *threadNode) {
		slices.SortStableFunc(n.children, func(a, b *threadNode) int {
			return compareMessage(a.msg, b.msg)
		})
		for _, c := range n.children {
			sortChildren(c)
		}
	}
	sortChildren(root)

	rows = append(rows, displayRow{
		msg:          root.msg,
		isThreadRoot: true,
		threadSize:   len(bucket),
		depth:        0,
	})

	for node, step := range walkThread(root) {
		rows = append(rows, displayRow{
			msg:    node.msg,
			depth:  step.depth,
			prefix: buildPrefix(step.ancestorLastFlags, step.isLast),
		})
	}
	return rows
}
```

- [ ] **Step 6: Run the full messagelist suite**

```bash
go test ./internal/ui/messagelist/ 2>&1 | tail -10
```

Expected: PASS (the existing pipeline tests still go through `appendThreadRows` and exercise the prefix walk).

- [ ] **Step 7: Commit**

```bash
git add internal/ui/messagelist/walk.go internal/ui/messagelist/walk_test.go internal/ui/messagelist/model.go
git commit -m "$(cat <<'EOF'
Pass 17b: thread walker as iter.Seq2 (closes #46)

walkThread yields (*threadNode, walkStep) so appendThreadRows can
range over the descendants instead of carrying its own recursion.
Closes BACKLOG #46 — the iter.Seq2 idiom landed inline now that
ADR-0196 locked modern-Go conventions.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `rowDelegate`

**Files:**
- Create: `internal/ui/messagelist/delegate.go`

This task lifts the rendering logic without yet wiring `list.Model`. The delegate exists as a private type with a method that takes a `displayRow` and a `bool isSelected` — exactly the inputs `renderRow` already needs. Task 4 will wire `list.Model.Render` to call into it.

- [ ] **Step 1: Create `internal/ui/messagelist/delegate.go`**

```go
package messagelist

import (
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// rowDelegate carries the per-frame render context bubbles/v2/list
// needs to render each displayRow. messagelist.Model holds it as
// *rowDelegate so context refreshes (SetSize, SetLayout,
// SetSearchResults, SetMessages) mutate fields in place without
// rebuilding the list's item slice. The pointer escape parallels
// ADR-0130's overlay-cache carveout, scoped here to per-frame
// context — never memoized output.
type rowDelegate struct {
	styles      Styles
	layout      uicore.LayoutMode
	icons       uicore.IconSet
	now         time.Time
	width       int
	resultsMode bool
	originByUID map[mail.UID]string
}

func (d *rowDelegate) Height() int                             { return 1 }
func (d *rowDelegate) Spacing() int                            { return 0 }
func (d *rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *rowDelegate) Render(w io.Writer, lm list.Model, idx int, item list.Item) {
	row, ok := item.(displayRow)
	if !ok {
		return
	}
	fmt.Fprint(w, d.renderRow(row, idx == lm.Index()))
}

// renderRow draws one message-list row. Lifted from messagelist.Model
// — width math, SPUA-A flag-cell adjustment, sender column,
// `[Folder]` results-mode prefix, thread prefix, subject budget, date
// column, and FillRowToWidth are unchanged from the pre-list version.
func (d *rowDelegate) renderRow(row displayRow, isSelected bool) string {
	msg := row.msg
	isUnread := msg.Flags&mail.FlagSeen == 0

	bgStyle := d.styles.MsgListBg
	if isSelected {
		bgStyle = d.styles.MsgListSelected
	}

	var cursor string
	if isSelected {
		cursor = uicore.ApplyBg(d.styles.MsgListCursor, bgStyle).Render(mlCursorGlyph)
	} else {
		cursor = bgStyle.Render(" ")
	}

	senderStyle := d.styles.MsgListReadSender
	subjectStyle := d.styles.MsgListReadSubject
	if isUnread {
		senderStyle = d.styles.MsgListUnreadSender
		subjectStyle = d.styles.MsgListUnreadSubject
	}

	senderText := padRight(truncateCells(d.senderWithOrigin(msg), d.layout.Sender), d.layout.Sender)
	sender := uicore.ApplyBg(senderStyle, bgStyle).Render(senderText)

	var date string
	if d.layout.Date > 0 {
		dateText := padLeft(truncateCells(row.dateText, d.layout.Date), d.layout.Date)
		date = uicore.ApplyBg(d.styles.MsgListDate, bgStyle).Render(dateText)
	}

	// Fixed overhead: cursor(1) + 3×sp2(separators) + sp(trail) = 8 cells.
	// Flag column adds flag(2) + sp2 = 12 cells. When Date=0 the trailing
	// sp2+date block is omitted. FillRowToWidth absorbs the slack.
	var flag string
	fixed := 8
	if d.layout.FlagColumn {
		flag = d.renderFlagCell(msg, isUnread, bgStyle)
		fixed = 12
	}

	flagAdjust := 0
	if ansix.SPUACellWidth() > 1 && d.layout.FlagColumn {
		flagAdjust = ansix.SpuaCount(flag) * (ansix.SPUACellWidth() - 1)
	}
	subjectWidth := max(1, d.width-fixed-d.layout.Sender-d.layout.Date-flagAdjust)
	prefixCells := lipgloss.Width(row.prefix)
	subjectCells := max(0, subjectWidth-prefixCells)

	prefixStyled := uicore.ApplyBg(d.styles.MsgListThreadPrefix, bgStyle).Render(row.prefix)
	subjectText := padRight(truncateCells(msg.Subject, subjectCells), subjectCells)
	subjectStyled := uicore.ApplyBg(subjectStyle, bgStyle).Render(subjectText)
	subject := prefixStyled + subjectStyled

	sp2 := bgStyle.Render("  ")
	sp1 := bgStyle.Render(" ")

	var rowStr string
	if d.layout.FlagColumn {
		rowStr = cursor + sp2 + flag + sp2 + sender + sp2 + subject
	} else {
		rowStr = cursor + sp2 + sender + sp2 + subject
	}
	if d.layout.Date > 0 {
		rowStr += sp2 + date
	}
	rowStr += sp1

	return uicore.FillRowToWidth(rowStr, d.width, bgStyle)
}

func (d *rowDelegate) senderWithOrigin(msg mail.MessageInfo) string {
	if !d.resultsMode {
		return msg.From
	}
	folder := d.originByUID[msg.UID]
	if folder == "" {
		return msg.From
	}
	return "[" + folder + "] " + msg.From
}

func (d *rowDelegate) renderFlagCell(msg mail.MessageInfo, isUnread bool, bgStyle lipgloss.Style) string {
	iconStyle := d.styles.MsgListIconRead
	if isUnread {
		iconStyle = d.styles.MsgListIconUnread
	}
	var glyph string
	switch {
	case msg.Flags&mail.FlagFlagged != 0:
		glyph = d.icons.FlagFlagged
		if isUnread {
			iconStyle = d.styles.MsgListFlagFlagged
		}
	case msg.Flags&mail.FlagAnswered != 0:
		glyph = d.icons.FlagAnswered
	case isUnread:
		glyph = d.icons.FlagUnread
	default:
		return bgStyle.Render("  ")
	}
	rendered := uicore.ApplyBg(iconStyle, bgStyle).Render(glyph)
	for ansix.Width(rendered) < mlFlagWidth {
		rendered += bgStyle.Render(" ")
	}
	return rendered
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/ui/messagelist/ 2>&1 | tail -10
```

Expected: clean build. If `unused: <name>` errors fire, the symbol isn't yet referenced — that's fine, Task 4 wires it.

Note: at this point `model.go` still has the original `Model.renderRow`, `Model.renderFlagCell`, and `Model.senderWithOrigin` methods. They stay until Task 4 deletes them. Compilation succeeds because the delegate methods don't shadow.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/messagelist/delegate.go
git commit -m "$(cat <<'EOF'
Pass 17b: rowDelegate scaffold

Lifts renderRow into a list.ItemDelegate. Held by *rowDelegate on
messagelist.Model so context refreshes mutate in place without
rebuilding list items. Same pointer-escape carveout as ADR-0130's
overlay caches, scoped to per-frame render context.

Old Model.renderRow stays until Task 4 wires the list and removes
it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Embed `list.Model`, wire `Update`, remove imperative movers

This is the largest mechanical task. It replaces `selected`/`offset` with `list.Model`, wires `Update`, and deletes the imperative movement methods. `account.Model` call sites are updated in Task 5; this task leaves the package in a state where `account` won't yet build (the missing `MoveDown`/etc. cause errors). That's intentional — Task 5 is the call-site sweep.

**Files:**
- Modify: `internal/ui/messagelist/model.go`

- [ ] **Step 1: Update imports**

In `internal/ui/messagelist/model.go`, change the import block to:

```go
import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/search"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/mattn/go-runewidth"
)
```

- [ ] **Step 2: Make `displayRow` satisfy `list.Item`**

In `internal/ui/messagelist/model.go`, immediately after the `displayRow` struct (around current line 76), add:

```go
// FilterValue satisfies list.Item. Filtering is disabled on
// messagelist's embedded list.Model — sidebar shelf owns search.
func (r displayRow) FilterValue() string { return "" }
```

- [ ] **Step 3: Replace `Model` struct fields**

Replace the `Model` struct (current lines 90–121) with:

```go
// Model renders the message list panel: flags, sender, subject, date.
// Embeds bubbles/v2/list with a custom *rowDelegate (delegate.go) so
// the list owns cursor + viewport + key dispatch. Owns thread
// grouping, fold state, sort direction, and the source/derived rows
// pipeline. The embedded list sees only visible rows (hidden rows
// stay in m.rows for tests, ActionTargets thread expansion, etc.).
type Model struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	threaded bool
	styles   Styles
	icons    uicore.IconSet
	layout   uicore.LayoutMode
	width    int
	height   int
	now      time.Time

	list     list.Model
	delegate *rowDelegate
	keys     KeyMap

	filter          searchFilter
	preSearchCursor int
	savedByFilter   bool
	filterResults   int
	visualMode      bool
	marked          map[mail.UID]struct{}

	resultsMode  bool
	originByUID  map[mail.UID]string
	preResults   []mail.MessageInfo
	preThreaded  bool
	preCursorUID mail.UID
}
```

- [ ] **Step 4: Replace `New` constructor**

Replace `New` (current lines 126–141) with:

```go
// New constructs a Model. layout defaults to (Sender=22, Date=5,
// FlagColumn=true) so tests that bypass WindowSizeMsg get sensible
// output before any SetLayout call.
func New(styles Styles, msgs []mail.MessageInfo, width, height int, icons uicore.IconSet) Model {
	delegate := &rowDelegate{
		styles: styles,
		layout: uicore.LayoutMode{Sender: 22, Date: 5, FlagColumn: true},
		icons:  icons,
		now:    time.Now(),
		width:  width,
	}
	ls := list.New(nil, delegate, width, height)
	ls.SetShowTitle(false)
	ls.SetShowFilter(false)
	ls.SetShowStatusBar(false)
	ls.SetShowPagination(false)
	ls.SetShowHelp(false)
	ls.SetFilteringEnabled(false)
	ls.InfiniteScrolling = false
	ls.DisableQuitKeybindings()
	ls.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(),
		CursorDown: key.NewBinding(),
	}

	m := Model{
		styles:   styles,
		icons:    icons,
		layout:   delegate.layout,
		width:    width,
		height:   height,
		folded:   map[mail.UID]bool{},
		marked:   map[mail.UID]struct{}{},
		sort:     SortDateDesc,
		threaded: true,
		now:      delegate.now,
		list:     ls,
		delegate: delegate,
		keys:     DefaultKeyMap(),
	}
	m.SetMessages(msgs)
	return m
}

// KeyMap returns the binding set Update dispatches on. Exported so
// the help popover and external test code can introspect.
func (m Model) KeyMap() KeyMap { return m.keys }

// SetKeyMap overrides the default bindings. Test seam.
func (m *Model) SetKeyMap(km KeyMap) { m.keys = km }
```

- [ ] **Step 5: Add `syncList` and `snapToUIDInList`; rewrite `rebuild` tail**

In `model.go`, find `rebuild` (current line 165). Replace its tail (the `for i := range rows ... m.rows = rows` block, current lines 208–211) with:

```go
	for i := range rows {
		rows[i].dateText = displayDate(rows[i].msg, m.now, m.layout.Date)
	}
	m.rows = rows
	m.delegate.now = m.now
	m.syncList()
}

// syncList copies the visible subset of m.rows into the embedded
// list.Model. Hidden rows (folded thread children) stay in m.rows
// for ActionTargets / Rows() / threadRootIndex but never reach the
// list's cursor or viewport.
func (m *Model) syncList() {
	visible := make([]list.Item, 0, len(m.rows))
	for _, r := range m.rows {
		if r.hidden {
			continue
		}
		visible = append(visible, r)
	}
	m.list.SetItems(visible)
}

// snapToUIDInList moves the list cursor onto the visible row whose
// msg.UID matches uid. Empty UID or no match → cursor at 0.
func (m *Model) snapToUIDInList(uid mail.UID) {
	if uid == "" {
		m.list.Select(0)
		return
	}
	for i, item := range m.list.Items() {
		if r, ok := item.(displayRow); ok && r.msg.UID == uid {
			m.list.Select(i)
			return
		}
	}
	m.list.Select(0)
}
```

- [ ] **Step 6: Remove imperative movement methods + accessors that read `m.selected`**

Delete the following methods from `model.go` entirely (they live at the noted current-line ranges; the actual line numbers may shift as you edit — search by name):

- `MoveDown` (line 830)
- `MoveUp` (line 831)
- `MoveToTop` (line 845)
- `MoveToBottom` (line 856)
- `HalfPageDown` (line 866)
- `HalfPageUp` (line 867)
- `PageDown` (line 868)
- `PageUp` (line 869)
- `clampOffset` (line 871)
- `moveBy` (line 799) — used only by the deleted movers
- `Model.renderRow` (line 919) — replaced by delegate
- `Model.renderFlagCell` (line 997) — replaced by delegate
- `Model.senderWithOrigin` (line 481) — replaced by delegate
- `Model.renderBlankLine` (line 1025) — `View()` rewrite below no longer needs it

`MoveCursor` survives (the viewer's `n`/`N` calls it programmatically). Rewrite it to drive the embedded list:

```go
// MoveCursor shifts by delta visible rows and returns the resulting
// UID plus whether the cursor moved. Boundaries are inert: calling
// at the first or last visible row returns ("", false). Programmatic
// entry point — the viewer's n/N path uses it; keyboard navigation
// goes through Update.
func (m *Model) MoveCursor(delta int) (mail.UID, bool) {
	before := m.list.Index()
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	for range delta {
		if step > 0 {
			m.list.CursorDown()
		} else {
			m.list.CursorUp()
		}
	}
	after := m.list.Index()
	if after == before {
		return "", false
	}
	return m.cursorUID(), true
}
```

- [ ] **Step 7: Rewrite `Selected`, `cursorUID`, `snapToUID`**

Replace the existing `Selected`, `SelectedMessage`, `cursorUID`, and `snapToUID` (lines 699–706, 747–768) with:

```go
// Selected returns the list cursor index over visible rows.
func (m Model) Selected() int { return m.list.Index() }

// SelectedMessage returns the message under the cursor.
func (m Model) SelectedMessage() (mail.MessageInfo, bool) {
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		return r.msg, true
	}
	return mail.MessageInfo{}, false
}

func (m Model) cursorUID() mail.UID {
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		return r.msg.UID
	}
	return ""
}

// snapToUID positions the list cursor on the visible row whose UID
// matches uid; falls through to snapToUIDInList semantics.
func (m *Model) snapToUID(uid mail.UID) {
	m.snapToUIDInList(uid)
}
```

- [ ] **Step 8: Rewrite `snapToVisible`**

Replace `snapToVisible` (lines 646–658) with:

```go
// snapToVisible re-anchors the list cursor on the nearest non-hidden
// row in m.rows. Called after fold toggles where the previously
// selected UID may now be hidden under a folded root.
func (m *Model) snapToVisible() {
	uid := ""
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		uid = r.msg.UID
	}
	// Find uid in m.rows; if hidden, walk backwards to the nearest
	// visible row (always a thread root for hidden children).
	if uid != "" {
		for i, r := range m.rows {
			if r.msg.UID != uid {
				continue
			}
			if !r.hidden {
				m.snapToUIDInList(uid)
				return
			}
			for j := i - 1; j >= 0; j-- {
				if !m.rows[j].hidden {
					m.snapToUIDInList(m.rows[j].msg.UID)
					return
				}
			}
			break
		}
	}
	m.list.Select(0)
}
```

- [ ] **Step 9: Update `SetSize`, `SetLayout`, `SetMessages`, `RefreshSource`, `AppendMessages`, `SetFilter`, `ClearFilter`, `SetSearchResults`, `ClearSearchResults`**

These all need delegate-context refreshes and/or `list.SetSize` calls. Apply these edits:

**`SetMessages`** (lines 146–158) — drop `m.selected = 0`, `m.offset = 0`. After `m.now = time.Now()` add `m.delegate.now = m.now`. Trailing `m.rebuild()` already calls `syncList`. Append `m.list.Select(0)` at the end.

```go
func (m *Model) SetMessages(msgs []mail.MessageInfo) {
	m.source = msgs
	m.folded = map[mail.UID]bool{}
	m.marked = map[mail.UID]struct{}{}
	m.visualMode = false
	m.filter = searchFilter{}
	m.savedByFilter = false
	m.preSearchCursor = 0
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.list.Select(0)
}
```

**`SetSize`** (lines 674–678) — forward to list and refresh delegate width.

```go
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.delegate.width = width
	m.list.SetSize(width, height)
}
```

**`SetLayout`** (lines 685–691) — refresh delegate layout, propagate flag/origin context.

```go
func (m *Model) SetLayout(l uicore.LayoutMode) {
	prevDate := m.layout.Date
	m.layout = l
	m.delegate.layout = l
	if prevDate != l.Date {
		m.rebuild()
	}
}
```

**`SetNow`** (lines 694–697) — refresh delegate clock.

```go
func (m *Model) SetNow(now time.Time) {
	m.now = now
	m.delegate.now = now
	m.rebuild()
}
```

**`AppendMessages`** (lines 778–784) — anchor by UID, no `m.selected` math.

```go
func (m *Model) AppendMessages(extra []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = append(m.source, extra...)
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(uid)
}
```

**`RefreshSource`** (lines 789–795) — same pattern.

```go
func (m *Model) RefreshSource(msgs []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = msgs
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(uid)
}
```

**`SetFilter`** (lines 550–558) — snapshot pre-filter cursor by index, rebuild, snap to 0.

```go
func (m *Model) SetFilter(q string) {
	if !m.savedByFilter && q != "" {
		m.preSearchCursor = m.list.Index()
		m.savedByFilter = true
	}
	m.filter = searchFilter{raw: q, query: lowerQueryTerms(search.Parse(q))}
	m.rebuild()
	m.list.Select(0)
}
```

**`ClearFilter`** (lines 563–574) — restore pre-filter index when valid.

```go
func (m *Model) ClearFilter() {
	m.filter = searchFilter{}
	m.rebuild()
	if m.savedByFilter {
		idx := m.preSearchCursor
		if idx >= len(m.list.Items()) {
			idx = 0
		}
		m.list.Select(idx)
		m.savedByFilter = false
	}
}
```

**`SetSearchResults`** (lines 498–514) — snapshot cursor UID, refresh delegate origin map, rebuild.

```go
func (m *Model) SetSearchResults(msgs []mail.MessageInfo, originByUID map[mail.UID]string) {
	if !m.resultsMode {
		m.preResults = m.source
		m.preThreaded = m.threaded
		m.preCursorUID = m.cursorUID()
	}
	m.resultsMode = true
	m.originByUID = originByUID
	m.delegate.resultsMode = true
	m.delegate.originByUID = originByUID
	m.threaded = false
	m.source = msgs
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.list.Select(0)
}
```

**`ClearSearchResults`** (lines 519–538):

```go
func (m *Model) ClearSearchResults() {
	if !m.resultsMode {
		return
	}
	m.resultsMode = false
	m.originByUID = nil
	m.delegate.resultsMode = false
	m.delegate.originByUID = nil
	m.source = m.preResults
	m.threaded = m.preThreaded
	m.preResults = nil
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(m.preCursorUID)
}
```

- [ ] **Step 10: Add `Update` entry point**

Insert after `SetKeyMap` (the new accessor added in Step 4):

```go
// Update is the canonical key-dispatch entry. account.Model
// forwards messages here after handling its own bindings (triage,
// open, fold, visual, search). Returns the updated Model and any
// Cmd; the embedded list does not produce Cmds in messagelist's
// configuration (filtering disabled, no spinner), so the result is
// always nil today — the signature stays Cmd-shaped for forward
// compatibility.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Down):
		m.list.CursorDown()
	case key.Matches(keyMsg, m.keys.Up):
		m.list.CursorUp()
	case key.Matches(keyMsg, m.keys.Top):
		m.list.GoToStart()
	case key.Matches(keyMsg, m.keys.Bottom):
		m.list.GoToEnd()
	}
	return m, nil
}
```

- [ ] **Step 11: Rewrite `View`**

Replace `View` (lines 889–917) and `renderEmpty` (lines 1031–1051) with:

```go
// View renders the visible window. Empty list shows the centered
// placeholder; otherwise the embedded list.Model handles cursor +
// viewport and the rowDelegate paints each row.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.list.Items()) == 0 {
		return m.renderEmpty()
	}
	out := m.list.View()
	lines := strings.Split(out, "\n")
	for len(lines) < m.height {
		lines = append(lines, m.styles.MsgListBg.Width(m.width).Render(""))
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderEmpty() string {
	label := "No messages"
	if m.filter.raw != "" {
		label = "No matches"
	}
	labelLine := m.styles.MsgListBg.Width(m.width).
		Foreground(m.styles.MsgListPlaceholder.GetForeground()).
		Align(lipgloss.Center).
		Render(label)
	blank := m.styles.MsgListBg.Width(m.width).Render("")

	mid := m.height / 2
	lines := make([]string, m.height)
	for i := range lines {
		if i == mid {
			lines[i] = labelLine
		} else {
			lines[i] = blank
		}
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 12: Update `IsNearBottom`**

Replace `IsNearBottom` (lines 772–774) with:

```go
// IsNearBottom reports whether the cursor is within k visible rows
// of the last visible row, used to trigger lazy-load before the
// user runs out of messages.
func (m *Model) IsNearBottom(k int) bool {
	n := len(m.list.Items())
	return n > 0 && m.list.Index() >= n-k
}
```

- [ ] **Step 13: Verify the package compiles**

```bash
go build ./internal/ui/messagelist/ 2>&1 | head -40
```

Expected: clean build.

If `Model.selected` / `Model.offset` references remain, search and remove. If unused-import errors fire, prune.

- [ ] **Step 14: Commit**

```bash
git add internal/ui/messagelist/model.go
git commit -m "$(cat <<'EOF'
Pass 17b: embed bubbles/v2/list, add Update, drop imperative movers

messagelist.Model now embeds list.Model with a *rowDelegate. Cursor
+ viewport + key dispatch live on the list; m.rows still backs
Rows()/ActionTargets thread expansion. Imperative movement methods
(MoveDown/MoveUp/MoveToTop/MoveToBottom/HalfPage*/Page*) are
removed; account.Model gets the Update fall-through in the next
task. MoveCursor survives for the viewer's programmatic n/N path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

(Build will fail in `internal/ui/account` after this commit — Task 5 fixes it. Acceptable mid-pass.)

---

## Task 5: account-side call-site sweep

**Files:**
- Modify: `internal/ui/account/model.go`

- [ ] **Step 1: Replace MsgList* navigation cases with the Update fall-through**

In `internal/ui/account/model.go` `handleKey`, find the block at current lines 407–414:

```go
case key.Matches(msg, m.keys.MsgListBottom):
    m.msglist.MoveToBottom()
case key.Matches(msg, m.keys.MsgListTop):
    m.msglist.MoveToTop()
case key.Matches(msg, m.keys.MsgListDown):
    m.msglist.MoveDown()
case key.Matches(msg, m.keys.MsgListUp):
    m.msglist.MoveUp()
```

Replace with:

```go
case key.Matches(msg, m.keys.MsgListBottom),
	key.Matches(msg, m.keys.MsgListTop),
	key.Matches(msg, m.keys.MsgListDown),
	key.Matches(msg, m.keys.MsgListUp):
	m.msglist, _ = m.msglist.Update(msg)
```

The `account.keys.MsgList*` bindings stay declared in `account/keys.go` for now — 17c's help-popover audit will deduplicate against `messagelist.KeyMap`.

- [ ] **Step 2: Verify the binary compiles**

```bash
go build ./... 2>&1 | head -20
```

Expected: clean build across the workspace.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/account/model.go
git commit -m "$(cat <<'EOF'
Pass 17b: route msglist nav keys through Update

handleKey collapses the four MsgList* cases into a fall-through to
m.msglist.Update(msg). The bindings stay on account.keys; 17c will
deduplicate them against messagelist.KeyMap.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Migrate messagelist tests

**Files:**
- Modify: `internal/ui/messagelist/model_test.go`

- [ ] **Step 1: Audit which tests call removed methods**

```bash
grep -nE 'MoveDown\(|MoveUp\(|MoveToTop\(|MoveToBottom\(|HalfPageDown\(|HalfPageUp\(|PageDown\(|PageUp\(|m\.selected|m\.offset|clampOffset' internal/ui/messagelist/model_test.go
```

Note every match — those lines need conversion in Step 2.

- [ ] **Step 2: Confirm `keyPress` is reachable**

`keyPress` lives in `internal/ui/messagelist/keys_test.go` (Task 1). Both files are in the same package, so `model_test.go` can call it directly — no duplicate, no extra import.

- [ ] **Step 3: Rewrite movement-method calls**

For each grep hit from Step 1, convert as follows:

- `m.MoveDown()` → `m, _ = m.Update(keyPress("j"))`
- `m.MoveUp()` → `m, _ = m.Update(keyPress("k"))`
- `m.MoveToTop()` → `m, _ = m.Update(keyPress("g"))`
- `m.MoveToBottom()` → `m, _ = m.Update(keyPress("G"))`
- `m.HalfPageDown()` / `m.HalfPageUp()` / `m.PageDown()` / `m.PageUp()` → use `MoveCursor(delta)`:
  - `m.HalfPageDown()` → `m.MoveCursor(max(1, height/2))` (test must know `height`)
  - `m.PageDown()` → `m.MoveCursor(max(1, height))`
  - Mirror for the Up variants with negative deltas.
- Direct field reads `m.selected` → `m.Selected()` accessor (already exists post-Task 4).
- Direct field reads `m.offset` → there is no equivalent; offset is internal to `list.Model`. If any test asserts on `m.offset`, replace with a behavioral assertion (e.g., "the visible window starts at row N" via checking `m.View()` slice).

If a test sets `m.now` or other private fields directly, leave that alone — they are legal in-package access. But `m.selected` / `m.offset` no longer exist, so those must be removed.

- [ ] **Step 4: Add fold-cursor preservation test**

Add to `model_test.go` (anywhere among the fold-section tests):

```go
func TestToggleFold_PreservesCursorAcrossSetItems(t *testing.T) {
	// Two threads. Cursor lands on a child of the first thread; folding
	// the second thread must keep the cursor on the same child UID.
	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "t1", SentAt: time.Unix(100, 0)},
		{UID: "2", ThreadID: "t1", InReplyTo: "1", SentAt: time.Unix(200, 0)},
		{UID: "3", ThreadID: "t2", SentAt: time.Unix(300, 0)},
		{UID: "4", ThreadID: "t2", InReplyTo: "3", SentAt: time.Unix(400, 0)},
	}
	m := New(NewStyles(theme.Nord), msgs, 80, 10, uicore.SimpleIcons)

	// Move cursor to UID "2" (child of thread t1).
	m, _ = m.Update(keyPress("j")) // sort: t2 newest first → row 0=t2 root, row 1=child 4, row 2=t1 root, row 3=child 2.
	m, _ = m.Update(keyPress("j"))
	m, _ = m.Update(keyPress("j"))
	want := mail.UID("2")
	if got, _ := m.SelectedMessage(); got.UID != want {
		t.Fatalf("setup: cursor on %q, want %q", got.UID, want)
	}

	// Fold thread t2 (the other thread). Cursor must stay on UID "2".
	prevSelected := m.Selected()
	// Move to t2 root, fold via ToggleFold (account-level action; call
	// directly on the model since this test exercises the model API).
	for i := 0; i < prevSelected; i++ {
		m, _ = m.Update(keyPress("k"))
	}
	// Find t2 root and toggle fold.
	rows := m.Rows()
	for i, r := range rows {
		if r.IsThreadRoot && r.Msg.UID == "3" {
			// Move cursor to row i, then fold.
			delta := i - m.Selected()
			if delta != 0 {
				m.MoveCursor(delta)
			}
			m.ToggleFold()
			break
		}
	}

	// Cursor should be back on UID "2".
	got, _ := m.SelectedMessage()
	if got.UID != "2" {
		t.Errorf("after fold, cursor on %q, want %q", got.UID, "2")
	}
}
```

The existing tests use `NewStyles(theme.Nord)` for the styles seed and `mockMessages()` for the corpus — both are already in scope in `model_test.go`. The new test reuses `NewStyles(theme.Nord)` directly.

- [ ] **Step 5: Run the full suite**

```bash
go test ./internal/ui/messagelist/ 2>&1 | tail -30
```

Expected: PASS. If a render-equality test fails by a few cells, inspect — `list.View()`'s output may include trailing newlines or padding shapes the previous hand-rolled `View()` didn't. Fix by trimming `lines` precisely in `View()` (Task 4 Step 11) or by adjusting the test's expected string. Do not adjust both.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/messagelist/model_test.go
git commit -m "$(cat <<'EOF'
Pass 17b: migrate messagelist tests to Update + KeyMap

Move-method calls become Update(keyPress(...)); m.selected reads
become m.Selected(); m.offset reads become behavioral assertions
on View() output. New TestToggleFold_PreservesCursorAcrossSetItems
covers the syncList → snapToUIDInList path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Full-suite verification

- [ ] **Step 1: `make check`**

```bash
cd /home/glw907/Projects/poplar
make check 2>&1 | tail -40
```

Expected: PASS — fmt-check, vet, voice, modern-go-check, test all green.

- [ ] **Step 2: Fix any failures inline**

If `voice-check.sh` flags T-numbers in newly added comments, rewrite those comments. If `modern-go-check.sh` flags pre-1.21 idioms, refactor. If a vet error fires, fix the offending site. Do not skip.

- [ ] **Step 3: Commit any fixes**

```bash
git add -u
git commit -m "$(cat <<'EOF'
Pass 17b: fix voice/vet findings from make check

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

(Skip if no fixes needed.)

---

## Task 8: Live tmux verification

Reference: `.claude/docs/tmux-testing.md`.

- [ ] **Step 1: Install the binary**

```bash
make install 2>&1 | tail -5
```

Expected: `~/.local/bin/poplar` updated.

- [ ] **Step 2: Capture 80×24 (Spartan tier)**

```bash
TMUX_SESS="poplar-17b-spartan"
tmux kill-session -t "$TMUX_SESS" 2>/dev/null
tmux new-session -d -s "$TMUX_SESS" -x 80 -y 24
tmux send-keys -t "$TMUX_SESS" "FASTMAIL_API_TOKEN=$FASTMAIL_API_TOKEN poplar" Enter
sleep 4
tmux capture-pane -t "$TMUX_SESS" -p > /tmp/poplar-17b-spartan.txt
cat /tmp/poplar-17b-spartan.txt
```

Verify visually:
- Sender column at 22 cells, no flag glyphs, no date column.
- Cursor `▐` visible on the selected row.
- Thread prefixes render (open a folder with threads).
- No layout artifacts (extra newlines, truncated rows).

Drive `j`/`k`/`g`/`G` and confirm cursor moves accordingly.

```bash
tmux send-keys -t "$TMUX_SESS" "j" && sleep 1 && tmux capture-pane -t "$TMUX_SESS" -p | head -3
tmux send-keys -t "$TMUX_SESS" "G" && sleep 1 && tmux capture-pane -t "$TMUX_SESS" -p | head -3
tmux send-keys -t "$TMUX_SESS" "g" && sleep 1 && tmux capture-pane -t "$TMUX_SESS" -p | head -3
```

- [ ] **Step 3: Capture 120×40 (Full tier)**

```bash
TMUX_SESS="poplar-17b-full"
tmux kill-session -t "$TMUX_SESS" 2>/dev/null
tmux new-session -d -s "$TMUX_SESS" -x 120 -y 40
tmux send-keys -t "$TMUX_SESS" "FASTMAIL_API_TOKEN=$FASTMAIL_API_TOKEN poplar" Enter
sleep 4
tmux capture-pane -t "$TMUX_SESS" -p > /tmp/poplar-17b-full.txt
cat /tmp/poplar-17b-full.txt
```

Verify:
- Flag column rendering (SPUA-A glyphs at 2 cells), date column, sender column at 22+.
- Selection background spans the full row width.
- `Space` toggles fold; cursor stays sensible.
- `/` opens search shelf; type a few chars; `Esc` clears.

```bash
tmux send-keys -t "$TMUX_SESS" "Space" && sleep 1 && tmux capture-pane -t "$TMUX_SESS" -p | head -10
tmux send-keys -t "$TMUX_SESS" "Space" && sleep 1
tmux send-keys -t "$TMUX_SESS" "/" && sleep 1
tmux send-keys -t "$TMUX_SESS" "test" && sleep 1 && tmux capture-pane -t "$TMUX_SESS" -p | head -10
tmux send-keys -t "$TMUX_SESS" "Escape" && sleep 1
```

- [ ] **Step 4: Open a message and exercise viewer n/N**

```bash
tmux send-keys -t "$TMUX_SESS" "Enter" && sleep 2 && tmux capture-pane -t "$TMUX_SESS" -p | head -20
tmux send-keys -t "$TMUX_SESS" "n" && sleep 2 && tmux capture-pane -t "$TMUX_SESS" -p | head -3
tmux send-keys -t "$TMUX_SESS" "N" && sleep 2
tmux send-keys -t "$TMUX_SESS" "q" && sleep 1
```

Confirm: viewer opens, `n`/`N` advance/retreat to siblings, `q` closes, cursor stays on the most recently opened message.

- [ ] **Step 5: Tear down sessions**

```bash
tmux kill-session -t "poplar-17b-spartan" 2>/dev/null
tmux kill-session -t "poplar-17b-full" 2>/dev/null
```

- [ ] **Step 6: §10 review checklist self-audit**

Walk through each box in `docs/poplar/bubbletea-conventions.md` §10 (the list also lives in this pass's spec under "Pass-end deliverables"). Document outcomes inline as a comment block in the upcoming ADR (next task) or in the commit message — whatever the conventions doc requires.

If any box fails, fix before continuing.

---

## Task 9: ADR + invariants

**Files:**
- Create: `docs/poplar/decisions/0199-pass-17b-messagelist-on-bubbles-list.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `.claude/rules/ui-invariants.md`

- [ ] **Step 1: Confirm ADR number**

```bash
ls docs/poplar/decisions/ | grep -E '^[0-9]{4}' | sort | tail -3
```

If the most recent ADR is `0198-…`, the new ADR is `0199`. Adjust if higher.

- [ ] **Step 2: Write ADR-0199**

Create `docs/poplar/decisions/0199-pass-17b-messagelist-on-bubbles-list.md`:

```markdown
---
title: Pass 17b — messagelist on bubbles/v2/list with custom item delegate
status: accepted
date: 2026-05-10
---

## Context

Pass 17a moved the sidebar onto a `bubbles/v2/tree` component
(ADR-0198). 17b is the second of the bubbles-adoption remainder
before Polish II and the v0.9.0 freeze: `messagelist.Model` was
hand-rolling cursor, viewport, and key dispatch even though
`bubbles/v2/list` provides them. The 16-series (ADR-0196) locked
the modern-Go conventions that let the `iter.Seq2` thread walk
land in this pass instead of as a follow-up.

## Decision

`messagelist.Model` embeds `bubbles/v2/list.Model` with a custom
`*rowDelegate` (held by pointer so per-frame context refreshes
mutate in place). The list owns cursor, viewport, and navigation
key dispatch via the exported `messagelist.KeyMap` (Down/Up/Top/
Bottom). The package's imperative movement methods
(MoveDown/MoveUp/MoveToTop/MoveToBottom/HalfPage*/Page*) are
removed; `account.Model.handleKey` falls through into
`m.msglist.Update(msg)` for those bindings. `MoveCursor(delta)`
survives — the viewer's programmatic `n`/`N` path needs the
`(UID, moved)` return.

Hidden rows (folded thread children) stay in `m.rows` for tests,
`Rows()`, `ActionTargets` thread-expansion, and `threadRootIndex`
lookups. Only the visible subset reaches `list.SetItems` via
`syncList`. Fold toggles re-rebuild and `snapToUIDInList`
re-anchors the cursor on the same UID.

`appendThreadRows`'s manual prefix walker becomes
`walkThread(root) iter.Seq2[*threadNode, walkStep]`. Closes
BACKLOG #46.

## Consequences

The `*rowDelegate` pointer is an Elm-immutable-model escape
hatch. ADR-0130 codified the pattern for overlay caches
(`*helppopoverCache`, `*movepickerCache`, `*conflictCache`); this
ADR extends the carveout to per-frame render context for
`bubbles/v2/list` delegates. The constraint stays: the pointer
holds context only, not memoized output; mutations route through
`messagelist.Model` setters, never from `View()` or a `tea.Cmd`
closure.

Pass 17c can audit `bubbles/v2/help` against the new exported
`messagelist.KeyMap` and the existing `sidebar.KeyMap` /
`account.keys` to wire help-popover rows directly off the
binding sources, deduplicating the help-vocabulary table.
`account.keys.MsgList*` bindings remain in 17b for the
fall-through; 17c removes them.

The list-style configuration disables every chrome surface
(filter, status bar, pagination, help, title) — none compose
with poplar's account-pane chrome and most are forbidden by
ui-invariants (no pgup/pgdown, modifier-free, sidebar-owned
search per ADR-0188).

The §10 review checklist passed at 80×24 and 120×40.
```

- [ ] **Step 3: Update `docs/poplar/decisions/INDEX.md`**

Add a row under the appropriate themed heading (likely "UI / bubbletea adoption" — match what 0198 did). Format mirrors the existing rows. Read the file first to copy the exact column layout:

```bash
grep -nE '^- \[ADR-019[0-9]\]' docs/poplar/decisions/INDEX.md
```

Add a row immediately after the ADR-0198 entry, e.g.:

```markdown
- [ADR-0199](0199-pass-17b-messagelist-on-bubbles-list.md) — messagelist on bubbles/v2/list with custom item delegate; iter.Seq2 thread walk
```

- [ ] **Step 4: Update `.claude/rules/ui-invariants.md`**

Find the "### Message list" section (currently starts with "`messagelist.Model` owns thread grouping + fold state. Holds…"). Replace its first paragraph with:

```markdown
- `messagelist.Model` owns thread grouping + fold state. Embeds
  `bubbles/v2/list.Model` with a custom `*rowDelegate` for per-row
  rendering (ADR-0199). Holds `source []MessageInfo` plus derived
  `rows []displayRow` rebuilt by a group→sort→flatten pipeline; the
  visible subset of `rows` is materialized into `list.SetItems` via
  `syncList`. Hidden rows (folded thread children) stay in `m.rows`
  for `Rows()`, `ActionTargets` thread expansion, and
  `threadRootIndex`. A transient `*threadNode` tree is built per
  bucket and walked via `walkThread` (an `iter.Seq2` pull
  iterator) to compute box-drawing prefixes, then discarded — the
  renderer never sees the tree.
- `messagelist.KeyMap` (Down/Up/Top/Bottom) is the dispatch surface
  consumed by `Model.Update`. `account.Model.handleKey` falls
  through into `Update` for nav keys; fold (`space`/`F`),
  visual-mode (`v`), triage (`d`/`a`/`s`/`r`), open (`Enter`),
  search (`/`), and folder-jump keys keep account-level guards and
  call mutator methods on `messagelist.Model` directly.
  `MoveCursor(delta) (UID, bool)` survives as the viewer's
  programmatic `n`/`N` entry point.
```

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/decisions/0199-pass-17b-messagelist-on-bubbles-list.md docs/poplar/decisions/INDEX.md .claude/rules/ui-invariants.md
git commit -m "$(cat <<'EOF'
Pass 17b: ADR-0199, ui-invariants update

ADR-0199 records the messagelist→bubbles/v2/list migration, the
ADR-0130 scope extension for *rowDelegate, and the iter.Seq2 walk
closure. ui-invariants.md drops the "Hand-rolled" framing and
documents the new dispatch surface.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: STATUS + archive

**Files:**
- Modify: `docs/poplar/STATUS.md`
- Move: `docs/superpowers/specs/2026-05-10-messagelist-list-design.md`
  → `docs/superpowers/archive/specs/`
- Move: `docs/superpowers/plans/2026-05-10-messagelist-list.md`
  → `docs/superpowers/archive/plans/`

- [ ] **Step 1: Update STATUS.md**

Replace the file's contents with:

```markdown
# Poplar Status

**Current pass:** Pass 17c — `bubbles/v2/help` audit + ADRs for
bubbles deviations that survived 15a/17a/17b. Third of the
bubbles-adoption remainder; closes the migration arc before
Polish II.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` with custom item delegate; iter.Seq2 thread walk (ADR-0199) | done |
| **17c** | **`bubbles/v2/help` audit + bubbles-deviation ADRs** | **pending — next** |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 17c)

> **Goal.** Audit poplar's help-popover wiring against
> `bubbles/v2/help`, adopt where it composes, and ADR every
> deviation that survives. Closes the bubbles-adoption arc
> (15a/17a/17b done; 17c is the wrap).
>
> **Scope.** `internal/ui/helppopover/`, the help-vocabulary
> table that backs it, and the now-duplicate `account.keys.MsgList*`
> bindings (deduplicate against `messagelist.KeyMap` per ADR-0199).
> Audit `bubbles/v2/help.Model` against the wireframe at
> `docs/poplar/wireframes.md` and confirm or write deviation ADRs
> for: (a) "advertise unwired bindings dimmed" (custom rendering
> per ADR-0072), (b) any binding-source composition `bubbles/v2/help`
> doesn't natively support.
>
> **Settled (do not re-brainstorm):** ADR-0072's wired/unwired
> distinction stays. `messagelist.KeyMap` and `sidebar.KeyMap` are
> the canonical binding sources for those panels; `account.keys`
> remains the canonical source for cross-panel actions.
>
> **Still open — brainstorm these:** how `bubbles/v2/help` composes
> (or doesn't) multiple `key.KeyMap` sources; whether the
> account.keys.MsgList* deduplication ships in 17c or as a follow-up;
> whether the help-popover's rounded-border deviation (ModalShell
> non-consumer) gets an ADR or stays implicit.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-help-bubbles-audit.md`, then
> implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
```

Verify the file is ≤60 lines:

```bash
wc -l docs/poplar/STATUS.md
```

Trim if over.

- [ ] **Step 2: Archive spec + plan**

```bash
git mv docs/superpowers/specs/2026-05-10-messagelist-list-design.md docs/superpowers/archive/specs/
git mv docs/superpowers/plans/2026-05-10-messagelist-list.md docs/superpowers/archive/plans/
```

- [ ] **Step 3: Verify worktree state**

```bash
git status
```

Expected: STATUS.md modified, two `renamed:` entries for the spec/plan move.

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass 17b close: STATUS to 17c, archive spec + plan

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Final ship

- [ ] **Step 1: `make check` once more**

```bash
make check 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 2: Push**

```bash
git push 2>&1 | tail -5
```

Expected: pushed to `origin/master`.

- [ ] **Step 3: `make install`**

```bash
make install 2>&1 | tail -5
```

Expected: `~/.local/bin/poplar` updated.

- [ ] **Step 4: Smoke test**

```bash
poplar --version 2>&1 || poplar --help 2>&1 | head -3
```

Expected: binary executes.

Pass 17b is shipped.
