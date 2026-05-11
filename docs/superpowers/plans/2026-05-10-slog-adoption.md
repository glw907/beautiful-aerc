# Pass 16d — `log/slog` adoption + logging-convention ADR

## Goal

Adopt stdlib `log/slog` for poplar's diagnostic logging. Land an
ADR that binds where slog is used, which handler/destination, and
how backends acquire their logger. Convert every `fmt.Fprintf(os
.Stderr, ...)` diagnostic in `internal/` (4 sites + 1 hand-rolled
seam) and add the previously-silent reconnect/idle transcript to
`internal/mailimap`.

## Why

The 16-series modernizes poplar onto Go 1.21+ stdlib idioms.
slog is the last big stdlib-default we haven't claimed. The
current diagnostic surface is `fmt.Fprintf(os.Stderr, ...)` —
invisible under bubbletea's altscreen, untaggable per account,
unfilterable by level, and inconsistent with the upcoming wave
of subsystems that want structured fields (search query timing,
backfill throughput, drainer retry traces). Pre-beta's no-scars
rule makes this the time to convert wholesale, not site-by-site.

## Scope

In:

- `internal/mailjmap/push.go` — 3 sites (handleStateChange,
  refreshBlobIDs, dropped update).
- `internal/mailjmap/jmap.go:859` — push-draft destroy prior.
- `internal/cache/drainer.go` — retire the hand-rolled
  `stderrLog` seam; route drainer diagnostics through slog.
- `internal/mailimap/` — add slog to reconnect / idle / SMTP
  reconnect paths (currently silent failures swallowed into
  reconnect backoff).
- `cmd/poplar/main.go` — install the root handler; route to
  file or stderr based on TTY state and `POPLAR_LOG`.
- New ADR (number TBD at write time, expected 0197).

Out:

- `cmd/poplar/root.go`, `reauth.go`, `main.go` user-facing CLI
  text (`Edit the file and run poplar again.`, etc.) — those
  stay on `os.Stderr`. They're UX strings, not log events.
- Any new diagnostic sites in subsystems not already listed.
  Search, backfill, contacts may grow slog calls later; this
  pass installs the substrate, not new instrumentation.
- Pass 17a/b/c (bubbles adoption remainder) — separate passes.

## Design

### Logger ownership (Option A from brainstorm)

Package-default with backend-snapshotted field. `cmd/poplar/main.go`
calls `slog.SetDefault(...)` once at startup. Each `Backend`
constructor snapshots into a struct field:

```go
// internal/mailjmap/jmap.go
type Backend struct {
    // ...
    log *slog.Logger
}

type Option func(*Backend)

func WithLogger(l *slog.Logger) Option {
    return func(b *Backend) { b.log = l }
}

func New(cfg Config, opts ...Option) (*Backend, error) {
    b := &Backend{
        // ...
        log: slog.Default().With("component", "mailjmap"),
    }
    for _, o := range opts {
        o(b)
    }
    // ...
}
```

`mailimap.New` mirrors the shape with `"component", "mailimap"`.
`cache.Open` mirrors it with `"component", "cache"` and an
optional `WithLogger` for the drainer's tests.

Account tagging happens at backend construction in
`cmd/poplar/root.go` if/when the call site knows the account name:
not threaded down through every helper.

### Handler + destination

```go
// cmd/poplar/main.go
func installLogger() {
    level := slog.LevelInfo
    if v := os.Getenv("POPLAR_LOG"); v == "debug" {
        level = slog.LevelDebug
    }
    var w io.Writer = os.Stderr
    if term.IsTerminal(int(os.Stdout.Fd())) {
        // TUI mode: stderr is hidden under altscreen.
        // Route to $XDG_STATE_HOME/poplar/poplar.log instead.
        if f, err := openStateLog(); err == nil {
            w = f
        }
    }
    h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
    slog.SetDefault(slog.New(h))
}
```

`openStateLog` resolves `$XDG_STATE_HOME/poplar/` (default
`~/.local/state/poplar/`), creates it if missing, opens
`poplar.log` for append. Failure to open falls back to stderr
silently — logging must never crash startup.

### Test capture

`WithLogger` on each backend constructor is the canonical test
seam (brainstorm Option C). Tests construct backends with a
`bytes.Buffer`-backed handler and assert on output:

```go
var buf bytes.Buffer
h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
b, _ := mailjmap.New(cfg, mailjmap.WithLogger(slog.New(h)))
// ... exercise b ...
if !strings.Contains(buf.String(), "refreshBlobIDs") { t.Fatal(...) }
```

No `slog.SetDefault` swap in tests. Parallel-safe.

### Migration sites

| Site | Before | After |
|---|---|---|
| `mailjmap/push.go:127` | `fmt.Fprintf(os.Stderr, "mailjmap: handleStateChange %s: %v\n", typ, err)` | `b.log.Error("handleStateChange failed", "type", typ, "err", err)` |
| `mailjmap/push.go:189` | `fmt.Fprintf(os.Stderr, "mailjmap: refreshBlobIDs: %v\n", err)` | `b.log.Error("refreshBlobIDs failed", "err", err)` |
| `mailjmap/push.go:265` | `fmt.Fprintln(os.Stderr, "mailjmap: dropped update, buffer full")` | `b.log.Warn("dropped update, channel full")` |
| `mailjmap/jmap.go:859` | `fmt.Fprintln(os.Stderr, "push-draft: destroy prior:", err)` | `b.log.Warn("push-draft destroy prior failed", "err", err)` |
| `cache/drainer.go:325` | `var stderrLog = func() io.Writer { return os.Stderr }` + Fprintf callers | `d.log.Error(...)` directly; seam deleted |
| `mailimap/idle.go` (new) | silent | `b.log.Info("idle reconnect", "attempt", n, "delay", d)` etc. |

Drainer cleanup: `stderrLog` and `encodeErr`'s Fprintln-to-it
callers are replaced by direct `d.log.Error("op failed", "kind",
kind, "err", err)`. The `encodeErr` JSON shape that lands in
`outbox.error_json` is unchanged (it's persisted state, not log
output).

## Tasks

1. **`cmd/poplar/main.go`**: add `installLogger()`, call it
   before `tea.NewProgram`. `openStateLog` helper. `term.IsTerminal`
   already imported via `internal/term`.
2. **`internal/mailjmap`**: add `Option` type + `WithLogger`.
   `New` accepts `...Option`. `Backend.log` field. Convert 4
   call sites in `push.go` and `jmap.go`. Update `New`'s test
   callers (mostly construction with no opts; no test changes
   required unless asserting on logs).
3. **`internal/mailimap`**: same `Option` + `WithLogger` shape.
   `Backend.log` field. Add slog calls in `imap.go` reconnect
   loop, idle restart path, and SMTP cached-client drop.
4. **`internal/cache`**: same option shape on `Open`. `Account.log`
   field. Drainer migrates from `stderrLog` to `acct.log`;
   delete `stderrLog` and its test override. Adjust drainer tests
   to construct accounts with `WithLogger(slog.New(...))`.
5. **Write ADR-0197**: log/slog adoption. Bind: stdlib only,
   `slog.NewTextHandler`, file-on-TTY routing, `POPLAR_LOG=debug`
   level control, `WithLogger` seam shape, "diagnostic logs go
   through slog; user-facing CLI output stays on stderr."
6. **Update invariants.md** — add a "Logging" subsection under
   "Build & verification" or its own short top-level. Remove
   any prior reference to `stderrLog`.
7. **Update STATUS.md** — Pass 16d done; next prompt = Pass 17a
   sidebar tree.
8. **`make check` green; `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh`
   exit 0.**
9. **Pass-end ritual** via `poplar-pass` (commit, push, install).

## Verification

- Existing unit tests in `mailjmap`, `mailimap`, `cache/`
  pass unmodified (their behavior doesn't depend on log shape).
- One new test per converted package asserts the `WithLogger`
  seam captures expected output (e.g.,
  `mailjmap.TestPushLoopLogsRefreshFailure`).
- Live tmux verification: launch poplar, confirm no stderr
  bleed under altscreen, confirm `~/.local/state/poplar/poplar.log`
  receives entries during a real Fastmail session.
- `poplar config check` (non-TUI) routes to stderr as before.

## ADR

ADR-0197 — `log/slog` adoption and logging convention. One
ADR, drafted at pass-end per the consolidation ritual.

## Open questions

None. Brainstorm settled three; the rest are mechanical.
