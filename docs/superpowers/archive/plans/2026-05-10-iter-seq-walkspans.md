# Pass 16c — `iter.Seq` for `catkin/walkSpans`

## Goal

Convert `internal/catkin/style.go::walkSpans` from a push-callback
shape (`func(s string, fn func(...))`) to a Go 1.23 `iter.Seq` push
iterator. Update all three call sites. Closure-captured loop state
collapses into ordinary local variables with real `break` and
`continue`.

## Why

ADR-0196 binds `iter.Seq` as the default for in-package scanners
that today take an `fn` callback. `walkSpans` is the most-called
example in the codebase: three consumers, two of which carry
sentinel state through a closure. Iterator form lets the consumer
`break` out of the scan when it has what it wants and reads as a
plain `for ... range`.

## Scope

In:

- `internal/catkin/style.go` — rename `walkSpans` → `spans`,
  change signature, adapt `tokenize` (the in-file self-caller).
- `internal/catkin/match.go` — adapt `scanSpans`.
- `internal/catkin/spellcheck.go` — adapt the fence-mask builder.

Out:

- BACKLOG #46 (`messagelist.appendThreadRows` → `iter.Seq2`) — that
  is Pass 17b.
- `log/slog` adoption — Pass 16d.
- Any non-`walkSpans` modernization.

## Design

Yield type (Go has no `iter.Seq3`, so bundle kind + text + submatch
into one value):

```go
type spanYield struct {
    kind spanKind
    text string
    sub  []string // nil except for spanLink (full, linkText, url)
}

// spans walks s as an inline-span scanner. Untouched bytes yield
// one rune at a time as spanText.
func spans(s string) iter.Seq[spanYield] {
    return func(yield func(spanYield) bool) {
        // existing scan loop; each fn(...) becomes:
        //   if !yield(spanYield{kind, text, sub}) { return }
    }
}
```

Callers:

```go
for sp := range spans(s) {
    // sp.kind / sp.text / sp.sub
}
```

`tokenize`'s `flush()` helper stays; just feeds from the range
body. `scanSpans` and the spellcheck fence-mask loop drop their
closures entirely — `pos` and `after` become plain locals.

## Tasks

1. Edit `internal/catkin/style.go`: add `spanYield`, rewrite
   `walkSpans` → `spans` returning `iter.Seq[spanYield]`, update
   `tokenize` to `for sp := range spans(s)`.
2. Edit `internal/catkin/match.go`: rewrite `scanSpans` to use
   `for sp := range spans(line)`.
3. Edit `internal/catkin/spellcheck.go`: rewrite the fence-mask
   builder's inner loop to use `for sp := range spans(l)`.
4. `make check` green; `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh`
   exit 0.
5. Pass-end ritual via `poplar-pass`.

## Verification

- Unit tests in `style_test.go`, `match_test.go`,
  `spellcheck_test.go` already exercise these paths — they must
  pass unmodified (the externally visible behaviour does not
  change).
- `make check` green.

## ADR

No new ADR. ADR-0196 already binds the convention; this is a
mechanical application. Pass 16c summary lands in STATUS.md only.
