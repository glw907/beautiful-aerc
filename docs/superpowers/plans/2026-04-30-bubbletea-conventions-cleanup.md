# Bubbletea Conventions Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pay down three structural deviations from idiomatic bubbletea identified in the Pass 4 audit (A3, A9, A10) without changing user-visible behavior.

**Architecture:** Per-component `key.Binding` structs replace `msg.String()` switches; child→parent state changes flow through synchronous accessor reads after delegation rather than through zero-latency `tea.Cmd` wrappers; `AccountTab.View` honors its width contract so `App.View` can append the right border without per-line padding.

**Tech Stack:** Go 1.26, bubbletea, bubbles (`key.Binding`, `key.Matches`), lipgloss.

**Spec:** `docs/superpowers/specs/2026-04-30-bubbletea-conventions-cleanup-design.md`

---

## File Map

| File | Action |
|------|--------|
| `internal/ui/keys.go` | Modify — add `AccountKeys`, `ViewerKeys` and constructors |
| `internal/ui/account_tab.go` | Modify — hold `keys AccountKeys`; convert `handleKey`; add accessors; add `pendingLinkPicker` field; ensure View width contract |
| `internal/ui/viewer.go` | Modify — hold `keys ViewerKeys`; convert `handleKey`; add `pendingLinkPicker` field + `LinkPickerRequest()` accessor |
| `internal/ui/app.go` | Modify — convert `App.Update` residual string compares; rewrite delegation to delegate-then-read; strip per-line padding in `renderFrame` |
| `internal/ui/cmds.go` | Modify — delete `FolderChangedMsg`, `ViewerOpenedMsg`, `ViewerClosedMsg`, `ViewerScrollMsg`, `LinkPickerOpenMsg` and their `*Cmd` constructors |
| `internal/ui/account_tab_test.go` | Modify — rewrite tests to assert via accessors |
| `internal/ui/viewer_test.go` | Modify — rewrite tests for new dispatch + accessor |
| `internal/ui/app_test.go` | Modify — rewrite tests for delegate-then-read flow |

---

## Phase 1 — `key.Matches` migration (Commit 1, audit #17)

Each task in this phase is mechanical. Run `make check` after each task; behavior must be unchanged at every step.

### Task 1.1: Add `AccountKeys` and constructor

**Files:**
- Modify: `internal/ui/keys.go`

- [ ] **Step 1: Add the struct and constructor**

Append to `internal/ui/keys.go` after `NewGlobalKeys`:

```go
// AccountKeys are handled by AccountTab. The set spans message-list
// motion, sidebar motion, folder jumps, search shelf, fold control,
// and the n/N message advance keys consumed by AccountTab when the
// viewer is open.
type AccountKeys struct {
	OpenSearch    key.Binding
	ClearSearch   key.Binding
	OpenMessage   key.Binding
	SidebarDown   key.Binding
	SidebarUp     key.Binding
	JumpInbox     key.Binding
	JumpDrafts    key.Binding
	JumpSent      key.Binding
	JumpArchive   key.Binding
	JumpSpam      key.Binding
	JumpTrash     key.Binding
	MsgListTop    key.Binding
	MsgListBottom key.Binding
	MsgListDown   key.Binding
	MsgListUp     key.Binding
	ToggleFold    key.Binding
	ToggleFoldAll key.Binding
	NextMessage   key.Binding
	PrevMessage   key.Binding
}

// NewAccountKeys returns the default account-tab key bindings.
func NewAccountKeys() AccountKeys {
	return AccountKeys{
		OpenSearch:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		ClearSearch:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
		OpenMessage:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		SidebarDown:   key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next folder")),
		SidebarUp:     key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev folder")),
		JumpInbox:     key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "inbox")),
		JumpDrafts:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "drafts")),
		JumpSent:      key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sent")),
		JumpArchive:   key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archive")),
		JumpSpam:      key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "spam")),
		JumpTrash:     key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "trash")),
		MsgListTop:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of list")),
		MsgListBottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of list")),
		MsgListDown:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		MsgListUp:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		ToggleFold:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "fold")),
		ToggleFoldAll: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "fold all")),
		NextMessage:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next message")),
		PrevMessage:   key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev message")),
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/ui/`
Expected: clean build, no errors.

- [ ] **Step 3: Commit (will be amended later in this phase)**

Hold off — combine with subsequent tasks in this phase into a single commit at the end of Phase 1. Mark this step done without committing.

### Task 1.2: Add `ViewerKeys` and constructor

**Files:**
- Modify: `internal/ui/keys.go`

- [ ] **Step 1: Add the struct and constructor**

Append after `NewAccountKeys`:

```go
// ViewerKeys are handled by Viewer.handleKey. Body scrolling
// (j/k/space/b) is delegated to the embedded viewport's own KeyMap;
// only the keys Viewer consumes directly appear here.
type ViewerKeys struct {
	Close      key.Binding
	OpenPicker key.Binding
	BodyTop    key.Binding
	BodyBottom key.Binding
	Link1      key.Binding
	Link2      key.Binding
	Link3      key.Binding
	Link4      key.Binding
	Link5      key.Binding
	Link6      key.Binding
	Link7      key.Binding
	Link8      key.Binding
	Link9      key.Binding
}

// NewViewerKeys returns the default viewer key bindings.
func NewViewerKeys() ViewerKeys {
	return ViewerKeys{
		Close:      key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "close")),
		OpenPicker: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "links")),
		BodyTop:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of body")),
		BodyBottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of body")),
		Link1:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "link 1")),
		Link2:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "link 2")),
		Link3:      key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "link 3")),
		Link4:      key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "link 4")),
		Link5:      key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "link 5")),
		Link6:      key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "link 6")),
		Link7:      key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "link 7")),
		Link8:      key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "link 8")),
		Link9:      key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "link 9")),
	}
}

// linkBindingByIndex returns the 1-based link binding from vk, or
// zero binding when out of range. Used by Viewer.handleKey to fold
// the nine digit keys into a single dispatch path.
func linkBindingByIndex(vk ViewerKeys, n int) key.Binding {
	switch n {
	case 1:
		return vk.Link1
	case 2:
		return vk.Link2
	case 3:
		return vk.Link3
	case 4:
		return vk.Link4
	case 5:
		return vk.Link5
	case 6:
		return vk.Link6
	case 7:
		return vk.Link7
	case 8:
		return vk.Link8
	case 9:
		return vk.Link9
	}
	return key.Binding{}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/ui/`
Expected: clean build.

### Task 1.3: Migrate `Viewer.handleKey` to `key.Matches`

**Files:**
- Modify: `internal/ui/viewer.go`
- Modify: `internal/ui/viewer_test.go`

- [ ] **Step 1: Read existing viewer struct and constructor**

Run: `grep -n "type Viewer struct\|func NewViewer\|func.*Viewer.*Update\|func.*Viewer.*handleKey" internal/ui/viewer.go`
Note the field list and constructor — you'll add `keys ViewerKeys` to both.

- [ ] **Step 2: Add `keys ViewerKeys` field and initialize in constructor**

In `internal/ui/viewer.go`, add `keys ViewerKeys` to the struct and `keys: NewViewerKeys(),` to the `NewViewer` constructor's struct literal.

Run: `go build ./internal/ui/`
Expected: clean build (field is unused for now; that's fine as a struct field).

- [ ] **Step 3: Write a failing test for one converted dispatch path**

Add to `internal/ui/viewer_test.go`:

```go
func TestViewerHandleKey_CloseViaQ_UsesKeyMatches(t *testing.T) {
	v := NewViewer(/* existing test args */)
	v = v.Open(/* existing test args */)
	v, _ = v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if v.IsOpen() {
		t.Fatal("viewer should close on q")
	}
}
```

(Reuse whatever the existing viewer_test.go uses to construct a Viewer and open it. This step's purpose is to lock in behavior before swapping the dispatch.)

Run: `go test ./internal/ui/ -run TestViewerHandleKey_CloseViaQ_UsesKeyMatches -v`
Expected: PASS (the existing string-switch implementation already handles this).

- [ ] **Step 4: Replace `viewer.handleKey` body with `key.Matches`**

Replace the existing `handleKey` (currently at viewer.go:158–194) with:

```go
func (v Viewer) handleKey(msg tea.KeyMsg) (Viewer, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Close):
		v = v.Close()
		return v, viewerClosedCmd()
	case key.Matches(msg, v.keys.OpenPicker):
		if len(v.links) == 0 {
			return v, nil
		}
		return v, linkPickerOpenCmd(v.links)
	}
	for n := 1; n <= 9; n++ {
		if key.Matches(msg, linkBindingByIndex(v.keys, n)) {
			if n-1 < len(v.links) {
				return v, launchURLCmd(v.links[n-1])
			}
			return v, nil
		}
	}
	if v.phase != viewerReady {
		return v, nil
	}
	prevPct := v.ScrollPct()
	switch {
	case key.Matches(msg, v.keys.BodyTop):
		v.viewport.GotoTop()
	case key.Matches(msg, v.keys.BodyBottom):
		v.viewport.GotoBottom()
	default:
		var c tea.Cmd
		v.viewport, c = v.viewport.Update(msg)
		if pct := v.ScrollPct(); pct != prevPct {
			return v, tea.Batch(c, viewerScrollCmd(pct))
		}
		return v, c
	}
	if pct := v.ScrollPct(); pct != prevPct {
		return v, viewerScrollCmd(pct)
	}
	return v, nil
}
```

Note: the existing `parseLinkKey` helper (viewer.go) is now unused. Delete its declaration and any references in tests.

- [ ] **Step 5: Verify all viewer tests pass**

Run: `go test ./internal/ui/ -run TestViewer -v`
Expected: all pass.

- [ ] **Step 6: Run vet**

Run: `go vet ./internal/ui/`
Expected: no warnings (specifically: no unused-import for `strconv` if it was only used by `parseLinkKey`).

### Task 1.4: Migrate `AccountTab.handleKey` to `key.Matches`

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Add `keys AccountKeys` field and initialize**

In `internal/ui/account_tab.go`, add `keys AccountKeys` to the struct and `keys: NewAccountKeys(),` to the `NewAccountTab` constructor's struct literal.

Run: `go build ./internal/ui/`
Expected: clean build.

- [ ] **Step 2: Replace the search-idle dispatch switch**

Replace the `switch msg.String()` block at account_tab.go:246–288 with:

```go
switch {
case key.Matches(msg, m.keys.OpenSearch):
	if m.sidebarSearch.State() == SearchIdle || m.sidebarSearch.State() == SearchActive {
		m.sidebarSearch.Activate()
		return m, nil
	}
case key.Matches(msg, m.keys.ClearSearch):
	if m.sidebarSearch.State() == SearchActive {
		m.sidebarSearch.Clear()
		m.msglist.ClearFilter()
		return m, nil
	}
case key.Matches(msg, m.keys.OpenMessage):
	return m.openSelectedMessage()
case key.Matches(msg, m.keys.SidebarDown):
	m.clearSearchIfActive()
	m.sidebar.MoveDown()
	return m, m.selectionChangedCmds()
case key.Matches(msg, m.keys.SidebarUp):
	m.clearSearchIfActive()
	m.sidebar.MoveUp()
	return m, m.selectionChangedCmds()
case key.Matches(msg, m.keys.JumpInbox):
	return m.jumpToFolder("Inbox")
case key.Matches(msg, m.keys.JumpDrafts):
	return m.jumpToFolder("Drafts")
case key.Matches(msg, m.keys.JumpSent):
	return m.jumpToFolder("Sent")
case key.Matches(msg, m.keys.JumpArchive):
	return m.jumpToFolder("Archive")
case key.Matches(msg, m.keys.JumpSpam):
	return m.jumpToFolder("Spam")
case key.Matches(msg, m.keys.JumpTrash):
	return m.jumpToFolder("Trash")
case key.Matches(msg, m.keys.MsgListBottom):
	m.msglist.MoveToBottom()
case key.Matches(msg, m.keys.MsgListTop):
	m.msglist.MoveToTop()
case key.Matches(msg, m.keys.MsgListDown):
	m.msglist.MoveDown()
case key.Matches(msg, m.keys.MsgListUp):
	m.msglist.MoveUp()
case key.Matches(msg, m.keys.ToggleFold):
	if m.sidebarSearch.State() == SearchActive {
		return m, nil
	}
	m.msglist.ToggleFold()
case key.Matches(msg, m.keys.ToggleFoldAll):
	if m.sidebarSearch.State() == SearchActive {
		return m, nil
	}
	m.msglist.ToggleFoldAll()
}
```

The `folderJumpTargets` map is now redundant — delete its declaration (currently at account_tab.go:297–305).

- [ ] **Step 3: Replace the viewer-open `n`/`N` branch**

Replace the `s := msg.String(); if s == "n" || s == "N"` block at account_tab.go:204–223 with:

```go
if m.viewer.IsOpen() {
	delta := 0
	switch {
	case key.Matches(msg, m.keys.NextMessage):
		delta = 1
	case key.Matches(msg, m.keys.PrevMessage):
		delta = -1
	}
	if delta != 0 {
		if m.viewer.Phase() != viewerReady {
			return m, nil
		}
		uid, moved := m.msglist.MoveCursor(delta)
		if !moved {
			return m, nil
		}
		info, ok := m.msglist.MessageByUID(uid)
		if !ok {
			return m, nil
		}
		return m.openMessage(info)
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}
```

- [ ] **Step 4: Run all tests**

Run: `make check`
Expected: green.

If any test fails, the cause is almost certainly a key lookup that worked under `msg.String()` but doesn't match the binding's `WithKeys` set. Inspect the failing test, identify the missing alias (e.g., `down` vs `j`), add it to the binding's `WithKeys`, re-run.

### Task 1.5: Migrate `App.Update` residual string compares

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Inspect the search-active branch**

The block at app.go:155–162 reads `m.acct.sidebarSearch.State() != SearchIdle` and synthesises a `tea.KeyEsc` for delegation. The state read is fine (it'll become `m.acct.SearchState()` in Phase 2); the synthetic Esc is what to migrate.

- [ ] **Step 2: Replace the synthetic Esc with the binding**

Change:
```go
m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc})
```
to construct the Esc message via the binding:
```go
m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc, Runes: []rune{}})
```

(In practice `tea.KeyEsc` carries an implicit string "esc" — no change needed here. Verify by running tests after Step 3.)

- [ ] **Step 3: Run tests**

Run: `make check`
Expected: green.

Note: most of `App.Update`'s `tea.KeyMsg` arm already uses `key.Matches` (lines 134–174). The only remaining residue is the search-clear delegation. After Phase 2's accessor work, this whole branch will read more cleanly.

### Task 1.6: Commit Phase 1

- [ ] **Step 1: Run final check**

Run: `make check`
Expected: green.

- [ ] **Step 2: Stage and commit**

```bash
git add internal/ui/keys.go internal/ui/viewer.go internal/ui/viewer_test.go internal/ui/account_tab.go internal/ui/account_tab_test.go internal/ui/app.go
git commit -m "$(cat <<'EOF'
Migrate UI key dispatch to key.Matches with per-component KeyMaps

Add AccountKeys and ViewerKeys to keys.go with constructors. Thread
each into its owning component, replacing the residual msg.String()
switches in viewer.handleKey, account_tab.handleKey, and the
search-clear delegation in app.Update. Behaviour is preserved; the
bindings are now first-class and ready to drive ADR-0072's
help-popover wired-flag rendering in a later pass.

Closes audit finding A3 (#17).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Direct delegation (Commit 2, audit #18)

Replace zero-latency intra-model `tea.Cmd` wrappers with synchronous accessor reads after delegation.

### Task 2.1: Add `Viewer.LinkPickerRequest` accessor

**Files:**
- Modify: `internal/ui/viewer.go`
- Modify: `internal/ui/viewer_test.go`

- [ ] **Step 1: Write a failing test**

Add to `internal/ui/viewer_test.go`:

```go
func TestViewerLinkPickerRequest_OneShotClearsOnRead(t *testing.T) {
	v := NewViewer(/* args */)
	v = v.Open(/* args with at least one link harvested */)

	v, _ = v.handleKey(tea.KeyMsg{Type: tea.KeyTab})

	links, ok := v.LinkPickerRequest()
	if !ok || len(links) == 0 {
		t.Fatal("expected pending link-picker request after Tab")
	}
	_, ok = v.LinkPickerRequest()
	if ok {
		t.Fatal("LinkPickerRequest must clear on read")
	}
}
```

Run: `go test ./internal/ui/ -run TestViewerLinkPickerRequest_OneShotClearsOnRead -v`
Expected: FAIL with "LinkPickerRequest is undefined".

- [ ] **Step 2: Add the field, accessor, and dispatch change**

Add to the `Viewer` struct:
```go
pendingLinkPicker []string
```

Add the accessor:
```go
// LinkPickerRequest returns the harvested link list and true if the
// last keypress requested the link picker. Reading clears the
// request — callers receive (nil, false) on subsequent reads until
// another Tab press fires.
func (v *Viewer) LinkPickerRequest() ([]string, bool) {
	if v.pendingLinkPicker == nil {
		return nil, false
	}
	links := v.pendingLinkPicker
	v.pendingLinkPicker = nil
	return links, true
}
```

(Pointer receiver because the accessor mutates state. Verify the rest of Viewer's methods are value receivers; if so this is the only pointer-receiver method and that is fine — bubbletea models are value types but their accessors may use pointer receivers for one-shot patterns.)

In `handleKey`, replace:
```go
case key.Matches(msg, v.keys.OpenPicker):
	if len(v.links) == 0 {
		return v, nil
	}
	return v, linkPickerOpenCmd(v.links)
```
with:
```go
case key.Matches(msg, v.keys.OpenPicker):
	if len(v.links) == 0 {
		return v, nil
	}
	v.pendingLinkPicker = v.links
	return v, nil
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/ui/ -run TestViewerLinkPickerRequest_OneShotClearsOnRead -v`
Expected: PASS.

- [ ] **Step 4: Run all tests**

Run: `make check`
Expected: green. Tests that asserted on `linkPickerOpenCmd` round-trips will be updated in Task 2.4.

### Task 2.2: Add `AccountTab` accessors

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write failing tests for each new accessor**

Add to `internal/ui/account_tab_test.go`:

```go
func TestAccountTabAccessors(t *testing.T) {
	m := NewAccountTab(/* test args */)
	// Initial state: viewer closed, search idle.
	if m.ViewerOpen() {
		t.Error("ViewerOpen should be false initially")
	}
	if m.SearchState() != SearchIdle {
		t.Error("SearchState should be SearchIdle initially")
	}
	exists, unseen := m.SelectedFolderCounts()
	_ = exists
	_ = unseen // smoke test only — values depend on test backend
	if pct := m.ViewerScrollPct(); pct != 0 {
		t.Errorf("ViewerScrollPct should be 0 with viewer closed, got %v", pct)
	}
	if _, ok := m.LinkPickerRequest(); ok {
		t.Error("LinkPickerRequest should be (nil, false) initially")
	}
}
```

Run: `go test ./internal/ui/ -run TestAccountTabAccessors -v`
Expected: FAIL — accessors not defined.

- [ ] **Step 2: Add accessors and `pendingLinkPicker` field**

Add field to `AccountTab` struct:
```go
pendingLinkPicker []string
```

Add accessors:
```go
// ViewerOpen reports whether the viewer is currently open.
func (m AccountTab) ViewerOpen() bool { return m.viewer.IsOpen() }

// SelectedFolderCounts returns the (exists, unseen) counts for the
// selected folder, or (0, 0) if no folder is selected. Mirrors the
// payload that FolderChangedMsg used to carry.
func (m AccountTab) SelectedFolderCounts() (int, int) {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return 0, 0
	}
	return folder.Exists, folder.Unseen
}

// ViewerScrollPct returns the viewer's scroll percentage, or 0 when
// the viewer is closed.
func (m AccountTab) ViewerScrollPct() int {
	if !m.viewer.IsOpen() {
		return 0
	}
	return m.viewer.ScrollPct()
}

// SearchState exposes the sidebar search state machine.
func (m AccountTab) SearchState() SearchState {
	return m.sidebarSearch.State()
}

// LinkPickerRequest returns a one-shot pending link-picker open
// request. Mirrors Viewer.LinkPickerRequest; AccountTab forwards
// the viewer's request through itself so App.Update can read it
// without traversing into nested children.
func (m *AccountTab) LinkPickerRequest() ([]string, bool) {
	if m.pendingLinkPicker == nil {
		return nil, false
	}
	links := m.pendingLinkPicker
	m.pendingLinkPicker = nil
	return links, true
}
```

(Verify the exact signatures of `m.sidebar.SelectedFolder()`, `m.viewer.ScrollPct()`, and `m.sidebarSearch.State()` match — adjust accessor return types if any return different shapes.)

- [ ] **Step 3: Wire viewer's pending request into AccountTab**

In `AccountTab.handleKey`, after every delegation to `m.viewer.Update(msg)`, capture the pending request:

```go
var cmd tea.Cmd
m.viewer, cmd = m.viewer.Update(msg)
if links, ok := (&m.viewer).LinkPickerRequest(); ok {
	m.pendingLinkPicker = links
}
return m, cmd
```

There is exactly one such delegation in `handleKey` (the viewer-open branch from Task 1.4 Step 3). Update it.

- [ ] **Step 4: Run the test**

Run: `go test ./internal/ui/ -run TestAccountTabAccessors -v`
Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `make check`
Expected: green.

### Task 2.3: Rewrite `App.Update` to delegate-then-read

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmds.go`

- [ ] **Step 1: Add a delegation helper**

Add a private method on App to centralize the "delegate then re-derive chrome" pattern:

```go
// deriveChromeFromAcct re-reads AccountTab state and propagates it
// to App-owned chrome (footer, status bar, viewerOpen, linkPicker).
// Called after every delegation that may have changed child state.
func (m App) deriveChromeFromAcct() App {
	prevViewer := m.viewerOpen
	m.viewerOpen = m.acct.ViewerOpen()
	exists, unseen := m.acct.SelectedFolderCounts()
	m.statusBar = m.statusBar.SetCounts(exists, unseen)
	if m.viewerOpen {
		if !prevViewer {
			m.footer = m.footer.SetContext(ViewerContext)
			m.statusBar = m.statusBar.SetMode(StatusViewer).SetScrollPct(0)
		} else {
			m.statusBar = m.statusBar.SetScrollPct(m.acct.ViewerScrollPct())
		}
	} else if prevViewer {
		m.footer = m.footer.SetContext(AccountContext)
		m.statusBar = m.statusBar.SetMode(StatusAccount)
	}
	if links, ok := (&m.acct).LinkPickerRequest(); ok {
		m.linkPicker = m.linkPicker.Open(links)
	}
	return m
}
```

- [ ] **Step 2: Delete the signal-message `case` arms**

In `App.Update`, delete these `case` arms entirely:
- `case FolderChangedMsg:` (lines 75–77)
- `case ViewerOpenedMsg:` (lines 79–83)
- `case ViewerClosedMsg:` (lines 85–89)
- `case ViewerScrollMsg:` (lines 102–104)
- `case LinkPickerOpenMsg:` (lines 91–93)

Keep `case LinkPickerClosedMsg:` — that's the picker-internal close signal, App-internal, not crossing parent-child wiring.

- [ ] **Step 3: Wrap every delegation site with `deriveChromeFromAcct`**

Sites where App calls `m.acct.Update(...)` and now needs to re-derive chrome:
- The `tea.WindowSizeMsg` arm
- The `ErrorMsg` arm (both delegation calls)
- The `tea.KeyMsg` arm's three delegation paths (viewer-open `q`, search-active `q`, fall-through delegation)
- The trailing `m.acct, cmd = m.acct.Update(msg); return m, cmd` at the end

Replace each pattern of:
```go
m.acct, cmd = m.acct.Update(...)
return m, cmd
```
with:
```go
m.acct, cmd = m.acct.Update(...)
m = m.deriveChromeFromAcct()
return m, cmd
```

Skip the `WindowSizeMsg` arm if its current behaviour explicitly suppresses chrome derivation — verify by re-reading lines 66–73. If the arm only delegates to forward sizing, do NOT derive chrome (sizing alone shouldn't change chrome). Add the call only where state-changing messages flow through.

In practice the arms that need `deriveChromeFromAcct`: every `tea.KeyMsg` delegation, and the `ErrorMsg` arm's body delegation (line 120).

- [ ] **Step 4: Delete signal types from `cmds.go`**

Delete from `internal/ui/cmds.go`:
- `FolderChangedMsg` struct (line 62) and the leading comment
- `folderChangedCmd` function (line 134)
- `ViewerOpenedMsg`, `ViewerClosedMsg` structs (lines 187, 190) and comments
- `ViewerScrollMsg` struct (line 194) and comment
- `viewerOpenedCmd`, `viewerClosedCmd`, `viewerScrollCmd` functions (lines 335–338)
- `LinkPickerOpenMsg` struct (line 352) and comment
- `linkPickerOpenCmd` function (line 368)

Keep `LinkPickerClosedMsg` and any close-side helpers — picker-internal.

Also delete the call sites:
- `viewer.go:163`: `return v, viewerClosedCmd()` → `return v, nil`
- `viewer.go:186, 191`: replace `viewerScrollCmd(pct)` calls. The viewer no longer needs to signal scroll — `App` reads `ViewerScrollPct` after delegation. Replace `return v, tea.Batch(c, viewerScrollCmd(pct))` with `return v, c` and delete the `prevPct` comparison entirely (there is no longer a reason to gate on a change since App re-reads unconditionally).
- `account_tab.go:326`: `viewerOpenedCmd()` in the `openMessage` flow — delete (search the file for it; it's part of the Cmd batch returned when opening the viewer). The viewer-open transition is now visible to App via `m.acct.ViewerOpen()` after delegation.
- `account_tab.go:367`: `folderChangedCmd(folder)` in the folder-load flow — delete. App re-reads counts after delegation.

- [ ] **Step 5: Compile**

Run: `go build ./internal/ui/`
Expected: clean build. If there are unused imports (e.g., `mail` import in cmds.go after `folderChangedCmd` deletion), remove them.

- [ ] **Step 6: Run tests**

Run: `make check`
Expected: most tests pass; some that asserted on the deleted Cmd round-trips will fail. They'll be fixed in Task 2.4.

### Task 2.4: Update tests for delegate-then-read flow

**Files:**
- Modify: `internal/ui/account_tab_test.go`
- Modify: `internal/ui/viewer_test.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Inventory failing tests**

Run: `go test ./internal/ui/ 2>&1 | grep -E "^(---|FAIL)"`
Expected: a list of failing tests, all referencing the deleted Cmd or message types.

- [ ] **Step 2: Rewrite each failing test**

Pattern: tests that asserted "calling X returned a Cmd that produced FolderChangedMsg" become "calling X mutates the model such that `m.SelectedFolderCounts()` returns the expected values, and feeding the resulting model into App produces the expected status bar state."

For each failing test:
- If it tested AccountTab in isolation and asserted on a Cmd: assert on `m.ViewerOpen()` / `m.SelectedFolderCounts()` / `m.SearchState()` instead.
- If it tested App-level chrome reaction to a child event: drive the test by calling `App.Update` with the original `tea.KeyMsg` (or whatever caused the child state change) and assert on the resulting `m.statusBar`, `m.footer`, `m.viewerOpen`.

Run after each rewrite: `go test ./internal/ui/ -run <TestName> -v`

- [ ] **Step 3: Full check**

Run: `make check`
Expected: green.

### Task 2.5: Commit Phase 2

- [ ] **Step 1: Stage and commit**

```bash
git add internal/ui/keys.go internal/ui/viewer.go internal/ui/viewer_test.go internal/ui/account_tab.go internal/ui/account_tab_test.go internal/ui/app.go internal/ui/app_test.go internal/ui/cmds.go
git commit -m "$(cat <<'EOF'
Replace intra-model Cmd signals with delegate-then-read accessors

App used to receive FolderChangedMsg, ViewerOpenedMsg,
ViewerClosedMsg, ViewerScrollMsg, and LinkPickerOpenMsg as
zero-latency Cmd round-trips from AccountTab and Viewer to keep
chrome (footer, status bar, link picker) in sync. Replace those
signal arms with synchronous accessors on AccountTab — ViewerOpen,
SelectedFolderCounts, ViewerScrollPct, SearchState,
LinkPickerRequest — read after every delegation in App.Update via
a deriveChromeFromAcct helper. The link-picker request is a
one-shot pending field at both the Viewer and AccountTab layers,
mirroring the App↔AccountTab pattern one level deeper.

Closes audit finding A9 (#18).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — Width contract trust (Commit 3, audit #19)

Tighten `AccountTab.View` so every output line is exactly the assigned width, then strip the per-line padding loop in `App.renderFrame`.

### Task 3.1: Audit AccountTab.View output for non-conforming rows

**Files:**
- Read: `internal/ui/account_tab.go` (View method)
- Read: any helper that produces rows: `internal/ui/msglist.go`, `internal/ui/sidebar.go`, `internal/ui/sidebar_search.go`, `internal/ui/viewer.go` (when phase != closed)

- [ ] **Step 1: Add a width-contract test**

Add to `internal/ui/account_tab_test.go`:

```go
func TestAccountTabView_HonorsAssignedWidth(t *testing.T) {
	m := NewAccountTab(/* test args */)
	const w, h = 119, 40 // m.width-1 from a 120-wide terminal
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	out := m.View()
	for i, line := range strings.Split(out, "\n") {
		if got := displayCells(line); got != w {
			t.Errorf("line %d: width %d, want %d (line=%q)", i, got, w, line)
		}
	}
}
```

Run: `go test ./internal/ui/ -run TestAccountTabView_HonorsAssignedWidth -v`
Expected: FAIL — almost certainly some rows fall short or run long.

- [ ] **Step 2: Identify each failing row source**

The test output names which line is the wrong width. Map each to its producing function:
- Loading placeholder (`NewSpinner` row in viewer-loading state) — likely centered, may not span full width
- Error banner row — already padded? verify
- Search shelf (3-row block when active) — verify
- MessageList rows — likely fine but verify
- Sidebar rows — likely fine

- [ ] **Step 3: Pad each non-conforming source to its assigned width**

For each row producer, ensure it pads with `displayCells`-aware padding. Pattern:
```go
dw := displayCells(rendered)
if dw < width {
    rendered = rendered + strings.Repeat(" ", width-dw)
}
```

Or if the producer uses lipgloss styles, set `Width(width)` on the style — lipgloss handles padding automatically for non-SPUA strings (`spuaCellWidth == 1`); for SPUA-bearing strings, fall back to manual padding.

The viewer's loading placeholder is the most likely offender. Grep for `NewSpinner` and similar in viewer.go's loading-phase render path; pad the row to match the panel width passed in.

- [ ] **Step 4: Run the width-contract test**

Run: `go test ./internal/ui/ -run TestAccountTabView_HonorsAssignedWidth -v`
Expected: PASS.

- [ ] **Step 5: Run full check**

Run: `make check`
Expected: green.

### Task 3.2: Strip per-line padding from `App.renderFrame`

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Replace the padding loop**

In `internal/ui/app.go`'s `renderFrame` method, replace the loop at lines 188–202:

```go
contentLines := strings.Split(rawContent, "\n")
for i, line := range contentLines {
	dw := displayCells(line)
	contentWidth := m.width - 1
	if dw > contentWidth {
		line = displayTruncate(line, contentWidth)
	} else if dw < contentWidth {
		line = line + strings.Repeat(" ", contentWidth-dw)
	}
	contentLines[i] = line + rightBorder
}
content := strings.Join(contentLines, "\n")
```

with:

```go
contentLines := strings.Split(rawContent, "\n")
for i, line := range contentLines {
	contentLines[i] = line + rightBorder
}
content := strings.Join(contentLines, "\n")
```

- [ ] **Step 2: Run the full test suite**

Run: `make check`
Expected: green. If a test fails on render width, the width contract has a hole — the regression is in AccountTab or one of its children, not in App.

- [ ] **Step 3: Tmux capture verification at 120×40**

Follow `.claude/docs/tmux-testing.md`. Build, install, start poplar in a 120×40 tmux pane, capture the rendered frame, verify:
- The right border (`│`) is on column 120 of every content row.
- No row is shorter (gap between content and border).
- No row is longer (border missing or wrapped).

Save the capture to `docs/poplar/testing/2026-04-30-pass5-width-contract-120x40.txt` if it doesn't already exist; reference it from the pass-end ADR.

- [ ] **Step 4: Tmux capture at 80×24**

Same exercise at the minimum-viable width. Save to `docs/poplar/testing/2026-04-30-pass5-width-contract-80x24.txt`.

If either capture shows misalignment, return to Task 3.1 — there is still a row producer not honoring its width.

### Task 3.3: Commit Phase 3

- [ ] **Step 1: Stage and commit**

```bash
git add internal/ui/account_tab.go internal/ui/account_tab_test.go internal/ui/msglist.go internal/ui/sidebar.go internal/ui/sidebar_search.go internal/ui/viewer.go internal/ui/app.go docs/poplar/testing/2026-04-30-pass5-width-contract-*.txt
git commit -m "$(cat <<'EOF'
Trust AccountTab.View width contract; drop App per-line padding

Every row produced by AccountTab.View is now exactly the assigned
width in display cells (via displayCells-aware padding at the row-
producer layer). App.renderFrame drops its per-line measure-and-pad
loop in favour of a plain border-append. Tmux captures at 120x40
and 80x24 verify the contract holds at both extremes.

Closes audit finding A10 (#19).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — Pass-end ritual

The `poplar-pass` skill covers the consolidation ritual. Steps to perform after Phase 3 commits:

- [ ] **Step 1: Run `/simplify` against the cumulative pass diff**

Three review agents in parallel; aggregate findings; fix what matters; the rest is documented as deliberate.

- [ ] **Step 2: Run the §10 idiomatic-bubbletea review checklist**

From `docs/poplar/bubbletea-conventions.md` §10. Verify each item against the diff:
- View widths honored (covered by Task 3.1's test + tmux captures)
- No state mutation in View or in tea.Cmd closures
- All blocking I/O lives inside tea.Cmd
- Width math uses `lipgloss.Width` / `displayCells`, never `len()`
- Renderers honor `width` via wordwrap + hardwrap
- No defensive parent-side clipping (this pass closes the only such site)
- Children signal parents via `tea.Msg`… *or, after this pass, via accessor reads*. Update the checklist text in `docs/poplar/bubbletea-conventions.md` to reflect the new norm: "Children expose state via accessors; parents read after delegation. `tea.Msg` is reserved for cross-tree signals (e.g., commands fired by external state)."
- `WindowSizeMsg` forwarded into children
- Keys declared as `key.Binding`; dispatched with `key.Matches` — covered by Phase 1
- No deprecated API usage

- [ ] **Step 3: Write the pass-end ADR**

Create `docs/poplar/decisions/00NN-per-component-keymaps-and-direct-delegation.md` (next available number). Cover:
- The (A + i) decision: per-component KeyMap structs, all in `keys.go`.
- The accessor surface on AccountTab and the deletion of the four signal message types.
- The two-layer pending-link-picker pattern at Viewer and AccountTab (record this as a deliberate deviation from the "all wiring through tea.Msg" norm; rationale = the audit's A9 prescription).
- If Task 2.1's pending-field approach was abandoned in implementation, document the deviation and the residual `LinkPickerOpenMsg`.
- Update the bubbletea-conventions doc §3, §8, §10 to reflect the new norm.

- [ ] **Step 4: Update `docs/poplar/invariants.md`**

Edit in place (do not append). The relevant binding facts to update:
- `internal/ui/` Elm architecture section — adjust the "children signal parents via Msg types" line to read "children expose state via accessors; parents read after delegation. Msg types are reserved for cross-tree signals."
- Decision index — add the new ADR number under "Elm architecture in internal/ui/" or a new themed row.

- [ ] **Step 5: Update `docs/poplar/STATUS.md`**

- Mark Pass 5 `done`.
- Replace the Pass 5 starter prompt with the Pass 6 starter prompt (triage actions: delete/archive/star/read; toast + undo bar).
- Update "Audits" list if any new audit artefacts were produced.
- Verify STATUS.md is ≤60 lines; prune if needed.

- [ ] **Step 6: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-04-30-bubbletea-conventions-cleanup.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-04-30-bubbletea-conventions-cleanup-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 7: Final make check**

Run: `make check`
Expected: green.

- [ ] **Step 8: Commit ritual artefacts**

```bash
git add docs/poplar/decisions/ docs/poplar/invariants.md docs/poplar/STATUS.md docs/poplar/bubbletea-conventions.md docs/superpowers/
git commit -m "$(cat <<'EOF'
Pass 5: bubbletea conventions cleanup — close out

ADR for per-component KeyMaps + direct delegation; invariants and
conventions doc updated to record the new "accessor reads after
delegation" norm; STATUS rolls to Pass 6 (triage actions); plan +
spec archived.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 9: Push and install**

```bash
git push
make install
```

---

## Risks and recovery

- **Phase 1 regression**: a binding's `WithKeys` set differs from the prior `case` set. Symptom: a key dispatch becomes a no-op or matches the wrong action. Caught by tests; fix in the binding declaration.
- **Phase 2 regression**: a delegation path forgets to call `deriveChromeFromAcct`. Symptom: chrome stale (e.g., status bar shows old folder counts after a folder jump). Caught by App-level tests asserting on chrome state. If subtle, revert just commit 2 and restore the Cmd-wrapped signals.
- **Phase 3 regression**: a row producer still doesn't honor its width. Symptom: visible misalignment at the right border. Caught by tmux captures. Fix in the producer, not in App.

Each phase commits independently and is independently revertable.
