# Claude Tidy — Pass 9o design

**Date:** 2026-05-08
**Pass:** 9o
**Status:** spec

## Goal

Wire the existing `internal/tidy/` package into the compose surface
as an explicit, user-invoked action. Pressing a key inside the
compose body sends the body to the Claude messages API, replaces
the body with the corrected text, and highlights the changed
character ranges so the user can see what tidy did at a glance.

Tidy never runs automatically. It is not on the send path.

## Non-goals

- Tidy on subject, headers, or any field other than the body.
- Suggest-and-confirm flow with per-correction accept/reject.
- Diff overlay or modal preview.
- Navigation between highlighted spans (1.1 candidate if eye-scan
  proves insufficient).
- Token / cost telemetry, request retries, response caching.
- Batch tidy across drafts.

## User flow

1. User is composing in the body. `[ui.tidy] enabled = true` and an
   API key resolves (config or `$ANTHROPIC_API_KEY`).
2. User presses `Ctrl+T`.
3. Compose enters a transient in-flight state. The footer shows a
   spinner hint (`tidying…`). Input keys to the body queue but
   normal Update keeps running for chrome.
4. The API call runs in a `tea.Cmd`. The package handles its own
   30 s HTTP timeout.
5. On return:
   - **`StatusCorrected`** — catkin buffer is replaced with the
     corrected text. A character-range diff between the old and
     new text feeds a `tidyAnnotator` that paints changed spans.
     A toast on the chrome row reads `Tidy: <N> corrections`.
   - **`StatusNoChanges`** — toast `Tidy: no changes needed`. Body
     untouched. Annotator unchanged.
   - **`StatusNoAuthorText`** — `c.err = "Tidy: no author text"`.
     Body untouched.
   - **`StatusError`** — `c.err = "Tidy: <message>"`. Body
     untouched. (The package's silent-fallback default behavior is
     translated to an explicit error at the seam.)
6. Highlights persist until the user touches the body. Any
   keystroke that mutates the buffer changes `src`, so
   `tidyAnnotator.Annotate(src)` sees `src != lastTidiedSrc` and
   returns no annotations on the next debounced tick. Toasts and
   error lines clear on the existing schedules.
7. Catkin's 50-step undo (ADR-0144 family) reverses the tidy
   buffer replacement; one undo step covers the whole rewrite.

When `enabled = false`, `Ctrl+T` is inert. When enabled but no API
key resolves, `Ctrl+T` shows `c.err = "Tidy: ANTHROPIC_API_KEY not
set"` without an API call.

## Key binding

`Ctrl+T` inside the catkin body editor. ADR-0076 exempts text-
entry surfaces from the modifier-free rule, so this fits alongside
the existing Catkin command vocabulary
(`Ctrl+B/I/K/L/Q/Space/\`). Outside the body the key is unbound.

## Config

`config.UIConfig` grows a `Tidy tidy.Config` field decoded from
`[ui.tidy]` plus an `Enabled bool` gate.

```toml
[ui.tidy]
enabled = false

[ui.tidy.api]
model   = "claude-haiku-4-5-20251001"
api_key = ""

[ui.tidy.rules]
spelling             = true
grammar              = true
punctuation          = true
whitespace           = true
capitalization       = true
repeated_words       = true
missing_punctuation  = true
oxford_comma         = "ignore"

[ui.tidy.style]
em_dash_spaces       = false
ellipsis             = "character"
time_format          = "ignore"
custom_instructions  = []
```

The `[ui.tidy.api]`, `[ui.tidy.rules]`, and `[ui.tidy.style]`
sub-tables decode one-to-one into `tidy.APIConfig`,
`tidy.RulesConfig`, and `tidy.StyleConfig` via the package's
existing TOML tags. The `Enabled` gate is poplar-side, separate
from the package config.
The package's existing enum validators (`oxford_comma`,
`ellipsis`, `time_format`) run on `LoadUI` and surface errors
through the same `config.Load` error path that other `[ui]`
fields use.

`tidy.ResolveAPIKey` continues to be the only API-key resolver:
config first, env var fallback.

## Architecture

### Package additions

**`internal/tidy/diff.go`** — new file, ~120 LOC:

```go
// ByteRange identifies a half-open byte range [Start, End) in the
// new text that differs from the old text.
type ByteRange struct{ Start, End int }

// DiffRanges returns the byte ranges in newText where newText
// differs from oldText, computed by a rune-level LCS.
func DiffRanges(oldText, newText string) []ByteRange
```

LCS over runes; coalesces adjacent change runs into one range.
Pure, table-driven tests, no third-party dep. `O(n·m)` is fine
for email bodies (typical < 4 KB).

### UI wiring

**`internal/ui/compose/`** owns the action. New `tidy.go`:

- `tidyResultMsg{ res tidy.Result; err error; oldBody string }`
- `tidyCmd(cfg tidy.Config, apiKey, body string) tea.Cmd` — runs
  `tidy.Tidy(...)` and emits `tidyResultMsg`.
- `(*Model) handleTidyKey()` returns the cmd when the body is
  focused, tidy is enabled, key resolves, and no tidy is in
  flight; otherwise returns nil and (when needed) sets `c.err`.
- `(*Model) Update` switch on `tidyResultMsg` routes by `Status`
  and (on `StatusCorrected`) calls `editor.SetBody` plus
  `editor.SetTidyHighlights(newBody, tidy.DiffRanges(oldBody,
  newBody))`.

**`internal/catkin/`** grows a `tidyAnnotator`:

- `Catkin.SetTidyHighlights(src string, ranges []Range)` stores
  the pair on the model.
- `tidyAnnotator{ src string; ranges []Range }` implements
  `Annotator`: returns annotations only when input `src` matches
  the stored `src` exactly. Any buffer mutation invalidates the
  match → no annotations on next tick → highlights gone.
- New `AnnotationKind`: `KindTidyChange`. Style comes from the
  catkin Styles struct: `TidyChange lipgloss.Style` with the
  palette's accent + underline.

**`internal/ui/app.go`** — retire `TidyFn`, `WithTidy`, `tidy
TidyFn`, `identityTidy`, and the `tidy` parameter on
`composeSendCmd`. `composeSendCmd` reverts to a straight
assemble→queue path. Send is unchanged.

### Data flow

```
Ctrl+T (body focus)
  → compose.handleTidyKey
    → tidyCmd  (tea.Cmd)
       → tidy.Tidy   (HTTP, 30s timeout)
       → tidyResultMsg

tidyResultMsg{ Corrected, oldBody, res.Text }
  → editor.SetBody(res.Text)
  → editor.SetTidyHighlights(res.Text, DiffRanges(oldBody, res.Text))
  → toast "Tidy: <N> corrections"

next keystroke that mutates body
  → src != tidyAnnotator.src
  → returns []Annotation{}
  → highlights cleared on next debounced tick
```

## Error handling

- `StatusError` from package → `c.err = "Tidy: <message>"`. Body
  unchanged.
- Network failure → wrapped by the package, surfaces via
  `StatusError`.
- Malformed config (bad enum) → caught at `LoadUI`, shown via the
  existing first-run / config error path.
- Tidy in flight when user presses `Ctrl+T` again → ignored
  (single-flight); the spinner is the affordance.
- User presses `Ctrl+C` (cancel compose) while tidy is in flight
  → standard cancel flow runs; `tidyResultMsg` arrives at a closed
  compose and is dropped by App's nil-check on `m.compose`.

## Theming

Catkin gains one new style slot, `Styles.TidyChange`. Default
styling: palette `AccentPrimary` foreground + underline. The
semantic-map doc (`docs/poplar/styling.md`) gets a row binding
`TidyChange` → `AccentPrimary`. All 15 themes pick up the slot
through their existing `theme.CompiledTheme` fan-out.

## Footer + chrome

- New compose footer hint when `[ui.tidy] enabled = true` and
  body is focused: `^T tidy`. Drop rank ~6 (mid-priority — drops
  before `^X send` and `^C cancel`, after content-edit hints).
- New help-popover row in the compose section: `Ctrl+T` →
  "Run Claude Tidy on the body" (wired flag flips on this pass).

## Testing

### Unit

- `internal/tidy/diff_test.go` — table-driven `DiffRanges`:
  empty, no change, single-rune insertion, single-rune deletion,
  contiguous run, multi-line change, UTF-8 / multi-byte runes,
  trailing newline parity.
- `internal/catkin/annotate_test.go` — `tidyAnnotator` returns
  ranges only on matching src; returns nil on any modification.
- `internal/ui/compose/tidy_test.go` — `handleTidyKey` cases:
  disabled, no API key, body-not-focused, in-flight reentry,
  StatusCorrected, StatusNoChanges, StatusNoAuthorText,
  StatusError. Uses an injectable `tidyFn func(...) (Result,
  error)` field on `Model`, defaulting to the real
  `tidy.Tidy`.
- `internal/config/ui_test.go` — `[ui.tidy]` decode: defaults,
  full population, bad enums (each of `oxford_comma`,
  `ellipsis`, `time_format`).

### Live

Pass-end live verification through tmux against Fastmail
(`geoff@907.life`):

1. Compose with intentional typos in body
   (e.g. "this  is teh  message").
2. Press `Ctrl+T`. Confirm spinner, then corrected body with
   highlighted character ranges.
3. Press a key (e.g. arrow). Confirm highlights clear.
4. Press catkin's undo. Confirm pre-tidy text returns.
5. Send. Confirm Sent copy contains corrected text.
6. Capture at 80×24 and 120×40.

## Migration / supersession

ADR-0159 introduced `TidyFn` as a function-pointer seam on App
anticipating a send-path interceptor. This pass supersedes that
clause: tidy is not on the send path. The new ADR (next available
number) records the user-invoked design and notes ADR-0159's
TidyFn clause as superseded; ADR-0159's frontmatter gets
`status: superseded by NNNN` only on the relevant clause if the
ADR template supports it, otherwise an inline note in the
Consequences section pointing at the new ADR.

## Open items folded in (not asked)

- **Diff algorithm:** rune-level LCS, hand-rolled in
  `internal/tidy/diff.go`. No third-party dep. ~120 LOC.
- **Highlight clear rule:** any buffer mutation invalidates the
  annotator src match. No timer, no per-span tracking.
- **Config gate:** global `[ui.tidy] enabled` (not per-account).
  Tidy is a UX preference; account-level config is for
  credentials and identity.
- **Single-flight:** repeated `Ctrl+T` while in flight is ignored.

## Risks

- Rune-LCS on a pathological 100 KB body could be slow. Bound:
  cap diff input at 200 KB with a fallback to whole-body
  highlighting if exceeded. Email bodies are not normally that
  large.
- Highlight density on heavy rewrites could be visually noisy.
  Eye-scan only (no nav) is accepted as "look once, then edit;
  highlights clear on first keystroke." If feedback says noise
  swamps signal, granularity tuning is a follow-up pass.
- Catkin annotator runs on a 350 ms idle tick, so highlights
  appear on the same cadence as the spellcheck pipeline. Brief
  delay between the buffer replacement and the underline
  rendering is acceptable.

## ADR plan for the pass

One ADR: "Claude Tidy — user-invoked, in-place, highlighted."
Captures the invocation model, the body-only scope, the
character-level highlight rule, the clear-on-keystroke discipline,
the supersession of ADR-0159's TidyFn clause, and the package /
catkin / compose split. Inline rationale references this spec.
