# Pass 9h — ComposeTab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land ComposeTab — an inline compose surface wrapping a Catkin body editor and stacked address/subject text inputs. Wire `c`/`r`/`R`/`f` to open it. Send goes through the cache outbox via a new `cache.Account.QueueOutbound` helper that branches IMAP (two ops) vs JMAP (one op). Tidy seam ships as a no-op for Pass 9i to replace.

**Architecture:** App grows a `compose *ComposeTab` mode field. While compose is open, App routes keys into ComposeTab and renders it in place of AccountTab's right pane (chrome stays drawn). Backend gains an `IsJMAP() bool` predicate so the protocol branch lives in cache, not UI. ComposeTab is presentation-only — it emits `tea.Msg` values that App translates into cache ops.

**Tech Stack:** Go 1.26.1 · charmbracelet/bubbletea · charmbracelet/bubbles · charmbracelet/lipgloss · existing internal packages: `mail`, `mailjmap`, `mailimap`, `cache`, `compose`, `catkin`, `theme`, `config`.

**Spec:** `docs/superpowers/specs/2026-05-06-compose-tab-design.md`

---

## File Structure

**New:**
- `internal/ui/compose_tab.go` — `ComposeTab` model.
- `internal/ui/compose_tab_test.go` — focus, draft, dirty, key tests.

**Modified:**
- `internal/mail/backend.go` — add `IsJMAP() bool` to `Backend` interface.
- `internal/mail/mock.go` — `MockBackend.IsJMAP()` (`false`; tests override).
- `internal/mailjmap/backend.go` — `(*Backend).IsJMAP()` returns `true`.
- `internal/mailimap/backend.go` — `(*Backend).IsJMAP()` returns `false`.
- `internal/cache/ops.go` — add `(*Account).QueueOutbound`.
- `internal/cache/send_test.go` — add `TestQueueOutbound_*` cases.
- `internal/ui/keys.go` — add `ComposeKeys` struct + constructor.
- `internal/ui/app.go` — `compose *ComposeTab`, `composeOpen bool`, `tidy TidyFn`, `WithTidy`, `c`/`r`/`R`/`f` routing, `composeSendMsg` handler, `composeDiscardMsg` handler, View pane swap, WindowSizeMsg forward.
- `internal/ui/app_test.go` — App-level integration tests.
- `cmd/poplar/root.go` — wire `App.WithTidy(identityTidy)`.
- `docs/poplar/keybindings.md` — Compose context.
- `docs/poplar/wireframes.md` — fresh + reply-seeded compose wireframes.
- `.claude/rules/ui-invariants.md` — flesh out Compose subsection.

---

## Task 1: Backend.IsJMAP() predicate

**Files:**
- Modify: `internal/mail/backend.go` — add method to interface.
- Modify: `internal/mail/mock.go` — add MockBackend impl.
- Modify: `internal/mailjmap/backend.go` — add Backend impl.
- Modify: `internal/mailimap/backend.go` — add Backend impl.
- Test: `internal/mail/mock_test.go` (already asserts MockBackend implements Backend; no new test file).

- [ ] **Step 1: Add `IsJMAP` to the Backend interface**

In `internal/mail/backend.go`, inside the `Backend interface { ... }` block, append before the trailing `Updates()` method:

```go
	// IsJMAP reports whether this backend is the JMAP adapter. Used by
	// the cache outbox to decide whether the Sent copy is appended
	// separately (IMAP) or implicit in Send (JMAP).
	IsJMAP() bool
```

- [ ] **Step 2: Add MockBackend impl**

In `internal/mail/mock.go`, near the other one-line accessors (around the existing `AccountName`/`AccountEmail` block):

```go
// IsJMAP defaults to false. JMAP-shaped tests set this true via the
// exported field.
func (m *MockBackend) IsJMAP() bool { return m.isJMAP }
```

Add the field `isJMAP bool` to the `MockBackend` struct definition in the same file. Do not export a setter; tests construct `MockBackend{isJMAP: true}` directly via the struct literal.

- [ ] **Step 3: Add JMAP impl**

In `internal/mailjmap/backend.go`, add a one-line method on `*Backend` near the other accessors (search for `AccountEmail()` to find the cluster):

```go
func (b *Backend) IsJMAP() bool { return true }
```

- [ ] **Step 4: Add IMAP impl**

In `internal/mailimap/backend.go`, add a one-line method on `*Backend` in the same neighborhood:

```go
func (b *Backend) IsJMAP() bool { return false }
```

- [ ] **Step 5: Run interface conformance + build**

Run: `cd /home/glw907/Projects/poplar && go build ./... && go test ./internal/mail/...`
Expected: build PASS; `TestMockBackendImplementsBackend` PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/mail/backend.go internal/mail/mock.go internal/mailjmap/backend.go internal/mailimap/backend.go
git commit -m "Pass 9h: Backend.IsJMAP() predicate"
```

---

## Task 2: cache.Account.QueueOutbound

**Files:**
- Modify: `internal/cache/ops.go` — add method.
- Modify: `internal/cache/send_test.go` — add tests.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cache/send_test.go`:

```go
func TestQueueOutbound_JMAP_OneOp(t *testing.T) {
	t.Helper()
	mb := mail.NewMockBackend()
	mb.SetJMAP(true) // helper added in this same step — see below
	a := openTestAccount(t, mb)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"x@y.com"}}
	if err := a.QueueOutbound(context.Background(), "Sent", env, []byte("MIME bytes")); err != nil {
		t.Fatalf("QueueOutbound: %v", err)
	}

	rows := dumpOutbox(t, a)
	if len(rows) != 1 {
		t.Fatalf("want 1 outbox row, got %d", len(rows))
	}
	if rows[0].kind != "send" {
		t.Fatalf("want kind=send, got %q", rows[0].kind)
	}
}

func TestQueueOutbound_IMAP_TwoOps(t *testing.T) {
	t.Helper()
	mb := mail.NewMockBackend() // IsJMAP defaults to false
	a := openTestAccount(t, mb)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"x@y.com"}}
	if err := a.QueueOutbound(context.Background(), "Sent", env, []byte("MIME bytes")); err != nil {
		t.Fatalf("QueueOutbound: %v", err)
	}

	rows := dumpOutbox(t, a)
	if len(rows) != 2 {
		t.Fatalf("want 2 outbox rows, got %d", len(rows))
	}
	if rows[0].kind != "send" || rows[1].kind != "append" {
		t.Fatalf("want [send, append], got [%q, %q]", rows[0].kind, rows[1].kind)
	}
}
```

If `openTestAccount` and `dumpOutbox` don't exist with this exact shape, mirror the existing `TestQueueSendRoundTrip`/`TestQueueAppendRoundTrip` helpers — read `internal/cache/send_test.go` first to align with the actual fixture style. The two tests above are the behavioral assertions; adapt the harness calls only.

The `mb.SetJMAP(true)` is a convenience setter to add to mock.go in this task — exporting a setter is cleaner than the struct literal noted in Task 1 once tests need it. Keep both: the field is exported via the setter; tests now use `mb.SetJMAP(true)`.

Update `internal/mail/mock.go`:

```go
// SetJMAP overrides the default false return from IsJMAP. Test seam.
func (m *MockBackend) SetJMAP(v bool) { m.isJMAP = v }
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/ -run TestQueueOutbound -v`
Expected: FAIL — `QueueOutbound` undefined.

- [ ] **Step 3: Implement QueueOutbound**

Append to `internal/cache/ops.go`:

```go
// QueueOutbound enqueues outbound mail through the outbox. JMAP
// backends place the Sent copy atomically inside Send, so one op
// is enough. IMAP requires a separate APPEND for the Sent copy,
// so two ops are queued in order. The send op runs first; if it
// fails to enqueue, the append is not attempted.
func (a *Account) QueueOutbound(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) error {
	if _, err := a.QueueSend(ctx, sentFolder, env, mime); err != nil {
		return err
	}
	if a.Backend.IsJMAP() {
		return nil
	}
	_, err := a.QueueAppend(ctx, sentFolder, mail.FlagSeen, mime)
	return err
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/ -run TestQueueOutbound -v`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/ops.go internal/cache/send_test.go internal/mail/mock.go
git commit -m "Pass 9h: cache.Account.QueueOutbound (JMAP one-op, IMAP two-op)"
```

---

## Task 3: TidyFn seam on App

**Files:**
- Modify: `internal/ui/app.go` — add `TidyFn` type, App field, `WithTidy` setter, default no-op.
- Modify: `internal/ui/app_test.go` — verify default identity behavior.

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/app_test.go`:

```go
func TestApp_TidyDefaultIsIdentity(t *testing.T) {
	app := newTestApp(t)
	out, err := app.tidy(context.Background(), "hello\n")
	if err != nil {
		t.Fatalf("tidy: %v", err)
	}
	if out != "hello\n" {
		t.Fatalf("want identity passthrough, got %q", out)
	}
}

func TestApp_WithTidy_Replaces(t *testing.T) {
	app := newTestApp(t).WithTidy(func(_ context.Context, s string) (string, error) {
		return s + " [tidied]", nil
	})
	out, _ := app.tidy(context.Background(), "x")
	if out != "x [tidied]" {
		t.Fatalf("want WithTidy override, got %q", out)
	}
}
```

If `newTestApp` does not exist, build it the same way other tests in `app_test.go` construct an App for testing. Read the file first to follow existing conventions.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp_Tidy -v`
Expected: FAIL — `tidy` field/method does not exist.

- [ ] **Step 3: Add TidyFn + App field + setter**

In `internal/ui/app.go`:

After the `pendingEmptyConfirm` type (near the top of the file), add:

```go
// TidyFn rewrites the markdown body before MIME assembly. Pass 9h
// ships a no-op identity. Pass 9i swaps in Claude Tidy.
type TidyFn func(ctx context.Context, body string) (string, error)

func identityTidy(_ context.Context, body string) (string, error) {
	return body, nil
}
```

In the `App` struct, add a field next to `opener URLOpener`:

```go
	tidy   TidyFn
```

In `NewApp`, add to the returned literal:

```go
		tidy:   identityTidy,
```

After the existing `WithOpener` method, add:

```go
// WithTidy returns a copy of m with the body tidy seam replaced.
func (m App) WithTidy(fn TidyFn) App {
	m.tidy = fn
	return m
}
```

Add `"context"` to the imports if not already present.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp_Tidy -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 9h: TidyFn seam on App, no-op default"
```

---

## Task 4: ComposeTab skeleton + width contract

**Files:**
- Create: `internal/ui/compose_tab.go`
- Create: `internal/ui/compose_tab_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/compose_tab_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func newTestCompose(t *testing.T) *ComposeTab {
	t.Helper()
	styles := NewStyles(theme.OneDark())
	c := NewComposeTab(styles, theme.OneDark(), "geoff@907.life", SimpleIcons)
	c.SetSize(80, 24)
	return c
}

func TestComposeTab_View_HonorsAssignedWidth(t *testing.T) {
	c := newTestCompose(t)
	c.SetSize(60, 20)
	for i, line := range strings.Split(c.View(), "\n") {
		if w := lipgloss.Width(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60: %q", i, w, line)
		}
	}
}

func TestComposeTab_View_HasHeaderRows(t *testing.T) {
	c := newTestCompose(t)
	v := c.View()
	for _, want := range []string{"From:", "To:", "Cc:", "Bcc:", "Subject:"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View missing %q\n%s", want, v)
		}
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab -v`
Expected: FAIL — `NewComposeTab` undefined.

- [ ] **Step 3: Create ComposeTab skeleton**

Create `internal/ui/compose_tab.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
)

// ComposeTab is the inline compose surface. Owns header inputs, body
// editor, focus, and the Draft/dirty contract. Presentation-only —
// does not hold a *cache.Account; send and discard surface as
// tea.Msg values that App translates into cache ops.
type ComposeTab struct {
	styles Styles
	icons  IconSet

	from string

	to      textinput.Model
	cc      textinput.Model
	bcc     textinput.Model
	subject textinput.Model
	editor  compose.Editor

	focus int    // 0=To, 1=Cc, 2=Bcc, 3=Subject, 4=Body
	err   string // inline error row above the rule, "" when none

	width  int
	height int
}

const (
	composeFocusTo = iota
	composeFocusCc
	composeFocusBcc
	composeFocusSubject
	composeFocusBody
)

const composeLabelWidth = 9 // "Subject: " is the longest

// NewComposeTab returns a fresh, empty ComposeTab focused on To.
func NewComposeTab(styles Styles, t *theme.CompiledTheme, self string, icons IconSet) *ComposeTab {
	mk := func(prompt string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = ""
		_ = prompt // header label is rendered separately
		return ti
	}
	c := &ComposeTab{
		styles:  styles,
		icons:   icons,
		from:    self,
		to:      mk("To"),
		cc:      mk("Cc"),
		bcc:     mk("Bcc"),
		subject: mk("Subject"),
		editor:  compose.NewCatkinEditor(),
	}
	c.to.Focus()
	c.focus = composeFocusTo
	return c
}

func (c *ComposeTab) Init() tea.Cmd {
	return c.editor.Init()
}

func (c *ComposeTab) SetSize(w, h int) {
	c.width = w
	c.height = h

	inputW := w - composeLabelWidth - 1 // -1 for right border space
	if inputW < 1 {
		inputW = 1
	}
	c.to.Width = inputW
	c.cc.Width = inputW
	c.bcc.Width = inputW
	c.subject.Width = inputW

	bodyHeight := h - 5 - 1 // 5 header rows + 1 rule row
	if c.err != "" {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	c.editor.SetSize(w, bodyHeight)
}

// View renders the compose surface. Self-enforces width: every
// returned line is exactly c.width cells.
func (c *ComposeTab) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	var rows []string
	rows = append(rows, c.headerRow("From:", c.from))
	rows = append(rows, c.headerRow("To:", c.to.View()))
	rows = append(rows, c.headerRow("Cc:", c.cc.View()))
	rows = append(rows, c.headerRow("Bcc:", c.bcc.View()))
	rows = append(rows, c.headerRow("Subject:", c.subject.View()))
	if c.err != "" {
		rows = append(rows, c.padRow(c.styles.Error.Render(c.err)))
	}
	rows = append(rows, c.padRow(strings.Repeat("─", c.width)))
	for _, line := range strings.Split(c.editor.View(), "\n") {
		rows = append(rows, c.padRow(line))
	}
	for len(rows) < c.height {
		rows = append(rows, c.padRow(""))
	}
	if len(rows) > c.height {
		rows = rows[:c.height]
	}
	return strings.Join(rows, "\n")
}

func (c *ComposeTab) headerRow(label, value string) string {
	pad := composeLabelWidth - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}
	row := label + strings.Repeat(" ", pad) + value
	return c.padRow(row)
}

func (c *ComposeTab) padRow(s string) string {
	w := lipgloss.Width(s)
	if w >= c.width {
		return displayTruncate(s, c.width, 1)
	}
	return s + strings.Repeat(" ", c.width-w)
}
```

If `c.styles.Error` does not exist, use `lipgloss.NewStyle().Foreground(lipgloss.Color("9"))` as a temporary inline style — but check `internal/ui/styles.go` first; the project already exposes an error color slot. Use whatever the existing `error_banner.go` uses, by example.

If `displayTruncate` has a different signature, mirror its existing call sites under `internal/ui/`.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab -v`
Expected: PASS — both tests.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose_tab.go internal/ui/compose_tab_test.go
git commit -m "Pass 9h: ComposeTab skeleton with width contract"
```

---

## Task 5: ComposeTab focus model (Tab/Shift+Tab + Esc)

**Files:**
- Modify: `internal/ui/compose_tab.go` — Update method, focus helpers.
- Modify: `internal/ui/compose_tab_test.go` — focus cycling tests.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/compose_tab_test.go`:

```go
import "github.com/charmbracelet/bubbles/key"

func sendKey(c *ComposeTab, k string) *ComposeTab {
	msg := keyMsgFromString(k)
	next, _ := c.Update(msg)
	return next
}

func TestComposeTab_TabCyclesFields(t *testing.T) {
	c := newTestCompose(t)
	want := []int{composeFocusCc, composeFocusBcc, composeFocusSubject, composeFocusBody, composeFocusTo}
	for i, w := range want {
		c = sendKey(c, "tab")
		if c.focus != w {
			t.Fatalf("step %d: focus = %d, want %d", i, c.focus, w)
		}
	}
}

func TestComposeTab_ShiftTabCyclesBackward(t *testing.T) {
	c := newTestCompose(t)
	c = sendKey(c, "shift+tab")
	if c.focus != composeFocusBody {
		t.Fatalf("Shift+Tab from To should wrap to Body, got %d", c.focus)
	}
}

func TestComposeTab_EscFromBodyReturnsToSubject(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusBody
	c.editor.Focus()
	c.to.Blur()
	c = sendKey(c, "esc")
	if c.focus != composeFocusSubject {
		t.Fatalf("Esc from Body should focus Subject, got %d", c.focus)
	}
}

func TestComposeTab_EscFromHeaderReturnsToBody(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusTo
	c = sendKey(c, "esc")
	if c.focus != composeFocusBody {
		t.Fatalf("Esc from header should focus Body, got %d", c.focus)
	}
}
```

`keyMsgFromString` — helper that wraps `tea.KeyMsg` from a key.String() form. Look in `internal/ui/keys_test.go` or similar tests for an existing helper. If none, write one inline:

```go
func keyMsgFromString(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
```

Place it once in `compose_tab_test.go` and reuse across the file. Also import `key` only if needed (the helper above does not require it).

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab_Tab -v`
Expected: FAIL — `Update` not implemented.

- [ ] **Step 3: Implement Update with focus cycling**

Append to `internal/ui/compose_tab.go`:

```go
// Update implements tea.Model semantics. Returns *ComposeTab so
// callers can chain without a type assertion.
func (c *ComposeTab) Update(msg tea.Msg) (*ComposeTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			c.advanceFocus(+1)
			return c, nil
		case tea.KeyShiftTab:
			c.advanceFocus(-1)
			return c, nil
		case tea.KeyEsc:
			if c.focus == composeFocusBody {
				c.setFocus(composeFocusSubject)
			} else {
				c.setFocus(composeFocusBody)
			}
			return c, nil
		}
	}

	// Forward to the focused field.
	var cmd tea.Cmd
	switch c.focus {
	case composeFocusTo:
		c.to, cmd = c.to.Update(msg)
	case composeFocusCc:
		c.cc, cmd = c.cc.Update(msg)
	case composeFocusBcc:
		c.bcc, cmd = c.bcc.Update(msg)
	case composeFocusSubject:
		c.subject, cmd = c.subject.Update(msg)
	case composeFocusBody:
		c.editor, cmd = c.editor.Update(msg)
	}
	return c, cmd
}

func (c *ComposeTab) advanceFocus(delta int) {
	const fields = 5
	c.setFocus(((c.focus + delta) + fields) % fields)
}

func (c *ComposeTab) setFocus(target int) {
	c.to.Blur()
	c.cc.Blur()
	c.bcc.Blur()
	c.subject.Blur()
	c.editor.Blur()
	switch target {
	case composeFocusTo:
		c.to.Focus()
	case composeFocusCc:
		c.cc.Focus()
	case composeFocusBcc:
		c.bcc.Focus()
	case composeFocusSubject:
		c.subject.Focus()
	case composeFocusBody:
		c.editor.Focus()
	}
	c.focus = target
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose_tab.go internal/ui/compose_tab_test.go
git commit -m "Pass 9h: ComposeTab focus cycling and Esc-as-focus key"
```

---

## Task 6: ComposeTab.Draft() / IsDirty / Seed

**Files:**
- Modify: `internal/ui/compose_tab.go`
- Modify: `internal/ui/compose_tab_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/compose_tab_test.go`:

```go
func TestComposeTab_DraftReflectsInputs(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("alice@example.com, bob@example.com")
	c.cc.SetValue("c@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("hello world")

	d, err := c.Draft()
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if len(d.To) != 2 || d.To[0].Address != "alice@example.com" {
		t.Fatalf("To not parsed: %+v", d.To)
	}
	if len(d.Cc) != 1 || d.Cc[0].Address != "c@example.com" {
		t.Fatalf("Cc not parsed: %+v", d.Cc)
	}
	if d.Subject != "hi" || d.Body != "hello world" {
		t.Fatalf("subject/body wrong: %q %q", d.Subject, d.Body)
	}
	if d.From.Address != "geoff@907.life" {
		t.Fatalf("From wrong: %+v", d.From)
	}
}

func TestComposeTab_DraftBadAddressFails(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("not an address")
	if _, err := c.Draft(); err == nil {
		t.Fatalf("want parse error on bad address, got nil")
	}
}

func TestComposeTab_IsDirty(t *testing.T) {
	c := newTestCompose(t)
	if c.IsDirty() {
		t.Fatalf("fresh compose should not be dirty")
	}
	c.editor.SetValue("hi")
	if !c.IsDirty() {
		t.Fatalf("body content should mark dirty")
	}
}

func TestComposeTab_Seed(t *testing.T) {
	c := newTestCompose(t)
	d := compose.Draft{
		Subject: "Re: hi",
		Body:    "> original\n\n",
	}
	d.To = append(d.To, gomailAddress("alice@example.com"))
	c.Seed(d)
	if c.subject.Value() != "Re: hi" {
		t.Fatalf("subject not seeded: %q", c.subject.Value())
	}
	if c.editor.Value() != "> original\n\n" {
		t.Fatalf("body not seeded: %q", c.editor.Value())
	}
	if c.to.Value() != "alice@example.com" {
		t.Fatalf("To not seeded: %q", c.to.Value())
	}
}
```

`gomailAddress` is a one-line helper:

```go
func gomailAddress(addr string) gomail.Address {
	return gomail.Address{Address: addr}
}
```

Add the imports `"github.com/glw907/poplar/internal/compose"` and `gomail "github.com/emersion/go-message/mail"` to the test file as needed.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab_Draft -v`
Expected: FAIL — `Draft`/`IsDirty`/`Seed` undefined.

- [ ] **Step 3: Implement Draft / IsDirty / Seed**

Append to `internal/ui/compose_tab.go`:

```go
// Draft rebuilds a compose.Draft from the current input values.
// Address fields are parsed via content.ParseAddressList; a parse
// error is returned (and the inline err row is set) if any
// non-empty address field fails to parse.
func (c *ComposeTab) Draft() (compose.Draft, error) {
	to, err := parseAddrField(c.to.Value(), "To")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	cc, err := parseAddrField(c.cc.Value(), "Cc")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	bcc, err := parseAddrField(c.bcc.Value(), "Bcc")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	c.err = ""
	return compose.Draft{
		From:    gomail.Address{Address: c.from},
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: c.subject.Value(),
		Body:    c.editor.Value(),
	}, nil
}

// IsDirty reports whether the compose has any user-entered content.
// A seeded reply with an empty body but a To header counts as dirty.
func (c *ComposeTab) IsDirty() bool {
	return c.to.Value() != "" || c.cc.Value() != "" || c.bcc.Value() != "" ||
		c.subject.Value() != "" || c.editor.Value() != ""
}

// Seed populates the inputs from d. Called when reply/forward
// pre-fills the compose surface.
func (c *ComposeTab) Seed(d compose.Draft) {
	c.to.SetValue(joinAddresses(d.To))
	c.cc.SetValue(joinAddresses(d.Cc))
	c.bcc.SetValue(joinAddresses(d.Bcc))
	c.subject.SetValue(d.Subject)
	c.editor.SetValue(d.Body)
}

func parseAddrField(raw, label string) ([]gomail.Address, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	addrs, err := content.ParseAddressList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return addrs, nil
}

func joinAddresses(addrs []gomail.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a.Name != "" {
			parts = append(parts, fmt.Sprintf("%q <%s>", a.Name, a.Address))
		} else {
			parts = append(parts, a.Address)
		}
	}
	return strings.Join(parts, ", ")
}
```

Add imports: `"fmt"`, `gomail "github.com/emersion/go-message/mail"`, `"github.com/glw907/poplar/internal/content"`.

If `content.ParseAddressList` returns a different concrete type than `[]gomail.Address`, adapt — read `internal/content/` to confirm. Per ADR-0156 it returns `[]gomail.Address`.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab -v`
Expected: PASS — all ComposeTab tests.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose_tab.go internal/ui/compose_tab_test.go
git commit -m "Pass 9h: ComposeTab.Draft/IsDirty/Seed with address parsing"
```

---

## Task 7: ComposeKeys + Ctrl+X / Ctrl+C handling

**Files:**
- Modify: `internal/ui/keys.go` — add `ComposeKeys`.
- Modify: `internal/ui/compose_tab.go` — emit `ComposeSendMsg` / `ComposeCancelMsg`.
- Modify: `internal/ui/compose_tab_test.go` — key tests.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/compose_tab_test.go`:

```go
func TestComposeTab_CtrlXEmitsSendMsg(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("alice@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("body")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil {
		t.Fatal("Ctrl+X should return a Cmd that emits ComposeSendMsg")
	}
	msg := cmd()
	send, ok := msg.(ComposeSendMsg)
	if !ok {
		t.Fatalf("want ComposeSendMsg, got %T", msg)
	}
	if send.Draft.Subject != "hi" {
		t.Fatalf("send carries wrong draft: %+v", send.Draft)
	}
}

func TestComposeTab_CtrlCEmitsCancelMsg(t *testing.T) {
	c := newTestCompose(t)
	c.editor.SetValue("dirty")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should return a Cmd")
	}
	msg := cmd()
	cancel, ok := msg.(ComposeCancelMsg)
	if !ok {
		t.Fatalf("want ComposeCancelMsg, got %T", msg)
	}
	if !cancel.Dirty {
		t.Fatalf("dirty draft should set Dirty=true")
	}
}

func TestComposeTab_CtrlXBadAddressInlinesError(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("not an address")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd != nil {
		t.Fatalf("Ctrl+X with bad address should not emit send")
	}
	if c.err == "" {
		t.Fatalf("inline err row should be set")
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab_Ctrl -v`
Expected: FAIL — Ctrl+X/C not handled.

- [ ] **Step 3: Add ComposeKeys + Send/Cancel msgs**

In `internal/ui/keys.go`, add at the bottom (after the existing `AccountKeys`/`GlobalKeys` definitions):

```go
// ComposeKeys are the bindings active while ComposeTab has focus.
// Per ADR-0076 text-entry surfaces are exempt from the modifier-
// free rule; Ctrl chords here are deliberate.
type ComposeKeys struct {
	Send       key.Binding
	Cancel     key.Binding
	NextField  key.Binding
	PrevField  key.Binding
	EscapeBody key.Binding
}

func NewComposeKeys() ComposeKeys {
	return ComposeKeys{
		Send:       key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("^X", "send")),
		Cancel:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "cancel")),
		NextField:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "next")),
		PrevField:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧⇥", "prev")),
		EscapeBody: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "focus")),
	}
}
```

In `internal/ui/compose_tab.go`, add the message types near the type declarations:

```go
// ComposeSendMsg is emitted by ComposeTab when the user presses
// Ctrl+X with a draft that parses cleanly. App's handler runs the
// tidy seam, assembles MIME, and queues the outbox op.
type ComposeSendMsg struct {
	Draft compose.Draft
}

// ComposeCancelMsg is emitted by ComposeTab when the user presses
// Ctrl+C. App opens the discard ConfirmModal when Dirty, otherwise
// closes compose immediately.
type ComposeCancelMsg struct {
	Dirty bool
}
```

In `Update`, add a case before the `tea.KeyTab`/`KeyShiftTab` branch:

```go
		case tea.KeyCtrlX:
			d, err := c.Draft()
			if err != nil {
				return c, nil
			}
			return c, func() tea.Msg { return ComposeSendMsg{Draft: d} }
		case tea.KeyCtrlC:
			dirty := c.IsDirty()
			return c, func() tea.Msg { return ComposeCancelMsg{Dirty: dirty} }
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestComposeTab -v`
Expected: PASS — all ComposeTab tests.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/keys.go internal/ui/compose_tab.go internal/ui/compose_tab_test.go
git commit -m "Pass 9h: ComposeKeys + Ctrl+X/Ctrl+C msg emission"
```

---

## Task 8: App: `c` opens compose, View pane swap, WindowSizeMsg forward

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_C_OpensCompose(t *testing.T) {
	app := newTestApp(t)
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	if !a.composeOpen {
		t.Fatalf("c should set composeOpen")
	}
	if a.compose == nil {
		t.Fatalf("c should construct ComposeTab")
	}
}

func TestApp_View_RendersComposeWhenOpen(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	v := a.View()
	if !strings.Contains(v, "From:") || !strings.Contains(v, "Subject:") {
		t.Fatalf("View should include compose headers when composeOpen")
	}
}
```

`handleWindowSize` is the existing helper used elsewhere in `app_test.go`; mirror its current shape — read the test file first.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp_C -v`
Expected: FAIL — `c` not handled.

- [ ] **Step 3: Add App fields + key wiring + View swap**

In `internal/ui/app.go`:

Add fields to the `App` struct:

```go
	compose     *ComposeTab
	composeOpen bool
```

Add to the global key handler (find the existing key dispatch — likely a switch statement matching runes). Add a branch for `'c'` before delegating to AccountTab:

```go
		case "c":
			if !m.composeOpen {
				m.compose = NewComposeTab(m.styles, /* theme: */ accountTabThemeRef(m), m.acct.AccountEmail(), m.icons)
				m.compose.SetSize(m.acct.RightPaneWidth(), m.acct.RightPaneHeight())
				m.composeOpen = true
				return m, m.compose.Init()
			}
```

If `accountTabThemeRef(m)` does not exist, expose the theme via an `AccountTab.Theme()` accessor or thread the theme through `App` directly — read how `Viewer` got its theme in `account_tab.go` and follow the same pattern.

If `RightPaneWidth`/`RightPaneHeight` accessors don't exist on AccountTab, add them — they should return whatever dims AccountTab already computes for the right pane (msglist/viewer area). Mirror existing accessors (`SelectedFolderCounts`, etc.).

Add a Update branch for `ComposeSendMsg` and `ComposeCancelMsg`:

```go
	case ComposeSendMsg:
		// Handled in Task 9. For now, close compose.
		m.composeOpen = false
		m.compose = nil
		return m, nil
	case ComposeCancelMsg:
		// Handled in Task 10. For now, close compose unconditionally.
		m.composeOpen = false
		m.compose = nil
		return m, nil
```

In the `WindowSizeMsg` handler, after the existing forwards, add:

```go
		if m.compose != nil {
			m.compose.SetSize(/* same dims used for the right pane */)
		}
```

Use the same width/height the AccountTab already uses for its right pane. If derivation is non-trivial, factor a helper `(App) rightPaneSize() (w, h int)` and reuse from both the `c` open branch and WindowSizeMsg.

In `View()`, find the place AccountTab renders its right pane (via row-by-row join with the sidebar). Add a branch: if `m.composeOpen && m.compose != nil`, emit `m.compose.View()` rows in place of AccountTab's right pane rows.

The exact integration depends on the existing `App.View()` shape. Read it before editing. The chrome (top line, status bar, footer) stays drawn from the existing path — only the right pane content swaps.

While `composeOpen`, route key dispatch into compose first:

```go
	case tea.KeyMsg:
		if m.composeOpen && m.compose != nil {
			next, cmd := m.compose.Update(msg)
			m.compose = next
			return m, cmd
		}
		// existing key dispatch follows
```

This block goes near the top of the App key handler, after the always-on overlays (confirm, conflict, outbox, help) and before AccountTab delegation.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp -v`
Expected: PASS — `TestApp_C_OpensCompose`, `TestApp_View_RendersComposeWhenOpen`, plus existing tests still green.

- [ ] **Step 5: Verify build + full test suite**

Run: `cd /home/glw907/Projects/poplar && go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go internal/ui/account_tab.go
git commit -m "Pass 9h: App opens compose on 'c', View pane swap, key routing"
```

---

## Task 9: App: composeSendMsg → tidy → assemble → QueueOutbound

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmds.go` — add `composeSendCmd`.
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_ComposeSend_QueuesOutboundAndClosesCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	a.compose.to.SetValue("alice@example.com")
	a.compose.subject.SetValue("hi")
	a.compose.editor.SetValue("hello")

	d, err := a.compose.Draft()
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	out, cmd := a.Update(ComposeSendMsg{Draft: d})
	a = out.(App)
	if a.composeOpen {
		t.Fatalf("composeOpen should be false after ComposeSendMsg")
	}
	if cmd == nil {
		t.Fatalf("ComposeSendMsg should return a Cmd that runs QueueOutbound")
	}
	// Drain the Cmd; the test cache should now hold a queued send.
	_ = cmd()
	if depth := a.acct.OutboxDepth(); depth.Inflight == 0 {
		t.Fatalf("outbox should have at least one queued op, got %+v", depth)
	}
}

func TestApp_ComposeSend_NoSentFolder_SurfacesError(t *testing.T) {
	app := newTestAppWithoutSentFolder(t) // helper: sets folder list with no Sent
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	a.compose.to.SetValue("alice@example.com")
	a.compose.subject.SetValue("hi")
	a.compose.editor.SetValue("hello")
	d, _ := a.compose.Draft()

	out, cmd := a.Update(ComposeSendMsg{Draft: d})
	a = out.(App)
	if !a.composeOpen {
		t.Fatalf("Sent folder missing should keep compose open")
	}
	if cmd != nil {
		_ = cmd() // may return nil msg
	}
	if a.compose.err == "" {
		t.Fatalf("missing Sent folder should surface inline err")
	}
}
```

If `newTestAppWithoutSentFolder` doesn't exist, add it next to `newTestApp` — it constructs an App where `mail.MockBackend.ListFolders()` returns a list without a Sent role.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp_ComposeSend -v`
Expected: FAIL — handler is a stub.

- [ ] **Step 3: Implement send handler**

In `internal/ui/cmds.go`, add:

```go
// composeSendCmd runs the tidy seam, assembles MIME, and queues
// the outbox op via cache.Account.QueueOutbound. Returns an
// ErrorMsg on any failure.
func composeSendCmd(acct *cache.Account, sentFolder string, tidy TidyFn, d compose.Draft) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		body, err := tidy(ctx, d.Body)
		if err != nil {
			return ErrorMsg{Op: "tidy body", Err: err}
		}
		d.Body = body
		mime, err := compose.AssembleMIME(d, time.Now())
		if err != nil {
			return ErrorMsg{Op: "assemble MIME", Err: err}
		}
		env := envelopeFromDraft(d)
		if err := acct.QueueOutbound(ctx, sentFolder, env, mime); err != nil {
			return ErrorMsg{Op: "queue outbound", Err: err}
		}
		return composeSentMsg{}
	}
}

func envelopeFromDraft(d compose.Draft) mail.Envelope {
	env := mail.Envelope{From: d.From.Address}
	for _, a := range d.To {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	for _, a := range d.Cc {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	for _, a := range d.Bcc {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	return env
}

// composeSentMsg fires after QueueOutbound returns. App uses it to
// stage the "Sending…" toast (no inverse, undo not advertised).
type composeSentMsg struct{}
```

Add imports `"context"`, `"time"`, plus `compose`/`cache`/`mail` paths.

In `internal/ui/app.go`, replace the Task 8 stub for `ComposeSendMsg`:

```go
	case ComposeSendMsg:
		sent := resolveSentFolder(m.acct)
		if sent == "" {
			if m.compose != nil {
				m.compose.err = "no Sent folder configured"
			}
			return m, nil
		}
		m.composeOpen = false
		c := m.compose
		m.compose = nil
		_ = c
		return m, composeSendCmd(m.acct, sent, m.tidy, msg.Draft)

	case composeSentMsg:
		// Stage a non-undoable "Sending…" toast. Reuse the chrome
		// row by setting pendingAction with op=opSending; renderToast
		// suppresses [u undo] when op is non-triage.
		m.toast = pendingAction{
			op:       opSending,
			deadline: m.now().Add(2 * time.Second),
		}
		return m, nil
```

Add `opSending` to `internal/ui/toast.go` next to existing `triageOp` constants. Update `renderToast` to render `Sending…` for `opSending` without an undo affordance.

Add `resolveSentFolder` helper near `composeSendCmd` in cmds.go:

```go
// resolveSentFolder picks the Sent folder for outbound mail. Prefers
// the cached folder list's RoleSent classification; falls back to the
// first folder named "Sent" (case-insensitive). Returns "" if none.
func resolveSentFolder(acct *cache.Account) string {
	folders, err := acct.ListFolders()
	if err != nil {
		return ""
	}
	classified := mail.Classify(folders)
	for _, f := range classified {
		if f.Role == mail.RoleSent {
			return f.Folder.Name
		}
	}
	for _, f := range folders {
		if strings.EqualFold(f.Name, "Sent") {
			return f.Name
		}
	}
	return ""
}
```

If `mail.Classify`'s shape differs (e.g. `ClassifiedFolder` field naming), adjust to match what `internal/mail/` already exposes — read it once before writing.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run TestApp_ComposeSend -v`
Expected: PASS — both cases.

- [ ] **Step 5: Run the full suite**

Run: `cd /home/glw907/Projects/poplar && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/cmds.go internal/ui/toast.go internal/ui/app_test.go
git commit -m "Pass 9h: composeSendMsg handler — tidy, assemble, QueueOutbound"
```

---

## Task 10: App: seed paths (`r`/`R`/`f`) + cancel-confirm flow

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmds.go` — `composeSeedCmd`.
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_R_OpensReplySeededCompose(t *testing.T) {
	app := newTestAppWithMessage(t) // helper: cache loaded with one message
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open viewer first; reply seeds from selected message.
	app2, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = app2.(App)

	out, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	app = out.(App)
	if cmd == nil {
		t.Fatalf("r should return a Cmd to fetch parent body and seed compose")
	}
	// Drain the Cmd to deliver the seeded-compose msg.
	for msg := cmd(); msg != nil; {
		out, cmd2 := app.Update(msg)
		app = out.(App)
		if cmd2 == nil {
			break
		}
		msg = cmd2()
	}
	if !app.composeOpen {
		t.Fatalf("r should result in composeOpen")
	}
	if !strings.HasPrefix(app.compose.subject.Value(), "Re:") {
		t.Fatalf("subject should be Re:-prefixed, got %q", app.compose.subject.Value())
	}
}

func TestApp_CtrlC_DirtyOpensConfirm(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	a.compose.editor.SetValue("dirty")
	out, _ := a.Update(ComposeCancelMsg{Dirty: true})
	a = out.(App)
	if !a.confirm.Visible() {
		t.Fatalf("dirty cancel should open ConfirmModal")
	}
	if !a.composeOpen {
		t.Fatalf("compose should remain open while confirming")
	}
}

func TestApp_CtrlC_EmptyClosesImmediately(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	out, _ := a.Update(ComposeCancelMsg{Dirty: false})
	a = out.(App)
	if a.composeOpen {
		t.Fatalf("empty cancel should close immediately")
	}
}

func TestApp_ConfirmModalYes_DiscardsCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	next, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := next.(App)
	a.compose.editor.SetValue("dirty")
	out, _ := a.Update(ComposeCancelMsg{Dirty: true})
	a = out.(App)
	out, _ = a.Update(ConfirmModalYesMsg{})
	a = out.(App)
	if a.composeOpen {
		t.Fatalf("Yes on discard should close compose")
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run "TestApp_R_OpensReply|TestApp_CtrlC|TestApp_ConfirmModalYes_DiscardsCompose" -v`
Expected: FAIL — handlers stubbed in Task 8.

- [ ] **Step 3: Implement seed paths**

In `internal/ui/cmds.go`, add:

```go
// composeSeedCmd fetches the parent body for reply/forward seeding,
// builds the Draft via compose.Seed*, and returns a composeSeededMsg.
type composeSeedKind int

const (
	seedReply composeSeedKind = iota
	seedReplyAll
	seedForward
)

func composeSeedCmd(acct *cache.Account, parent mail.MessageInfo, self string, kind composeSeedKind) tea.Cmd {
	return func() tea.Msg {
		body, err := acct.FetchBody(context.Background(), parent.UID)
		if err != nil {
			return ErrorMsg{Op: "fetch parent body", Err: err}
		}
		var d compose.Draft
		switch kind {
		case seedReply:
			d = compose.SeedReply(parent, body)
		case seedReplyAll:
			d = compose.SeedReplyAll(parent, body, gomail.Address{Address: self})
		case seedForward:
			d = compose.SeedForward(parent, body)
		}
		d.From = gomail.Address{Address: self}
		return composeSeededMsg{Draft: d}
	}
}

// composeSeededMsg carries a pre-filled draft from r/R/f.
type composeSeededMsg struct {
	Draft compose.Draft
}
```

If `acct.FetchBody` has a different signature (no ctx, or returns more values), adapt. Read `internal/cache/` to confirm.

In `internal/ui/app.go`, add key handler branches near the existing global key dispatch (before AccountTab delegation):

```go
		case "r", "R", "f":
			parent, ok := m.selectedMessage()
			if !ok {
				return m, nil
			}
			kind := seedReply
			if msg.String() == "R" {
				kind = seedReplyAll
			} else if msg.String() == "f" {
				kind = seedForward
			}
			return m, composeSeedCmd(m.acct, parent, m.acct.AccountEmail(), kind)
```

`selectedMessage()` is a helper to add: returns the currently-cursor-selected `mail.MessageInfo` from the active surface (viewer if open, msglist otherwise). Mirror existing accessors on AccountTab.

Add a `composeSeededMsg` handler:

```go
	case composeSeededMsg:
		m.compose = NewComposeTab(m.styles, /* theme ref */ , m.acct.AccountEmail(), m.icons)
		w, h := m.rightPaneSize()
		m.compose.SetSize(w, h)
		m.compose.Seed(msg.Draft)
		m.composeOpen = true
		return m, m.compose.Init()
```

- [ ] **Step 4: Implement cancel-confirm flow**

Replace the Task 8 stub for `ComposeCancelMsg` in `internal/ui/app.go`:

```go
	case ComposeCancelMsg:
		if !msg.Dirty {
			m.composeOpen = false
			m.compose = nil
			return m, nil
		}
		m.confirm = m.confirm.Show("Discard draft?", "Yes (y)", "No (n)")
		m.pendingComposeDiscard = true
		return m, nil
```

Add `pendingComposeDiscard bool` to the `App` struct.

Update the existing `ConfirmModalYesMsg` handler — it currently has a `pendingEmpty` branch. Add a new branch:

```go
	case ConfirmModalYesMsg:
		switch {
		case m.pendingComposeDiscard:
			m.pendingComposeDiscard = false
			m.composeOpen = false
			m.compose = nil
			return m, nil
		case m.pendingEmpty.folder != "":
			// existing branch unchanged
		}
```

Also update the `ConfirmModalNoMsg` handler (or whatever the no/cancel msg is named) to clear `pendingComposeDiscard` without closing compose.

- [ ] **Step 5: Run tests, verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/ui/ -run "TestApp_R_OpensReply|TestApp_CtrlC|TestApp_ConfirmModalYes_DiscardsCompose" -v`
Expected: PASS.

- [ ] **Step 6: Run the full suite**

Run: `cd /home/glw907/Projects/poplar && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/cmds.go internal/ui/app_test.go
git commit -m "Pass 9h: r/R/f seed compose; Ctrl+C discard confirm flow"
```

---

## Task 11: Wire identityTidy in cmd/poplar; docs + tmux goldens; pass-end

**Files:**
- Modify: `cmd/poplar/root.go`
- Modify: `docs/poplar/keybindings.md`
- Modify: `docs/poplar/wireframes.md`
- Modify: `.claude/rules/ui-invariants.md`
- Add: tmux capture artifacts (verified manually).

- [ ] **Step 1: Wire identityTidy in root.go**

In `cmd/poplar/root.go`, find where `ui.NewApp` is called and the resulting `App` is passed to `tea.NewProgram`. Add a chain call:

```go
	app := ui.NewApp(t, acct, uiCfg, icons).
		WithOpener(/* existing opener */).
		WithTidy(func(_ context.Context, body string) (string, error) {
			return body, nil
		})
```

If the existing call already chains `WithOpener`, just append `.WithTidy(...)`. Add `"context"` to imports if needed.

- [ ] **Step 2: Update docs/poplar/keybindings.md**

Add a new "Compose" section after the existing "Reply & Compose" table:

```markdown
## Compose context

Bindings active while ComposeTab has focus. Per ADR-0076, text-entry
surfaces are exempt from the modifier-free rule.

| Key       | Action               |
|-----------|----------------------|
| `Tab`     | Next field           |
| `⇧Tab`    | Previous field       |
| `Enter`   | Advance (in headers) |
| `Esc`     | Toggle focus headers ↔ body |
| `^X`      | Send                 |
| `^C`      | Cancel (confirms if dirty) |
```

Update the responsive footer entries to include the compose context: drop rank table grows a row for `^X send  ^C cancel` at rank 1.

- [ ] **Step 3: Update docs/poplar/wireframes.md**

Add two wireframes — a fresh `c` compose at 120×40, and a reply-seeded compose at 120×40 with `Re:` subject and quoted body. Use the wireframe shape from the spec doc verbatim, ASCII-art only.

- [ ] **Step 4: Update .claude/rules/ui-invariants.md**

Replace the existing two-line "Compose (planned)" subsection under `## Components` with the implemented contract:

```markdown
### Compose

- `ComposeTab` (`internal/ui/compose_tab.go`) is the inline compose
  surface. Owns five focusable fields: To/Cc/Bcc/Subject as
  `bubbles/textinput`, Body as `compose.Editor` (CatkinEditor in
  v1). Cc/Bcc rows always visible, empty when unused.
- Focus model: `Tab`/`Shift+Tab` cycles To→Cc→Bcc→Subject→Body and
  wraps; `Enter` in a header advances; `Esc` is a focus key only
  (Body→Subject; header→Body); `Esc` never closes.
- `Ctrl+X` sends — emits `ComposeSendMsg` with the assembled draft.
  `Ctrl+C` cancels — emits `ComposeCancelMsg{Dirty}`; App opens
  `ConfirmModal` when Dirty, closes immediately otherwise. Per
  ADR-0076 text-entry surfaces are exempt from the modifier-free
  rule; these chords coexist with Catkin's `Ctrl+B/I/K/L/Q/Space`.
- App owns the compose lifecycle: `compose *ComposeTab` +
  `composeOpen bool` + `tidy TidyFn`. `c` opens fresh; `r`/`R`/`f`
  open via `composeSeedCmd` after fetching parent body. Send path
  runs the tidy seam, calls `compose.AssembleMIME`, then
  `cache.Account.QueueOutbound` (one op JMAP, two ops IMAP). Sent
  folder resolves via `mail.Classify` `RoleSent`, with
  case-fold "Sent" name fallback; missing surfaces inline as
  `c.err`.
- Single-instance for 9h. Drafts persistence (multi-compose) is
  9h.5; address autocomplete is 9.1; signatures + identities is
  9.4.
```

- [ ] **Step 5: Run make check**

Run: `cd /home/glw907/Projects/poplar && make check`
Expected: PASS — fmt, vet, voice, tests all green.

- [ ] **Step 6: Run /simplify on the diff**

Use the `simplify` skill against the full Pass 9h diff. Apply any genuine wins it surfaces; don't apply churny changes.

- [ ] **Step 7: Run the idiomatic-bubbletea checklist**

Open `docs/poplar/bubbletea-conventions.md` §10 and walk every item against `internal/ui/compose_tab.go` and the App diff. Specifically verify:

- ComposeTab.View() output is exactly width × height (test already covers width).
- No state mutation in View() or in any tea.Cmd closure.
- Width math uses `lipgloss.Width` / `displayCells`, never `len()`.
- Renderers honor width via wordwrap+hardwrap (Catkin already does; verify).
- No defensive parent-side clipping (App doesn't MaxWidth ComposeTab.View()).
- Children signal via tea.Msg (ComposeSendMsg/ComposeCancelMsg) — not callbacks.
- WindowSizeMsg forwards into ComposeTab when present.
- ComposeKeys uses key.Binding + key.Matches.

- [ ] **Step 8: Live tmux verification**

Per the project's `tmux-testing.md`: install via `make install`, capture poplar at 80×24 and 120×40 in three states:

1. Fresh `c` compose, focus on To.
2. Reply-seeded compose with quoted body visible.
3. Dirty compose with ConfirmModal open over it.

Visually confirm the size contract (no overflow, no underflow), header alignment, and that overlays composite cleanly.

- [ ] **Step 9: Write ADRs 0159–0161**

Per the spec's ADR list, write three short ADRs in `docs/poplar/decisions/`:

- `0159-compose-tab-shape.md` — App-level mode, focus model, Esc-as-focus, Ctrl+X/Ctrl+C, single-instance for 9h.
- `0160-queue-outbound-protocol-branch.md` — `cache.Account.QueueOutbound` + `Backend.IsJMAP()`.
- `0161-tidy-seam-function-pointer.md` — `TidyFn` on App, no interface.

If 0161 reads thin, fold it into 0159 — one ADR for the full ComposeTab surface — and skip writing 0161.

- [ ] **Step 10: Update invariants.md decision index + Compose section**

In `docs/poplar/invariants.md`:

- Replace the existing `### Compose` subsection's "Editor seam" paragraph with a one-line pointer to `.claude/rules/ui-invariants.md` for the implemented contract — the binding facts now live there.
- Add a new row to the decision index:
  `| ComposeTab — App-level mode + focus model + Ctrl+X/C; QueueOutbound + IsJMAP; TidyFn seam | 0159, 0160, 0161 |`

Confirm `invariants.md` total line count remains ≤300 (size hook gate).

- [ ] **Step 11: Update STATUS.md**

- Mark Pass 9h `done` in the table.
- Replace the starter prompt with the Pass 9h.5 starter (drafts persistence — issue #33).
- Trim "Next steps" if needed.
- Confirm STATUS.md ≤60 lines.

- [ ] **Step 12: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-06-compose-tab.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-06-compose-tab-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 13: Final make check + ship**

```bash
cd /home/glw907/Projects/poplar
make check
git add -A
git commit -m "Pass 9h: ComposeTab — inline compose surface, send via outbox"
git push
make install
```

Expected: green check, clean push, `~/.local/bin/poplar` updated.

---

## Notes for the executor

- **Read before editing.** Several tasks reference helpers (`displayTruncate`, `handleWindowSize`, `selectedMessage`, `rightPaneSize`, `accountTabThemeRef`) by name without locking the exact signature. Open the file first to confirm the existing shape, then mirror it. The codebase has strong conventions; matching them is more important than verbatim adherence to the snippets above.
- **Theme reference plumbing.** ComposeTab needs `*theme.CompiledTheme` for Catkin styling. AccountTab already holds it for Viewer; expose via `AccountTab.Theme()` accessor and read it from App at compose-construction time.
- **Address parsing.** `content.ParseAddressList` is the shared parser per ADR-0156; do not import `net/mail` directly in compose_tab.go.
- **Send key chord.** `tea.KeyCtrlX` is the bubbletea constant; no string lookup needed.
- **Voice.** Every comment, error string, and test name follows the human-voice rules in the `go-conventions` skill — no "we", no "this function", no "for example", no AI-tells. Run the voice-check (`make check` includes it).
- **Pass-end ritual.** Task 11 is the consolidation ritual. Per `poplar-pass`, this is the last task; do not skip steps.
