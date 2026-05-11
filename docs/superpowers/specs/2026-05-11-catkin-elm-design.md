# Catkin Elm conformance — design

Date: 2026-05-11
Passes: 27 + 28 (conservative split)
Companion plan: `docs/superpowers/plans/2026-05-11-catkin-elm.md`
(to be written by writing-plans after this spec is approved)

## Goal

Convert `internal/catkin/` to the Elm all-value path so the editor
flows through Update as a value, not a pointer. The pass closes the
catkin all-value straddle logged from the 2026-05-11 audit. Pass 28
follows immediately to delete the `mailcompose.Editor` wrapper —
which exists only to insulate compose from catkin's pointer shape —
so the finished tree reads as natively-on-the-new-architecture.

## Motivation

Catkin today is half-Elm. `Model` and `Buffer` carry pointer-receiver
mutators (`SetValue`, `SetSize`, `Focus`, `Blur`, `SetStyles`,
`SetTidyHighlights`, `RegisterAnnotator`, `SetUserWordlistPath`).
The contagion source is `bubbles/v2/textarea.Model`, which is
itself pointer-shaped. As a result:

- `compose.Model` cannot hold `catkin.Model` directly without
  losing reassignment safety, so `mailcompose.Editor` interface +
  `CatkinEditor` adapter exist to give compose a stable pointer.
  The Editor interface advertises a future neovim adapter
  (ADR-0033) but is already inconsistent: the wizard's signature
  section imports `catkin.Model` directly.
- The `elm-conventions` rule "mutations only in Update" is
  satisfied in spirit but not enforced by types — any caller with
  a `*catkin.Model` can mutate it from a Cmd closure or a View
  method, and nothing catches that at the boundary.
- `mailcompose.Editor` is a single-impl interface (CatkinEditor)
  preserved for a hypothetical post-v1 neovim adapter — exactly
  the anti-pattern called out in `go-conventions` and CLAUDE.md.

## Architectural decisions

### Catkin all-value path (ADR-0212)

`catkin.Model` and `catkin.Buffer` become value types end-to-end.
The wrapped `textarea.Model` pointer is sealed inside `Buffer` and
never escapes. All runtime mutations route through value-returning
setters; mount-time configuration uses a fluent builder.

**Mount-time builder** — called once before the tea loop sees the
model. Returns `Model` (not pointer):

```go
m := catkin.New().
    WithAnnotator(a).
    WithStyles(s).
    WithUserWordlistPath(p)
```

`New() Model`. Each `With*` returns `Model`.

**Runtime setters** — called from the parent's Update:

| Setter | Signature |
| --- | --- |
| Replace buffer + reseed undo | `WithValue(s string) Model` |
| Set width + height + reflow | `WithSize(w, h int) Model` |
| Set width only | `WithWidth(w int) Model` |
| Focus textarea | `WithFocus() (Model, tea.Cmd)` |
| Blur textarea | `WithBlur() Model` |
| Set tidy highlights | `WithTidyHighlights(src string, ranges []Range) Model` |

**Update** signature unchanged: `Update(msg tea.Msg) (Model, tea.Cmd)`.
The handler keeps every external-event branch it has today (keys,
paste, annotation results); internal `m.foo = bar` mutations now
happen on a local value before return.

**Accessors** stay value-receivers: `Value()`, `Focused()`,
`Mode()`, `View()`, `WordCount()`, `CharCount()`, `Init()`.

`Buffer` mirrors the same shape:

```go
type Buffer struct { ta textarea.Model } // pointer sealed

func (b Buffer) Update(msg tea.Msg) (Buffer, tea.Cmd)
func (b Buffer) Value() string
func (b Buffer) Focused() bool
func (b Buffer) RuneOffset() int

func (b Buffer) WithValue(s string) Buffer
func (b Buffer) WithWidth(w int) Buffer
func (b Buffer) WithHeight(h int) Buffer
func (b Buffer) WithRuneOffset(off int) Buffer
func (b Buffer) WithFocus() (Buffer, tea.Cmd)
func (b Buffer) WithBlur() Buffer
```

The textarea is never returned, never aliased. Buffer is fully
addressable-by-value: any caller that wants to mutate must
reassign.

### Why value setters and not Msgs

The brainstorm considered routing parent→catkin state pushes
through a Msg vocabulary (`SetValueMsg`, `SetSizeMsg`, …). Setters
win on four grounds:

1. **Bubbles convention.** `list.Model`, `textarea.Model`,
   `textinput.Model` are all parent-calls-setters-from-Update.
   Poplar invariants pin idiomatic-bubbletea as the default.
2. **Honest Msg surface.** Catkin's existing Msgs
   (`tea.KeyPressMsg`, `tea.PasteMsg`, `annotateRequestMsg`,
   `annotationsReadyMsg`) describe events the world delivers.
   Adding `SetValueMsg` mixes events-from-the-world with
   RPCs-from-the-parent — a category error.
3. **Paired-mutation correctness.** Tidy returns text + diff
   ranges. With setters, compose applies both in one statement:
   `c.editor = c.editor.WithValue(text).WithTidyHighlights(text, ranges)`.
   With Msgs, two Cmds fire and can interleave with other Update
   ticks. The Elm-purist path introduces a correctness hazard.
4. **Elm invariant still holds.** "Mutations only in Update" does
   not require Msg ceremony — a value-returning setter called
   from `compose.Update` is a mutation in Update. The discipline
   the convention enforces (no mutations from View, no mutations
   from Cmd closures) is delivered equally by both shapes; the
   setter shape delivers it without the ceremony.

### Compose holds catkin directly (ADR-0213)

`mailcompose.Editor` interface and `CatkinEditor` adapter delete
entirely (Pass 28). `compose.Model.editor` becomes
`catkin.Model` (value). Every call site rewrites: `c.editor.SetX(...)`
becomes `c.editor = c.editor.WithX(...)`; `c.editor.Update(msg)`
already returns a value and stays unchanged.

ADR-0033 (neovim editor adapter, post-v1) is not superseded — its
rationale survives. Only the v1-era *implementation strategy* via
the Editor interface drops. ADR-0033 gets a Consequences update
referencing 0213 and noting the adapter shape will be designed
fresh when concrete v1.1 requirements exist (per CLAUDE.md's
"don't design for hypothetical future requirements").

## Pass split

The work cleaves naturally along a stable seam: `CatkinEditor`
absorbs the catkin-internal change in Pass 27 with body-only edits
(its method signatures stay put), which leaves `compose.Model`
untouched until Pass 28. The two passes are independently
shippable; each ends with a green `make check` and tmux capture.

### Pass 27 — Catkin all-value conversion (internal)

Files touched: `internal/catkin/**`, `internal/mailcompose/editor.go`
(bodies only), `internal/ui/wizard/section_signature.go`,
`docs/poplar/decisions/`, `.claude/rules/catkin-invariants.md`.

Tasks:

1. `internal/catkin/buffer.go` — convert `Buffer` to value type;
   add `With*` setters returning `Buffer`; the wrapped
   `textarea.Model` becomes a private value field accessed only
   via methods. `Update`/`Value`/`Focused`/`RuneOffset` value
   receivers.
2. `internal/catkin/catkin.go` — convert `Model` to value type;
   add `New()` returning `Model`; mount-time builders
   `WithAnnotator`, `WithStyles`, `WithUserWordlistPath`;
   runtime setters `WithValue`, `WithSize`, `WithWidth`,
   `WithFocus`, `WithBlur`, `WithTidyHighlights`. The internal
   `m.foo = bar` lines in Update rewrite to operate on a local
   value before return.
3. Catkin test files — mechanical pointer→value conversion.
   Setup paths swap pointer mutators for builder calls;
   `m.SetX(...)` → `m = m.WithX(...)`; assertions unchanged.
4. `internal/mailcompose/editor.go` — `CatkinEditor` method
   bodies rewrite. Example: `func (e *CatkinEditor) SetValue(s
   string) { e.inner.SetValue(s) }` becomes `func (e
   *CatkinEditor) SetValue(s string) { e.inner =
   e.inner.WithValue(s) }`. **Interface signature is unchanged**
   — this is the load-bearing seam that lets compose stay out of
   Pass 27.
5. `internal/ui/wizard/section_signature.go` — wizard imports
   catkin directly (not via Editor). Two call sites convert:
   `ed.SetSize(64, 8)` → `ed = ed.WithSize(64, 8)`;
   `ed.SetValue(...)` → `ed = ed.WithValue(...)`.
6. ADR-0212 — Catkin all-value path. `.claude/rules/catkin-
   invariants.md` gains a one-line fact about Model + Buffer
   being value types with `With*` setters.
7. tmux capture — compose body + wizard signature section at
   120×40. Pass-end checklist (`poplar-pass` skill).

### Pass 28 — Delete Editor wrapper, compose holds catkin directly

Files touched: `internal/mailcompose/editor.go` (deleted),
`internal/ui/compose/**`, `docs/poplar/decisions/`,
`docs/poplar/invariants.md`.

Tasks:

1. Delete `internal/mailcompose/editor.go` entirely.
2. `internal/ui/compose/model.go` — field `editor mailcompose.Editor`
   becomes `editor catkin.Model`. Constructor swap:
   `mailcompose.NewCatkinEditor()` →
   `catkin.New().WithStyles(styles.CatkinStyles())`. Rewrite ~15
   call sites: `c.editor.SetX(...)` → `c.editor =
   c.editor.WithX(...)`. Tidy result handler in
   `internal/ui/compose/tidy.go` uses paired-mutation single
   statement.
3. Compose test files — mechanical conversion mirroring Pass 27
   task 3.
4. ADR-0213 (Compose holds catkin directly); ADR-0033
   Consequences update. `docs/poplar/invariants.md` Compose
   section rewrites to describe compose-holds-catkin-directly
   (drops the Editor seam paragraph).
5. tmux capture — compose body renders identically to Pass 27
   output. Pass-end checklist.

## Invariants delta

`docs/poplar/invariants.md` Compose section currently:

> `internal/mailcompose/` is the UI-free outbound-mail surface:
> the `Editor` seam (CatkinEditor wraps `catkin.Model`; v1.1 will
> add a neovim adapter), the `Draft` value type, …

Rewrites to:

> `internal/mailcompose/` is the UI-free outbound-mail surface:
> the `Draft` value type, pure `AssembleMIME(d, now)` …,
> and `SeedReply`/`SeedReplyAll`/`SeedForward` parsing parent
> Message-Id … . `compose.Model` (`internal/ui/compose/`) embeds
> `catkin.Model` directly as the body editor. The v1.1 neovim
> adapter (ADR-0033) will land its own adapter shape post-1.0
> when concrete requirements exist.

`.claude/rules/catkin-invariants.md` gains one line in the
top Catkin paragraph:

> Model and Buffer are value types; runtime mutations are value-
> returning `With*` setters called from the parent's Update;
> mount-time uses a `New().With*` builder. The wrapped
> `textarea.Model` pointer is sealed inside Buffer and never
> escapes the package.

## Risks and mitigations

**Risk: textarea cursor semantics drift under value copies.**
`textarea.Model` carries internal state (selection anchor, line
viewport, history). Copying it by value risks aliasing the
internal slices it holds.

*Mitigation:* `Buffer.WithValue` and friends never copy the
textarea — they call a pointer mutator on a local addressable
copy, then return that copy. The textarea inside is only ever
addressable for the lifetime of one method call. Existing tests
(`buffer_test.go`'s cursor cases) catch any aliasing regression.

**Risk: Pass 27 leaves `CatkinEditor` with an awkward double-
indirect shape during the window between 27 and 28.**

*Mitigation:* That window is one commit. Pass 28 lands
immediately after; STATUS.md sequences them adjacent. The shim
isn't user-visible.

**Risk: hidden pointer-receiver call from a Cmd closure breaks
silently when converted.**

*Mitigation:* The Go compiler flags every reassignment-required
call site at build time. `make check` is the gate; the tasks
list is complete only when the build is green.

## Out of scope

- The `app.go` decomposition (Pass 29) is not touched. Compose's
  Update routing into the editor stays where it is.
- The post-1.0 neovim adapter (ADR-0033) is not designed here.
- Buffer's internal scroll / cursor algorithms (handled by
  textarea upstream) are not revisited.
- The catkin annotation pipeline (spellcheck, tidy) is not
  restructured — only the surface it exposes to consumers.

## Definition of done

Both passes ship when:

- `make check` green
- Catkin's existing test suite passes unchanged in intent
- `compose.Model.editor` is a `catkin.Model` value field (Pass 28)
- `internal/mailcompose/editor.go` does not exist on master (Pass 28)
- ADR-0212 + ADR-0213 written; ADR-0033 Consequences updated
- `docs/poplar/invariants.md` + `.claude/rules/catkin-invariants.md`
  reflect the new shape
- tmux capture at 120×40 shows compose body + wizard signature
  section rendering identically to pre-pass behavior
