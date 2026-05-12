# v2 Declarative View Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `tea.View.ProgressBar` (OSC 9;4 priority ladder), `tea.View.ReportFocus` + a new-mail toast gated on blur, and `tea.View.KeyboardEnhancements` with capability-aware help filtering — set per frame in `App.View()`.

**Architecture:** Three independent sub-features sharing one entry point (`App.View()`). Each adds: (a) state on `App` or `cache.Account`, (b) one accessor / handler, (c) the field assignment in `App.View()`, (d) one user-visible consumer. No backend interface changes; pre-beta lets a future pass add server-side IDLE pause if measurement justifies it.

**Tech Stack:** Go 1.26.1, `charm.land/bubbletea/v2` v2.0.6, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, SQLite via `cache.Account`, BurntSushi/toml.

**Spec:** `docs/superpowers/specs/2026-05-11-v2-view-fields-design.md`

---

## File Map

**Create:**
- `internal/ui/uicore/keys.go` — `GatedBinding` type + `RequiresKittyKbd` tag.

**Modify:**
- `internal/cache/account.go` — three new accessors (`OutboxDrainProgress`, `AttachmentDownloadProgress`, `SyncProgress`) + supporting fields.
- `internal/cache/drainer.go` — burst totals bookkeeping inside the drainer loop.
- `internal/cache/bodies.go` (or wherever attachment save lives) — increment / decrement the in-flight counter.
- `internal/ui/app.go` — new fields (`focused bool`, `kbdCaps tea.KeyboardEnhancementsMsg`, `progressErrorUntil time.Time`); `FocusMsg`/`BlurMsg`/`KeyboardEnhancementsMsg` handlers in `Update`.
- `internal/ui/app_view.go` — `frameProgressBar()` accessor; `App.View()` sets four new declarative fields.
- `internal/ui/toast.go` — `newMail` variant on `pendingAction`; renderer branch.
- `internal/ui/cmds.go` — `coalesceTimerMsg`; new-mail toast queueing.
- `internal/config/ui.go` — `UIConfig.NewMailToast bool` (default `true`); `rawUI.NewMailToast *bool`; decode and round-trip.
- `internal/config/render.go` — emit `new-mail-toast` when non-default.
- `internal/ui/helppopover/model.go` — `WithKbdCaps` constructor option; render filter for `GatedBinding` entries.
- `internal/catkin/dispatch.go` (or new `bindings.go`) — declare chord set as `[]GatedBinding`; gate footer rendering.

**Test:**
- `internal/cache/drainer_test.go` (extend) — `OutboxDrainProgress` table test.
- `internal/cache/attachments_test.go` (extend) — `AttachmentDownloadProgress` activity test.
- `internal/ui/app_view_test.go` (new or extend) — `frameProgressBar` priority ladder table test.
- `internal/ui/app_test.go` (extend) — `FocusMsg` / `BlurMsg` toggle; new-mail toast gate.
- `internal/ui/toast_test.go` (extend) — `newMail` variant render.
- `internal/ui/cmds_test.go` (extend) — coalesce timer collapses arrivals.
- `internal/config/ui_test.go` (extend) — `new-mail-toast` decode + default.
- `internal/ui/helppopover/model_test.go` (extend) — gated bindings filtered when caps off.
- `internal/catkin/dispatch_test.go` (extend or create) — chord set tagged.

---

## Task 1: Outbox drain progress accessor

**Files:**
- Modify: `internal/cache/account.go`
- Modify: `internal/cache/drainer.go`
- Test: `internal/cache/drainer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cache/drainer_test.go`:

```go
func TestOutboxDrainProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	// Idle: no progress.
	if pct, ok := a.OutboxDrainProgress(); ok || pct != 0 {
		t.Fatalf("idle: got (%d, %v), want (0, false)", pct, ok)
	}

	// Queue three sends (drainer paused).
	a.pauseDrainerForTest()
	for range 3 {
		if _, err := a.QueueOutbound(testSendArgs(t)); err != nil {
			t.Fatalf("queue: %v", err)
		}
	}
	a.startBurstForTest(3)

	if pct, ok := a.OutboxDrainProgress(); !ok || pct != 0 {
		t.Fatalf("burst start: got (%d, %v), want (0, true)", pct, ok)
	}

	a.recordBurstDoneForTest(2)
	if pct, ok := a.OutboxDrainProgress(); !ok || pct != 66 {
		t.Fatalf("after 2/3: got (%d, %v), want (66, true)", pct, ok)
	}

	a.recordBurstDoneForTest(1)
	if pct, ok := a.OutboxDrainProgress(); ok || pct != 0 {
		t.Fatalf("burst empty: got (%d, %v), want (0, false)", pct, ok)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/cache/ -run TestOutboxDrainProgress -v`
Expected: FAIL — `OutboxDrainProgress`, `pauseDrainerForTest`, `startBurstForTest`, `recordBurstDoneForTest` undefined.

- [ ] **Step 3: Add the burst counters and accessor on Account**

Add to `internal/cache/account.go` (inside `type Account struct { ... }`):

```go
// Outbox drain burst bookkeeping. burstTotal is set when the drainer
// transitions from idle to draining; burstDone increments per OpDone
// tx; both reset to zero when the queue empties.
burstTotal atomic.Int32
burstDone  atomic.Int32
```

Add the accessor (place near `BackfillProgress` to keep progress-style methods together):

```go
// OutboxDrainProgress reports the current outbox drain burst's
// progress. Returns (pct, true) while the drainer is working a non-
// empty queue; (0, false) when idle.
func (a *Account) OutboxDrainProgress() (pct int, active bool) {
	total := a.burstTotal.Load()
	if total == 0 {
		return 0, false
	}
	done := a.burstDone.Load()
	return int(done * 100 / total), true
}
```

- [ ] **Step 4: Wire the counters into the drainer loop**

Modify `internal/cache/drainer.go`. Inside the drainer goroutine, locate the loop that polls the outbox queue. On the transition from empty-to-non-empty, set `burstTotal` to the current pending count; on each `OpDone` tx commit, increment `burstDone`; on transition back to empty, reset both to zero.

```go
// At start of a drain pass that finds n > 0 pending rows:
if a.burstTotal.Load() == 0 {
	a.burstTotal.Store(int32(n))
	a.burstDone.Store(0)
}

// After each OpDone commit:
a.burstDone.Add(1)

// After the drain pass when no pending rows remain:
a.burstTotal.Store(0)
a.burstDone.Store(0)
```

(Wire each block at its actual callsite; the comments above describe placement, not literal code blocks to drop in standalone.)

- [ ] **Step 5: Add the test seams**

Add to `internal/cache/drainer.go` (or a new `drainer_test_seams.go` file under build tag `//go:build test_seams`, or just place directly in the package — these are internal helpers used only by tests in the same package):

```go
func (a *Account) pauseDrainerForTest()       { a.drainerPaused.Store(true) }
func (a *Account) startBurstForTest(n int)    { a.burstTotal.Store(int32(n)); a.burstDone.Store(0) }
func (a *Account) recordBurstDoneForTest(n int) {
	for range n {
		a.burstDone.Add(1)
		if a.burstDone.Load() >= a.burstTotal.Load() {
			a.burstTotal.Store(0)
			a.burstDone.Store(0)
			return
		}
	}
}
```

Add `drainerPaused atomic.Bool` to `Account` and gate the drainer loop's work step on `!a.drainerPaused.Load()`.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test -tags=dev ./internal/cache/ -run TestOutboxDrainProgress -v`
Expected: PASS.

- [ ] **Step 7: Run the broader cache suite to confirm no regressions**

Run: `go test -tags=dev ./internal/cache/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cache/account.go internal/cache/drainer.go internal/cache/drainer_test.go
git commit -m "cache: add outbox drain burst progress accessor

Track burst totals on Account so the drainer's progress can drive
the OSC 9;4 ProgressBar. Accessor returns (pct, true) while a burst
is in flight, (0, false) when idle.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Attachment download + sync progress accessors

**Files:**
- Modify: `internal/cache/account.go`
- Modify: `internal/cache/attachments.go` (or wherever the save path lives — verify with `grep -n "SaveAttachment\|saveAttachment" internal/cache/`)
- Test: `internal/cache/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cache/attachments_test.go`:

```go
func TestAttachmentDownloadProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	if _, ok := a.AttachmentDownloadProgress(); ok {
		t.Fatalf("idle: ok = true, want false")
	}

	a.beginAttachmentDownloadForTest()
	if _, ok := a.AttachmentDownloadProgress(); !ok {
		t.Fatalf("in-flight: ok = false, want true")
	}
	a.endAttachmentDownloadForTest()
	if _, ok := a.AttachmentDownloadProgress(); ok {
		t.Fatalf("after end: ok = true, want false")
	}
}

func TestSyncProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	if _, ok := a.SyncProgress(); ok {
		t.Fatalf("idle: ok = true, want false")
	}
	a.beginSyncForTest()
	if _, ok := a.SyncProgress(); !ok {
		t.Fatalf("in-flight: ok = false, want true")
	}
	a.endSyncForTest()
	if _, ok := a.SyncProgress(); ok {
		t.Fatalf("after end: ok = true, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/cache/ -run "TestAttachmentDownloadProgress|TestSyncProgress" -v`
Expected: FAIL — accessors and seams undefined.

- [ ] **Step 3: Add counters and accessors on Account**

Add to `Account`:

```go
attachInFlight atomic.Int32
syncInFlight   atomic.Int32
```

Add the accessors:

```go
// AttachmentDownloadProgress reports whether any attachment save is
// in flight. The (pct, _) return is always 0 — attachment saves
// have no per-byte progress today; the bar rides as Indeterminate.
func (a *Account) AttachmentDownloadProgress() (pct int, active bool) {
	return 0, a.attachInFlight.Load() > 0
}

// SyncProgress reports whether a backend Connect or refresh fetch
// is mid-flight. Indeterminate-only this pass.
func (a *Account) SyncProgress() (pct int, active bool) {
	return 0, a.syncInFlight.Load() > 0
}
```

Add the test seams (same file, package-private):

```go
func (a *Account) beginAttachmentDownloadForTest() { a.attachInFlight.Add(1) }
func (a *Account) endAttachmentDownloadForTest()   { a.attachInFlight.Add(-1) }
func (a *Account) beginSyncForTest()               { a.syncInFlight.Add(1) }
func (a *Account) endSyncForTest()                 { a.syncInFlight.Add(-1) }
```

Also expose non-test counterparts the production callers will use:

```go
// BeginAttachmentDownload / EndAttachmentDownload bracket a save
// for ProgressBar reporting. Pair them with defer.
func (a *Account) BeginAttachmentDownload() { a.attachInFlight.Add(1) }
func (a *Account) EndAttachmentDownload()   { a.attachInFlight.Add(-1) }

func (a *Account) BeginSync() { a.syncInFlight.Add(1) }
func (a *Account) EndSync()   { a.syncInFlight.Add(-1) }
```

- [ ] **Step 4: Wire the production counters at the call sites**

`grep -n "save.*attachment\|SaveAttachment\|FetchAttachment" internal/cache/ internal/ui/cmds.go` to locate the attachment save command. Bracket the body:

```go
acct.BeginAttachmentDownload()
defer acct.EndAttachmentDownload()
// ... existing save logic ...
```

For sync: locate `pumpUpdatesCmd` and any folder-refresh fetch in `internal/ui/cmds.go`. Bracket the fetch body the same way.

- [ ] **Step 5: Run tests**

Run: `go test -tags=dev ./internal/cache/ -run "TestAttachmentDownloadProgress|TestSyncProgress" -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/cache/ ./internal/ui/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cache/account.go internal/cache/attachments.go internal/cache/attachments_test.go internal/ui/cmds.go
git commit -m "cache: add attachment + sync in-flight accessors

Both report binary active/idle for now. Attachments and sync ride
the ProgressBar as Indeterminate until the underlying ops gain
per-byte progress in a future pass.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: frameProgressBar + App.View wiring

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_view.go`
- Test: `internal/ui/app_view_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

Create or extend `internal/ui/app_view_test.go`:

```go
package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestFrameProgressBarPriorityLadder(t *testing.T) {
	cases := []struct {
		name      string
		attach    bool
		outbox    bool
		outboxPct int
		sync      bool
		wantState tea.ProgressBarState
		wantValue int
	}{
		{"all idle", false, false, 0, false, tea.ProgressBarNone, 0},
		{"sync only", false, false, 0, true, tea.ProgressBarIndeterminate, 0},
		{"outbox only", false, true, 42, false, tea.ProgressBarDefault, 42},
		{"attach only", true, false, 0, false, tea.ProgressBarIndeterminate, 0},
		{"attach beats outbox", true, true, 90, true, tea.ProgressBarIndeterminate, 0},
		{"outbox beats sync", false, true, 50, true, tea.ProgressBarDefault, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := App{}
			m.testProgressSources(tc.attach, tc.outbox, tc.outboxPct, tc.sync)
			pb := m.frameProgressBar()
			if tc.wantState == tea.ProgressBarNone {
				if pb != nil {
					t.Fatalf("got %+v, want nil", pb)
				}
				return
			}
			if pb == nil {
				t.Fatalf("got nil, want state=%v", tc.wantState)
			}
			if pb.State != tc.wantState {
				t.Fatalf("state = %v, want %v", pb.State, tc.wantState)
			}
			if pb.State == tea.ProgressBarDefault && pb.Value != tc.wantValue {
				t.Fatalf("value = %d, want %d", pb.Value, tc.wantValue)
			}
		})
	}
}

func TestFrameProgressBarErrorDecay(t *testing.T) {
	m := App{progressErrorUntil: time.Now().Add(2 * time.Second)}
	pb := m.frameProgressBar()
	if pb == nil || pb.State != tea.ProgressBarError {
		t.Fatalf("got %+v, want state=Error", pb)
	}
	m.progressErrorUntil = time.Now().Add(-time.Second)
	if pb := m.frameProgressBar(); pb != nil {
		t.Fatalf("expired: got %+v, want nil", pb)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/ui/ -run "TestFrameProgressBar" -v`
Expected: FAIL — `frameProgressBar`, `progressErrorUntil`, `testProgressSources` undefined.

- [ ] **Step 3: Add fields to App**

Add to `App` struct in `internal/ui/app.go`:

```go
progressErrorUntil time.Time

// testProgressOverride is non-nil only in tests; supplies the source
// triple in lieu of consulting m.acct. Production code never sets it.
testProgressOverride *testProgressTriple
```

```go
type testProgressTriple struct {
	attach, outbox, sync bool
	outboxPct            int
}

func (m *App) testProgressSources(attach, outbox bool, outboxPct int, sync bool) {
	m.testProgressOverride = &testProgressTriple{attach, outbox, outboxPct, sync}
}
```

- [ ] **Step 4: Implement frameProgressBar**

Add to `internal/ui/app_view.go`:

```go
import "time"

// frameProgressBar resolves the OSC 9;4 progress source per the
// fixed priority ladder: attachment download > outbox drain > sync.
// Recent errors decay over ~3s as ProgressBarError. Returns nil when
// nothing is active.
func (m App) frameProgressBar() *tea.ProgressBar {
	if !m.progressErrorUntil.IsZero() && time.Now().Before(m.progressErrorUntil) {
		return tea.NewProgressBar(tea.ProgressBarError, 0)
	}

	attach, outbox, outboxPct, sync := m.progressSources()
	switch {
	case attach:
		return tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
	case outbox:
		return tea.NewProgressBar(tea.ProgressBarDefault, outboxPct)
	case sync:
		return tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
	}
	return nil
}

func (m App) progressSources() (attach, outbox bool, outboxPct int, sync bool) {
	if t := m.testProgressOverride; t != nil {
		return t.attach, t.outbox, t.outboxPct, t.sync
	}
	if m.acct == nil {
		return false, false, 0, false
	}
	_, attach = m.acct.AttachmentDownloadProgress()
	outboxPct, outbox = m.acct.OutboxDrainProgress()
	_, sync = m.acct.SyncProgress()
	return
}
```

- [ ] **Step 5: Wire into App.View**

In `internal/ui/app_view.go`, find the `view(content string) tea.View` helper (line ~68). Set the new field on the returned `tea.View`:

```go
func (m App) view(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()
	v.ProgressBar = m.frameProgressBar()
	return v
}
```

Apply the same change in `viewWithCursor` and `viewOverlay` if they construct their own `tea.View` rather than delegating.

- [ ] **Step 6: Run test**

Run: `go test -tags=dev ./internal/ui/ -run "TestFrameProgressBar" -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/ui/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/app_view.go internal/ui/app_view_test.go
git commit -m "ui: wire ProgressBar via priority ladder

App.View now sets tea.View.ProgressBar from a fixed precedence
attachment > outbox > sync. Error decay (~3s) shows on the OS
taskbar after a transition to errored state.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: ReportFocus + FocusMsg/BlurMsg handling

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_view.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/app_test.go`:

```go
func TestFocusBlurTogglesAppFocused(t *testing.T) {
	m := newTestApp(t)
	if !m.focused {
		t.Fatalf("initial: focused = false, want true")
	}
	m, _ = m.Update(tea.BlurMsg{})
	if m.focused {
		t.Fatalf("after Blur: focused = true, want false")
	}
	m, _ = m.Update(tea.FocusMsg{})
	if !m.focused {
		t.Fatalf("after Focus: focused = false, want true")
	}
}
```

If `newTestApp` doesn't exist, define a minimal helper:

```go
func newTestApp(t *testing.T) App {
	t.Helper()
	return App{focused: true}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/ui/ -run TestFocusBlurTogglesAppFocused -v`
Expected: FAIL — `focused` field undefined or `tea.FocusMsg` not handled.

- [ ] **Step 3: Add the field**

Add to `App` struct: `focused bool`.

In the `NewApp` constructor (around line 115 of `internal/ui/app.go`), initialize: `focused: true,`.

- [ ] **Step 4: Handle FocusMsg / BlurMsg in App.Update**

In `internal/ui/app.go` (or `app_chrome.go` if focus belongs in chrome — pick the file already routing other lifecycle msgs), add a case in `App.Update`:

```go
case tea.FocusMsg:
	m.focused = true
	m.activeToast = pendingAction{} // clear any new-mail toast
	return m, nil
case tea.BlurMsg:
	m.focused = false
	return m, nil
```

(`m.activeToast` is the existing toast field — adjust the name if the actual field is different. Verify with `grep -n "pendingAction\|activeToast\|toast" internal/ui/app.go`.)

- [ ] **Step 5: Set ReportFocus in View**

In `internal/ui/app_view.go`'s `view` helper, add:

```go
v.ReportFocus = true
```

- [ ] **Step 6: Run tests**

Run: `go test -tags=dev ./internal/ui/ -run TestFocusBlurTogglesAppFocused -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/ui/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/app_view.go internal/ui/app_test.go
git commit -m "ui: handle terminal focus events

App.View sets tea.View.ReportFocus = true; Update toggles m.focused
on FocusMsg / BlurMsg and clears any active toast on focus regain
(the messagelist now shows the truth).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: New-mail toast + coalesce + config gate

**Files:**
- Modify: `internal/config/ui.go`
- Modify: `internal/config/render.go` (verify exact filename — it's the `Render` emitter)
- Modify: `internal/ui/toast.go`
- Modify: `internal/ui/cmds.go`
- Modify: `internal/ui/app.go` (the `mail.Update` handler)
- Test: `internal/config/ui_test.go`, `internal/ui/toast_test.go`, `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing config test**

Add to `internal/config/ui_test.go`:

```go
func TestLoadUI_NewMailToast(t *testing.T) {
	cases := []struct {
		toml string
		want bool
	}{
		{"", true},                              // default
		{"[ui]\n", true},                        // explicit table, default
		{"[ui]\nnew-mail-toast = true\n", true}, // explicit true
		{"[ui]\nnew-mail-toast = false\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.toml, func(t *testing.T) {
			path := writeTempUI(t, tc.toml)
			cfg, err := LoadUI(path)
			if err != nil {
				t.Fatalf("LoadUI: %v", err)
			}
			if cfg.NewMailToast != tc.want {
				t.Fatalf("NewMailToast = %v, want %v", cfg.NewMailToast, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run config test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadUI_NewMailToast -v`
Expected: FAIL — `NewMailToast` undefined.

- [ ] **Step 3: Add NewMailToast to UIConfig + decode + default**

In `internal/config/ui.go`, add to `UIConfig`:

```go
// NewMailToast gates the unfocused-terminal new-mail toast.
// Default true; set false to silence.
NewMailToast bool
```

Add to `rawUI`:

```go
NewMailToast *bool `toml:"new-mail-toast"`
```

In `DefaultUIConfig`, add: `NewMailToast: true,`.

In `LoadUI`, after the existing decoded fields:

```go
if raw.UI.NewMailToast != nil {
	out.NewMailToast = *raw.UI.NewMailToast
}
```

- [ ] **Step 4: Wire round-trip in config.Render**

`grep -n "undo_seconds\|undo-send-window\|new-mail-toast" internal/config/render.go` to find the `[ui]` emitter. Add a line that emits `new-mail-toast = false` only when `cfg.UI.NewMailToast == false` (omit when default). Pattern follows the other booleans in the emitter.

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/config/ -v`
Expected: PASS (including the new test and existing round-trip tests).

- [ ] **Step 6: Write the failing toast renderer test**

Add to `internal/ui/toast_test.go`:

```go
func TestRenderToast_NewMail(t *testing.T) {
	styles := defaultTestStyles(t)

	t.Run("single sender", func(t *testing.T) {
		p := pendingAction{
			newMailCount:  1,
			newMailSender: "Alice",
		}
		got := renderToast(p, 80, styles)
		if !strings.Contains(got, "1 new from Alice") {
			t.Fatalf("got %q, want substring %q", got, "1 new from Alice")
		}
	})

	t.Run("mixed senders", func(t *testing.T) {
		p := pendingAction{
			newMailCount:  3,
			newMailFolder: "Inbox",
		}
		got := renderToast(p, 80, styles)
		if !strings.Contains(got, "3 new in Inbox") {
			t.Fatalf("got %q, want substring %q", got, "3 new in Inbox")
		}
	})
}
```

- [ ] **Step 7: Run toast test to verify it fails**

Run: `go test -tags=dev ./internal/ui/ -run TestRenderToast_NewMail -v`
Expected: FAIL — fields undefined.

- [ ] **Step 8: Extend pendingAction + renderToast**

In `internal/ui/toast.go`, add to `pendingAction`:

```go
newMailCount  int
newMailSender string // populated when single sender
newMailFolder string // populated when mixed senders
```

Update `IsZero` so a non-zero `newMailCount` keeps the toast active.

In `renderToast`, add a branch ahead of the triage-render path:

```go
if p.newMailCount > 0 {
	var body string
	switch {
	case p.newMailSender != "":
		body = fmt.Sprintf(" · %d new from %s ·", p.newMailCount, p.newMailSender)
	default:
		body = fmt.Sprintf(" · %d new in %s ·", p.newMailCount, p.newMailFolder)
	}
	return styles.Toast.Render(uicore.TruncateToWidth(body, width))
}
```

(Verify the existing `renderToast` signature accepts `pendingAction` by value; if it takes a pointer, adapt accordingly.)

- [ ] **Step 9: Run toast test**

Run: `go test -tags=dev ./internal/ui/ -run TestRenderToast_NewMail -v`
Expected: PASS.

- [ ] **Step 10: Write the failing coalesce + gate test**

Add to `internal/ui/app_test.go`:

```go
func TestNewMailToast_GatedOnFocusAndConfig(t *testing.T) {
	mkUpd := func() mail.Update {
		return mail.Update{Folder: "Inbox", NewArrivals: 1, LatestSender: "Alice"}
	}

	t.Run("focused: no toast", func(t *testing.T) {
		m := newTestApp(t)
		m.focused = true
		m.cfg.UI.NewMailToast = true
		m, _ = m.Update(mkUpd())
		if m.activeToast.newMailCount != 0 {
			t.Fatalf("focused should not toast: got count = %d", m.activeToast.newMailCount)
		}
	})

	t.Run("blurred + cfg off: no toast", func(t *testing.T) {
		m := newTestApp(t)
		m.focused = false
		m.cfg.UI.NewMailToast = false
		m, _ = m.Update(mkUpd())
		if m.activeToast.newMailCount != 0 {
			t.Fatalf("cfg off should not toast: got count = %d", m.activeToast.newMailCount)
		}
	})
}

func TestNewMailToast_CoalesceCollapses(t *testing.T) {
	m := newTestApp(t)
	m.focused = false
	m.cfg.UI.NewMailToast = true

	m, _ = m.Update(mail.Update{Folder: "Inbox", NewArrivals: 1, LatestSender: "Alice"})
	m, _ = m.Update(mail.Update{Folder: "Inbox", NewArrivals: 1, LatestSender: "Bob"})
	m, _ = m.Update(coalesceTimerMsg{})

	if got := m.activeToast.newMailCount; got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if m.activeToast.newMailSender != "" || m.activeToast.newMailFolder != "Inbox" {
		t.Fatalf("mixed senders should fall back to folder; got sender=%q folder=%q",
			m.activeToast.newMailSender, m.activeToast.newMailFolder)
	}
}
```

(If `mail.Update` doesn't currently carry `NewArrivals` / `LatestSender`, add them — these are the fields the toast pipeline needs. Verify with `grep -n "type Update" internal/mail/`.)

- [ ] **Step 11: Run tests to verify they fail**

Run: `go test -tags=dev ./internal/ui/ -run TestNewMailToast -v`
Expected: FAIL — `coalesceTimerMsg`, gate logic, `mail.Update` fields undefined.

- [ ] **Step 12: Add coalesceTimerMsg + queueing**

Add to `internal/ui/cmds.go`:

```go
// coalesceTimerMsg fires 1s after the first arrival in a coalesce
// window, signalling that any pending new-mail toast should render
// with the accumulated count.
type coalesceTimerMsg struct{}

func coalesceNewMailCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return coalesceTimerMsg{} })
}
```

Add fields to `App`:

```go
// pendingNewMail accumulates arrivals during the 1s coalesce window;
// flushed into activeToast on coalesceTimerMsg.
pendingNewMail struct {
	count       int
	sender      string // single-sender path
	mixedSender bool   // true once two distinct senders have arrived
	folder      string
}
coalesceArmed bool
```

In the `mail.Update` case in `App.Update`, after applying the update:

```go
if !m.focused && m.cfg.UI.NewMailToast && msg.NewArrivals > 0 {
	if m.pendingNewMail.count == 0 {
		m.pendingNewMail.sender = msg.LatestSender
		m.pendingNewMail.folder = msg.Folder
	} else if m.pendingNewMail.sender != msg.LatestSender {
		m.pendingNewMail.mixedSender = true
	}
	m.pendingNewMail.count += msg.NewArrivals
	if !m.coalesceArmed {
		m.coalesceArmed = true
		cmds = append(cmds, coalesceNewMailCmd())
	}
}
```

Add the timer handler:

```go
case coalesceTimerMsg:
	p := m.pendingNewMail
	m.activeToast = pendingAction{
		newMailCount:  p.count,
		newMailSender: ifThenElse(p.mixedSender, "", p.sender),
		newMailFolder: ifThenElse(p.mixedSender, p.folder, ""),
		deadline:      time.Now().Add(toastDecay),
	}
	m.pendingNewMail = struct{ count int; sender string; mixedSender bool; folder string }{}
	m.coalesceArmed = false
	return m, toastExpireCmd(m.activeToast.deadline)
```

(Inline the small `ifThenElse` helper if there isn't one already; or use an explicit `if`.)

- [ ] **Step 13: Add NewArrivals + LatestSender to mail.Update if needed**

Inspect `internal/mail/types.go` (or wherever `mail.Update` lives — `grep -rn "type Update" internal/mail/`). Extend the struct with:

```go
NewArrivals  int    // count of new messages arriving with this update
LatestSender string // most recent sender's display name, "" if mixed/unknown
```

Populate at the JMAP / IMAP push sites where `mail.Update` is constructed. For this pass the IMAP / JMAP sides can populate conservatively (count from added UIDs; sender from a single fetch of the latest UID's `From` header).

- [ ] **Step 14: Run tests**

Run: `go test -tags=dev ./internal/ui/ -run TestNewMailToast -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/ui/ ./internal/mail/`
Expected: PASS.

- [ ] **Step 15: Commit**

```bash
git add internal/config/ui.go internal/config/render.go internal/config/ui_test.go internal/ui/toast.go internal/ui/toast_test.go internal/ui/cmds.go internal/ui/app.go internal/ui/app_test.go internal/mail/
git commit -m "ui: incoming-mail toast gated on blur

Coalesces arrivals across a 1s window; renders only when the
terminal is unfocused and [ui] new-mail-toast is true (default).
Single-sender bursts read 'N new from Foo'; mixed bursts collapse
to 'N new in Inbox'.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: KeyboardEnhancements field + capability storage

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_view.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/app_test.go`:

```go
func TestKeyboardEnhancementsMsgStored(t *testing.T) {
	m := newTestApp(t)
	msg := tea.KeyboardEnhancementsMsg{}
	msg.DisambiguateEscapeCodes = true
	msg.ReportEventTypes = true

	m, _ = m.Update(msg)
	if !m.kbdCaps.DisambiguateEscapeCodes {
		t.Fatalf("DisambiguateEscapeCodes not stored")
	}
	if !m.kbdCaps.ReportEventTypes {
		t.Fatalf("ReportEventTypes not stored")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/ui/ -run TestKeyboardEnhancementsMsgStored -v`
Expected: FAIL — `kbdCaps` undefined.

- [ ] **Step 3: Add field + handler + accessor**

Add to `App`:

```go
kbdCaps tea.KeyboardEnhancementsMsg
```

Add to `App.Update`:

```go
case tea.KeyboardEnhancementsMsg:
	m.kbdCaps = msg
	return m, nil
```

Add a public accessor:

```go
// KbdCaps returns the negotiated keyboard-enhancement capabilities.
// The zero value (no fields set) means the protocol isn't active.
func (m App) KbdCaps() tea.KeyboardEnhancementsMsg { return m.kbdCaps }
```

- [ ] **Step 4: Set KeyboardEnhancements in View**

In `internal/ui/app_view.go`'s `view` helper:

```go
v.KeyboardEnhancements.DisambiguateEscapeCodes = true
v.KeyboardEnhancements.ReportEventTypes = true
```

- [ ] **Step 5: Run tests**

Run: `go test -tags=dev ./internal/ui/ -run TestKeyboardEnhancementsMsgStored -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/ui/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/app_view.go internal/ui/app_test.go
git commit -m "ui: request and store Kitty keyboard capabilities

App.View requests DisambiguateEscapeCodes + ReportEventTypes;
KeyboardEnhancementsMsg lands in App.kbdCaps for downstream
consumers (catkin chord help filter, future IsRepeat handlers).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: GatedBinding type + catkin chord tagging

**Files:**
- Create: `internal/ui/uicore/keys.go`
- Modify: `internal/catkin/dispatch.go` (or new `internal/catkin/bindings.go`)
- Test: `internal/ui/uicore/keys_test.go`, `internal/catkin/bindings_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/uicore/keys_test.go`:

```go
package uicore

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestGatedBinding(t *testing.T) {
	gb := GatedBinding{
		Binding:          key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("^i", "italic")),
		RequiresKittyKbd: true,
	}
	if !gb.RequiresKittyKbd {
		t.Fatalf("tag not preserved")
	}
	if got := gb.Binding.Help().Key; got != "^i" {
		t.Fatalf("help key = %q, want %q", got, "^i")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/ui/uicore/ -run TestGatedBinding -v`
Expected: FAIL — `GatedBinding` undefined.

- [ ] **Step 3: Create GatedBinding**

Create `internal/ui/uicore/keys.go`:

```go
// Package uicore — keys.go declares GatedBinding, a key.Binding
// paired with a capability tag. Bindings whose semantics require
// the Kitty keyboard protocol (Ctrl+letter chords that the legacy
// ASCII mapping confuses with Tab/Enter/Backspace/etc.) carry
// RequiresKittyKbd = true; helppopover filters them out when the
// protocol isn't negotiated.
package uicore

import "charm.land/bubbles/v2/key"

type GatedBinding struct {
	Binding          key.Binding
	RequiresKittyKbd bool
}
```

- [ ] **Step 4: Run uicore test**

Run: `go test -tags=dev ./internal/ui/uicore/ -run TestGatedBinding -v`
Expected: PASS.

- [ ] **Step 5: Write the failing catkin chord declaration test**

Create `internal/catkin/bindings_test.go`:

```go
package catkin

import (
	"testing"

	"github.com/glw907/poplar/internal/ui/uicore"
)

func TestChordSet_GatedSubset(t *testing.T) {
	chords := ChordSet()
	gated := map[string]bool{
		"^B": true, "^I": true, "^K": true, "^L": true, "^Q": true, "^@": true,
	}
	seen := map[string]bool{}
	for _, c := range chords {
		key := c.Binding.Help().Key
		seen[key] = true
		if gated[key] && !c.RequiresKittyKbd {
			t.Errorf("chord %q expected RequiresKittyKbd=true", key)
		}
	}
	for k := range gated {
		if !seen[k] {
			t.Errorf("chord %q missing from ChordSet", k)
		}
	}
	_ = uicore.GatedBinding{} // keep import live
}
```

- [ ] **Step 6: Run catkin test to verify it fails**

Run: `go test -tags=dev ./internal/catkin/ -run TestChordSet_GatedSubset -v`
Expected: FAIL — `ChordSet` undefined.

- [ ] **Step 7: Declare the chord set**

Create `internal/catkin/bindings.go`:

```go
package catkin

import (
	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// ChordSet returns Catkin's command vocabulary. Entries tagged
// RequiresKittyKbd carry semantics (bold, italic, link, list, quote,
// task) that depend on Ctrl+letter chord disambiguation; on terminals
// that don't negotiate the Kitty keyboard protocol, the helppopover
// hides them and the catkin status footer collapses to the plain
// markdown hint.
func ChordSet() []uicore.GatedBinding {
	return []uicore.GatedBinding{
		{Binding: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("^B", "bold")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("^I", "italic")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("^K", "link")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("^L", "list")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+q"), key.WithHelp("^Q", "quote")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+@", "ctrl+ "), key.WithHelp("^@", "task")), RequiresKittyKbd: true},
	}
}
```

- [ ] **Step 8: Run tests**

Run: `go test -tags=dev ./internal/catkin/ -run TestChordSet_GatedSubset -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/catkin/ ./internal/ui/uicore/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/uicore/keys.go internal/ui/uicore/keys_test.go internal/catkin/bindings.go internal/catkin/bindings_test.go
git commit -m "uicore+catkin: GatedBinding tag for protocol-dependent chords

Catkin's six Ctrl+letter chords (bold, italic, link, list, quote,
task) require the Kitty keyboard protocol to disambiguate from
Tab/Enter/Backspace. ChordSet returns them tagged so helppopover
and the catkin status footer can render the right subset.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: helppopover capability filter + catkin footer

**Files:**
- Modify: `internal/ui/helppopover/model.go`
- Modify: `internal/ui/compose/model.go` (or wherever the catkin status footer renders)
- Test: `internal/ui/helppopover/model_test.go`

- [ ] **Step 1: Write the failing helppopover filter test**

Add to `internal/ui/helppopover/model_test.go`:

```go
func TestPopoverFiltersGatedBindings(t *testing.T) {
	caps := tea.KeyboardEnhancementsMsg{} // protocol absent
	m := New(testStyles(t), Compose).WithKbdCaps(caps).WithSize(80, 40)
	out := m.View()
	if strings.Contains(out, "^I") || strings.Contains(out, "italic") {
		t.Fatalf("gated chord rendered with protocol absent: %s", out)
	}

	caps.DisambiguateEscapeCodes = true
	m = New(testStyles(t), Compose).WithKbdCaps(caps).WithSize(80, 40)
	out = m.View()
	if !strings.Contains(out, "italic") {
		t.Fatalf("gated chord missing with protocol active: %s", out)
	}
}
```

(Adapt `Compose` to whatever the helppopover Context constant is named for the compose surface.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/ui/helppopover/ -run TestPopoverFiltersGatedBindings -v`
Expected: FAIL — `WithKbdCaps` undefined or chord not gated.

- [ ] **Step 3: Add WithKbdCaps + filter**

In `internal/ui/helppopover/model.go`, add a field on `Model`:

```go
kbdCaps tea.KeyboardEnhancementsMsg
```

Add the setter:

```go
func (h Model) WithKbdCaps(caps tea.KeyboardEnhancementsMsg) Model {
	h.kbdCaps = caps
	return h
}
```

Where the Compose context's binding list is built (search the file for the Compose case in the switch around line 199), source the chord rows from `catkin.ChordSet()` and filter:

```go
import "github.com/glw907/poplar/internal/catkin"

func composeBindings(caps tea.KeyboardEnhancementsMsg) []bindingGroup {
	var chordRows []bindingRow
	for _, gb := range catkin.ChordSet() {
		if gb.RequiresKittyKbd && !caps.DisambiguateEscapeCodes {
			continue
		}
		help := gb.Binding.Help()
		chordRows = append(chordRows, bindingRow{key: help.Key, desc: help.Desc})
	}
	return []bindingGroup{
		{title: "Markdown chords", rows: chordRows},
		// ... existing groups ...
	}
}
```

(Adapt to the actual `bindingGroup` / `bindingRow` types used in the file.)

Wire `caps` through to the renderer — the `View` method passes `h.kbdCaps` into `composeBindings` (or whatever the equivalent helper is named).

- [ ] **Step 4: Wire helppopover construction in App**

In `internal/ui/app.go`'s `NewApp`, find where `helppopover.New(...)` is called. Chain `.WithKbdCaps(m.kbdCaps)` once at construction. To keep the help popover up to date when `KeyboardEnhancementsMsg` arrives later, add a refresh in the `KeyboardEnhancementsMsg` case from Task 6:

```go
case tea.KeyboardEnhancementsMsg:
	m.kbdCaps = msg
	m.help = m.help.WithKbdCaps(msg)
	return m, nil
```

- [ ] **Step 5: Add the catkin status footer branch**

`grep -n "footer\|chord hint\|^B bold\|markdown:" internal/ui/compose/` to find the existing footer renderer. If absent, add it as a new method on `compose.Model`:

```go
func (m Model) chordHint(caps tea.KeyboardEnhancementsMsg) string {
	if !caps.DisambiguateEscapeCodes {
		return m.styles.FooterDim.Render("markdown: type **bold** *italic* [link](url) — richer chords in Kitty-protocol terminals")
	}
	parts := []string{}
	for _, gb := range catkin.ChordSet() {
		help := gb.Binding.Help()
		parts = append(parts, help.Key+" "+help.Desc)
	}
	return m.styles.FooterDim.Render(strings.Join(parts, " · "))
}
```

Have `compose.Model.View` render this row. Thread `caps` from the App into compose via a `WithKbdCaps` setter, mirroring helppopover.

- [ ] **Step 6: Run tests**

Run: `go test -tags=dev ./internal/ui/helppopover/ ./internal/ui/compose/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/helppopover/model.go internal/ui/helppopover/model_test.go internal/ui/compose/model.go internal/ui/app.go
git commit -m "ui: capability-aware help filtering + catkin chord hint

helppopover hides catkin chords tagged RequiresKittyKbd when the
Kitty keyboard protocol isn't negotiated; catkin status footer
collapses to the plain markdown hint with a one-line discoverability
nudge.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Live tmux verification

**Files:** none modified — capture artifacts go under `docs/poplar/captures/2026-05-11/`.

- [ ] **Step 1: Build + install**

Run: `make install`
Expected: PASS, binary at `~/.local/bin/poplar`.

- [ ] **Step 2: Capture outbox progress (120×40)**

Per `.claude/docs/tmux-testing.md`: launch poplar in a 120×40 tmux pane, queue an outbound message, capture the pane while drain is in flight:

```bash
mkdir -p docs/poplar/captures/2026-05-11
tmux new-session -d -s pop120 -x 120 -y 40
tmux send-keys -t pop120 "poplar" Enter
# ... navigate to compose, send mail ...
tmux capture-pane -t pop120 -p > docs/poplar/captures/2026-05-11/outbox-progress-120x40.txt
```

Capture the OSC 9;4 byte stream by piping through a logger if your terminal multiplexer doesn't natively show the title — verify by inspecting the terminal title bar shows the progress percentage briefly during drain.

- [ ] **Step 3: Capture help popover, Kitty vs xterm (120×40)**

Open poplar in Ghostty/Kitty (Kitty kbd negotiated), open compose, hit `?`:

```bash
tmux capture-pane -t pop120 -p > docs/poplar/captures/2026-05-11/helppopover-kitty-120x40.txt
```

Repeat in plain xterm or `TERM=xterm-256color tmux` (no Kitty kbd):

```bash
tmux capture-pane -t pop120 -p > docs/poplar/captures/2026-05-11/helppopover-noproto-120x40.txt
```

Confirm: first capture shows `^B bold · ^I italic …`; second omits those rows.

- [ ] **Step 4: Capture compose footer, both modes (80×24)**

Same procedure at 80×24. Confirm the footer reads the chord list in Kitty mode and the plain markdown hint elsewhere.

```bash
tmux capture-pane -t pop80 -p > docs/poplar/captures/2026-05-11/compose-footer-kitty-80x24.txt
tmux capture-pane -t pop80 -p > docs/poplar/captures/2026-05-11/compose-footer-noproto-80x24.txt
```

- [ ] **Step 5: Manual focus toast verification**

Launch poplar focused, send mail to the test account from another client (or use the JMAP curl recipe from `~/.claude/projects/-home-glw907-Projects-poplar/memory/reference_jmap_email_access.md`), then:
- a) blur the terminal (alt-tab to another window) before the next `mail.Update` arrives
- b) wait for the arrival
- c) confirm a `· N new from Foo · ◉` row appears within ~1.5s
- d) re-focus mid-decay; confirm the toast clears immediately
- e) set `[ui] new-mail-toast = false` in `~/.config/poplar/config.toml`, restart poplar, repeat — confirm no toast appears

- [ ] **Step 6: Commit captures**

```bash
git add docs/poplar/captures/2026-05-11/
git commit -m "captures: Pass 32 verification artifacts

ProgressBar (outbox drain), help popover (Kitty vs no-proto),
compose footer (both modes). Manual focus-toast checks logged in
the pass commentary.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Pass-end consolidation

This task is the standard `poplar-pass` ritual. Invoke the skill rather than re-listing steps inline.

- [ ] **Step 1: Run /simplify on the diff**

Run the `simplify` skill against the pass's full diff. Apply genuine wins; ignore noise.

- [ ] **Step 2: Run the bubbletea conventions §10 review checklist**

Open `docs/poplar/bubbletea-conventions.md` §10 and verify each item against the diff + the captures from Task 9. Note any deviations to call out in the ADR.

- [ ] **Step 3: Write ADR-0217**

Path: `docs/poplar/decisions/0217-v2-view-fields.md`. Use the four-section template from `poplar-pass`:

```markdown
---
title: v2 declarative View fields — ProgressBar, ReportFocus, KeyboardEnhancements
status: accepted
date: 2026-05-11
---

## Context

tea.View exposes ProgressBar, ReportFocus, and KeyboardEnhancements
per frame. Pass 30 wired the tea.View return shape but only set
AltScreen and WindowTitle.

## Decision

App.View() sets all five declarative fields per frame. ProgressBar
follows a fixed priority ladder (attachment > outbox > sync); error
state decays over ~3s. ReportFocus = true; an unfocused-only new-mail
toast (1s coalesce, opt-out via [ui] new-mail-toast) consumes the
focus signal. KeyboardEnhancements requests DisambiguateEscapeCodes +
ReportEventTypes; the negotiated capabilities filter Catkin's chord
set in helppopover and collapse the compose footer hint when the
protocol is absent.

## Consequences

OS-taskbar progress reflects long-running ops without poplar drawing
a bar; unfocused users see a transient toast on new mail; Catkin's
full chord set becomes honest (visible only on terminals that can
deliver it). Server-side IDLE / push pause on blur and IsRepeat
consumers (held-key acceleration) deferred to future passes — the
field plumbing is now in place to consume them without re-touching
App.View.
```

- [ ] **Step 4: Update invariants.md in place**

Per the spec's "Pass-end deliverables" §4:
- *Elm architecture & idiomatic bubbletea* section: extend the `tea.View` line to enumerate `AltScreen`, `WindowTitle`, `ProgressBar`, `ReportFocus`, `KeyboardEnhancements`.
- Add a one-sentence statement of the priority ladder.
- Add a one-sentence statement of the `RequiresKittyKbd` tag.
- *Config & theming* section: add `new-mail-toast` to the `[ui]` table description.

- [ ] **Step 5: Update INDEX.md**

Add a row to `docs/poplar/decisions/INDEX.md` pointing the four binding facts to ADR-0217.

- [ ] **Step 6: Update ui-invariants.md**

Add the new toast variant + `RequiresKittyKbd` tag to `.claude/rules/ui-invariants.md`.

- [ ] **Step 7: Update keybindings.md**

Add the "Catkin chords (Kitty keyboard protocol)" section as described in the spec.

- [ ] **Step 8: Update bubbletea-conventions.md**

§10 checklist parenthetical: extend the `tea.View` declarative-fields list.

- [ ] **Step 9: Update STATUS.md**

Mark Pass 32 done; rewrite the next starter prompt to Pass 33 (mouse support — see ROADMAP `mouse-support`).

- [ ] **Step 10: Update ROADMAP.md**

Move `v2-view-fields` to the Done section. Note deferred consumers (`IsRepeat` for held-key accel, server-side IDLE pause).

- [ ] **Step 11: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-11-v2-view-fields.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-11-v2-view-fields-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 12: make check**

Run: `make check`
Expected: PASS (vet + voice scan + modern-go + tests).

- [ ] **Step 13: Commit + push + install**

```bash
git add -A
git commit -m "Pass 32: v2 declarative View fields

Wires ProgressBar (priority ladder), ReportFocus + new-mail toast,
KeyboardEnhancements (capability-aware help filtering) per frame in
App.View. ADR-0217.

Co-Authored-By: Claude <noreply@anthropic.com>"
git push
make install
```
