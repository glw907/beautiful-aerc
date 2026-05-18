# Pass 42: Correlation Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Thread `context.Context` through `mail.Backend` mutating methods and propagate drainer op IDs via a custom slog handler so every log record emitted during a queued operation automatically carries `op_id`.

**Architecture:** New `internal/logctx` package holds the context key and a `Handler` wrapper that injects context-carried attrs. `installLogger` wraps the text handler with `logctx.Handler`. Six `mail.Backend` mutating methods gain a `ctx` first parameter; `cache.Drainer.dispatch` attaches `logctx.WithOpID(ctx, row.ID)` before calling them.

**Tech Stack:** Go 1.26, `log/slog`, `context`

---

### Task 1: `internal/logctx` package

**Files:**
- Create: `internal/logctx/logctx.go`
- Create: `internal/logctx/logctx_test.go`

- [ ] **Write the failing tests**

```go
// internal/logctx/logctx_test.go
package logctx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithOpID_RoundTrip(t *testing.T) {
	ctx := WithOpID(context.Background(), "42")
	id, ok := ctx.Value(opIDKey{}).(string)
	if !ok || id != "42" {
		t.Fatalf("got %q ok=%v, want 42 true", id, ok)
	}
}

func TestHandler_InjectsOpID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := Handler{inner}
	log := slog.New(h)

	ctx := WithOpID(context.Background(), "99")
	log.DebugContext(ctx, "test event")

	if !strings.Contains(buf.String(), "op_id=99") {
		t.Errorf("op_id not in output: %s", buf.String())
	}
}

func TestHandler_NoOpID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := Handler{inner}
	log := slog.New(h)

	log.DebugContext(context.Background(), "no op")

	if strings.Contains(buf.String(), "op_id") {
		t.Errorf("unexpected op_id in output: %s", buf.String())
	}
}

func TestHandler_WithAttrs_PreservesInjection(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	// Simulate slog.Default().With("component", "cache") derivation.
	log := slog.New(Handler{inner}).With("component", "cache")

	ctx := WithOpID(context.Background(), "7")
	log.DebugContext(ctx, "cache event")

	out := buf.String()
	if !strings.Contains(out, "op_id=7") {
		t.Errorf("op_id missing after With: %s", out)
	}
	if !strings.Contains(out, "component=cache") {
		t.Errorf("component missing after With: %s", out)
	}
}
```

- [ ] **Run tests to verify they fail**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/logctx/... 2>&1
```
Expected: package not found or compile error.

- [ ] **Implement `internal/logctx/logctx.go`**

```go
package logctx

import (
	"context"
	"log/slog"
)

type opIDKey struct{}

// WithOpID returns a context carrying id as the operation identifier.
func WithOpID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, opIDKey{}, id)
}

// Handler wraps a slog.Handler and injects context-carried op_id.
type Handler struct {
	slog.Handler
}

func (h Handler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(opIDKey{}).(string); ok {
		r.AddAttrs(slog.String("op_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup must re-wrap so derived loggers (via
// slog.Logger.With) keep the injection behaviour.
func (h Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return Handler{h.Handler.WithAttrs(attrs)}
}

func (h Handler) WithGroup(name string) slog.Handler {
	return Handler{h.Handler.WithGroup(name)}
}
```

- [ ] **Run tests to verify they pass**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/logctx/... -v 2>&1
```
Expected: all four tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/logctx/ && git commit -m "Add internal/logctx: context-carried op_id slog handler

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Wire `logctx.Handler` into `installLogger`

**Files:**
- Modify: `cmd/poplar/log.go`
- Modify: `cmd/poplar/log_test.go`

- [ ] **Update `installLogger` to wrap with `logctx.Handler`**

In `cmd/poplar/log.go`, replace:

```go
import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/term"
)
```

with:

```go
import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/logctx"
	"golang.org/x/term"
)
```

Replace the handler construction at the end of `installLogger`:

```go
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
```

with:

```go
	h := logctx.Handler{Handler: slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})}
	slog.SetDefault(slog.New(h))
```

- [ ] **Add test for op_id propagation through the default logger**

Add to `cmd/poplar/log_test.go`:

```go
func TestInstallLogger_OpIDPropagation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "debug")

	installLogger("")

	// Capture output: use a bytes.Buffer handler on top of the global.
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logctx.Handler{Handler: inner}))

	ctx := logctx.WithOpID(context.Background(), "42")
	slog.DebugContext(ctx, "probe")

	if !strings.Contains(buf.String(), "op_id=42") {
		t.Errorf("op_id not propagated: %s", buf.String())
	}
}
```

Add imports `"bytes"`, `"context"`, `"strings"`, `"github.com/glw907/poplar/internal/logctx"` to the test file's import block.

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add cmd/poplar/log.go cmd/poplar/log_test.go && git commit -m "Wire logctx.Handler into installLogger

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: `mail.Backend` interface + `MockBackend`

**Files:**
- Modify: `internal/mail/backend.go`
- Modify: `internal/mail/mock.go`
- Modify: `internal/mail/mock_test.go`

- [ ] **Update the six Backend interface methods**

In `internal/mail/backend.go`, change:

```go
Move(uids []UID, dest string) error
Destroy(uids []UID) error
Flag(uids []UID, flag Flag, set bool) error
Send(env Envelope, mime []byte) error
Append(folder string, mime []byte, flags Flag) error
PushDraft(folder string, mime []byte, prevUID UID) (UID, error)
```

to:

```go
Move(ctx context.Context, uids []UID, dest string) error
Destroy(ctx context.Context, uids []UID) error
Flag(ctx context.Context, uids []UID, flag Flag, set bool) error
Send(ctx context.Context, env Envelope, mime []byte) error
Append(ctx context.Context, folder string, mime []byte, flags Flag) error
PushDraft(ctx context.Context, folder string, mime []byte, prevUID UID) (UID, error)
```

Ensure `"context"` is already imported in `backend.go` (it is, for `Connect`).

- [ ] **Update MockBackend method signatures**

In `internal/mail/mock.go`, update all six methods to accept `ctx context.Context` as first parameter (the ctx is unused — `_` is fine):

```go
func (m *MockBackend) Move(_ context.Context, uids []UID, dest string) error {
	m.MoveCalls = append(m.MoveCalls, struct {
		UIDs []UID
		Dest string
	}{UIDs: append([]UID(nil), uids...), Dest: dest})
	return nil
}

func (m *MockBackend) Destroy(_ context.Context, uids []UID) error {
	if len(uids) == 0 {
		return nil
	}
	m.DestroyCalls = append(m.DestroyCalls, append([]UID(nil), uids...))
	gone := make(map[UID]struct{}, len(uids))
	for _, u := range uids {
		gone[u] = struct{}{}
	}
	kept := m.msgs[:0]
	for _, msg := range m.msgs {
		if _, drop := gone[msg.UID]; drop {
			continue
		}
		kept = append(kept, msg)
	}
	m.msgs = kept
	return nil
}

func (m *MockBackend) Flag(_ context.Context, uids []UID, flag Flag, set bool) error {
	m.FlagCalls = append(m.FlagCalls, struct {
		UIDs []UID
		Flag Flag
		Set  bool
	}{UIDs: append([]UID(nil), uids...), Flag: flag, Set: set})
	return nil
}

func (m *MockBackend) Send(_ context.Context, env Envelope, mime []byte) error {
	m.SendCalls = append(m.SendCalls, SendCall{Env: env, MIME: append([]byte(nil), mime...)})
	return m.SendErr
}

func (m *MockBackend) Append(_ context.Context, folder string, mime []byte, flag Flag) error {
	m.AppendCalls = append(m.AppendCalls, AppendCall{Folder: folder, MIME: append([]byte(nil), mime...), Flag: flag})
	return m.AppendErr
}

func (m *MockBackend) PushDraft(_ context.Context, _ string, _ []byte, _ UID) (UID, error) {
	return "", ErrUnsupported
}
```

Add `"context"` to mock.go's import block.

- [ ] **Update mock_test.go callers**

In `internal/mail/mock_test.go`, add `context.Background()` as the first argument to all calls of the six methods. Example:

```go
// Before:
m.Destroy(target)
m.Send(env, []byte("body"))
m.Append("Sent", []byte("mime"), FlagSeen)

// After:
m.Destroy(context.Background(), target)
m.Send(context.Background(), env, []byte("body"))
m.Append(context.Background(), "Sent", []byte("mime"), FlagSeen)
```

Add `"context"` to mock_test.go imports.

- [ ] **Verify compilation**

```bash
cd /home/glw907/Projects/poplar && go build ./internal/mail/... 2>&1
```
Expected: the `mail` package compiles. Other packages will be broken — that is expected and fixed in the next two tasks.

- [ ] **Run mail package tests only**

```bash
cd /home/glw907/Projects/poplar && go test -tags=dev ./internal/mail/... 2>&1
```
Expected: PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mail/ && git commit -m "Add ctx to six mail.Backend mutating methods

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Update `mailimap` implementations

**Files:**
- Modify: `internal/mailimap/actions.go`
- Modify: `internal/mailimap/smtp.go`
- Modify: `internal/mailimap/actions_test.go`
- Modify: `internal/mailimap/smtp_test.go`

- [ ] **Update `actions.go` method signatures**

In `internal/mailimap/actions.go`, update:

```go
func (b *Backend) Move(_ context.Context, uids []mail.UID, dest string) error {
```
```go
func (b *Backend) Destroy(_ context.Context, uids []mail.UID) error {
```
```go
func (b *Backend) Flag(_ context.Context, uids []mail.UID, f mail.Flag, set bool) error {
```

Add `"context"` import if not already present.

- [ ] **Update `smtp.go` method signatures**

In `internal/mailimap/smtp.go`, update:

```go
func (b *Backend) Send(_ context.Context, env mail.Envelope, mime []byte) error {
```
```go
func (b *Backend) Append(_ context.Context, folder string, mime []byte, flags mail.Flag) error {
```
```go
func (b *Backend) PushDraft(_ context.Context, folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
```

Add `"context"` import if not already present.

- [ ] **Update test callers in `actions_test.go` and `smtp_test.go`**

Add `context.Background()` as first arg to every call of Move, Flag, Destroy, Send, Append, PushDraft. Add `"context"` import to each test file.

- [ ] **Run mailimap tests**

```bash
cd /home/glw907/Projects/poplar && go test -tags=dev ./internal/mailimap/... 2>&1
```
Expected: PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailimap/ && git commit -m "Update mailimap Backend methods to accept ctx

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Update `mailjmap` implementations

**Files:**
- Modify: `internal/mailjmap/jmap.go`
- Modify: `internal/mailjmap/jmap_test.go`

- [ ] **Update `jmap.go` method signatures**

In `internal/mailjmap/jmap.go`, update:

```go
func (b *Backend) Move(_ context.Context, uids []mail.UID, destFolder string) error {
```
```go
func (b *Backend) Destroy(_ context.Context, uids []mail.UID) error {
```
```go
func (b *Backend) Flag(_ context.Context, uids []mail.UID, flag mail.Flag, set bool) error {
```
```go
func (b *Backend) Send(_ context.Context, env mail.Envelope, mime []byte) error {
```
```go
func (b *Backend) Append(_ context.Context, folder string, mime []byte, flags mail.Flag) error {
```
```go
func (b *Backend) PushDraft(_ context.Context, folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
```

Add `"context"` import if not already present.

- [ ] **Update test callers in `jmap_test.go`**

Add `context.Background()` as first arg to every call of Flag, Move, Send, Append, Destroy, PushDraft in the test file. Add `"context"` import.

- [ ] **Run mailjmap tests**

```bash
cd /home/glw907/Projects/poplar && go test -tags=dev ./internal/mailjmap/... 2>&1
```
Expected: PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailjmap/ && git commit -m "Update mailjmap Backend methods to accept ctx

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: Drainer — op_id correlation + debug logging

**Files:**
- Modify: `internal/cache/drainer.go`

- [ ] **Update `dispatch` to accept and thread `ctx`**

In `internal/cache/drainer.go`, change the `dispatch` signature from:

```go
func (a *Account) dispatch(args OpArgs, row *outboxRow) error {
```

to:

```go
func (a *Account) dispatch(ctx context.Context, args OpArgs, row *outboxRow) error {
```

Update all six backend calls inside `dispatch` to pass `ctx`:

```go
case MoveArgs:
    return a.Backend.Move(ctx, uids, v.Dest)
case FlagArgs:
    return a.Backend.Flag(ctx, uids, v.Flag, v.Set)
case DestroyArgs:
    return a.Backend.Destroy(ctx, uids)
case SendArgs:
    return a.Backend.Send(ctx, v.Envelope, row.Payload)
case AppendArgs:
    return a.Backend.Append(ctx, row.FolderName, row.Payload, v.Flag)
case PushDraftArgs:
    newUID, err := a.Backend.PushDraft(ctx, row.FolderName, row.Payload, v.PrevServerUID)
```

- [ ] **Attach op_id in `executeOne` and log dispatch entry**

In `executeOne`, replace the existing `a.dispatch(args, row)` call with:

```go
opCtx := logctx.WithOpID(ctx, fmt.Sprint(row.ID))
a.log.DebugContext(opCtx, "drainer dispatch", "kind", row.Kind)
dispatchErr := a.dispatch(opCtx, args, row)
```

Add imports `"github.com/glw907/poplar/internal/logctx"` and `"fmt"` (if not already present) to `drainer.go`.

- [ ] **Add debug log on terminal outcomes**

In `executeOne`, after the switch on `dispatchErr`, the existing `logTerminal` calls handle terminal states. Add a single `DebugContext` on the happy path (OpDone):

In the `case dispatchErr == nil:` branch, before `a.logTerminal(...)`:

```go
a.log.DebugContext(opCtx, "drainer done", "kind", row.Kind)
```

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/cache/drainer.go && git commit -m "Thread ctx + op_id through drainer dispatch

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 7: ADR + final gate

**Files:**
- Create: `docs/poplar/decisions/0240-backend-ctx-correlation.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `docs/poplar/invariants.md`

- [ ] **Write ADR-0240**

```markdown
# ADR-0240: Context Threading Through mail.Backend + Op ID Correlation

**Status:** accepted
**Date:** 2026-05-17

## Decision

Six mutating `mail.Backend` methods — `Move`, `Flag`, `Destroy`, `Send`,
`Append`, `PushDraft` — gain a `ctx context.Context` first parameter.

A new `internal/logctx` package provides `WithOpID(ctx, id)` and a
`logctx.Handler` slog wrapper that injects context-carried `op_id` attrs
into every log record. `installLogger` wraps the text handler with it.

The cache drainer's `executeOne` attaches the outbox `row.ID` as the
correlation key before calling `dispatch`, so all log records emitted
during a queued operation automatically carry `op_id`.

## Rationale

Without correlation, log records from a drainer dispatch, its backend
call, and its cache commit are unrelated entries. With `op_id` threaded
via context and injected globally by the handler, no per-site changes
are needed when new log calls are added. The handler wrapper pattern
is idiomatic slog; `WithAttrs`/`WithGroup` overrides ensure derived
loggers (via `slog.Logger.With`) preserve the injection.

## Consequences

Read-path Backend methods (`FetchBody`, `FetchHeaders`, etc.) are
unchanged — they are synchronous UI operations, not queued. Non-drainer
callers of the mutated methods pass `context.Background()`.
```

- [ ] **Update `INDEX.md` and `invariants.md`**

Add ADR-0240 to `docs/poplar/decisions/INDEX.md` under the Logging section (create the section if absent). Update the Logging section of `docs/poplar/invariants.md` to note that `mail.Backend` mutating methods now accept ctx and that `installLogger` wraps with `logctx.Handler`.

- [ ] **Run final `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add docs/ && git commit -m "Pass 42: ctx threading through mail.Backend + op_id correlation (ADR-0240)

Co-Authored-By: Claude <noreply@anthropic.com>"
```
