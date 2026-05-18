# Pass 43: Visibility Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add wire-level protocol tracing (IMAP/SMTP/JMAP), Debug checkpoints throughout backends and cache, size-based log rotation via lumberjack, and a startup session marker — all built on the Pass 42 correlation infrastructure.

**Architecture:** `logctx.WireWriter` (exported from internal/logctx) implements `io.Writer` as a slog-emitting line splitter; backend constructors gain `wireTrace bool`; `openBackend` threads `wireTrace` from `runRoot` where the config flag and env override are resolved. Lumberjack replaces the hand-rolled `openStateLog`. Debug checkpoints use `b.log.DebugContext(ctx, ...)` and carry `op_id` automatically from the Pass 42 handler. Pass 43 requires Pass 42 to be complete.

**Tech Stack:** Go 1.26, `log/slog`, `gopkg.in/natefinch/lumberjack.v2`, `net/http/httputil`

---

### Task 1: `wire-trace` config option

**Files:**
- Modify: `internal/config/ui.go`
- Modify: `internal/config/writer.go`
- Modify: `internal/config/template.go`
- Modify: `internal/config/ui_test.go`

- [ ] **Add `WireTrace` to UIConfig and rawUI**

In `internal/config/ui.go`, add to `UIConfig` after `LogLevel`:

```go
// WireTrace enables protocol-level wire logging for IMAP, SMTP, and
// JMAP. Logs all traffic including credentials; use only for debugging.
// POPLAR_WIRE_TRACE=1 overrides this setting.
WireTrace bool
```

Add to `rawUI` struct after `LogLevel`:

```go
WireTrace *bool `toml:"wire-trace"`
```

Add to `LoadUI` after the `LogLevel` block:

```go
if raw.UI.WireTrace != nil {
    out.WireTrace = *raw.UI.WireTrace
}
```

- [ ] **Update `renderUIBlock` in `writer.go`**

After the `log-level` entry block, add:

```go
if ui.WireTrace {
    rows = append(rows, entry{"wire-trace", "true"})
}
```

- [ ] **Update the config template**

In `internal/config/template.go`, add after the `spam_retention_days` comment block inside the `[ui]` section:

```go
// # log-level = "info"
// #     Diagnostic log level. "info" (default) or "debug".
// #     POPLAR_LOG=debug overrides this setting.
// #
// # wire-trace = false
// #     Log all IMAP, SMTP, and JMAP protocol traffic at DEBUG level.
// #     POPLAR_WIRE_TRACE=1 overrides this setting. Warning: wire
// #     trace logs contain credentials and message content — enable
// #     only when debugging connection problems, and rotate or delete
// #     the log when done.
```

Add this block to the `templateBody` constant just before the `\n\n# ──────────────────` that opens the CACHE section.

- [ ] **Write tests**

Add to `internal/config/ui_test.go` in `TestLoadUI_LogLevel` or as a standalone function:

```go
func TestLoadUI_WireTrace(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "config.toml")

    cases := []struct {
        name string
        toml string
        want bool
    }{
        {"absent", "[ui]\n", false},
        {"true", "[ui]\nwire-trace = true\n", true},
        {"false", "[ui]\nwire-trace = false\n", false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if err := os.WriteFile(path, []byte(tc.toml), 0o600); err != nil {
                t.Fatal(err)
            }
            cfg, err := LoadUI(path)
            if err != nil {
                t.Fatalf("LoadUI: %v", err)
            }
            if cfg.WireTrace != tc.want {
                t.Errorf("WireTrace = %v, want %v", cfg.WireTrace, tc.want)
            }
        })
    }
}
```

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/config/ && git commit -m "Add wire-trace config option to UIConfig

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Lumberjack log rotation

**Files:**
- Modify: `cmd/poplar/log.go`
- Modify: `cmd/poplar/log_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Add lumberjack dependency**

```bash
cd /home/glw907/Projects/poplar && go get gopkg.in/natefinch/lumberjack.v2 2>&1
```
Expected: go.mod and go.sum updated.

- [ ] **Replace `openStateLog` with lumberjack in `installLogger`**

In `cmd/poplar/log.go`, replace the import block with:

```go
import (
    "io"
    "log/slog"
    "os"
    "path/filepath"

    "github.com/glw907/poplar/internal/logctx"
    "golang.org/x/term"
    "gopkg.in/natefinch/lumberjack.v2"
)
```

Replace the TTY branch inside `installLogger`:

```go
var w io.Writer = os.Stderr
if term.IsTerminal(int(os.Stdout.Fd())) {
    w = &lumberjack.Logger{
        Filename:   filepath.Join(stateDir(), "poplar.log"),
        MaxSize:    10,
        MaxBackups: 2,
    }
}
h := logctx.Handler{Handler: slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})}
slog.SetDefault(slog.New(h))
```

Remove the `openStateLog` function entirely (its body is replaced by the `lumberjack.Logger` literal above).

Full `installLogger` after the change:

```go
func installLogger(cfgLevel string) {
    level := slog.LevelInfo
    switch {
    case os.Getenv("POPLAR_LOG") == "debug":
        level = slog.LevelDebug
    case cfgLevel == "debug":
        level = slog.LevelDebug
    }
    var w io.Writer = os.Stderr
    if term.IsTerminal(int(os.Stdout.Fd())) {
        w = &lumberjack.Logger{
            Filename:   filepath.Join(stateDir(), "poplar.log"),
            MaxSize:    10,
            MaxBackups: 2,
        }
    }
    h := logctx.Handler{Handler: slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})}
    slog.SetDefault(slog.New(h))
}
```

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS. (Tests use `XDG_STATE_HOME` override so they write to temp dirs, not the real log.)

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add cmd/poplar/log.go go.mod go.sum && git commit -m "Replace openStateLog with lumberjack rotation (10 MB, 2 backups)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: `logctx.WireWriter` type

**Files:**
- Modify: `internal/logctx/logctx.go`
- Modify: `internal/logctx/logctx_test.go`

`WireWriter` is defined in `internal/logctx` so both backend packages and `cmd/poplar` can import it without circular dependencies.

- [ ] **Add `WireWriter` to `internal/logctx/logctx.go`**

Add after the `WithGroup` method:

```go
// WireWriter implements io.Writer by splitting on newlines and emitting
// one slog.Debug record per non-empty line. Component labels the source
// protocol in the "component" field.
type WireWriter struct{ Component string }

func (w WireWriter) Write(p []byte) (int, error) {
    start := 0
    for i, b := range p {
        if b == '\n' {
            if i > start {
                slog.Debug("wire", "component", w.Component, "data", string(p[start:i]))
            }
            start = i + 1
        }
    }
    if start < len(p) {
        slog.Debug("wire", "component", w.Component, "data", string(p[start:]))
    }
    return len(p), nil
}
```

Add `"bytes"` to imports if needed (it's not needed here — using index loop instead).

- [ ] **Add test to `logctx_test.go`**

```go
func TestWireWriter_EmitsLines(t *testing.T) {
    var buf bytes.Buffer
    inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    slog.SetDefault(slog.New(inner))
    t.Cleanup(func() { slog.SetDefault(slog.Default()) })

    w := WireWriter{Component: "imap"}
    _, _ = w.Write([]byte("A001 LOGIN user pass\nA002 SELECT INBOX\n"))

    out := buf.String()
    if !strings.Contains(out, "A001 LOGIN user pass") {
        t.Errorf("first line missing: %s", out)
    }
    if !strings.Contains(out, "A002 SELECT INBOX") {
        t.Errorf("second line missing: %s", out)
    }
    if !strings.Contains(out, "component=imap") {
        t.Errorf("component missing: %s", out)
    }
}

func TestWireWriter_SkipsEmptyLines(t *testing.T) {
    var buf bytes.Buffer
    inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    slog.SetDefault(slog.New(inner))
    t.Cleanup(func() { slog.SetDefault(slog.Default()) })

    w := WireWriter{Component: "smtp"}
    _, _ = w.Write([]byte("\n\nfoo\n"))

    out := buf.String()
    lines := strings.Count(out, "\n")
    if lines != 1 {
        t.Errorf("want 1 line, got %d: %s", lines, out)
    }
}
```

- [ ] **Run tests**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/logctx/... -v 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/logctx/ && git commit -m "Add logctx.WireWriter: slog-emitting io.Writer for protocol tracing

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: mailimap IMAP wire-trace wiring

**Files:**
- Modify: `internal/mailimap/imap.go`
- Modify: `internal/mailimap/auth.go`
- Modify: `internal/mailimap/imap_test.go` (if it exists; check with find)

- [ ] **Add `wireTrace bool` to `Backend` and constructors**

In `internal/mailimap/imap.go`, add `wireTrace bool` to the `Backend` struct after `oauth`:

```go
oauth     *mailauth.Client
wireTrace bool
```

Update `New` signature and body:

```go
func New(cfg config.AccountConfig, log *slog.Logger, wireTrace bool) *Backend {
    if log == nil {
        log = slog.Default().With("component", "mailimap")
    }
    b := &Backend{cfg: cfg, log: log, wireTrace: wireTrace}
    b.dialFn = func(ctx context.Context, role string) (imapClient, error) { return dial(ctx, b, role) }
    return b
}
```

Update `NewWithOAuth` signature and body:

```go
func NewWithOAuth(cfg config.AccountConfig, c *mailauth.Client, log *slog.Logger, wireTrace bool) *Backend {
    if log == nil {
        log = slog.Default().With("component", "mailimap")
    }
    b := &Backend{cfg: cfg, log: log, oauth: c, wireTrace: wireTrace}
    b.dialFn = func(ctx context.Context, role string) (imapClient, error) { return dial(ctx, b, role) }
    return b
}
```

Add `"github.com/glw907/poplar/internal/logctx"` to `imap.go` imports.

- [ ] **Set `DebugWriter` in `dial()`**

In `internal/mailimap/auth.go`, inside `dial()`, after the `opts` literal is constructed (after the `UnilateralDataHandler` closing `}`), add:

```go
if b.wireTrace {
    opts.DebugWriter = logctx.WireWriter{Component: "imap"}
}
```

Add `"github.com/glw907/poplar/internal/logctx"` to `auth.go` imports.

- [ ] **Fix callers in `cmd/poplar/backend.go`**

In `cmd/poplar/backend.go`, update both `mailimap.New` and `mailimap.NewWithOAuth` calls — the `wireTrace bool` argument is added in Task 7. For now, pass `false` as a placeholder:

```go
return mailimap.NewWithOAuth(acct, c, nil, false), nil
...
return mailimap.New(acct, nil, false), nil
```

Also update the probe paths in any test-only or cmd code that calls these constructors (check with grep):

```bash
grep -rn "mailimap.New\|mailimap.NewWithOAuth" /home/glw907/Projects/poplar/ --include="*.go" | grep -v "_test.go"
```

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailimap/imap.go internal/mailimap/auth.go cmd/poplar/backend.go && git commit -m "Wire IMAP DebugWriter via logctx.WireWriter (placeholder false)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: mailimap SMTP wire-trace wiring

**Files:**
- Modify: `internal/mailimap/smtp.go`

- [ ] **Set `DebugWriter` in `realSMTPDial`**

In `internal/mailimap/smtp.go`, inside `realSMTPDial`, add after both SMTP client construction branches (right before `smtpAuth`):

The current code ends with:
```go
    } else {
        tlsConn := tls.Client(raw, tlsCfg)
        if err := tlsConn.Handshake(); err != nil {
            _ = raw.Close()
            return nil, fmt.Errorf("smtp tls handshake %s: %w", addr, err)
        }
        cli = gosmtp.NewClient(tlsConn)
    }

    if err := smtpAuth(ctx, cli, b); err != nil {
```

Insert after `cli = gosmtp.NewClient(tlsConn)` / after the StartTLS branch closes, before `smtpAuth`:

```go
    if b.wireTrace {
        cli.DebugWriter = logctx.WireWriter{Component: "smtp"}
    }

    if err := smtpAuth(ctx, cli, b); err != nil {
```

Full insertion point in context:

```go
    var cli *gosmtp.Client
    if smtp.StartTLS {
        cli, err = gosmtp.NewClientStartTLS(raw, tlsCfg)
        if err != nil {
            _ = raw.Close()
            return nil, fmt.Errorf("smtp starttls %s: %w", addr, err)
        }
    } else {
        tlsConn := tls.Client(raw, tlsCfg)
        if err := tlsConn.Handshake(); err != nil {
            _ = raw.Close()
            return nil, fmt.Errorf("smtp tls handshake %s: %w", addr, err)
        }
        cli = gosmtp.NewClient(tlsConn)
    }

    if b.wireTrace {
        cli.DebugWriter = logctx.WireWriter{Component: "smtp"}
    }

    if err := smtpAuth(ctx, cli, b); err != nil {
        _ = cli.Close()
        return nil, fmt.Errorf("smtp authenticate: %w", err)
    }
    return &smtpClientAdapter{c: cli}, nil
```

Add `"github.com/glw907/poplar/internal/logctx"` to `smtp.go` imports.

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailimap/smtp.go && git commit -m "Wire SMTP DebugWriter via logctx.WireWriter

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: mailjmap JMAP wire-trace wiring

**Files:**
- Create: `internal/mailjmap/wire.go`
- Modify: `internal/mailjmap/jmap.go`

- [ ] **Create `internal/mailjmap/wire.go` with `loggingTransport`**

```go
package mailjmap

import (
    "net/http"
    "net/http/httputil"

    "github.com/glw907/poplar/internal/logctx"
)

type loggingTransport struct{ inner http.RoundTripper }

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    if dump, err := httputil.DumpRequestOut(req, true); err == nil {
        w := logctx.WireWriter{Component: "jmap"}
        _, _ = w.Write(dump)
    }
    resp, err := t.inner.RoundTrip(req)
    if err != nil {
        return nil, err
    }
    if dump, err := httputil.DumpResponse(resp, true); err == nil {
        w := logctx.WireWriter{Component: "jmap"}
        _, _ = w.Write(dump)
    }
    return resp, nil
}
```

- [ ] **Add `wireTrace bool` to `Backend` and `New`**

In `internal/mailjmap/jmap.go`, add `wireTrace bool` to the `Backend` struct after `log`:

```go
log       *slog.Logger
wireTrace bool
```

Update `New` signature:

```go
func New(cfg config.AccountConfig, log *slog.Logger, wireTrace bool) *Backend {
    if log == nil {
        log = slog.Default().With("component", "mailjmap")
    }
    return &Backend{
        cfg:         cfg,
        log:         log,
        wireTrace:   wireTrace,
        folders:     make(map[string]folderEntry),
        blobIDs:     make(map[mail.UID]string),
        partBlobIDs: make(map[mail.UID]map[string]string),
        states:      make(map[string]string),
        identityIDs: make(map[string]jmap.ID),
    }
}
```

`NewWithClient` is test-only and doesn't need the parameter — it calls `New(cfg, nil, false)` internally. Update `NewWithClient` to call `New` with `false`:

```go
func NewWithClient(cfg config.AccountConfig, c jmapClient) *Backend {
    b := New(cfg, nil, false)
    b.client = c
    ...
}
```

- [ ] **Wrap `HttpClient.Transport` in `Connect`**

In `internal/mailjmap/jmap.go`, inside `Connect()`, after `cli.WithAccessToken(pw)` and before `cli.Authenticate()`:

```go
cli.WithAccessToken(pw)
if b.wireTrace {
    cli.HttpClient.Transport = &loggingTransport{inner: cli.HttpClient.Transport}
}
if err := cli.Authenticate(); err != nil {
```

- [ ] **Fix caller in `cmd/poplar/backend.go`**

Update `mailjmap.New` call to pass `false` for now (will be replaced in Task 7):

```go
return mailjmap.New(acct, nil, false), nil
```

Also check for any other callers:

```bash
grep -rn "mailjmap.New[^W]" /home/glw907/Projects/poplar/ --include="*.go"
```

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailjmap/ cmd/poplar/backend.go && git commit -m "Wire JMAP loggingTransport via logctx.WireWriter (placeholder false)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 7: Thread `wireTrace` through `openBackend` → `runRoot`

**Files:**
- Modify: `cmd/poplar/backend.go`
- Modify: `cmd/poplar/root.go`

- [ ] **Update `openBackend` signature**

In `cmd/poplar/backend.go`, change:

```go
func openBackend(acct config.AccountConfig) (mail.Backend, error) {
```

to:

```go
func openBackend(acct config.AccountConfig, wireTrace bool) (mail.Backend, error) {
```

Replace all three backend constructor calls inside `openBackend` to pass `wireTrace`:

```go
case "mock":
    return openMockBackend(acct)
case "jmap":
    return mailjmap.New(acct, nil, wireTrace), nil
case "imap":
    if acct.OAuth != nil {
        c, err := buildOAuthClient(acct)
        if err != nil {
            return nil, fmt.Errorf("oauth client for %q: %w", acct.Name, err)
        }
        return mailimap.NewWithOAuth(acct, c, nil, wireTrace), nil
    }
    return mailimap.New(acct, nil, wireTrace), nil
```

- [ ] **Resolve `wireTrace` in `runRoot` and pass to `openBackend`**

In `cmd/poplar/root.go`, the existing call is:

```go
installLogger(uiCfg.LogLevel)

slog.Debug("opening backend", "account", accts[0].Name, "provider", accts[0].Backend)
backend, err := openBackend(accts[0])
```

Replace with:

```go
installLogger(uiCfg.LogLevel)

wireTrace := uiCfg.WireTrace || os.Getenv("POPLAR_WIRE_TRACE") == "1"
slog.Debug("opening backend", "account", accts[0].Name, "provider", accts[0].Backend)
backend, err := openBackend(accts[0], wireTrace)
```

(`os` is already imported in `root.go`.)

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add cmd/poplar/backend.go cmd/poplar/root.go && git commit -m "Thread wireTrace from config+env through openBackend to constructors

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 8: Debug checkpoints — mailimap, mailjmap, cache

**Files:**
- Modify: `internal/mailimap/auth.go`
- Modify: `internal/mailimap/idle.go`
- Modify: `internal/mailimap/redial.go`
- Modify: `internal/mailjmap/jmap.go`
- Modify: `internal/cache/account.go`
- Modify: `internal/cache/schema.go`

All checkpoints use `b.log.DebugContext(ctx, ...)` (mailimap/mailjmap) or `slog.Default().With("component","cache").Debug(...)` (cache, where the logger isn't threaded to `applyMigrations`). In cache.Open / drainer pickup, use `log.Debug(...)` since `log` is in scope.

- [ ] **mailimap — dial checkpoints in `auth.go`**

Inside `dial()` in `internal/mailimap/auth.go`, add after the `addr` line:

```go
b.log.Debug("imap dial", "addr", addr, "role", role)
```

After `layerTLS` succeeds (before `resolveXOAUTH2Token`):

```go
b.log.Debug("imap tls", "server", cfg.Host, "role", role)
```

After `authenticate` succeeds (before `rc.c = cli`):

```go
b.log.Debug("imap auth", "mechanism", resolvedMechanism(cfg), "role", role)
```

`resolvedMechanism` is a one-liner helper in `auth.go` (add it):

```go
func resolvedMechanism(cfg config.AccountConfig) string {
    if cfg.Auth == "" {
        return "plain"
    }
    return cfg.Auth
}
```

- [ ] **mailimap — IDLE start/stop in `idle.go`**

In `runIdleSession` (or `runIDLE` — check the function that starts the IDLE command), add at IDLE start:

```go
b.log.Debug("imap idle start", "folder", current)
```

Add at IDLE stop / clean cycle end:

```go
b.log.Debug("imap idle stop", "reason", "refresh")
```

For error path stops (already have `b.log.Warn`), no additional Debug needed.

To find the exact insertion point, look for where `b.idle.Idle()` or similar is called in `idle.go`.

- [ ] **mailimap — cmd-path redial checkpoint in `redial.go`**

In `internal/mailimap/redial.go`, after the existing `b.log.Info("imap cmd redialed")` line (line ~51), add:

```go
b.log.Debug("imap cmd redial complete", "folder", current)
```

(The `Info` log already marks the redial; the Debug adds the folder context for correlation.)

- [ ] **mailjmap — Connect checkpoints in `jmap.go`**

In `Connect()`, add after `cli.Authenticate()` succeeds:

```go
b.log.Debug("jmap session", "endpoint", b.cfg.Source)
```

In `runEventSource` (or the EventSource goroutine start), at the point where the EventSource connection is established, add:

```go
b.log.Debug("jmap eventsource connect")
```

In the state-change dispatch loop (where `StateChange` events are processed and `mail.Update` values are emitted), add:

```go
b.log.Debug("jmap state change", "type", changeType)
```

To find the exact location, look for `mail.Update{Type:` in `jmap.go` or the event loop goroutine.

- [ ] **cache — Open and migration checkpoints**

In `internal/cache/account.go`, inside `Open`, after `applyMigrations` succeeds (before constructing `Account`):

```go
log.Debug("cache open", "schema", schemaVersion, "account", accountName)
```

(At this point `log` is the package-scoped logger set at the top of `Open`.)

In `internal/cache/schema.go`, inside `applyMigrationsTo`, in the `for current < target` loop, add after `migrations[current](tx)` succeeds:

```go
slog.Default().With("component", "cache").Debug("cache migrate", "from", current, "to", current+1)
```

(Add this right before the `UPDATE schema_version` exec.)

The loop body currently looks like:

```go
for current < target {
    if current >= len(migrations) {
        return fmt.Errorf("missing migration step v%d→v%d", current, current+1)
    }
    if err := migrations[current](tx.Begin()...); err != nil { // simplified
        ...
    }
    current++
}
```

Find the exact commit-step in `applyMigrationsTo` and add the log after the step succeeds.

- [ ] **Run `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add internal/mailimap/ internal/mailjmap/ internal/cache/ && git commit -m "Add Debug checkpoints to mailimap, mailjmap, and cache

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 9: Startup session marker + ADR-0241 + final gate

**Files:**
- Modify: `cmd/poplar/root.go`
- Create: `docs/poplar/decisions/0241-wire-tracing-checkpoints-rotation.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `docs/poplar/invariants.md`

- [ ] **Add startup session marker to `runRoot`**

In `cmd/poplar/root.go`, immediately after `installLogger(uiCfg.LogLevel)`, add:

```go
slog.Info("poplar start", "account", accts[0].Name, "config", configPath)
```

Full context after the change:

```go
uiCfg, err := config.LoadUI(configPath)
if err != nil {
    return fmt.Errorf("ui config: %v", err)
}
installLogger(uiCfg.LogLevel)
slog.Info("poplar start", "account", accts[0].Name, "config", configPath)

wireTrace := uiCfg.WireTrace || os.Getenv("POPLAR_WIRE_TRACE") == "1"
slog.Debug("opening backend", ...)
```

- [ ] **Write ADR-0241**

```markdown
# ADR-0241: Wire Tracing, Debug Checkpoints, and Log Rotation

**Status:** accepted
**Date:** 2026-05-17

## Decision

**Wire tracing:** `wire-trace = true` in `[ui]` (or `POPLAR_WIRE_TRACE=1`) enables
protocol-level traffic logging via `imapclient.Options.DebugWriter` (IMAP),
`gosmtp.Client.DebugWriter` (SMTP), and a `loggingTransport` wrapping
`jmap.Client.HttpClient.Transport` (JMAP). All three use `logctx.WireWriter`,
which splits on newlines and emits one `slog.Debug` record per line with
`component=<protocol>`. Wire-trace is independent of `log-level` — both flags
can be set or unset in any combination. Credential exposure is documented in
the config template comment.

**Debug checkpoints:** `slog.Debug` / `b.log.Debug` calls at:
- mailimap: dial, TLS handshake, auth mechanism, IDLE start/stop, cmd redial
- mailjmap: session fetch, EventSource connect, StateChange dispatch
- cache: Open (schema version), each migration step, drainer dispatch/done
  (the drainer sites were added in ADR-0240)

**Log rotation:** `openStateLog` replaced by `*lumberjack.Logger` (10 MB,
2 backups). Size-based rotation is more appropriate than session-based given
that a single wire-trace session can exceed a prior session's worth of data.

**Startup marker:** `slog.Info("poplar start", "account", ..., "config", ...)`
in `runRoot` immediately after `installLogger`. Makes session boundaries
trivially locatable in a multi-session log file.

## Rationale

Wire-level tracing is the most useful forensic tool for connection hangs.
Size-based rotation prevents unbounded growth under wire-trace. Debug
checkpoints fill the "normal operations are silent" gap without changing error
paths. All three build on the Pass 42 `logctx.Handler` so checkpoints inside a
drainer dispatch automatically carry `op_id`.

## Consequences

New dependency: `gopkg.in/natefinch/lumberjack.v2`. Wire-trace logs contain
credentials and message content; the config template warns about this. Backend
constructors gain a `wireTrace bool` parameter; all callers outside tests pass
through `openBackend`.
```

- [ ] **Update `INDEX.md` and `invariants.md`**

In `docs/poplar/decisions/INDEX.md`, add under the Logging section:

```
ADR-0241: wire tracing, debug checkpoints, log rotation
```

In `docs/poplar/invariants.md`, update the Logging section to add:

- `wire-trace = true` in `[ui]` (or `POPLAR_WIRE_TRACE=1`) enables protocol traffic logging.
- `lumberjack` handles log rotation (10 MB, 2 backups) replacing the hand-rolled `openStateLog`.
- Debug checkpoints in `mailimap` (dial, TLS, auth, IDLE, redial), `mailjmap` (session, EventSource, StateChange), and `cache` (Open, migrate, drainer dispatch/done).
- `slog.Info("poplar start", ...)` marks session boundaries.
- Backend constructors (`mailimap.New`, `mailimap.NewWithOAuth`, `mailjmap.New`) take `wireTrace bool`; threaded from `runRoot` via `openBackend`. ADR-0241.

- [ ] **Run final `make check`**

```bash
cd /home/glw907/Projects/poplar && make check 2>&1
```
Expected: all tests PASS, no vet errors.

- [ ] **Commit**

```bash
cd /home/glw907/Projects/poplar && git add cmd/poplar/root.go docs/ && git commit -m "Pass 43: startup marker + ADR-0241 (wire tracing, checkpoints, rotation)

Co-Authored-By: Claude <noreply@anthropic.com>"
```
