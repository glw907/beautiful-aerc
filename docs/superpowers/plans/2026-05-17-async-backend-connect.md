# Async Backend Connect Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** UI launches in ~100 ms from cache; `backend.Connect` runs in a `tea.Cmd` and wires the cache on success via `BackendReadyMsg`.

**Architecture:** Split `cache.Open` into `Open` (sqlite + migrations only, no backend) and `WireBackend(backend, ct)` (starts drainer + backfiller + push). `cmd/poplar/root.go` opens the cache pre-UI; `App.Init` returns a `connectBackendCmd` that calls `Connect` and wires the cache before emitting `BackendReadyMsg`. Backend-touching cache read paths return a new `cache.ErrNotConnected` sentinel pre-wire; the UI tolerates empty state.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `modernc.org/sqlite`, `slog`. Project conventions in `go-conventions` + `elm-conventions` skills; pre-beta stance in `CLAUDE.md`.

**Spec:** `docs/superpowers/specs/2026-05-17-async-backend-connect-design.md`

---

## File Structure

**Cache layer split.** `internal/cache/account.go` gains `WireBackend` and an `accountName string` field; `Open` loses its `backend` and `ct` parameters. `internal/cache/backfill.go` start moves from `Open` to `WireBackend`. Backend-touching paths in `reads.go`, `attachments.go`, and `syncer.go` gate on `a.Backend != nil` and return `ErrNotConnected`.

**UI state additions.** `internal/uicore/types.go` (or a new `backend_state.go`) gains a `BackendState` enum. `internal/ui/cmds.go` gains `BackendReadyMsg`, `BackendErrMsg`, `connectBackendCmd`. `internal/ui/app.go` gains `backendState` + `backendErr` fields and a handler claim path (kept in `app_chrome.go` since chrome owns the status bar). `internal/ui/status_bar.go` extends the connection-state render to cover `Connecting`. `internal/ui/messagelist/messagelist.go` extends its empty-state render. A new `retryConnect` `key.Binding` lives in `app_keys.go`.

**Root wiring.** `cmd/poplar/root.go` reorders: `cache.Open` (no backend) → `ui.NewApp` → `tea.Run`. The synchronous `backend.Connect` call is deleted.

**One new ADR:** `docs/poplar/decisions/0242-async-backend-connect.md`.

---

## Task 1: `cache.ErrNotConnected` sentinel

Add the sentinel and a `WireBackend`/`Connected()` accessor pair so subsequent tasks have something to gate on. No callers yet.

**Files:**
- Modify: `internal/cache/account.go`
- Test: `internal/cache/account_test.go` (existing? create if missing)

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/account_test.go` (create the file if absent — standard `package cache` test file):

```go
func TestAccount_Connected_ReportsBackendPresence(t *testing.T) {
	a := &Account{}
	if a.Connected() {
		t.Fatalf("nil backend: Connected() = true, want false")
	}
	a.Backend = &mail.MockBackend{}
	if !a.Connected() {
		t.Fatalf("non-nil backend: Connected() = false, want true")
	}
}

func TestErrNotConnected_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("fetch headers: %w", ErrNotConnected)
	if !errors.Is(wrapped, ErrNotConnected) {
		t.Fatalf("errors.Is wrapped ErrNotConnected = false")
	}
}
```

Imports: `"errors"`, `"fmt"`, `"testing"`, `"github.com/glw907/poplar/internal/mail"`.

- [ ] **Step 2: Run and confirm failure**

```bash
go test -tags=dev ./internal/cache/ -run TestAccount_Connected_ReportsBackendPresence -v
go test -tags=dev ./internal/cache/ -run TestErrNotConnected_IsSentinel -v
```

Expected: compile error (`undefined: ErrNotConnected`, `Account.Connected undefined`).

- [ ] **Step 3: Implement**

In `internal/cache/account.go`, add near the other error sentinels (search for `ErrNotPending` or `ErrNotConflict` to find the section):

```go
// ErrNotConnected is returned by cache methods that require an
// authenticated backend before a successful WireBackend call.
// Callers tolerant of degraded reads (empty headers, missing body)
// should branch on errors.Is(err, ErrNotConnected) and render a
// "waiting for connection" hint instead of a hard error.
var ErrNotConnected = errors.New("cache: backend not yet wired")
```

Add the accessor near `Name()`:

```go
// Connected reports whether WireBackend has been called and a
// backend is available for backfill, sync, and outbox dispatch.
func (a *Account) Connected() bool { return a.Backend != nil }
```

- [ ] **Step 4: Run and confirm pass**

```bash
go test -tags=dev ./internal/cache/ -run 'TestAccount_Connected_ReportsBackendPresence|TestErrNotConnected_IsSentinel' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/account.go internal/cache/account_test.go
git commit -m "$(cat <<'EOF'
cache: add ErrNotConnected sentinel + Connected()

Foundation for splitting cache.Open from backend wiring. No
callers gate on these yet; subsequent tasks introduce the read-
path guards and the WireBackend entry point.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Split `cache.Open` from backend wiring

Make `Open` take only what it needs to open SQLite and run migrations. Introduce `WireBackend(backend, ct)` that assigns the fields and starts the backfiller (drainer wiring moves in Task 3). Fix the `AccountName` layering bug by storing the name at `Open` time.

**Files:**
- Modify: `internal/cache/account.go:146-204`
- Modify: every caller of `cache.Open` (search first; see Step 1)
- Test: `internal/cache/account_test.go`

- [ ] **Step 1: Find all callers**

```bash
grep -rn 'cache\.Open(' --include='*.go' .
```

Expected: `cmd/poplar/root.go:173`, `cmd/poplar/cache.go` (any CLI subcommand), `internal/cache/integration_test.go` and any other `_test.go` under `internal/cache/`.

- [ ] **Step 2: Write failing tests**

Add to `internal/cache/account_test.go`:

```go
func TestOpen_NoBackend_SucceedsAndDeferredWire(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if a.Connected() {
		t.Fatalf("Connected() after Open without WireBackend = true, want false")
	}
	if got := a.AccountName(); got != "Test" {
		t.Fatalf("AccountName() pre-wire = %q, want %q", got, "Test")
	}
	if got := a.AccountEmail(); got != "" {
		t.Fatalf("AccountEmail() pre-wire = %q, want \"\"", got)
	}
}

func TestWireBackend_AssignsAndStartsBackfiller(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	b := mail.NewMockBackend()
	if err := a.WireBackend(b, b); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	if !a.Connected() {
		t.Fatalf("Connected() after WireBackend = false")
	}
	// Second call must error — backend is wired exactly once.
	if err := a.WireBackend(b, b); err == nil {
		t.Fatalf("second WireBackend = nil, want error")
	}
}
```

Imports already cover `mail`. `mail.NewMockBackend` exists today behind the `dev` build tag; if its signature differs, adapt the test to match.

- [ ] **Step 3: Confirm failure**

```bash
go test -tags=dev ./internal/cache/ -run 'TestOpen_NoBackend|TestWireBackend_AssignsAndStarts' -v
```

Expected: compile error (`too few arguments`, `Account.WireBackend undefined`).

- [ ] **Step 4: Change `Open` signature**

Replace `internal/cache/account.go:146-189` with:

```go
// Open returns an Account with the SQLite store opened and migrated
// to the current schema version. The returned Account has no
// backend wired; reads that hit the local store work immediately,
// but anything that requires the network (FetchBody on cache miss,
// FetchHeaders, Attachments, drainer dispatch) returns
// ErrNotConnected until WireBackend succeeds.
//
// dir is the cache base directory. The per-account subdirectory is
// created if absent. A leading ~ is expanded to the user's home.
// A nil log defaults to slog.Default() tagged with the package name.
func Open(accountName string, dir string, cfg Config, log *slog.Logger) (*Account, error) {
	if log == nil {
		log = slog.Default().With("component", "cache")
	}
	dbPath, err := DBPath(accountName, dir)
	if err != nil {
		return nil, err
	}
	acctDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(acctDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache: %w", err)
	}
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	log.Debug("cache open", "schema", schemaVersion, "account", accountName)
	return &Account{
		log:               log,
		db:                db,
		dir:               acctDir,
		name:              accountName,
		maxSize:           cfg.MaxSize,
		maxAttachmentSize: cfg.MaxAttachmentSize,
		maxOutboxBytes:    cfg.MaxOutboxBytes,
		events:            make(chan Event, 32),
		drainSignal:       make(chan struct{}, 1),
		stop:              make(chan struct{}),
	}, nil
}

// WireBackend assigns the backend and change tracker and starts the
// per-account backfiller. Call exactly once per Account lifetime,
// after the backend's Connect has succeeded. Returns an error if a
// backend is already wired.
func (a *Account) WireBackend(backend mail.Backend, ct mail.ChangeTracker) error {
	if a.Backend != nil {
		return errors.New("cache: backend already wired")
	}
	a.Backend = backend
	a.ChangeTracker = ct
	a.backfiller = newBackfiller(a)
	bfCtx, cancel := context.WithCancel(context.Background())
	a.backfillStop = cancel
	go a.backfiller.Run(bfCtx)
	return nil
}
```

Update `AccountName` to drop the backend delegation (the bug noted in the spec):

```go
func (a *Account) AccountName() string { return a.name }
```

`AccountEmail` keeps its current nil-guard; pre-wire it already returns `""`.

- [ ] **Step 5: Update every `cache.Open` caller**

For every call site found in Step 1, change:

```go
acct, err := cache.Open(name, backend, ct, dir, cfg, log)
```

to:

```go
acct, err := cache.Open(name, dir, cfg, log)
if err != nil { /* unchanged */ }
if err := acct.WireBackend(backend, ct); err != nil {
    return fmt.Errorf("wire backend: %w", err)
}
```

For `cmd/poplar/root.go:173`, this is a temporary form — Task 8 moves the wiring out of `runRoot` entirely. Leave it inline for this task to keep `make test` green between tasks.

For `cmd/poplar/cache.go` CLI subcommands that only read the SQLite store, drop the `WireBackend` call entirely (CLI inspection doesn't need a backend; this is the whole point of the split). Verify by reading each subcommand to confirm no `acct.Backend.*` access; if any subcommand does need the backend, keep the inline wire.

- [ ] **Step 6: Confirm tests pass**

```bash
make check
```

Expected: PASS. If any test in `internal/cache/` fails because it constructed an `Account{}` directly with a nil backend and now hits a panic via a guarded path, defer that to Task 4 — note the failure here and add an explicit `t.Skip("guarded in task 4: <name>")` only if absolutely necessary. Prefer fixing inline.

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
cache: split Open from backend wiring

Open is now sqlite + migrations only. WireBackend assigns the
backend, change tracker, and starts the backfiller. AccountName
no longer delegates to the backend — name comes from Open.

All callers updated to the two-step form. cmd/poplar/root.go
keeps the inline wire for now; a later task hoists Connect into a
tea.Cmd.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Drainer start moves into `WireBackend`

Today the drainer is started by `(*Account).StartDrainer(ctx)`, called from `cmd/poplar/root.go:183`. Move the call into `WireBackend` so a wired account is always a drainer-running account, and `runRoot` doesn't need to know about it.

**Files:**
- Modify: `internal/cache/account.go` (`WireBackend`, possibly `StartDrainer`)
- Modify: `cmd/poplar/root.go` (drop the `StartDrainer` call)
- Test: `internal/cache/account_test.go`

- [ ] **Step 1: Read the current StartDrainer signature**

```bash
grep -n 'func.*StartDrainer' internal/cache/*.go
```

Note the signature. If it takes a `ctx context.Context`, the wired form needs to derive its own (the drainer outlives any single caller's ctx).

- [ ] **Step 2: Write the failing test**

Add to `internal/cache/account_test.go`:

```go
func TestWireBackend_StartsDrainer(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	b := mail.NewMockBackend()
	if err := a.WireBackend(b, b); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	// Queue a no-op flag op and confirm the drainer picks it up.
	// (Exact mechanics depend on MockBackend's hooks; assert on
	// b.FlagCalls() or equivalent within 200ms.)
	// ... use the existing drainer integration-test idiom.
}
```

If the test scaffolding for drainer assertions is complex, lift it from `internal/cache/integration_test.go` instead of reinventing.

- [ ] **Step 3: Confirm failure**

```bash
go test -tags=dev ./internal/cache/ -run TestWireBackend_StartsDrainer -v
```

Expected: drainer never observes the queued op (no Start happened).

- [ ] **Step 4: Move drainer start into WireBackend**

In `internal/cache/account.go`, update `WireBackend`:

```go
func (a *Account) WireBackend(backend mail.Backend, ct mail.ChangeTracker) error {
	if a.Backend != nil {
		return errors.New("cache: backend already wired")
	}
	a.Backend = backend
	a.ChangeTracker = ct
	a.backfiller = newBackfiller(a)
	bfCtx, cancel := context.WithCancel(context.Background())
	a.backfillStop = cancel
	go a.backfiller.Run(bfCtx)
	drainerCtx, drainerCancel := context.WithCancel(context.Background())
	a.drainerStop = drainerCancel
	if err := a.startDrainer(drainerCtx); err != nil {
		return fmt.Errorf("start drainer: %w", err)
	}
	return nil
}
```

Rename the existing exported `StartDrainer` to unexported `startDrainer` (it now has only one caller — `WireBackend`). Add the `drainerStop context.CancelFunc` field to the `Account` struct and call it in `Close()` (search for the existing close path).

- [ ] **Step 5: Drop the call from root.go**

In `cmd/poplar/root.go` around line 182–185, delete:

```go
log.Debug("starting drainer")
if err := acct.StartDrainer(ctx); err != nil {
    return fmt.Errorf("start drainer: %v", err)
}
```

The drainer now starts inside `WireBackend`, which is still called inline in `runRoot` until Task 8.

- [ ] **Step 6: Run all tests**

```bash
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
cache: WireBackend starts the drainer

StartDrainer is no longer a separate ceremony — a wired account
is a drainer-running account. cmd/poplar/root.go drops the
explicit call; the drainer's lifetime now matches the wired
backend's.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Gate cache read paths on `Connected()`

Backend-touching reads (`FetchHeaders` backfill, `FetchBody` miss, `Attachments`, `FetchAttachment`, `SyncFolders`) must return `ErrNotConnected` pre-wire. Cache-only reads (`QueryFolder`, `ListFolders`, `FetchBodyCached`, `SuggestAddresses`, `LookupContact`) must work unchanged.

**Files:**
- Modify: `internal/cache/reads.go` (around lines 229, 261 per the spec)
- Modify: `internal/cache/attachments.go` (lines 26, 115)
- Modify: `internal/cache/syncer.go` (lines 69, 109)
- Modify: `internal/cache/backfill.go` (line 71)
- Test: `internal/cache/account_test.go`

- [ ] **Step 1: Find every `a.Backend.` or `acct.Backend.` access**

```bash
grep -n 'a\.Backend\.\|acct\.Backend\.\|b\.acct\.Backend\.' internal/cache/*.go | grep -v _test.go
```

Verify the list matches the spec's enumeration (FetchBody, FetchHeaders, FetchAttachment, Attachments, ListFolders, Move/Flag/Destroy/Send/Append/PushDraft). The drainer's mutating calls run only after `WireBackend`, so they're safe without explicit gating, but cheap belt-and-suspenders gating is fine if the test for "drainer dispatch pre-wire" would otherwise crash.

- [ ] **Step 2: Write failing tests for each read path**

Add to `internal/cache/account_test.go`:

```go
func TestFetchBody_PreWire_ReturnsErrNotConnected(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	_, err = a.FetchBody(mail.UID("nonexistent"))
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("FetchBody pre-wire = %v, want ErrNotConnected", err)
	}
}

func TestAttachments_PreWire_ReturnsErrNotConnected(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	_, err = a.Attachments(context.Background(), mail.UID("nonexistent"))
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Attachments pre-wire = %v, want ErrNotConnected", err)
	}
}

func TestListFolders_PreWire_WorksFromCache(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	folders, err := a.ListFolders()
	if err != nil {
		t.Fatalf("ListFolders pre-wire = %v, want nil (empty cache)", err)
	}
	if len(folders) != 0 {
		t.Fatalf("ListFolders pre-wire on empty cache = %d folders, want 0", len(folders))
	}
}
```

- [ ] **Step 3: Confirm failure**

```bash
go test -tags=dev ./internal/cache/ -run 'TestFetchBody_PreWire|TestAttachments_PreWire|TestListFolders_PreWire' -v
```

Expected: nil-pointer panic or unrelated errors.

- [ ] **Step 4: Add the guards**

For each backend-touching read path, insert at the top of the function (after argument validation):

```go
if !a.Connected() {
    return /* zero value */, ErrNotConnected
}
```

Example for `reads.go` `FetchBody`:

```go
func (a *Account) FetchBody(uid mail.UID) ([]byte, error) {
    if cached, ok := a.bodyCacheGet(uid); ok {
        return cached, nil
    }
    if !a.Connected() {
        return nil, ErrNotConnected
    }
    body, err := a.Backend.FetchBody(uid)
    // ... unchanged
}
```

Note: the cache-hit branch returns before the gate — pre-wire reads still succeed when the body is cached. This is exactly the behavior the spec calls for. Mirror this pattern for `Attachments` / `FetchAttachment` (return cached metadata / bytes when present; gate before backend dispatch).

For `ListFolders` in `reads.go`, the existing implementation already reads from the `folders` table — verify by inspection. It should not need a guard (cache-only read). If it currently delegates to `Backend.ListFolders()`, that's a layering bug — refactor inline to read from the cache table.

For `syncer.go` and `backfill.go`, the gate prevents the syncer from running pre-wire. The backfiller's `Run` loop already gates on `connOnline` per `cache-invariants.md`; verify it gracefully no-ops when the Backend itself is nil (the inner `FetchBody` call now returns `ErrNotConnected`, so the backfiller sees an error and backs off). If the backfiller's start in `WireBackend` removes the pre-wire concern entirely, no change needed in `backfill.go`.

- [ ] **Step 5: Run tests**

```bash
make check
```

Expected: PASS. Resolve any breakage in existing tests by either: (a) the test now needs to call `WireBackend` before exercising the path, or (b) a code path is incorrectly delegating to backend for cached data — fix the layering.

- [ ] **Step 6: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
cache: gate backend-touching reads on Connected()

FetchBody (on miss), Attachments, FetchAttachment, and syncer
return ErrNotConnected pre-wire. Cache-only reads (ListFolders,
QueryFolder, FetchBodyCached on hit) work unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Add `mail.ConnConnecting` enum value

The status bar currently distinguishes `ConnOffline`, `ConnReconnecting`, `ConnConnected`. Add `ConnConnecting` for the pre-authenticated initial state. Distinct from `Reconnecting` (which implies "was connected, lost it").

**Files:**
- Modify: `internal/mail/types.go:52-54`
- Modify: any switch over `mail.ConnState` that needs the new case

- [ ] **Step 1: Find consumers**

```bash
grep -rn 'ConnState\|ConnOffline\|ConnReconnecting\|ConnConnected' --include='*.go' . | grep -v _test.go
```

Note every `switch` statement — each must add a `case ConnConnecting:` or fall through with explicit acknowledgment.

- [ ] **Step 2: Add the enum value**

In `internal/mail/types.go` around line 52:

```go
const (
	ConnOffline ConnState = iota
	ConnReconnecting
	ConnConnected
	ConnConnecting
)
```

Append, don't reorder — the underlying ints are stable per ADR convention for zero-value safety; `ConnOffline = 0` must stay the zero value so existing zero-value `ConnState` fields keep meaning "offline".

- [ ] **Step 3: Add a String() if one exists**

If `ConnState` has a `String()` method (search), add the case for `ConnConnecting` returning `"connecting"`.

- [ ] **Step 4: Run tests to find broken switches**

```bash
make check 2>&1 | head -30
```

`go vet`'s `exhaustive` lint isn't enabled by default; missing cases will compile silently. Run targeted greps from Step 1 and confirm each switch site is intentional about the new case. Most will need `case ConnConnecting:` rendering like `ConnReconnecting` (spinner + orange).

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
mail: add ConnConnecting state

Distinct from ConnReconnecting — Connecting is the pre-
authenticated initial state, Reconnecting implies a session was
established and dropped. Appended to preserve zero-value
meaning for ConnOffline.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `BackendReadyMsg` / `BackendErrMsg` / `connectBackendCmd`

Add the three pieces of plumbing the App will consume in Task 7. The Cmd owns the wire step — by the time `App.Update` sees `BackendReadyMsg`, the account is fully wired.

**Files:**
- Modify: `internal/ui/cmds.go`
- Test: `internal/ui/cmds_test.go` (existing? create if missing)

- [ ] **Step 1: Write failing test**

Add to `internal/ui/cmds_test.go`:

```go
func TestConnectBackendCmd_Success_WiresAndEmitsReady(t *testing.T) {
	dir := t.TempDir()
	acct, err := cache.Open("Test", dir, cache.Config{}, nil)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer acct.Close()
	b := mail.NewMockBackend()
	cmd := connectBackendCmd(context.Background(), b, acct)
	msg := cmd()
	if _, ok := msg.(BackendReadyMsg); !ok {
		t.Fatalf("msg = %T, want BackendReadyMsg", msg)
	}
	if !acct.Connected() {
		t.Fatalf("Connected() after BackendReadyMsg = false")
	}
}

func TestConnectBackendCmd_Failure_EmitsErr(t *testing.T) {
	dir := t.TempDir()
	acct, err := cache.Open("Test", dir, cache.Config{}, nil)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer acct.Close()
	b := mail.NewMockBackend()
	wantErr := errors.New("network down")
	b.SetConnectErr(wantErr) // MockBackend hook; add if missing
	cmd := connectBackendCmd(context.Background(), b, acct)
	msg := cmd()
	got, ok := msg.(BackendErrMsg)
	if !ok {
		t.Fatalf("msg = %T, want BackendErrMsg", msg)
	}
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("Err = %v, want %v", got.Err, wantErr)
	}
	if acct.Connected() {
		t.Fatalf("Connected() after failed connect = true, want false")
	}
}
```

If `MockBackend` doesn't have a `SetConnectErr` hook, add one in the same task (it's the dev-tag-only mock; adding hooks is appropriate).

- [ ] **Step 2: Confirm failure**

```bash
go test -tags=dev ./internal/ui/ -run TestConnectBackendCmd -v
```

Expected: compile error (`undefined: connectBackendCmd`, `BackendReadyMsg`, `BackendErrMsg`).

- [ ] **Step 3: Implement**

Append to `internal/ui/cmds.go`:

```go
// BackendReadyMsg fires after a successful backend.Connect and a
// successful cache.WireBackend. The account is fully wired by the
// time the App handles this msg — Update should kick off the
// initial sync and the push pump.
type BackendReadyMsg struct{}

// BackendErrMsg fires when backend.Connect fails or WireBackend
// returns an error. The account remains unwired; cached reads
// still work. The user retries via the status-bar binding.
type BackendErrMsg struct{ Err error }

// connectBackendCmd runs backend.Connect and on success calls
// acct.WireBackend so subsequent Cmds see a wired account.
func connectBackendCmd(ctx context.Context, b mail.Backend, acct *cache.Account) tea.Cmd {
	return func() tea.Msg {
		if err := b.Connect(ctx); err != nil {
			return BackendErrMsg{Err: err}
		}
		ct, _ := b.(mail.ChangeTracker)
		if err := acct.WireBackend(b, ct); err != nil {
			return BackendErrMsg{Err: fmt.Errorf("wire backend: %w", err)}
		}
		return BackendReadyMsg{}
	}
}
```

The `mail.ChangeTracker` assertion mirrors today's `cmd/poplar/root.go:168-170` check. If the cast fails, `WireBackend` accepts a nil `ct` (matching the pre-existing nil-safe code paths).

- [ ] **Step 4: Run tests**

```bash
go test -tags=dev ./internal/ui/ -run TestConnectBackendCmd -v
make check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
ui: add BackendReadyMsg + BackendErrMsg + connectBackendCmd

The Cmd owns the wire step so App.Update always sees a fully-
wired account on BackendReadyMsg. Failure surfaces as
BackendErrMsg; the user retries from the status bar.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `BackendState` enum on App + Update claims

App gains `backendState` + `backendErr`. `Init` returns the connect Cmd instead of `pumpUpdatesCmd(m.acct.Backend())` (which would panic on nil). On `BackendReadyMsg`, kick off the pump + initial sync + contacts sync. On `BackendErrMsg`, store the error.

**Files:**
- Modify: `internal/uicore/` (new `backend_state.go` or extend an existing types file)
- Modify: `internal/ui/app.go` (struct fields, `Init`)
- Modify: `internal/ui/app_chrome.go` (handler claim path — chrome already owns connection state)
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Add BackendState enum**

Create `internal/uicore/backend_state.go`:

```go
package uicore

// BackendState reflects the App's view of the backend lifecycle.
// Distinct from mail.ConnState — BackendState tracks whether
// connectBackendCmd has succeeded; mail.ConnState reflects the
// backend's own transport health once wired.
type BackendState int

const (
	BackendConnecting BackendState = iota
	BackendConnected
	BackendFailed
)
```

- [ ] **Step 2: Add App fields**

In `internal/ui/app.go`, find the `App` struct and add:

```go
backendState uicore.BackendState
backendErr   error
```

`uicore.BackendConnecting` is the zero value — no constructor change needed; `App` defaults to "connecting".

- [ ] **Step 3: Rewrite `App.Init`**

Replace `internal/ui/app.go:218-227`:

```go
func (m App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.acct.Init(),
		connectBackendCmd(context.Background(), m.acct.Backend(), m.acct),
	}
	return tea.Batch(cmds...)
}
```

Wait — `m.acct.Backend()` will be nil pre-wire. We need a different accessor: the App needs the *unwired* backend value so it can pass it to `connectBackendCmd`. Add a field to `App`:

```go
backend mail.Backend // held until connectBackendCmd wires it into m.acct
```

Update `NewApp` (line 145) to take `backend mail.Backend` as a parameter and stash it:

```go
func NewApp(t *theme.CompiledTheme, backend mail.Backend, acct *cache.Account, uiCfg config.UIConfig, icons uicore.IconSet, m ansix.Measurer, contactsCfg *config.ContactsConfig, identities []config.Identity) App {
    // ...
    app.backend = backend
    // ...
}
```

Then `Init` becomes:

```go
func (m App) Init() tea.Cmd {
	return tea.Batch(
		m.acct.Init(),
		connectBackendCmd(context.Background(), m.backend, m.acct),
	)
}
```

Contacts sync and pump move to the `BackendReadyMsg` handler.

- [ ] **Step 4: Add Update handlers**

In `internal/ui/app_chrome.go`, find the existing `updateChromeMsg` (or similar `updateXMsg`-shaped function — see CLAUDE.md's note that `App` splits into per-domain `app_X.go` files). Add cases or a new domain handler `app_connect.go`:

```go
package ui

import (
    tea "charm.land/bubbletea/v2"
    "github.com/glw907/poplar/internal/mail"
    "github.com/glw907/poplar/internal/ui/uicore"
)

// updateConnectMsg claims BackendReadyMsg / BackendErrMsg and
// fans out the post-wire follow-ups (push pump, contacts sync).
func (m App) updateConnectMsg(msg tea.Msg) (App, tea.Cmd, bool) {
    switch msg := msg.(type) {
    case BackendReadyMsg:
        m.backendState = uicore.BackendConnected
        m.backendErr = nil
        cmds := []tea.Cmd{pumpUpdatesCmd(m.backend)}
        if m.contactsCfg != nil {
            cmds = append(cmds,
                syncContactsCmd(m.acct.Cache(), m.contactsCfg),
                scheduleSyncCmd(m.contactsRefresh),
            )
        }
        return m, tea.Batch(cmds...), true
    case BackendErrMsg:
        m.backendState = uicore.BackendFailed
        m.backendErr = msg.Err
        return m, nil, true
    }
    return m, nil, false
}
```

Wire `updateConnectMsg` into the `App.Update` dispatcher chain. Follow whatever pattern is already used for `updateOutboxMsg`, `updateComposeMsg`, etc. (this is the project's `(App, tea.Cmd, claimed bool)` convention per CLAUDE.md).

- [ ] **Step 5: Write App-level tests**

Add to `internal/ui/app_test.go`:

```go
func TestApp_InitialBackendStateIsConnecting(t *testing.T) {
    app := newTestApp(t) // existing helper
    if app.backendState != uicore.BackendConnecting {
        t.Fatalf("initial backendState = %v, want Connecting", app.backendState)
    }
}

func TestApp_BackendReadyMsg_TransitionsAndKicksFollowups(t *testing.T) {
    app := newTestApp(t)
    next, cmd := app.Update(BackendReadyMsg{})
    if next.(App).backendState != uicore.BackendConnected {
        t.Fatalf("after BackendReadyMsg: state = %v, want Connected", next.(App).backendState)
    }
    if cmd == nil {
        t.Fatalf("after BackendReadyMsg: cmd = nil, want pump + sync batch")
    }
}

func TestApp_BackendErrMsg_StoresError(t *testing.T) {
    app := newTestApp(t)
    wantErr := errors.New("boom")
    next, _ := app.Update(BackendErrMsg{Err: wantErr})
    got := next.(App)
    if got.backendState != uicore.BackendFailed {
        t.Fatalf("state = %v, want Failed", got.backendState)
    }
    if !errors.Is(got.backendErr, wantErr) {
        t.Fatalf("backendErr = %v, want %v", got.backendErr, wantErr)
    }
}
```

If `newTestApp` doesn't exist with the new `backend` parameter, extend the helper to take a mock backend.

- [ ] **Step 6: Run all tests**

```bash
make check
```

Expected: PASS. The existing tests that call `pumpUpdatesCmd(m.acct.Backend())` directly will need adjusting — they were testing post-wire behavior; either feed them through a `BackendReadyMsg` first, or call the helper that does so.

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
ui: App tracks BackendState, dispatches connect from Init

App.Init returns connectBackendCmd; BackendReadyMsg fans out the
pump + contacts sync. BackendErrMsg stores the error for the
status bar. App holds the unwired backend value until wire.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Reorder `cmd/poplar/root.go`

Remove the synchronous `backend.Connect` + inline `WireBackend` from `runRoot`. Open the cache pre-UI, pass the backend into `NewApp`, let `App.Init` drive the rest.

**Files:**
- Modify: `cmd/poplar/root.go:139-186`

- [ ] **Step 1: Rewrite the startup block**

Replace lines 139–186 of `cmd/poplar/root.go` with:

```go
wireTrace := uiCfg.WireTrace || os.Getenv("POPLAR_WIRE_TRACE") == "1"
log := slog.With("account", accts[0].Name)
log.Debug("opening backend", "provider", accts[0].Backend)
backend, err := openBackend(accts[0], wireTrace)
if err != nil {
    return fmt.Errorf("open backend %q: %v", accts[0].Name, err)
}
defer backend.Disconnect()

hasNF := term.HasNerdFont()
probe := term.MeasureSPUACells()
mode, cellWidth := term.Resolve(uiCfg.Icons, hasNF, probe)

iconSet := uicore.SimpleIcons
if mode == term.IconModeFancy {
    iconSet = uicore.FancyIcons
}
measurer := ansix.NewMeasurer(cellWidth)

cacheCfg, err := config.LoadCache(configPath)
if err != nil {
    return fmt.Errorf("cache config: %v", err)
}

log.Debug("opening cache")
acct, err := cache.Open(accts[0].Name, "", cache.Config{
    MaxSize:           cacheCfg.MaxSize,
    MaxAttachmentSize: cacheCfg.MaxAttachmentSize,
    MaxOutboxBytes:    cacheCfg.MaxOutboxBytes,
}, nil)
if err != nil {
    return fmt.Errorf("open cache for %s: %v", accts[0].Name, err)
}
defer acct.Close()
log.Debug("startup complete, launching UI")

app := ui.NewApp(t, backend, acct, uiCfg, iconSet, measurer, accts[0].Contacts, accts[0].Identities)

p := tea.NewProgram(appModel{app: app})
if _, err := p.Run(); err != nil {
    return err
}
return nil
```

Notes:
- The `ChangeTracker` cast (today at lines 168–171) moves into `connectBackendCmd` (already added in Task 6). Delete the check from `runRoot`.
- `backend.Disconnect()` deferred at start. Verify that calling `Disconnect()` on an unconnected backend is a no-op for both `mailjmap` and `mailimap`; add a guard if needed in a follow-up commit within this task.
- The `ctx, cancel := context.WithCancel(...)` lines around 144 can be deleted if no remaining caller uses `ctx` — bubbletea owns its own lifecycle now. Search for `cancel(` and `ctx` references after the rewrite and prune.

- [ ] **Step 2: Verify Disconnect-pre-connect is safe**

```bash
grep -n -A 10 'func.*Disconnect' internal/mailjmap/jmap.go internal/mailimap/imap.go
```

Read both `Disconnect` implementations. They should tolerate being called when `Connect` was never invoked (no panic on nil session/connection). If either panics, add a guard:

```go
func (b *Backend) Disconnect() {
    if b.session == nil {
        return
    }
    // ... existing logic
}
```

Commit this guard separately if it requires real changes.

- [ ] **Step 3: Run all tests**

```bash
make check
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

```bash
make install
poplar --help
```

Then launch poplar against your real Fastmail account in a tmux pane (see `.claude/docs/tmux-testing.md` if unfamiliar). Confirm:
- UI renders within ~100 ms (visibly instant).
- Status bar shows a connecting indicator (it'll be empty / wrong until Task 9, but no crash).
- Folders + messages populate within a few seconds.
- `q` quits cleanly.

If anything panics or the UI hangs forever, debug before committing.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
cmd/poplar: connect backend asynchronously via tea.Cmd

cache.Open runs pre-UI without a backend; ui.NewApp receives the
unwired backend and account, App.Init dispatches connectBackendCmd.
The UI renders immediately from cache; the synchronous network
wait between launch and first frame is gone.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Status bar renders `Connecting` and `Disconnected`

Extend the connection-state segment of the status bar to cover the new states, sourcing from `App.backendState` + `App.backendErr` (not from `mail.ConnState`, which only becomes meaningful post-wire).

**Files:**
- Modify: `internal/ui/status_bar.go:132` and surrounding render path
- Modify: `internal/ui/app_view.go` (or wherever `statusBar.SetConnectionState` is called)
- Test: `internal/ui/status_bar_test.go` (existing? extend; create if missing)

- [ ] **Step 1: Read the current connection-state render**

```bash
grep -n -B 2 -A 15 'connText\|ConnectionState\|SetConnection' internal/ui/status_bar.go
```

Note the existing glyph + color rules (per `ui-invariants.md`: `●` green connected, `◐` orange reconnecting, `○` red offline). Add a fourth case for `Connecting`: same `◐` orange glyph as `Reconnecting` but text "connecting…". For `Failed` (the new App-level state), use `○` red with text like "disconnected · r retry".

- [ ] **Step 2: Write failing test**

Add to `internal/ui/status_bar_test.go`:

```go
func TestStatusBar_Connecting_ShowsSpinnerAndText(t *testing.T) {
    sb := NewStatusBar(...).SetBackendState(uicore.BackendConnecting, nil)
    out := sb.View()
    if !strings.Contains(out, "connecting") {
        t.Fatalf("Connecting render missing 'connecting' text: %q", out)
    }
}

func TestStatusBar_Failed_ShowsErrorAndRetryHint(t *testing.T) {
    sb := NewStatusBar(...).SetBackendState(uicore.BackendFailed, errors.New("dial: timeout"))
    out := sb.View()
    if !strings.Contains(out, "dial: timeout") {
        t.Fatalf("Failed render missing error text: %q", out)
    }
    if !strings.Contains(out, "r retry") {
        t.Fatalf("Failed render missing 'r retry' hint: %q", out)
    }
}
```

Use the test fixture pattern already used in `status_bar_test.go`; the `NewStatusBar(...)` constructor signature may need extending.

- [ ] **Step 3: Confirm failure**

```bash
go test -tags=dev ./internal/ui/ -run 'TestStatusBar_Connecting|TestStatusBar_Failed' -v
```

Expected: compile error (`SetBackendState undefined`).

- [ ] **Step 4: Add `SetBackendState`**

In `internal/ui/status_bar.go`, add a method:

```go
// SetBackendState sets the App-level connection lifecycle.
// Pre-wire, BackendConnecting / BackendFailed override the
// mail.ConnState-derived render. Once BackendConnected, the
// rendering falls through to mail.ConnState (live transport
// health from the backend's update stream).
func (s StatusBar) SetBackendState(state uicore.BackendState, err error) StatusBar {
    s.backendState = state
    s.backendErr = err
    return s
}
```

Add the corresponding fields to `StatusBar` and extend the render branch (around line 132) to consult `s.backendState` first; only fall through to the `s.connState` switch when `s.backendState == BackendConnected`.

- [ ] **Step 5: Plumb from App**

In `internal/ui/app_chrome.go` (or wherever `m.statusBar = m.statusBar.Set...` calls live), add a call after every `BackendReadyMsg` / `BackendErrMsg` handler:

```go
m.statusBar = m.statusBar.SetBackendState(m.backendState, m.backendErr)
```

The simplest path: extend `deriveChromeFromAcct` (line 231) to read App's backend state, since chrome is re-derived on most events anyway. Add the SetBackendState call there.

- [ ] **Step 6: Run tests**

```bash
make check
```

Expected: PASS.

- [ ] **Step 7: Manual smoke test**

```bash
make install && poplar
```

Watch the status bar: spinner + "connecting…" for 1–6 s, then green "connected". With the network blocked (`sudo iptables -A OUTPUT -d api.fastmail.com -j REJECT` then `poplar`; remove with `-D` after), confirm "disconnected · r retry" appears.

- [ ] **Step 8: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
ui: status bar renders Connecting and Failed states

App.backendState overrides the mail.ConnState-driven render
pre-wire; Connecting shows the spinner glyph + 'connecting…',
Failed shows the error string + 'r retry' hint. Falls through to
the existing live-transport rendering once wired.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Retry binding + messagelist empty-state

Bind `r` (only when `backendState == Failed`) to re-run `connectBackendCmd`. Extend `messagelist.Model`'s empty-state render to show "Connecting to Fastmail…" with a spinner when `backendState == Connecting` and the cached list is empty (first run, no data on disk).

**Files:**
- Modify: `internal/ui/app_keys.go` (retry binding)
- Modify: `internal/ui/app_chrome.go` or `app_connect.go` (key handler)
- Modify: `internal/ui/messagelist/messagelist.go` (empty-state branch)
- Test: `internal/ui/app_test.go`, `internal/ui/messagelist/messagelist_test.go`

- [ ] **Step 1: Write failing test for retry**

Add to `internal/ui/app_test.go`:

```go
func TestApp_RetryKey_OnlyActiveWhenFailed(t *testing.T) {
    app := newTestApp(t)
    // Connecting state: 'r' should not fire connect.
    _, cmd := app.Update(tea.KeyPressMsg{Code: 'r'})
    if cmd != nil {
        // 'r' may be bound to reply elsewhere; assert it's not the connect cmd
        // by inspecting cmd() if practical. Otherwise:
        t.Logf("note: r emitted a Cmd in Connecting state (may be reply binding)")
    }
    // Failed state: 'r' should fire connect.
    failed, _ := app.Update(BackendErrMsg{Err: errors.New("x")})
    _, cmd = failed.(App).Update(tea.KeyPressMsg{Code: 'r'})
    if cmd == nil {
        t.Fatalf("r in Failed state: cmd = nil, want connectBackendCmd")
    }
}
```

If `r` is already bound to another action that the test would falsely trigger, scope the test more narrowly or use a dedicated retry key. **Settle the key choice inline if `r` conflicts** — `R` is also a candidate; the spec calls for "single key, no modifiers, surfaced in status-bar help when state == Failed". Whatever key wins, document it in the ADR.

- [ ] **Step 2: Bind retry**

In `internal/ui/app_keys.go`, add (next to other top-level bindings):

```go
RetryConnect: key.NewBinding(
    key.WithKeys("r"),
    key.WithHelp("r", "retry connection"),
),
```

In the key dispatch (likely `app_chrome.go` or a new `app_connect.go`), claim `r` only when `m.backendState == uicore.BackendFailed`:

```go
case key.Matches(km, m.keys.RetryConnect):
    if m.backendState != uicore.BackendFailed {
        return m, nil, false // not claimed; let other handlers see it
    }
    m.backendState = uicore.BackendConnecting
    m.backendErr = nil
    return m, connectBackendCmd(context.Background(), m.backend, m.acct), true
```

- [ ] **Step 3: Extend messagelist empty-state**

In `internal/ui/messagelist/messagelist.go`, find the existing empty-state render (search for the empty-folder placeholder string). Add a branch:

```go
if len(m.source) == 0 && m.connecting {
    // render "Connecting to Fastmail…" + spinner centered
}
```

Add a `connecting bool` field + `SetConnecting(bool) Model` setter. App calls `SetConnecting(m.backendState == uicore.BackendConnecting)` in `deriveChromeFromAcct` (or wherever the messagelist size/state is refreshed).

The spinner instance: use `uicore.NewSpinner(theme)` per the existing pattern (see `ui-invariants.md` — spinners go through the shared factory).

- [ ] **Step 4: Write failing test for empty-state**

Add to `internal/ui/messagelist/messagelist_test.go`:

```go
func TestMessageList_EmptyAndConnecting_ShowsConnectingPlaceholder(t *testing.T) {
    m := New(...).SetConnecting(true)
    out := m.View()
    if !strings.Contains(out, "Connecting to Fastmail") {
        t.Fatalf("empty+connecting render missing placeholder: %q", out)
    }
}

func TestMessageList_NonEmpty_IgnoresConnecting(t *testing.T) {
    m := New(...).SetConnecting(true)
    m = m.SetMessages([]mail.MessageInfo{{UID: "1", Subject: "test"}})
    out := m.View()
    if strings.Contains(out, "Connecting to Fastmail") {
        t.Fatalf("non-empty list incorrectly shows connecting placeholder: %q", out)
    }
}
```

- [ ] **Step 5: Run all tests**

```bash
make check
```

Expected: PASS.

- [ ] **Step 6: Manual verification**

```bash
make install
```

Two scenarios:
1. **Existing cache.** `poplar` — cached messages visible at frame 1; spinner in status bar resolves within seconds.
2. **Fresh cache.** Move your real cache aside (`mv ~/.cache/poplar/fastmail ~/.cache/poplar/fastmail.bak`), then `poplar` — message list shows "Connecting to Fastmail…" placeholder, populates after connect. Restore after (`rm -rf ~/.cache/poplar/fastmail && mv ~/.cache/poplar/fastmail.bak ~/.cache/poplar/fastmail`).

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "$(cat <<'EOF'
ui: retry binding + messagelist connecting placeholder

r re-dispatches connectBackendCmd when backendState == Failed.
Empty message list during initial Connecting shows a centered
'Connecting to Fastmail…' placeholder + spinner.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: ADR + invariants update

Write the ADR codifying the cache lifecycle split, the UI-owned connect, and the layering-bug fix in `AccountName`. Update `docs/poplar/invariants.md` to reflect the new shape.

**Files:**
- Create: `docs/poplar/decisions/0242-async-backend-connect.md`
- Modify: `docs/poplar/invariants.md` (cache section, startup section)
- Modify: `docs/poplar/decisions/INDEX.md` (link new ADR)

- [ ] **Step 1: Write the ADR**

Template — match the format of recent ADRs (read `docs/poplar/decisions/0241-*.md` for tone):

```markdown
# ADR-0242: Async Backend Connect; Cache Opens Before UI

## Status

Accepted — 2026-05-17.

## Context

Synchronous `backend.Connect` in `cmd/poplar/root.go` blocked
the bubbletea program from starting until the JMAP session fetch
completed. Typical Fastmail session response is 1–3 s cold; an
instance of 5.9 s was observed on 2026-05-17. Nothing rendered
during the wait; users perceived the binary as frozen.

The data the UI rendered was already on disk in cache schema v13.

## Decision

- `cache.Open(name, dir, cfg, log)` opens SQLite and runs
  migrations only. No backend, no ChangeTracker.
- `(*Account).WireBackend(backend, ct)` assigns the fields,
  starts the drainer, and starts the backfiller. Called exactly
  once per Account lifetime.
- Backend-touching cache read paths return `cache.ErrNotConnected`
  pre-wire. Cache-only reads work unchanged.
- `App.Init` returns `connectBackendCmd(ctx, backend, acct)`.
  The Cmd calls `Connect`, then `WireBackend`, then emits
  `BackendReadyMsg`. On failure it emits `BackendErrMsg{err}`.
- App owns `backendState uicore.BackendState` +
  `backendErr error`. Status bar renders these directly
  pre-wire; falls through to `mail.ConnState` post-wire.
- New `mail.ConnConnecting` distinguishes pre-authenticated
  initial state from `Reconnecting` (was-connected-lost-it).
- `r` retries `connectBackendCmd` when state is Failed.

## Consequences

- UI renders within ~100 ms of binary launch.
- Offline-first falls out: a flaky Fastmail or blocked egress
  yields a degraded experience, not a hung terminal.
- `cache.Account.AccountName()` no longer delegates to the
  backend — a layering bug fixed inline.
- `cmd/poplar/root.go` shrinks; the drainer + push-subscription
  lifetimes now live with the wired backend, not with the
  process.
- Drainer-blocked outbox ops sit in `queued` until wire, then
  dispatch normally — same as during a mid-session
  `ConnReconnecting` window.

## Alternatives considered

- **Splash screen + sync connect.** Cosmetic; doesn't fix the
  layering or unlock offline reads.
- **Full async with cache-as-UI-source from day one.** This is
  what we built; no alternative beat it once the cache was
  already write-through for bodies and metadata.
```

- [ ] **Step 2: Update invariants**

In `docs/poplar/invariants.md`, find the existing line about cache `Open` (search for "cache.Open" or the Cache section). Replace the relevant binding facts to reflect:
- `cache.Open` is sqlite + migrations only.
- `(*Account).WireBackend(backend, ct)` is the post-Connect wiring step; starts drainer + backfiller.
- App owns the initial backend.Connect via `connectBackendCmd`.
- `ErrNotConnected` is the pre-wire sentinel for backend-touching reads.

Add a line under the appropriate section noting `mail.ConnConnecting` distinct from `Reconnecting`.

Update the Decisions section at the end to cite ADR-0242.

- [ ] **Step 3: Update INDEX.md**

Add `0242` to `docs/poplar/decisions/INDEX.md` in the appropriate theme bucket (likely "Architecture" or "Startup / lifecycle").

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/decisions/0242-async-backend-connect.md \
        docs/poplar/decisions/INDEX.md \
        docs/poplar/invariants.md
git commit -m "$(cat <<'EOF'
ADR-0242: async backend connect; cache opens before UI

Codify the cache.Open / WireBackend split, the UI-owned
connectBackendCmd, the ErrNotConnected sentinel, and the
AccountName layering fix. Invariants updated.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Pass-end ritual

Invoke the `poplar-pass` skill for the consolidation ritual:
- Confirm `make check` green.
- Run `/simplify` on the cumulative diff (vs `master`'s pre-pass tip).
- Update `docs/poplar/STATUS.md` with the next starter prompt.
- Archive the plan under `docs/superpowers/archive/plans/`.
- Push to origin.
- `make install` and one final live-tmux verification with the freshly-installed binary.

The ritual itself is owned by the skill; this task is just the trigger.

---

## Self-review notes

- **Spec coverage:** Tasks 1–4 cover the cache split; Tasks 5–7 cover the UI plumbing; Task 8 covers the root.go reorder; Task 9 covers status-bar render; Task 10 covers retry + messagelist; Task 11 covers ADR + invariants. Every "Components" section of the spec maps to a task.
- **Out-of-scope reminders honored:** No retry-with-backoff (left to follow-up if it bites). No multi-account parallel connect. No cache-only body fetch new path.
- **Adjacent-fix per pre-beta stance:** `AccountName` layering bug fixed inline in Task 2. `Disconnect()`-pre-`Connect()` safety verified in Task 8 (guard added if needed).
