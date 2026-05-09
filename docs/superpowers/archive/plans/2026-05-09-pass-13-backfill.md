# Pass 13: Background Body Sync + Status Indicator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-account background worker that fills the body cache newest-first, with idle-gating, server back-pressure handling, and a status-bar progress indicator. Substrate for Pass 13.1 search.

**Architecture:** New `internal/cache/backfill.go` runs one goroutine per `*cache.Account`, started in `Open` and stopped in `Close`. Work queue is implicit via SQL (`LEFT JOIN bodies WHERE bytes IS NULL ORDER BY sent_at DESC`). Throttle shape from Thunderbird `nsAutoSyncManager`: 2 MB batch ceiling + timer-slack + idle gate (5s threshold on `tea.KeyMsg`). Server back-pressure (`[THROTTLED]` / 429) triggers exponential backoff capped at 60s, mirroring the outbox drainer. Status bar gains a sibling segment alongside connection-state and outbox-depth.

**Tech Stack:** Go 1.26.1, `modernc.org/sqlite`, `bubbletea` v1, existing `internal/cache/`, `internal/ui/`, `internal/mail/`.

**Spec:** `docs/superpowers/specs/2026-05-09-pass-13-backfill-design.md`

**Research:**
- `docs/poplar/research/2026-05-09-mail-client-search-survey.md`
- `docs/poplar/research/2026-05-09-background-body-sync-survey.md`

---

## File structure

**Create:**
- `internal/cache/backfill.go` — `Backfiller` struct, `Run` loop, queue query, throttle decision tree, server-throttle classification.
- `internal/cache/backfill_test.go` — tests for queue ordering, throttle gates, error classification, backoff curve.

**Modify:**
- `internal/cache/account.go` — embed `*Backfiller`, start in `Open`, stop in `Close`, expose `BackfillProgress`, `NotifyActivity`, `NotifyConnState`.
- `internal/config/cache.go` — flip `MaxSize` default from `2 GB` to `0` (unlimited).
- `internal/config/template.go` (or wherever `Template()` lives) — update commented default.
- `internal/ui/account/model.go` — `NotifyActivity` / `NotifyConnState` accessors that forward to `*cache.Account`.
- `internal/ui/app.go` — call `NotifyActivity` on `tea.KeyMsg`; call `NotifyConnState` on connection-state change.
- `internal/ui/status_bar.go` — render the new `↓ N/M` segment with paused / warn substates and Spartan-tier collapse.
- `internal/ui/status_bar_test.go` — segment formatter tests.

**Notes:**
- `cache.Account` already has `maxSize int64` and `bodies.go:storeBody` already treats `maxSize == 0` as unlimited (`a.maxSize > 0` gate). Only the config default needs flipping; cache code is already correct for the new semantics.
- `account.Model.Backend()` exists; mirror with `NotifyActivity()` / `NotifyConnState()` accessors.

---

## Task 1: Queue query — `nextUnfetchedUID`

**Files:**
- Create: `internal/cache/backfill.go`
- Create: `internal/cache/backfill_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/backfill_test.go`:

```go
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestNextUnfetchedUID_NewestFirst(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	now := time.Now()
	older := mail.UID("older-uid")
	newer := mail.UID("newer-uid")
	seedMessage(t, a, older, now.Add(-time.Hour))
	seedMessage(t, a, newer, now)

	uid, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if !ok {
		t.Fatalf("nextUnfetchedUID: ok=false, want true")
	}
	if uid != newer {
		t.Errorf("nextUnfetchedUID = %s, want %s (newest)", uid, newer)
	}
}

func TestNextUnfetchedUID_SkipsCachedBodies(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	now := time.Now()
	cached := mail.UID("cached")
	uncached := mail.UID("uncached")
	cachedID := seedMessage(t, a, cached, now)
	seedMessage(t, a, uncached, now.Add(-time.Hour))
	if _, err := a.db.Exec(
		`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
		cachedID, []byte("body"), now.UnixNano(),
	); err != nil {
		t.Fatalf("seed body: %v", err)
	}

	uid, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if !ok || uid != uncached {
		t.Errorf("nextUnfetchedUID = (%s, %v), want (%s, true)", uid, ok, uncached)
	}
}

func TestNextUnfetchedUID_AllCached(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("all-cached")
	id := seedMessage(t, a, uid, time.Now())
	if _, err := a.db.Exec(
		`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
		id, []byte("b"), time.Now().UnixNano(),
	); err != nil {
		t.Fatalf("seed body: %v", err)
	}

	_, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if ok {
		t.Errorf("nextUnfetchedUID with all bodies cached: ok=true, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cache/ -run TestNextUnfetchedUID -v
```

Expected: FAIL — `a.nextUnfetchedUID undefined`.

- [ ] **Step 3: Implement `nextUnfetchedUID`**

Create `internal/cache/backfill.go`:

```go
package cache

import (
	"context"
	"database/sql"
	"errors"

	"github.com/glw907/poplar/internal/mail"
)

// nextUnfetchedUID returns the newest message UID without a stored
// body, or ok=false when every cached message has bytes. The query
// is the implicit work queue for the backfill worker — sent_at DESC
// puts new mail at the top, eviction restores rows naturally.
func (a *Account) nextUnfetchedUID(ctx context.Context) (mail.UID, bool, error) {
	var pid string
	err := a.db.QueryRowContext(ctx, `
		SELECT m.protocol_id
		FROM messages m
		LEFT JOIN bodies b ON b.message = m.id
		WHERE b.bytes IS NULL
		ORDER BY m.sent_at DESC
		LIMIT 1
	`).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return mail.UID(pid), true, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cache/ -run TestNextUnfetchedUID -v
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cache/backfill.go internal/cache/backfill_test.go
git commit -m "Pass 13 task 1: nextUnfetchedUID — newest-first work queue"
```

---

## Task 2: Default `max-size = 0` (unlimited)

**Files:**
- Modify: `internal/config/cache.go`

- [ ] **Step 1: Locate the existing default**

Read `internal/config/cache.go` lines 18–30. The line:

```go
MaxSize: 2 * 1024 * 1024 * 1024,
```

is the only place changing the default takes effect for new configs. `cache.Account.storeBody` already treats `maxSize == 0` as "unlimited" via the `a.maxSize > 0` gate in `internal/cache/bodies.go:35` — no code change needed there.

- [ ] **Step 2: Write a test that locks in the new default**

Append to `internal/config/cache_test.go` (create if absent):

```go
func TestDefaultCacheConfig_MaxSizeUnlimited(t *testing.T) {
	c := DefaultCacheConfig()
	if c.MaxSize != 0 {
		t.Errorf("DefaultCacheConfig.MaxSize = %d, want 0 (unlimited; matrix-aligned default per ADR-Pass-13)", c.MaxSize)
	}
}
```

If `DefaultCacheConfig()` does not exist, expose the existing initializer under that name (rename or add a thin function returning the literal).

- [ ] **Step 3: Run test to verify it fails**

```
go test ./internal/config/ -run TestDefaultCacheConfig_MaxSizeUnlimited -v
```

Expected: FAIL — `MaxSize = 2147483648, want 0`.

- [ ] **Step 4: Flip the default**

Edit `internal/config/cache.go` line 24:

```go
// Before:
MaxSize: 2 * 1024 * 1024 * 1024,

// After:
MaxSize: 0, // 0 = unlimited; users opt in to a cap via [cache] max-size.
```

Also locate `Template()` in the config package (search: `grep -n "Template\|max-size" internal/config/*.go`) and update the commented example to match:

```toml
# [cache]
# max-size = "0"   # unlimited; set to e.g. "5GB" to cap stored bodies
```

- [ ] **Step 5: Run test + full cache tests**

```
go test ./internal/config/ -v
go test ./internal/cache/ -v
```

Expected: PASS. The cache tests already exercise `maxSize == 0` and `maxSize > 0` paths.

- [ ] **Step 6: Commit**

```bash
git add internal/config/cache.go internal/config/cache_test.go
git commit -m "Pass 13 task 2: cache max-size default 0 (unlimited)"
```

---

## Task 3: Backfiller skeleton + lifecycle

**Files:**
- Modify: `internal/cache/backfill.go`
- Modify: `internal/cache/account.go`

- [ ] **Step 1: Write a failing test**

Append to `internal/cache/backfill_test.go`:

```go
func TestBackfiller_OneShot(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("u-1")
	seedMessage(t, a, uid, time.Now())
	a.Backend = &fakeBackendWithBody{body: []byte("hello")}

	bf := newBackfiller(a)
	if err := bf.fetchOne(context.Background()); err != nil {
		t.Fatalf("fetchOne: %v", err)
	}

	got, ok, err := a.lookupBody(context.Background(), uid)
	if err != nil || !ok {
		t.Fatalf("lookupBody: ok=%v err=%v", ok, err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
}
```

`fakeBackendWithBody` already exists in `internal/cache/bodies_test.go`.

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cache/ -run TestBackfiller_OneShot -v
```

Expected: FAIL — `newBackfiller undefined`.

- [ ] **Step 3: Implement `Backfiller` skeleton`**

Append to `internal/cache/backfill.go`:

```go
import (
	"sync/atomic"
	"time"
)

// Backfiller runs a per-account background loop that fills the body
// cache newest-first. Lifecycle: constructed in Account.Open, started
// via Run, stopped by canceling the context passed to Run.
type Backfiller struct {
	acct          *Account
	rate          time.Duration
	idleThreshold time.Duration
	maxBatchBytes int64

	lastActivity atomic.Int64 // unix nanos
	connOnline   atomic.Bool
}

func newBackfiller(a *Account) *Backfiller {
	bf := &Backfiller{
		acct:          a,
		rate:          500 * time.Millisecond,
		idleThreshold: 5 * time.Second,
		maxBatchBytes: 2 * 1024 * 1024,
	}
	bf.connOnline.Store(true) // assume online until told otherwise
	return bf
}

// fetchOne fetches the next eligible UID's body. Returns nil when
// the cache is caught up. Errors propagate; the run loop classifies
// them.
func (b *Backfiller) fetchOne(ctx context.Context) error {
	uid, ok, err := b.acct.nextUnfetchedUID(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	body, err := b.acct.Backend.FetchBody(uid)
	if err != nil {
		return err
	}
	return b.acct.storeBody(ctx, uid, body)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cache/ -run TestBackfiller_OneShot -v
```

Expected: PASS.

- [ ] **Step 5: Wire lifecycle in `Open` / `Close`**

In `internal/cache/account.go`, add field:

```go
type Account struct {
	// ... existing fields ...
	backfiller    *Backfiller
	backfillStop  context.CancelFunc
}
```

In `Open`, after the existing init (right before `return a, nil`):

```go
a.backfiller = newBackfiller(a)
ctx, cancel := context.WithCancel(context.Background())
a.backfillStop = cancel
go a.backfiller.Run(ctx)
```

In `Close`, before existing cleanup:

```go
if a.backfillStop != nil {
	a.backfillStop()
}
```

`Run` is implemented in Task 4 — for now add a stub:

```go
// Run runs the backfill loop until ctx is canceled.
func (b *Backfiller) Run(ctx context.Context) {
	<-ctx.Done()
}
```

- [ ] **Step 6: Run all cache tests**

```
go test ./internal/cache/ -v
```

Expected: PASS — existing tests still pass with the lifecycle wired (the stub Run does nothing).

- [ ] **Step 7: Commit**

```bash
git add internal/cache/backfill.go internal/cache/backfill_test.go internal/cache/account.go
git commit -m "Pass 13 task 3: Backfiller skeleton + lifecycle"
```

---

## Task 4: Run loop with throttle + idle gate

**Files:**
- Modify: `internal/cache/backfill.go`
- Modify: `internal/cache/backfill_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backfill_test.go`:

```go
func TestBackfiller_RunFillsThenIdles(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("body")}

	for i := 0; i < 3; i++ {
		seedMessage(t, a, mail.UID(fmt.Sprintf("u-%d", i)), time.Now().Add(-time.Duration(i)*time.Hour))
	}

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond // fast for tests
	bf.idleThreshold = 0           // no activity gate in this test

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n); err != nil {
		t.Fatalf("count bodies: %v", err)
	}
	if n != 3 {
		t.Errorf("bodies stored = %d, want 3", n)
	}
}

func TestBackfiller_RespectsActivityGate(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("body")}
	seedMessage(t, a, mail.UID("u-1"), time.Now())

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 100 * time.Millisecond
	bf.NotifyActivity() // start "active"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n)
	if n != 0 {
		t.Errorf("active gate failed: bodies = %d, want 0", n)
	}
}
```

Add `"fmt"` to the imports.

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/cache/ -run TestBackfiller_Run -v
```

Expected: FAIL — `bf.NotifyActivity undefined` and the loop doesn't actually fill.

- [ ] **Step 3: Implement Run + activity gate**

Replace the stub `Run` in `internal/cache/backfill.go`:

```go
// NotifyActivity records a user-input event. The Run loop suspends
// fetches until idleThreshold has elapsed.
func (b *Backfiller) NotifyActivity() {
	b.lastActivity.Store(time.Now().UnixNano())
}

func (b *Backfiller) idle() bool {
	if b.idleThreshold == 0 {
		return true
	}
	last := b.lastActivity.Load()
	if last == 0 {
		return true
	}
	return time.Since(time.Unix(0, last)) >= b.idleThreshold
}

// Run drives the backfill loop until ctx is canceled. Each tick
// checks gates (idle, connection, cache cap), then fetches up to
// maxBatchBytes worth of bodies before sleeping rate.
func (b *Backfiller) Run(ctx context.Context) {
	t := time.NewTicker(b.rate)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !b.connOnline.Load() || !b.idle() {
			continue
		}
		b.runBatch(ctx)
	}
}

func (b *Backfiller) runBatch(ctx context.Context) {
	var bytesFetched int64
	for bytesFetched < b.maxBatchBytes {
		if !b.idle() || !b.connOnline.Load() {
			return
		}
		uid, ok, err := b.acct.nextUnfetchedUID(ctx)
		if err != nil || !ok {
			return
		}
		body, err := b.acct.Backend.FetchBody(uid)
		if err != nil {
			return // task 6 will classify
		}
		if err := b.acct.storeBody(ctx, uid, body); err != nil {
			return
		}
		bytesFetched += int64(len(body))
	}
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/cache/ -run TestBackfiller_Run -v
```

Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cache/backfill.go internal/cache/backfill_test.go
git commit -m "Pass 13 task 4: Backfiller Run loop + idle gate"
```

---

## Task 5: Connection-state gate + cap-aware bail

**Files:**
- Modify: `internal/cache/backfill.go`
- Modify: `internal/cache/backfill_test.go`

- [ ] **Step 1: Write failing tests**

Append to `backfill_test.go`:

```go
func TestBackfiller_PausesWhenOffline(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("b")}
	seedMessage(t, a, mail.UID("u-1"), time.Now())

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 0
	bf.NotifyConnState(false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n)
	if n != 0 {
		t.Errorf("offline gate failed: bodies = %d, want 0", n)
	}
}

func TestBackfiller_BailsAtCap(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.maxSize = 100 // 100 bytes; one body of 80 fills 80% — under the 90% floor
	a.Backend = &fakeBackendWithBody{body: bytes.Repeat([]byte("x"), 95)}

	for i := 0; i < 3; i++ {
		seedMessage(t, a, mail.UID(fmt.Sprintf("u-%d", i)), time.Now().Add(-time.Duration(i)*time.Hour))
	}

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 0

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var stored int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&stored)
	if stored != 1 {
		t.Errorf("at-cap bail: stored=%d, want 1 (further fetches should bail)", stored)
	}
}
```

Add `"bytes"` to imports if not present.

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/cache/ -run "TestBackfiller_(PausesWhenOffline|BailsAtCap)" -v
```

Expected: FAIL — `bf.NotifyConnState undefined`; cap test fills more than expected.

- [ ] **Step 3: Implement connection gate + cap check**

In `internal/cache/backfill.go`, add:

```go
// NotifyConnState flips the online flag. Set false on Offline /
// Reconnecting; true on Online. Backfill suspends when false.
func (b *Backfiller) NotifyConnState(online bool) {
	b.connOnline.Store(online)
}

func (b *Backfiller) atCap(ctx context.Context) bool {
	if b.acct.maxSize <= 0 {
		return false
	}
	var total int64
	if err := b.acct.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(LENGTH(bytes)), 0) FROM bodies`).Scan(&total); err != nil {
		return false
	}
	return total >= (b.acct.maxSize*9)/10 // 90% floor
}
```

Update `runBatch` to check cap each iteration:

```go
func (b *Backfiller) runBatch(ctx context.Context) {
	var bytesFetched int64
	for bytesFetched < b.maxBatchBytes {
		if !b.idle() || !b.connOnline.Load() || b.atCap(ctx) {
			return
		}
		// ... rest unchanged
	}
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/cache/ -v
```

Expected: PASS — all backfill tests + existing cache tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/backfill.go internal/cache/backfill_test.go
git commit -m "Pass 13 task 5: connection gate + cache-cap bail"
```

---

## Task 6: Server-throttle classification + exponential backoff

**Files:**
- Modify: `internal/cache/backfill.go`
- Modify: `internal/cache/backfill_test.go`

- [ ] **Step 1: Locate existing backoff helper**

```
grep -n "backoff\|nextEligibleAt\|ExpBackoff" internal/cache/*.go
```

If a reusable helper exists in the outbox drainer (e.g. `nextBackoff(attempts int) time.Duration`), reuse it. If not, implement the same curve in `backfill.go` (1s, 2s, 4s, …, cap 60s).

- [ ] **Step 2: Write the failing test**

Append to `backfill_test.go`:

```go
func TestIsThrottleErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"throttled", errors.New("[THROTTLED] too many requests"), true},
		{"429", errors.New("HTTP 429: rate limited"), true},
		{"random", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isThrottleErr(c.err); got != c.want {
				t.Errorf("isThrottleErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestBackoffCurve(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{6, 60 * time.Second},  // cap
		{20, 60 * time.Second}, // still cap
	}
	for _, c := range cases {
		got := backfillBackoff(c.attempts)
		if got != c.want {
			t.Errorf("backfillBackoff(%d) = %v, want %v", c.attempts, got, c.want)
		}
	}
}
```

Add `"errors"` to imports if absent.

- [ ] **Step 3: Run tests to verify they fail**

```
go test ./internal/cache/ -run "TestIsThrottleErr|TestBackoffCurve" -v
```

Expected: FAIL — undefined symbols.

- [ ] **Step 4: Implement classification + backoff**

Append to `internal/cache/backfill.go`:

```go
import "strings"

// isThrottleErr matches IMAP [THROTTLED], JMAP rate-limit, and HTTP
// 429 error strings. Substring match is intentionally loose — the
// underlying libraries surface these as opaque error.Error() text
// without typed sentinels at this layer.
func isThrottleErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "THROTTLED") ||
		strings.Contains(s, "rate limited") ||
		strings.Contains(s, "rateLimit") ||
		strings.Contains(s, "429")
}

// backfillBackoff returns the sleep duration for the Nth consecutive
// throttle response. 1s, 2s, 4s, …, capped at 60s. Mirrors the
// outbox drainer's curve.
func backfillBackoff(attempts int) time.Duration {
	d := time.Second << attempts
	if d > 60*time.Second || d <= 0 {
		return 60 * time.Second
	}
	return d
}
```

Wire into `runBatch`. Replace the body-fetch error handling:

```go
		body, err := b.acct.Backend.FetchBody(uid)
		if err != nil {
			if isThrottleErr(err) {
				b.throttleAttempts++
				select {
				case <-ctx.Done():
				case <-time.After(backfillBackoff(b.throttleAttempts)):
				}
				return
			}
			b.throttleAttempts = 0
			return
		}
		b.throttleAttempts = 0
```

Add `throttleAttempts int` to the `Backfiller` struct.

- [ ] **Step 5: Run tests**

```
go test ./internal/cache/ -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cache/backfill.go internal/cache/backfill_test.go
git commit -m "Pass 13 task 6: server-throttle classification + backoff"
```

---

## Task 7: `BackfillProgress` accessor

**Files:**
- Modify: `internal/cache/account.go`
- Create: `internal/cache/backfill_progress_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cache/backfill_progress_test.go`:

```go
package cache

import (
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestBackfillProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	for i := 0; i < 5; i++ {
		seedMessage(t, a, mail.UID(string(rune('a'+i))), time.Now())
	}
	// cache 2 of 5 bodies
	for i := 0; i < 2; i++ {
		var msgID int64
		a.db.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`, string(rune('a'+i))).Scan(&msgID)
		a.db.Exec(`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
			msgID, []byte("b"), time.Now().UnixNano())
	}

	done, total, err := a.BackfillProgress()
	if err != nil {
		t.Fatalf("BackfillProgress: %v", err)
	}
	if done != 2 || total != 5 {
		t.Errorf("BackfillProgress = (%d, %d), want (2, 5)", done, total)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cache/ -run TestBackfillProgress -v
```

Expected: FAIL — `BackfillProgress undefined`.

- [ ] **Step 3: Implement the accessor**

Append to `internal/cache/account.go`:

```go
// BackfillProgress returns (done, total) — bodies cached vs total
// known messages. Used by the status-bar segment. Two cheap counts;
// safe to call per cache event.
func (a *Account) BackfillProgress() (done, total int, err error) {
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&total); err != nil {
		return 0, 0, err
	}
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM bodies WHERE bytes IS NOT NULL`).Scan(&done); err != nil {
		return 0, 0, err
	}
	return done, total, nil
}
```

- [ ] **Step 4: Run test**

```
go test ./internal/cache/ -run TestBackfillProgress -v
```

Expected: PASS.

- [ ] **Step 5: Add Account-level NotifyActivity / NotifyConnState shims**

Append to `internal/cache/account.go`:

```go
// NotifyActivity is the App's hook into the backfill worker's
// idle gate. Forwarded on every tea.KeyMsg.
func (a *Account) NotifyActivity() {
	if a.backfiller != nil {
		a.backfiller.NotifyActivity()
	}
}

// NotifyConnState mirrors the Backfiller's connection gate. Called
// on every connection-state change.
func (a *Account) NotifyConnState(online bool) {
	if a.backfiller != nil {
		a.backfiller.NotifyConnState(online)
	}
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/cache/account.go internal/cache/backfill_progress_test.go
git commit -m "Pass 13 task 7: BackfillProgress + Account notify shims"
```

---

## Task 8: `account.Model` accessors

**Files:**
- Modify: `internal/ui/account/model.go`

- [ ] **Step 1: Write a failing test**

Append to `internal/ui/account/model_test.go`:

```go
func TestModel_NotifyActivityForwards(t *testing.T) {
	// Trivial sanity: NotifyActivity must not panic on a nil cache
	// handle (test setups commonly pass nil), and must forward when
	// non-nil. We can't observe the forward without a real cache;
	// this test only proves the method exists and is nil-safe.
	var m Model
	m.NotifyActivity()         // nil acct → no-op, must not panic
	m.NotifyConnState(true)    // same
	m.NotifyConnState(false)
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/ui/account/ -run TestModel_NotifyActivityForwards -v
```

Expected: FAIL — methods undefined.

- [ ] **Step 3: Add accessors**

Append to `internal/ui/account/model.go` (near the existing `Backend()` accessor on line 88):

```go
// NotifyActivity forwards a user-input event to the cache's
// backfill worker. Safe to call when the cache handle is nil.
func (m Model) NotifyActivity() {
	if m.acct != nil {
		m.acct.NotifyActivity()
	}
}

// NotifyConnState forwards a connection-state change to the
// cache's backfill worker.
func (m Model) NotifyConnState(online bool) {
	if m.acct != nil {
		m.acct.NotifyConnState(online)
	}
}
```

- [ ] **Step 4: Run test**

```
go test ./internal/ui/account/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/account/model.go internal/ui/account/model_test.go
git commit -m "Pass 13 task 8: account.Model NotifyActivity/NotifyConnState"
```

---

## Task 9: App wire-up — KeyMsg + ConnState

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Locate the KeyMsg handler**

```
grep -n "tea.KeyMsg\|case tea.KeyMsg" internal/ui/app.go | head
```

Find the top of `App.Update`'s switch, where `tea.KeyMsg` is dispatched.

- [ ] **Step 2: Add the activity hook**

In `App.Update`, at the top of the `case tea.KeyMsg:` branch (before any existing logic):

```go
case tea.KeyMsg:
	m.acct.NotifyActivity()
	// ... existing key dispatch ...
```

- [ ] **Step 3: Locate connection-state handling**

```
grep -n "UpdateConnState\|ConnState\|setConnState" internal/ui/app.go | head
```

Find where the App reacts to a connection-state change message (likely a `backendUpdateMsg` with `Type == mail.UpdateConnState` per `internal/ui/cmds.go:103`).

- [ ] **Step 4: Add the connection-state hook**

In the connection-state branch, after updating the existing `App.connState` / status bar, add:

```go
m.acct.NotifyConnState(newState == mail.ConnOnline)
```

(Replace `newState` with the actual variable name from the surrounding code.)

- [ ] **Step 5: Build + smoke test**

```
make build
go test ./internal/ui/...
```

Expected: build succeeds, tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go
git commit -m "Pass 13 task 9: App wires NotifyActivity + NotifyConnState"
```

---

## Task 10: Status-bar `↓ N/M` segment with tier collapse + paused/warn substates

**Files:**
- Modify: `internal/ui/status_bar.go`
- Modify: `internal/ui/status_bar_test.go`
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Add a backfill-state field to `StatusBar`**

In `internal/ui/status_bar.go`:

```go
type StatusBar struct {
	// ... existing fields ...
	backfillDone   int
	backfillTotal  int
	backfillPaused bool
	backfillWarn   bool
}

// SetBackfill updates the progress segment. done == total or
// total == 0 hides the segment.
func (sb StatusBar) SetBackfill(done, total int, paused, warn bool) StatusBar {
	sb.backfillDone = done
	sb.backfillTotal = total
	sb.backfillPaused = paused
	sb.backfillWarn = warn
	return sb
}
```

- [ ] **Step 2: Write failing tests**

Append to `internal/ui/status_bar_test.go`:

```go
func TestStatusBar_BackfillSegment(t *testing.T) {
	cases := []struct {
		name     string
		width    int
		done     int
		total    int
		paused   bool
		warn     bool
		contains string // substring expected; empty = segment hidden
	}{
		{"hidden when caught up", 120, 100, 100, false, false, ""},
		{"hidden when no messages", 120, 0, 0, false, false, ""},
		{"full active ≥90", 120, 18, 33, false, false, "↓ 18/33"},
		{"glyph-only Spartan", 80, 18, 33, false, false, "↓"},
		{"full paused ≥90", 120, 18, 33, true, false, "↓ paused"},
		{"glyph paused Spartan", 80, 18, 33, true, false, "↓⏸"},
		{"full warn ≥90", 120, 18, 33, false, true, "↓ ⚠"},
		{"glyph warn Spartan", 80, 18, 33, false, true, "↓⚠"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb := NewStatusBar(testStyles()).
				SetBackfill(c.done, c.total, c.paused, c.warn)
			out := sb.View(c.width, c.width-1)
			if c.contains == "" {
				if strings.Contains(out, "↓") {
					t.Errorf("expected no backfill segment; got %q", out)
				}
				return
			}
			if !strings.Contains(out, c.contains) {
				t.Errorf("View width=%d: missing %q in %q", c.width, c.contains, out)
			}
		})
	}
}
```

`testStyles()` is the existing test helper in `status_bar_test.go`. Add `"strings"` to imports if absent.

- [ ] **Step 3: Run tests to verify they fail**

```
go test ./internal/ui/ -run TestStatusBar_BackfillSegment -v
```

Expected: FAIL — `SetBackfill` exists but rendering does not include the segment.

- [ ] **Step 4: Implement segment rendering**

Find the `View` method in `status_bar.go`. Where the connection-state and outbox segments are assembled, splice the backfill segment between them. Add the formatter:

```go
// renderBackfill returns the segment string, or "" when hidden.
// width is the full status-bar width so we can pick the tier.
func (sb StatusBar) renderBackfill(width int) string {
	if sb.backfillTotal == 0 || sb.backfillDone >= sb.backfillTotal {
		return ""
	}
	spartan := width < 90
	switch {
	case sb.backfillWarn:
		if spartan {
			return "↓⚠"
		}
		return "↓ ⚠"
	case sb.backfillPaused:
		if spartan {
			return "↓⏸"
		}
		return "↓ paused"
	default:
		if spartan {
			return "↓"
		}
		return fmt.Sprintf("↓ %d/%d", sb.backfillDone, sb.backfillTotal)
	}
}
```

In `View`, after the existing connection segment is rendered and before the outbox segment, add:

```go
backfillPart := ""
if seg := sb.renderBackfill(width); seg != "" {
	backfillPart = sb.styles.StatusBar.Render(" " + seg + " ")
	if sb.backfillWarn {
		backfillPart = sb.styles.StatusBarWarn.Render(" " + seg + " ")
	}
}
```

Then include `backfillPart` in the assembled status row in the appropriate slot.

If `StatusBarWarn` does not exist, reuse the warn style from the outbox segment (search `grep -n "ColorWarning\|StatusBarWarn" internal/ui/styles.go internal/ui/status_bar.go`).

- [ ] **Step 5: Run tests**

```
go test ./internal/ui/ -run TestStatusBar_BackfillSegment -v
```

Expected: PASS (8 sub-tests).

- [ ] **Step 6: Wire `SetBackfill` from App**

In `internal/ui/app.go`, locate where the outbox depth is refreshed (likely an `outboxDepthMsg` handler per `internal/ui/cmds.go:148`) and dispatch a sibling refresh:

```go
// after outbox refresh:
done, total, _ := m.acct.BackfillProgress()
paused := !m.connOnline || /* activity within 5s — track via lastKeyAt timestamp */
warn := m.backfillWarn // set by Run loop on persistent throttle/error (Task 6 future hook)
m.statusBar = m.statusBar.SetBackfill(done, total, paused, warn)
```

If exposing `paused` from the Backfiller is cleaner than recomputing in App, add an accessor `Backfiller.Paused() bool` that returns `!connOnline || !idle()`.

- [ ] **Step 7: Build + manual smoke test**

```
make check
make install
poplar
```

Verify the segment renders during sync, hides when caught up, collapses to glyph at 80×24.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/status_bar.go internal/ui/status_bar_test.go internal/ui/app.go
git commit -m "Pass 13 task 10: status-bar backfill segment with tier collapse"
```

---

## Pass-end ritual (handled by `poplar-pass` skill)

After Task 10 lands and `make check` is green:

1. `/simplify` — review changes; fix anything flagged.
2. Idiomatic-bubbletea check — run §10 of `bubbletea-conventions.md` against the status-bar diff and the App wire-up diff. Capture 80×24 + 120×40 tmux frames showing the segment in each tier.
3. Write **ADR-0187 (or next available number)** — Background body sync default behavior, Thunderbird-shape throttle, implicit SQL queue, status-bar sibling segment, `max-size = 0` semantics. Mark **ADR-0122 partially superseded** (default flip).
4. Update `docs/poplar/invariants.md` — replace the lazy-only body-fetch language with the new backfill behavior; add the status-bar segment to the cache and chrome sections; update the `max-size` default reference.
5. Update `docs/poplar/decisions/INDEX.md` with the new ADR.
6. Update `STATUS.md` — Pass 13 done; advance to Pass 13.1 starter prompt (search).
7. Archive the plan + spec via `git mv` to `docs/superpowers/archive/`.
8. `make check`, `make install`, commit, push.

---

## Self-review

**Spec coverage:**
- Worker (`internal/cache/backfill.go`) — Tasks 1, 3, 4, 5, 6.
- Implicit SQL queue — Task 1.
- Newest-first ordering — Task 1 (`ORDER BY sent_at DESC`) + test.
- Throttle (batch ceiling + rate + idle gate) — Task 4.
- Connection gate — Task 5.
- Cache-cap gate — Task 5.
- Server back-pressure (1s, 2s, 4s, …, 60s) — Task 6.
- `[cache] max-size = 0` unlimited — Task 2.
- Status-bar segment with tier collapse + paused/warn substates — Task 10.
- `BackfillProgress` accessor — Task 7.
- Activity wiring (`tea.KeyMsg` → Backfiller) — Tasks 7, 8, 9.
- Connection wiring — Tasks 7, 8, 9.
- ADR + invariants update — pass-end ritual.

**Placeholder scan:** none — every step shows code, a command, or an exact file:line reference.

**Type consistency:** `Backfiller`/`newBackfiller`/`Run`/`fetchOne`/`runBatch`/`NotifyActivity`/`NotifyConnState`/`atCap`/`idle`/`isThrottleErr`/`backfillBackoff` consistent across tasks. `BackfillProgress(done, total int, err error)` matches the App wire-up. `SetBackfill(done, total int, paused, warn bool)` matches the test cases.

**Pass-size:** 10 tasks, well within the 8–12 budget.
