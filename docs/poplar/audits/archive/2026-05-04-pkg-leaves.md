# Human-voice audit — leaf packages

**Aggregate tally:** T2=6, T7=1 (15-chorus), T9=5, T10=1 (3-chorus), T13=4, T19=1, T24=1 (5-site), T31=1.

By category: C1=6, C4=2, C6=6, C7=9, C8=1.

---

## internal/theme

**Tally:** C1=2 (T2=2), C4=1 (T7/T31)

### themes.go:265–308 — C4 (T7 / T31)

```go
// OneDark is the compiled One Dark theme (default).
var OneDark = NewCompiledTheme("One Dark", oneDarkPalette)

// Nord is the compiled Nord theme.
var Nord = NewCompiledTheme("Nord", nordPalette)

// SolarizedDark is the compiled Solarized Dark theme.
var SolarizedDark = NewCompiledTheme("Solarized Dark", solarizedDarkPalette)
// … 12 more in identical pattern
```

Fifteen exported `var` declarations; every doc is `"X is the compiled Y theme."` Identical SVO shape, identical 5–8 word length, across all 15. The most visually obvious AI fingerprint in the entire leaf set.

### themes.go:310 — C1 (T2)

```go
// Themes maps lowercase CLI names to compiled themes.
var Themes = map[string]*CompiledTheme{
```

Restates the type. Map literal below conveys it.

### themes.go:329 — C1 (T2)

```go
// ThemeNames returns the available theme names in alphabetical order.
func ThemeNames() []string {
```

Word-for-word what the name + signature say. The "alphabetical order" note is the only earned content.

---

## internal/term

**Tally:** C1=1 (T2=1), C8=1 (T19=1)

### doc.go — C8 (T19)

A dedicated `doc.go` for five functions across three well-named files. The package doc could fit in the `package` clause of `resolve.go`. The "consumed only by cmd/poplar" detail is architectural commentary belonging in an ADR. Reflexive "every package gets a doc.go" pattern.

### font.go:43–46 — C1 (T2)

```go
// fcListFamilies shells out to fc-list to enumerate font families
// known to fontconfig. Returns (families, true) on success or
// (nil, false) when fc-list is not in PATH or exits non-zero.
// A 2-second context keeps a hung fontconfig from stalling startup.
func fcListFamilies() ([]string, bool) {
```

First two sentences restate the name. Only the timeout note is non-obvious. Trim to: `// shells out to fc-list; a 2 s context prevents a hung fontconfig stalling startup.`

---

## internal/backoff

Zero findings. Single function, tight earned doc, noun-phrase test cases.

---

## internal/filter

**Tally:** C7=3 (T10), C6=1 (T24), C1=1 (T2)

### tohtml.go:18–19 — C7 (T10/T13)

```go
return fmt.Errorf("reading input: %w", err)
return fmt.Errorf("converting markdown: %w", err)
return fmt.Errorf("writing output: %w", err)
```

Three-error chorus with `%w` everywhere. No caller branches on these via `errors.Is`/`errors.As`. `%v` is correct.

### convert.go:70 — C1 (T2)

```go
// Init registers the layout table renderer at PriorityEarly so it intercepts
// <table> nodes before the standard table plugin (PriorityStandard).
func (p *layoutTablePlugin) Init(conv *converter.Converter) error {
```

Doc on unexported method of unexported type. Body shows the priority race in one line.

### html_test.go:110, 137, 178 — C6 (T24)

`t.Errorf("got %q, want %q", got, tt.want)` appears verbatim in `TestStripHiddenElements`, `TestStripEmptyLinks`, `TestDeduplicateBlocks`, `TestCollapseShortBlocks`, `TestUnflattenQuotes` — five different test functions, byte-identical.

---

## internal/content

**Tally:** C1=2 (T2=2), C4=1 (T31), C6=5 (T9=5)

### render_footnote.go:96–105 — C1 (T2)

```go
// markerFor registers url in the picker list (if not already seen) and
// returns its 1-based index. The caller decides whether to flip
// hasMarker[idx-1] to true.
func (w *footnoteWalker) markerFor(url string) int {
```

Caller decision is visible at both call sites 6 lines away. The 1-based note alone justifies an inline comment; the rest restates code.

### render.go:220–236 — C1 (T2)

```go
// metadataPrefixWidth is the display-cell width of the row prefix —
// metadataIndent + headerKeyColWidth + the trailing space — used by
// the wrap accumulator. Computed against raw strings so it stays
// correct after surface-baking wraps the prefix in ANSI.
const metadataPrefixWidth = len(metadataIndent) + headerKeyColWidth + 1
```

Pure derivation; the initializer states the arithmetic the comment paraphrases.

### render_footnote_test.go:178, 210, 237, 276, 308 — C6 (T9)

Five test functions in `render_footnote_test.go` carry multi-paragraph prose docstrings. The numbering-design note duplicates from the production godoc and from sibling tests. The guide is explicit: test function names need no docstring.

---

## internal/tidy

**Tally:** C7=5 (T10/T13=5), C1=1 (T2=1)

### api.go:56–97 — C7 (T10 / T13)

```go
return "", fmt.Errorf("marshal request: %w", err)
return "", fmt.Errorf("build request: %w", err)
return "", fmt.Errorf("call API: %w", err)
return "", fmt.Errorf("read response: %w", err)
return "", fmt.Errorf("decode response: %w", err)
```

Five-error chorus in `CallAPI`. All `%w`; no caller branches on the wrapped error. `%v` is correct throughout.

### tidy.go:84 — C1 (T2)

```go
// countChangedLines returns the number of lines that differ between a and b.
func countChangedLines(a, b string) int {
```

Verbatim restatement of the function name.
