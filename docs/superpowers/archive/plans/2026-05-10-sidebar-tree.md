# Sidebar Tree Implementation Plan (Pass 17a)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render Custom folders with `/`-paths as a real tree in the sidebar — expand/collapse via `→`/`←`, collapsed parents sum descendant unread, Spartan tier caps indent at depth 1. Also closes #45 item (4): add `sidebar.KeyMap` + `Update` and drop the imperative `MoveUp`/`MoveDown`/`MoveToTop`/`MoveToBottom` mutators.

**Architecture:** Hand-rolled tree on the existing `sidebar.Model.renderRow`. A transient `*node` map is built from `folderEntry`s during view, DFS-walked honoring an `expanded map[string]bool`, emits `[]rowMeta{entry, depth, isLast, ancestorIsLast, aggUnread}`, then discarded — pattern mirrors `messagelist.appendThreadRows`. `renderRow` prepends a `│  ` / `   ` / `├─ ` / `└─ ` prefix in a new `SidebarTreeRule` style. Spartan caps `maxDepth = 1`; full tiers unbounded.

**Tech Stack:** Go 1.26, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2/key`, `internal/ansix`. No new dependencies.

**Spec:** [`docs/superpowers/specs/2026-05-10-sidebar-tree-design.md`](../specs/2026-05-10-sidebar-tree-design.md)

---

## File Structure

| File | Role |
|---|---|
| `internal/ui/sidebar/tree.go` | **New.** `rowMeta`, transient `node`, `walkCustom`. Pure builder. |
| `internal/ui/sidebar/tree_test.go` | **New.** Walk shapes, depth cap, aggregation. |
| `internal/ui/sidebar/model.go` | `expanded` map field, `IsExpanded`/`ToggleExpanded`/`pruneExpanded`, `renderRow` prefix arg, `KeyMap`, `Update`. Remove imperative movement methods. |
| `internal/ui/sidebar/model_test.go` | Expand-state + Update routing tests; existing rendering tests updated for prefix column. |
| `internal/ui/sidebar/styles.go` | Add `SidebarTreeRule` style (`FgDim`). |
| `internal/theme/palette.go` | Wire `SidebarTreeRule` into the compiled-theme `Styles` projection (`internal/ui/styles.go`). |
| `internal/ui/styles.go` | Add `SidebarTreeRule lipgloss.Style` field to the parent `Styles` struct so `NewStyles` can read it. |
| `internal/ui/account/model.go` | Replace `sb.MoveUp()` / `sb.MoveDown()` calls with `sidebar.Model.Update`. |
| `internal/ui/uicore/layout.go` | Read-only — confirm `LayoutMode.Spartan` (or equivalent) is already exposed; if not, surface a `MaxSidebarDepth() int` accessor. |
| `docs/poplar/styling.md` | Document `SidebarTreeRule` in the Sidebar surface row. |
| `docs/poplar/decisions/0197-sidebar-tree.md` | **New ADR.** Tree-on-Custom decision, key bindings, depth cap. |
| `.claude/rules/ui-invariants.md` | Replace "Nested folder names ... render flat" with the tree invariant. |
| `docs/poplar/STATUS.md` | Mark 17a done, draft 17b starter prompt. |
| `BACKLOG.md` | Mark #45 item (4) struck-through. |

---

## Pre-flight

- [ ] **Step 0: Confirm clean working tree**

```bash
git status
```

Expected: `nothing to commit, working tree clean` on `master`. If anything is staged, stop and resolve before starting.

- [ ] **Step 0.1: Inspect `LayoutMode` for a depth signal**

Read `internal/ui/uicore/layout.go`. Note whether `LayoutMode` exposes a Spartan/Intermediate/Full flag directly or only via derived fields (icons / flags / date width). The plan assumes a `Spartan bool` field exists; if it doesn't, Task 6 surfaces it. Don't change anything yet.

```bash
grep -n "Spartan\|tier\|Tier" internal/ui/uicore/layout.go
```

---

## Task 1: Tree builder skeleton (TDD)

**Files:**
- Create: `internal/ui/sidebar/tree.go`
- Test: `internal/ui/sidebar/tree_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/sidebar/tree_test.go`:

```go
package sidebar

import (
	"reflect"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func customEntry(name string, unseen int) folderEntry {
	return folderEntry{
		cf: mail.ClassifiedFolder{
			Folder:      mail.Folder{Name: name, Unseen: unseen},
			DisplayName: name,
			Group:       mail.GroupCustom,
		},
	}
}

func TestWalkCustom_FlatLeaves(t *testing.T) {
	in := []folderEntry{customEntry("Lists", 0), customEntry("Receipts", 2)}
	got := walkCustom(in, nil, 0)
	want := []rowMeta{
		{entry: in[0], depth: 0, isLast: false, aggUnread: 0, hasChildren: false},
		{entry: in[1], depth: 0, isLast: true, aggUnread: 2, hasChildren: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestWalkCustom_NestedExpanded(t *testing.T) {
	in := []folderEntry{
		customEntry("Lists/golang", 3),
		customEntry("Lists/rust", 1),
		customEntry("Receipts", 0),
	}
	expanded := map[string]bool{"Lists": true}
	got := walkCustom(in, expanded, 0)
	if len(got) != 4 {
		t.Fatalf("want 4 rows (Lists parent + 2 children + Receipts), got %d", len(got))
	}
	if got[0].entry.cf.DisplayName != "Lists" || got[0].depth != 0 || !got[0].hasChildren {
		t.Errorf("row 0: want synthesized parent Lists at depth 0 with children, got %+v", got[0])
	}
	if got[1].depth != 1 || got[1].entry.cf.Folder.Name != "Lists/golang" || got[1].isLast {
		t.Errorf("row 1: want golang at depth 1 non-last, got %+v", got[1])
	}
	if got[2].depth != 1 || got[2].entry.cf.Folder.Name != "Lists/rust" || !got[2].isLast {
		t.Errorf("row 2: want rust at depth 1 last, got %+v", got[2])
	}
	if got[3].depth != 0 || got[3].entry.cf.Folder.Name != "Receipts" || !got[3].isLast {
		t.Errorf("row 3: want Receipts at depth 0 last, got %+v", got[3])
	}
}

func TestWalkCustom_CollapsedAggregatesUnread(t *testing.T) {
	in := []folderEntry{
		customEntry("Lists/golang", 3),
		customEntry("Lists/rust", 1),
	}
	got := walkCustom(in, nil, 0) // Lists collapsed (no entry in expanded map)
	if len(got) != 1 {
		t.Fatalf("want 1 row (collapsed Lists), got %d", len(got))
	}
	if got[0].aggUnread != 4 {
		t.Errorf("collapsed Lists: want aggUnread=4, got %d", got[0].aggUnread)
	}
	if !got[0].hasChildren {
		t.Errorf("collapsed Lists: want hasChildren=true")
	}
}

func TestWalkCustom_MaxDepthCap(t *testing.T) {
	in := []folderEntry{
		customEntry("a/b/c/leaf", 5),
		customEntry("a/b/sibling", 1),
	}
	expanded := map[string]bool{"a": true, "a/b": true, "a/b/c": true}
	got := walkCustom(in, expanded, 1) // cap at depth 1
	for _, r := range got {
		if r.depth > 1 {
			t.Errorf("row %+v exceeds maxDepth=1", r)
		}
	}
	// Total unseen (5+1=6) must still be reachable on the deepest visible ancestor.
	var total int
	for _, r := range got {
		total += r.entry.cf.Folder.Unseen + r.aggUnread
	}
	if total < 6 {
		t.Errorf("want total >=6 reachable across visible rows, got %d", total)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/ui/sidebar/ -run TestWalkCustom -v
```

Expected: `undefined: rowMeta` / `undefined: walkCustom`. Compilation error.

- [ ] **Step 3: Implement `tree.go`**

Create `internal/ui/sidebar/tree.go`:

```go
package sidebar

import (
	"sort"
	"strings"
)

// rowMeta describes one visible sidebar row. Built by walkCustom from
// the transient *node tree, then consumed by renderRow.
type rowMeta struct {
	entry          folderEntry
	depth          int    // 0 for top-level Custom; ≥1 for nested
	isLast         bool   // last visible sibling at this depth
	ancestorIsLast []bool // for i in [0, depth), was ancestor at depth i a last child?
	aggUnread      int    // sum of descendant unseen when collapsed; 0 when expanded
	hasChildren    bool   // any descendants (visible or not)
	syntheticPath  string // path used as the expand-state key; provider name for real entries, parent-prefix for synthesized rollups
}

// expandKey returns the key under which this row's expand state is stored.
func (r rowMeta) expandKey() string {
	if r.syntheticPath != "" {
		return r.syntheticPath
	}
	return r.entry.cf.Folder.Name
}

// node is the transient tree. Built per walkCustom call, never escapes.
type node struct {
	name     string         // last path segment (display)
	path     string         // full provider path
	entry    *folderEntry   // nil iff this is a synthesized intermediate
	children map[string]*node
	order    []string       // child names in stable insertion order
}

func newNode(name, path string) *node {
	return &node{name: name, path: path, children: map[string]*node{}}
}

// walkCustom builds the tree from custom entries, walks it honoring
// expanded, applies maxDepth (0 = unbounded), and returns visible rows.
func walkCustom(custom []folderEntry, expanded map[string]bool, maxDepth int) []rowMeta {
	root := newNode("", "")
	for i := range custom {
		insertPath(root, &custom[i])
	}
	stableChildren(root)

	var out []rowMeta
	for i, childName := range root.order {
		isLast := i == len(root.order)-1
		visit(root.children[childName], 0, isLast, nil, expanded, maxDepth, &out)
	}
	return out
}

func insertPath(root *node, e *folderEntry) {
	segments := strings.Split(e.cf.Folder.Name, "/")
	cur := root
	for i, seg := range segments {
		next, ok := cur.children[seg]
		if !ok {
			path := strings.Join(segments[:i+1], "/")
			next = newNode(seg, path)
			cur.children[seg] = next
			cur.order = append(cur.order, seg)
		}
		cur = next
	}
	cur.entry = e
}

func stableChildren(n *node) {
	sort.Strings(n.order)
	for _, name := range n.order {
		stableChildren(n.children[name])
	}
}

// visit emits rows for n and its descendants. Returns the total unseen
// reachable through n (own + all descendants).
func visit(n *node, depth int, isLast bool, ancestorIsLast []bool, expanded map[string]bool, maxDepth int, out *[]rowMeta) int {
	// At the depth cap, fold this node and its subtree into a single row.
	atCap := maxDepth > 0 && depth >= maxDepth

	// Compute aggregate unseen across descendants regardless of expansion.
	var descUnseen int
	for _, name := range n.order {
		descUnseen += subtreeUnseen(n.children[name])
	}

	entry := materializeEntry(n)
	hasKids := len(n.order) > 0
	path := n.path
	isExpanded := expanded[path]

	// Cap forces a collapsed-rollup view; node displays own + descendant aggregate.
	if atCap {
		isExpanded = false
	}

	row := rowMeta{
		entry:          entry,
		depth:          depth,
		isLast:         isLast,
		ancestorIsLast: append([]bool{}, ancestorIsLast...),
		hasChildren:    hasKids,
		syntheticPath:  syntheticPath(n),
	}
	if hasKids && !isExpanded {
		row.aggUnread = descUnseen
	}
	*out = append(*out, row)

	if hasKids && isExpanded {
		childAncestors := append(append([]bool{}, ancestorIsLast...), isLast)
		for i, name := range n.order {
			childIsLast := i == len(n.order)-1
			visit(n.children[name], depth+1, childIsLast, childAncestors, expanded, maxDepth, out)
		}
	}
	own := 0
	if n.entry != nil {
		own = n.entry.cf.Folder.Unseen
	}
	return own + descUnseen
}

func subtreeUnseen(n *node) int {
	total := 0
	if n.entry != nil {
		total += n.entry.cf.Folder.Unseen
	}
	for _, name := range n.order {
		total += subtreeUnseen(n.children[name])
	}
	return total
}

// materializeEntry returns the folderEntry to render for n. Real nodes
// return *n.entry. Synthesized intermediate nodes return a stub entry
// using the segment name for display and the full path for provider name.
func materializeEntry(n *node) folderEntry {
	if n.entry != nil {
		// Real entry — render with the leaf segment as display name.
		e := *n.entry
		e.cf.DisplayName = n.name
		return e
	}
	return folderEntry{
		cf: synthFolder(n.path, n.name),
	}
}

// syntheticPath returns the expand-state key when n is a synthesized
// intermediate; otherwise empty.
func syntheticPath(n *node) string {
	if n.entry == nil {
		return n.path
	}
	return ""
}
```

Also add to `internal/ui/sidebar/model.go` (top of file, after imports):

```go
// synthFolder builds a ClassifiedFolder for a synthesized intermediate
// node — a path segment with no on-server folder of its own (e.g. "Lists"
// when only "Lists/golang" exists as a real folder).
func synthFolder(path, displayName string) mail.ClassifiedFolder {
	return mail.ClassifiedFolder{
		Folder:      mail.Folder{Name: path},
		DisplayName: displayName,
		Group:       mail.GroupCustom,
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/ui/sidebar/ -run TestWalkCustom -v
```

Expected: all four pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sidebar/tree.go internal/ui/sidebar/tree_test.go internal/ui/sidebar/model.go
git commit -m "Pass 17a: sidebar tree builder (rowMeta, walkCustom)"
```

---

## Task 2: Expand-state map on Model

**Files:**
- Modify: `internal/ui/sidebar/model.go`
- Test: `internal/ui/sidebar/model_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/ui/sidebar/model_test.go`:

```go
func TestExpanded_ToggleAndPrune(t *testing.T) {
	m := Model{expanded: map[string]bool{}}
	if m.IsExpanded("a") {
		t.Fatal("new model: nothing expanded")
	}
	m.ToggleExpanded("a")
	if !m.IsExpanded("a") {
		t.Fatal("after toggle: a should be expanded")
	}
	m.ToggleExpanded("a")
	if m.IsExpanded("a") {
		t.Fatal("after second toggle: a should be collapsed")
	}

	m.expanded = map[string]bool{"a": true, "vanished": true, "b/c": true}
	known := map[string]struct{}{"a": {}, "b/c": {}}
	m.pruneExpanded(known)
	if m.expanded["vanished"] {
		t.Errorf("pruneExpanded should drop vanished keys: %+v", m.expanded)
	}
	if !m.expanded["a"] || !m.expanded["b/c"] {
		t.Errorf("pruneExpanded must keep live keys: %+v", m.expanded)
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
go test ./internal/ui/sidebar/ -run TestExpanded -v
```

Expected: `m.IsExpanded undefined`.

- [ ] **Step 3: Add `expanded` field + accessors to `model.go`**

Find the `Model` struct (around line 21) and add:

```go
type Model struct {
	entries     []folderEntry
	selected    int
	outboxCount int
	expanded    map[string]bool // path -> expanded; missing means collapsed
	styles      Styles
	icons       uicore.IconSet
	layout      uicore.LayoutMode
	width       int
	height      int
}
```

Update `New` (around line 34) to initialize the map:

```go
func New(styles Styles, classified []mail.ClassifiedFolder, uiCfg config.UIConfig, width, height int, icons uicore.IconSet) Model {
	return Model{
		entries:  buildEntries(classified, uiCfg, icons),
		selected: 0,
		expanded: map[string]bool{},
		styles:   styles,
		icons:    icons,
		width:    width,
		height:   height,
	}
}
```

Add three methods near the other accessors:

```go
// IsExpanded reports whether the parent at path is expanded.
func (s Model) IsExpanded(path string) bool {
	return s.expanded[path]
}

// ToggleExpanded flips expansion at path.
func (s *Model) ToggleExpanded(path string) {
	if s.expanded == nil {
		s.expanded = map[string]bool{}
	}
	if s.expanded[path] {
		delete(s.expanded, path)
		return
	}
	s.expanded[path] = true
}

// pruneExpanded drops paths no longer present in known.
func (s *Model) pruneExpanded(known map[string]struct{}) {
	for k := range s.expanded {
		if _, ok := known[k]; !ok {
			delete(s.expanded, k)
		}
	}
}
```

Update `SetFolders` (around line 47) — after rebuilding entries, gather live custom paths (including intermediate synthesized parents) and prune:

```go
func (s *Model) SetFolders(classified []mail.ClassifiedFolder, uiCfg config.UIConfig) {
	var prevName string
	if s.selected < len(s.entries) {
		prevName = s.entries[s.selected].cf.Folder.Name
	}
	s.entries = buildEntries(classified, uiCfg, s.icons)
	s.selected = 0
	if prevName != "" {
		for i, e := range s.entries {
			if e.cf.Folder.Name == prevName {
				s.selected = i
				break
			}
		}
	}
	s.pruneExpanded(customPaths(s.entries))
}

// customPaths returns every full path that could appear as an expand-state
// key — real Custom folder names plus every synthesized intermediate.
func customPaths(entries []folderEntry) map[string]struct{} {
	out := map[string]struct{}{}
	for _, e := range entries {
		if e.cf.Group != mail.GroupCustom {
			continue
		}
		segs := strings.Split(e.cf.Folder.Name, "/")
		for i := range segs {
			out[strings.Join(segs[:i+1], "/")] = struct{}{}
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/ui/sidebar/ -v
```

Expected: existing tests still pass; new `TestExpanded_ToggleAndPrune` passes.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sidebar/model.go internal/ui/sidebar/model_test.go
git commit -m "Pass 17a: sidebar expand-state map with prune-on-reload"
```

---

## Task 3: Wire tree rows into View pipeline

**Files:**
- Modify: `internal/ui/sidebar/model.go`
- Test: `internal/ui/sidebar/model_test.go`

- [ ] **Step 1: Write failing test**

Append to `model_test.go`:

```go
func TestView_TreeCollapsedShowsAggregateBadge(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox", Unseen: 0}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Lists/golang", Unseen: 3}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust", Unseen: 1}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	view := m.View()
	if !strings.Contains(view, "Lists") {
		t.Fatalf("collapsed parent Lists must appear:\n%s", view)
	}
	if strings.Contains(view, "golang") || strings.Contains(view, "rust") {
		t.Errorf("collapsed parent must hide children:\n%s", view)
	}
	if !strings.Contains(view, "4") {
		t.Errorf("collapsed Lists must show aggregate unread (3+1=4):\n%s", view)
	}
}

func TestView_TreeExpandedShowsChildrenWithPrefix(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Lists/golang", Unseen: 3}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust", Unseen: 1}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	m.ToggleExpanded("Lists")
	view := m.View()
	if !strings.Contains(view, "golang") || !strings.Contains(view, "rust") {
		t.Fatalf("expanded: children must render:\n%s", view)
	}
	if !strings.Contains(view, "├") || !strings.Contains(view, "└") {
		t.Errorf("expanded: want box-drawing prefixes ├ and └:\n%s", view)
	}
}
```

- [ ] **Step 2: Run, expect failure**

```bash
go test ./internal/ui/sidebar/ -run TestView_Tree -v
```

Expected: tests fail because today's `View` renders flat `Lists/golang` / `Lists/rust`.

- [ ] **Step 3: Replace `effectiveEntries` with a `visibleRows` builder**

In `model.go`, replace the existing `effectiveEntries` method and update its callers to walk `[]rowMeta` instead of `[]folderEntry`.

Add:

```go
// visibleRows builds the ordered row list for View and selection: Primary,
// Disposal (with synthetic Outbox if non-zero), then Custom tree-walked.
func (s Model) visibleRows() []rowMeta {
	var primary, disposal, custom []folderEntry
	for _, e := range s.entries {
		switch e.cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, e)
		case mail.GroupDisposal:
			disposal = append(disposal, e)
		default:
			custom = append(custom, e)
		}
	}
	if s.outboxCount > 0 {
		disposal = append([]folderEntry{syntheticOutboxEntry(s.outboxCount, s.icons)}, disposal...)
	}
	maxDepth := 0
	if s.layout.Spartan {
		maxDepth = 1
	}
	customRows := walkCustom(custom, s.expanded, maxDepth)

	out := make([]rowMeta, 0, len(primary)+len(disposal)+len(customRows))
	for _, e := range primary {
		out = append(out, rowMeta{entry: e})
	}
	for _, e := range disposal {
		out = append(out, rowMeta{entry: e})
	}
	out = append(out, customRows...)
	return out
}
```

Update every reader of `effectiveEntries()` — these are the methods that today take an `entry := entries[s.selected]`. Replace the call with `s.visibleRows()` and read `.entry` off the resulting `rowMeta`. The methods to update: `SelectedFolder`, `SelectedCanonical`, `SelectedFolderInfo`, `ConfigKey`, `FolderNameByCanonical`, `FolderByProviderName`, `OrderedFolders`, `SelectByCanonical`, `SelectedIcon`, `MoveDown` (until Task 6 replaces it), `MoveToBottom`.

Mechanical example for `SelectedFolder`:

```go
func (s Model) SelectedFolder() string {
	rows := s.visibleRows()
	if s.selected < len(rows) {
		return rows[s.selected].entry.cf.Folder.Name
	}
	return ""
}
```

Delete `effectiveEntries` once nothing references it.

- [ ] **Step 4: Update `View`**

Replace the existing `View` method's body so it iterates `visibleRows()` and passes the `rowMeta` (not just `folderEntry`) into `renderRow`. The group-separator blank line still fires when `prev.entry.cf.Group != cur.entry.cf.Group`. Keep blank-padding-to-height and trim-to-height logic intact.

```go
func (s Model) View() string {
	rows := s.visibleRows()
	if len(rows) == 0 || s.width == 0 || s.height == 0 {
		return ""
	}

	plainBg := s.styles.SidebarBg
	selectedBg := s.styles.SidebarSelected

	var lines []string
	prevGroup := rows[0].entry.cf.Group

	for i, r := range rows {
		if i > 0 && r.entry.cf.Group != prevGroup {
			lines = append(lines, s.renderBlankLine())
		}
		prevGroup = r.entry.cf.Group
		bg := plainBg
		if i == s.selected {
			bg = selectedBg
		}
		lines = append(lines, s.renderRow(i, r, bg))
	}

	for len(lines) < s.height {
		lines = append(lines, s.renderBlankLine())
	}
	if len(lines) > s.height {
		lines = lines[:s.height]
	}
	return strings.Join(lines, "\n")
}
```

Update `renderRow`'s signature to accept `rowMeta`. For now, just pull `entry := r.entry` at the top; the prefix logic comes in Task 4. The unread badge must use `r.entry.cf.Folder.Unseen + r.aggUnread` rather than just `.Unseen`:

```go
func (s Model) renderRow(idx int, r rowMeta, bgStyle lipgloss.Style) string {
	entry := r.entry
	unseen := entry.cf.Folder.Unseen + r.aggUnread
	hasUnread := unseen > 0
	// ... existing body, replacing every `entry.cf.Folder.Unseen` with `unseen`.
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/ui/sidebar/ -v
```

Expected: `TestView_TreeCollapsedShowsAggregateBadge` passes; the prefix-presence test still fails (no prefix yet). All existing tests pass — the flat Primary/Disposal cases render the same because `rowMeta{entry: e}` zero-fields produce the same output path.

- [ ] **Step 6: Commit (partial — prefix not yet drawn)**

```bash
git add internal/ui/sidebar/model.go internal/ui/sidebar/model_test.go
git commit -m "Pass 17a: thread rowMeta through View, aggregate unread"
```

---

## Task 4: Render tree prefix in renderRow

**Files:**
- Modify: `internal/ui/sidebar/styles.go`, `internal/ui/styles.go`, `internal/theme/palette.go`, `internal/ui/sidebar/model.go`

- [ ] **Step 1: Add `SidebarTreeRule` to the parent `Styles`**

Find `internal/ui/styles.go` and add a `SidebarTreeRule lipgloss.Style` field alongside the other `Sidebar*` fields.

In `internal/theme/palette.go`, locate where `SidebarFolder` is assigned (search `t.SidebarFolder = `). Right after, add:

```go
t.SidebarTreeRule = lipgloss.NewStyle().Foreground(p.FgDim)
```

- [ ] **Step 2: Project into per-subpackage Styles**

In `internal/ui/sidebar/styles.go`, find the `Styles` struct and `NewStyles` constructor. Add `SidebarTreeRule lipgloss.Style` and copy from the parent:

```go
return Styles{
	// ... existing fields ...
	SidebarTreeRule: parent.SidebarTreeRule,
}
```

- [ ] **Step 3: Failing prefix test**

The Task 3 step "expanded: want box-drawing prefixes ├ and └" should still be failing. Verify:

```bash
go test ./internal/ui/sidebar/ -run TestView_TreeExpandedShowsChildrenWithPrefix -v
```

Expected: FAIL — `want box-drawing prefixes ├ and └`.

- [ ] **Step 4: Compute and prepend the prefix**

Add a helper to `tree.go`:

```go
// prefix renders the box-drawing column for a row. Returns "" for depth 0.
func (r rowMeta) prefix() string {
	if r.depth == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(r.depth * 3)
	for i := 0; i < r.depth-1; i++ {
		if i < len(r.ancestorIsLast) && r.ancestorIsLast[i] {
			b.WriteString("   ")
		} else {
			b.WriteString("│  ")
		}
	}
	if r.isLast {
		b.WriteString("└─ ")
	} else {
		b.WriteString("├─ ")
	}
	return b.String()
}
```

Note: `ancestorIsLast` in `walkCustom` currently appends the *parent's* `isLast` per recursion. The prefix logic needs each *ancestor's* isLast (not including the row's own). Re-read `visit`:

```go
childAncestors := append(append([]bool{}, ancestorIsLast...), isLast)
```

That appends the *current* row's `isLast` before recursing — so for a row at depth `d`, `ancestorIsLast` has length `d` and entry `i` corresponds to the ancestor at depth `i`. Correct for the prefix walk above.

- [ ] **Step 5: Plumb prefix into renderRow**

In `model.go`'s `renderRow`, compute and render the prefix before the icon block. Subtract its width from `labelBudget`:

```go
// After indicator, before icon block:
treePrefix := r.prefix()
treePrefixRendered := ""
treePrefixCells := 0
if treePrefix != "" {
	treePrefixRendered = uicore.ApplyBg(s.styles.SidebarTreeRule, bgStyle).Render(treePrefix)
	treePrefixCells = ansix.Width(treePrefixRendered)
}
```

Adjust `leadCells`:

```go
if s.layout.Icons {
	leadCells = ansix.Width(indicator) + 1 + treePrefixCells + ansix.Width(icon) + 2
} else {
	leadCells = ansix.Width(indicator) + 1 + treePrefixCells
}
```

Adjust `leftContent` assembly:

```go
if s.layout.Icons {
	leftContent = indicator + bgStyle.Render(" ") + treePrefixRendered + icon + bgStyle.Render("  ") + name
} else {
	leftContent = indicator + bgStyle.Render(" ") + treePrefixRendered + name
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/ui/sidebar/ -v
```

Expected: `TestView_TreeExpandedShowsChildrenWithPrefix` passes. All existing render tests pass — depth-0 rows have empty prefix, so flat rendering is unchanged.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/sidebar/model.go internal/ui/sidebar/styles.go internal/ui/sidebar/tree.go internal/ui/styles.go internal/theme/palette.go
git commit -m "Pass 17a: render box-drawing tree prefix in sidebar rows"
```

---

## Task 5: KeyMap + Update + drop imperative movement (closes #45 item 4)

**Files:**
- Modify: `internal/ui/sidebar/model.go`, `internal/ui/sidebar/model_test.go`, `internal/ui/account/model.go`

- [ ] **Step 1: Write failing tests**

Append to `model_test.go`:

```go
func TestUpdate_ExpandCollapseOnParentRow(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Lists/golang"}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust"}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	// Cursor on the synthesized "Lists" parent (only visible Custom row).
	if got := m.SelectedFolder(); got != "Lists" {
		t.Fatalf("want cursor on Lists (synthesized), got %q", got)
	}

	right := tea.KeyPressMsg{Code: tea.KeyRight}
	m, _ = m.Update(right)
	if !m.IsExpanded("Lists") {
		t.Fatal("right arrow on parent should expand")
	}

	left := tea.KeyPressMsg{Code: tea.KeyLeft}
	m, _ = m.Update(left)
	if m.IsExpanded("Lists") {
		t.Fatal("left arrow should collapse")
	}
}

func TestUpdate_MovementRoutedThroughKeyMap(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Sent"}, Canonical: "Sent", DisplayName: "Sent", Group: mail.GroupPrimary},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	km := DefaultKeyMap()
	m.SetKeyMap(km)

	// Fire the configured Down key (default "j") and expect cursor advance.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected() != 1 {
		t.Fatalf("Down should advance cursor, got %d", m.Selected())
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
go test ./internal/ui/sidebar/ -run TestUpdate -v
```

Expected: `m.Update undefined`, `DefaultKeyMap undefined`, `SetKeyMap undefined`.

- [ ] **Step 3: Add `KeyMap`, `Update`, drop imperative methods**

In `internal/ui/sidebar/model.go`:

```go
import (
	// existing imports
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// KeyMap binds sidebar actions to keys. Account.Model is responsible for
// rebinding Down/Up to capital J/K in the account context.
type KeyMap struct {
	Down     key.Binding
	Up       key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Expand   key.Binding
	Collapse key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down:     key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "folder down")),
		Up:       key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "folder up")),
		Top:      key.NewBinding(key.WithKeys("g")),
		Bottom:   key.NewBinding(key.WithKeys("G")),
		Expand:   key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "expand")),
		Collapse: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "collapse")),
	}
}
```

Add `keys KeyMap` field to `Model`, default to `DefaultKeyMap()` inside `New`.

Add accessor and `Update`:

```go
func (s *Model) SetKeyMap(km KeyMap) { s.keys = km }
func (s Model) KeyMap() KeyMap       { return s.keys }

// Update dispatches sidebar keys. Caller (account.Model) selects which
// tea.KeyPressMsgs to forward; this method is inert on non-key messages.
func (s Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch {
	case key.Matches(km, s.keys.Down):
		s.moveDown()
	case key.Matches(km, s.keys.Up):
		s.moveUp()
	case key.Matches(km, s.keys.Top):
		s.selected = 0
	case key.Matches(km, s.keys.Bottom):
		if n := len(s.visibleRows()); n > 0 {
			s.selected = n - 1
		}
	case key.Matches(km, s.keys.Expand):
		rows := s.visibleRows()
		if s.selected < len(rows) && rows[s.selected].hasChildren {
			s.expanded[rows[s.selected].expandKey()] = true
		}
	case key.Matches(km, s.keys.Collapse):
		rows := s.visibleRows()
		if s.selected < len(rows) {
			r := rows[s.selected]
			if r.hasChildren && s.expanded[r.expandKey()] {
				delete(s.expanded, r.expandKey())
			} else if r.depth > 0 {
				// Cursor on a child — jump to ancestor and collapse it.
				s.collapseToAncestor(rows)
			}
		}
	}
	return s, nil
}

func (s *Model) moveDown() {
	if s.selected < len(s.visibleRows())-1 {
		s.selected++
	}
}
func (s *Model) moveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

// collapseToAncestor finds the immediate ancestor of the selected row and
// jumps the cursor there, then collapses it.
func (s *Model) collapseToAncestor(rows []rowMeta) {
	cur := rows[s.selected]
	for i := s.selected - 1; i >= 0; i-- {
		if rows[i].depth < cur.depth && rows[i].hasChildren {
			s.selected = i
			delete(s.expanded, rows[i].expandKey())
			return
		}
	}
}
```

Delete the existing `MoveUp`, `MoveDown`, `MoveToTop`, `MoveToBottom` methods on `*Model`.

In `New`, initialize `keys`:

```go
return Model{
	// ... existing fields ...
	expanded: map[string]bool{},
	keys:     DefaultKeyMap(),
}
```

- [ ] **Step 4: Update `account.Model` callers**

In `internal/ui/account/model.go`, around lines 368–379, replace the imperative `sb.MoveDown()` / `sb.MoveUp()` blocks with `Update` calls. Account.Model already routes `J`/`K` through `m.keys.SidebarDown/Up` — those handlers translate to firing the *sidebar's* Down/Up via `Update`:

```go
case key.Matches(msg, m.keys.SidebarDown):
	m = m.clearSearchIfActive()
	sb := m.sidebarColumn.Sidebar()
	// Synthesize the sidebar's Down keypress (default "j") so Update routes correctly.
	sb, _ = sb.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	return m.selectionChangedCmds()
case key.Matches(msg, m.keys.SidebarUp):
	m = m.clearSearchIfActive()
	sb := m.sidebarColumn.Sidebar()
	sb, _ = sb.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	return m.selectionChangedCmds()
```

Add new handlers in `account.Model.Update` for the expand/collapse keys. The account-level KeyMap grows two bindings — `SidebarExpand` (`→`) and `SidebarCollapse` (`←`). Add them to `internal/ui/account/keys.go` (search for `SidebarDown` to find the surrounding bindings); add the cases in `Update` (after the existing sidebar movement cases):

```go
case key.Matches(msg, m.keys.SidebarExpand):
	sb := m.sidebarColumn.Sidebar()
	sb, _ = sb.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	return m.selectionChangedCmds()
case key.Matches(msg, m.keys.SidebarCollapse):
	sb := m.sidebarColumn.Sidebar()
	sb, _ = sb.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	return m.selectionChangedCmds()
```

`selectionChangedCmds` already handles the case where expand toggled the selected provider name (no change) — it's idempotent. Collapsing onto an ancestor will fire `selectionChangedCmds`, which reloads the new folder. That's the correct UX (matches how `J/K` moves).

- [ ] **Step 5: Run all sidebar + account tests**

```bash
go test ./internal/ui/sidebar/ ./internal/ui/account/ -v
```

Expected: all pass. If `account/model.go` tests reference the removed `MoveUp`/`MoveDown` directly, they need the same `Update`-via-KeyPressMsg substitution.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/sidebar/model.go internal/ui/sidebar/model_test.go internal/ui/account/model.go internal/ui/account/keys.go
git commit -m "Pass 17a: sidebar KeyMap + Update; closes #45 item 4"
```

---

## Task 6: Spartan tier depth cap

**Files:**
- Modify: `internal/ui/uicore/layout.go` (only if a `Spartan` flag isn't already exposed), `internal/ui/sidebar/model.go`, `internal/ui/sidebar/model_test.go`

- [ ] **Step 1: Check `LayoutMode`**

```bash
grep -n "type LayoutMode\|Spartan" internal/ui/uicore/layout.go
```

If `LayoutMode` already has a `Spartan bool` field, skip Step 2. If it only has `Icons`/`Flags`/`DateWidth`, add one.

- [ ] **Step 2 (conditional): Add `Spartan` flag**

In `internal/ui/uicore/layout.go`, find `ComputeLayout` and set `Spartan: termWidth < 90` (the existing tier boundary per ADR-0109). Add the field to `LayoutMode`.

- [ ] **Step 3: Failing test**

Append to `model_test.go`:

```go
func TestView_SpartanCapsDepthAtOne(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "a/b/c/leaf"}, DisplayName: "a/b/c/leaf", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 14, 10, uicore.SimpleIcons)
	m.SetLayout(uicore.LayoutMode{Spartan: true})
	m.ToggleExpanded("a")
	rows := m.visibleRows()
	for _, r := range rows {
		if r.depth > 1 {
			t.Errorf("Spartan must cap depth at 1, got row %+v", r)
		}
	}
}
```

- [ ] **Step 4: Confirm passes**

The wiring is already in `visibleRows` from Task 3 (`if s.layout.Spartan { maxDepth = 1 }`). Run:

```bash
go test ./internal/ui/sidebar/ -run TestView_Spartan -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sidebar/model_test.go internal/ui/uicore/layout.go
git commit -m "Pass 17a: Spartan tier caps sidebar tree at depth 1"
```

---

## Task 7: Live verification at 80×24 and 120×40

**Files:** none modified; tmux capture.

- [ ] **Step 1: Build and install**

```bash
make install
```

Expected: exit 0, `poplar` written to `~/.local/bin/`.

- [ ] **Step 2: Launch in tmux at 120×40**

Per `.claude/docs/tmux-testing.md`. Login with `FASTMAIL_API_TOKEN`. Confirm Fastmail has at least one nested folder (e.g. `Lists/...`) — if not, create one via JMAP or via the web client.

Capture and visually verify:
- Top-level Custom folder shows no prefix.
- `→` on a parent expands; children render with `├─` / `└─`.
- `←` on a child jumps to ancestor and collapses.
- Collapsed parent shows aggregate unread.

- [ ] **Step 3: Verify at 80×24 (Spartan)**

Resize tmux pane to 80×24. Confirm:
- Tree still renders.
- Depth-2+ folders fold under their depth-1 parent.
- Icons/flags/date column are correctly absent (Spartan invariant).
- 14-cell sidebar isn't overflowing.

- [ ] **Step 4: Note any visual issues**

If a visual issue appears (selection bar misaligns, prefix bleeds, count badge clobbered), stop and fix before commit. If clean, no commit needed — proceed to Task 8.

---

## Task 8: ADR + invariants + styling + STATUS + BACKLOG

**Files:**
- Create: `docs/poplar/decisions/0197-sidebar-tree.md`
- Modify: `docs/poplar/decisions/INDEX.md`, `.claude/rules/ui-invariants.md`, `docs/poplar/styling.md`, `docs/poplar/STATUS.md`, `BACKLOG.md`
- Move: `docs/superpowers/plans/2026-05-10-sidebar-tree.md` → `archive/plans/`, `docs/superpowers/specs/2026-05-10-sidebar-tree-design.md` → `archive/specs/`

- [ ] **Step 1: Write ADR 0197**

Create `docs/poplar/decisions/0197-sidebar-tree.md`:

```markdown
---
title: Sidebar folder hierarchy renders as a tree
status: accepted
date: 2026-05-10
---

## Context

Pre-0197, `.claude/rules/ui-invariants.md` declared "Nested folder
names (containing `/`) render flat. The `/` in the display name is
the only affordance." That was a placeholder — every major mail
client (Thunderbird, K-9, Geary, Apple Mail) renders nested folders
as a tree, and the flat rendering caps usefulness at one or two
nested folders before the sidebar becomes unreadable.

17a lifts the placeholder. Custom folders with `/`-paths render as
a tree with `├─` / `└─` prefixes; parents expand/collapse; the
Primary group (canonical leaves only) and Disposal group
(canonical + synthetic Outbox) are unaffected.

## Decision

The sidebar's Custom group is a tree built per-render from a
transient `*node` map and discarded; expand state lives on
`sidebar.Model` as `map[string]bool` keyed by full provider path.
`→` expands a parent row; `←` collapses (or, on a child, jumps to
the ancestor and collapses it). Default state is collapsed.
Collapsed parents show the sum-of-descendants unread count
synthesized in the walk.

`Digital-Shane/treeview/v2` was vetted (active, v2-compatible, 83
stars) and rejected — its `NodeProvider` interface would fight the
existing `renderRow` selection-indicator + icon + unread-badge
pipeline. The transient-walk pattern from
`messagelist.appendThreadRows` is reused: build tree, yield
`(path, depth, isLast)` triples, drop tree, render flat.

Spartan tier (W=80–89) caps `maxDepth` at 1. Depth-2+ entries fold
into their depth-1 ancestor (still selectable, count still
aggregates), keeping the 14-cell sidebar legible.

Keys: `→` / `←` were unbound; reusing `Space` was rejected because
the sidebar has no focus state and Space already disambiguates
across visual mode / thread fold / viewer page / attach toggle via
*focus*, so a 5th meaning disambiguated by *cursor row contents*
breaches ADR-0052's cognitive-load bar.

`sidebar.Model` gains a stock `KeyMap` + `Update`; the imperative
`MoveUp` / `MoveDown` / `MoveToTop` / `MoveToBottom` methods are
gone. `account.Model` synthesizes the sidebar's bindings as
`tea.KeyPressMsg`s when the account-level `J`/`K`/`→`/`←` fire.
This closes #45 item (4).

## Consequences

- The `Sidebar` section of `ui-invariants.md` loses "Nested folder
  names render flat" and gains the tree invariant.
- `Styles` gains `SidebarTreeRule` (FgDim); `styling.md` updated.
- A backlog issue (#46) was filed to apply the same transient-walk
  shape (yielding `(Node, Depth, IsLast)` via `iter.Seq2`) to
  `messagelist.appendThreadRows` post-17a.
- The tree expand state is per-session; no persistence to config
  or cache. A future ADR could add `[ui.sidebar.expanded]` if
  users push back.
- Keyboard discoverability: `?` help vocabulary grows two rows
  (`→ expand`, `← collapse`) wired immediately.
```

- [ ] **Step 2: Update invariants**

In `.claude/rules/ui-invariants.md`, find the "Nested folder names render flat" bullet and replace:

```markdown
- Custom folders with `/`-paths render as a tree. `→` expands the
  parent under the cursor; `←` collapses (or, on a child, jumps
  to the ancestor and collapses it). Expand state is per-session,
  keyed by full provider path on `sidebar.Model.expanded`,
  pruned to live paths on every `SetFolders`. Collapsed parents
  show sum-of-descendants unread synthesized in the walk;
  expanded parents show their own `Unseen` only. Synthesized
  intermediate nodes (path segment with no real folder, e.g.
  "Lists" when only "Lists/golang" exists) render with the same
  prefix machinery and aggregate unread. Spartan tier (W=80–89)
  caps depth at 1: depth-2+ entries fold into their depth-1
  ancestor. Primary and Disposal groups are always flat.
  `sidebar.Model` exports a `KeyMap` + `Update`; imperative
  movement methods have been removed. ADR-0197.
```

- [ ] **Step 3: Update styling.md**

In `docs/poplar/styling.md`, find the "Sidebar" section and add a row mapping `SidebarTreeRule` → "Tree prefix `│ ├─ └─` (FgDim)".

- [ ] **Step 4: Update decisions INDEX**

Append to `docs/poplar/decisions/INDEX.md` under the Sidebar theme:

```markdown
- 0197 — Sidebar folder hierarchy renders as a tree (17a)
```

- [ ] **Step 5: Update STATUS.md**

Mark 17a `done` in the pass table. Rewrite the "Next starter prompt" for Pass 17b:

```markdown
### Next starter prompt (Pass 17b)

> **Goal.** Migrate `messagelist` to `bubbles/v2/list` with a
> custom item renderer for the thread-prefix walk. Second of the
> bubbles-adoption remainder (15a, 15a.5 done; 17a done; **17b** /
> 17c queued) before Polish II and the v0.9.0 freeze. The 16
> series locked modern-Go conventions before this pass, so the
> `iter.Seq2` walk lands as native shape, not a follow-up.
>
> **Scope.** `internal/ui/messagelist/` — `Model`, `displayRow`,
> `appendThreadRows`. Replace the imperative `MoveUp`/`MoveDown`/
> `MoveCursor` with `Update` + exported `KeyMap` (#45 item 2).
> Replace `appendThreadRows`'s manual prefix buffer with an
> `iter.Seq2`-style `(Node, Depth, IsLast)` walk (#46). Thread
> fold state, visual mode, and `ActionTargets` semantics all
> stay.
>
> **Settled (do not re-brainstorm):** `bubbles/v2/list` is the
> target dep (same family as 15a). The thread-row data model
> (group → sort → flatten → display) stays. Visual mode
> (`v` / `Space` / `Esc`) routing stays.
>
> **Still open — brainstorm these:** how to interleave date /
> sender / subject columns with `bubbles/v2/list`'s item
> delegate (custom delegate vs. pre-rendered string per row);
> whether `list.Filter` replaces the search shelf or stays
> sidebar-owned; whether fold state lives on the list's items
> or stays on `Model`.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-messagelist-list.md`,
> then implement. Standard pass-end checklist applies.
```

Trim the "Next steps" list if present so `STATUS.md` stays ≤60 lines.

- [ ] **Step 6: Update BACKLOG.md**

Find #45 in BACKLOG.md and strike-through item (4) in its body (the sidebar bullet). Do not close #45 yet — items (1), (2), (5) remain for 17b/17c.

- [ ] **Step 7: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-10-sidebar-tree.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-10-sidebar-tree-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 8: Commit, push, install**

```bash
make check
```

Expected: green.

```bash
git add docs/poplar/decisions/0197-sidebar-tree.md docs/poplar/decisions/INDEX.md .claude/rules/ui-invariants.md docs/poplar/styling.md docs/poplar/STATUS.md BACKLOG.md docs/superpowers/archive/
git commit -m "Pass 17a close: ADR-0197, sidebar tree invariant, STATUS to 17b"
git push
make install
```

---

## Self-review checklist

After all tasks land:

- [ ] **Spec coverage:** Every numbered "Settled decisions" item in the spec has a corresponding task or test. Verify:
  - Primary stays single-level → Task 3 (Primary entries pass through `rowMeta{entry: e}` with no depth).
  - Collapsed parents aggregate → Task 1 test `TestWalkCustom_CollapsedAggregatesUnread`.
  - Per-session expand state, pruned on `SetFolders` → Task 2.
  - `→` / `←` keys → Task 5.
  - Spartan cap → Task 6.
  - Hand-rolled, no library → enforced by Task 1 (no new import).

- [ ] **Placeholder scan:** No TBDs, no "implement similar to". Done.

- [ ] **Type consistency:** `rowMeta` fields used identically in tree.go, model.go, tests. `KeyMap` field names match between `DefaultKeyMap`, `Update`, and account.Model handlers.

- [ ] **Idiomatic-bubbletea check** (from poplar-pass §1b) is performed in Task 7 (live tmux) and reviewed against `docs/poplar/bubbletea-conventions.md` §10 before the Task 8 commit.
