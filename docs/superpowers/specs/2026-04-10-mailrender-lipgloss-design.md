# Mailrender Lipgloss Migration

Replace the glamour-based rendering pipeline and TOML theme system
with a lipgloss-native block model. Makes mailrender a first-class
bubbletea citizen: reflow on resize, lipgloss-consistent styling,
and compiled themes. The same rendering code serves both the aerc
CLI filter and the poplar viewer.

## Goals

- Replace glamour with a structured block model rendered by lipgloss
- Replace TOML theme files with compiled Go theme structs
- Eliminate runtime theme file loading, ANSI escape construction,
  and external subprocess calls (wrap, colorize)
- Make email body reflow width-aware (viewer decides width, not the
  pipeline)
- Separate the content cleanup pipeline from rendering so cleanup
  rules can evolve independently
- Add a `mailrender preview` subcommand and `/fix-render` skill for
  fast cleanup rule iteration

## Dependencies Removed

| Library / Binary | Was used for |
|---|---|
| `github.com/charmbracelet/glamour` | Markdown to styled ANSI rendering |
| `github.com/charmbracelet/glamour/ansi` | Custom style config |
| `wrap` (external binary) | Plain text word wrapping |
| `colorize` (external binary) | Plain text syntax coloring |

## Dependencies Added

None. `lipgloss` is already an indirect dependency via glamour.
Promoting it to direct and removing glamour is a net reduction.

## Architecture

### Three-layer split

```
Layer 1: Content Pipeline (internal/filter/)
    Raw HTML or plain text --> clean normalized markdown
    No rendering, no styles, no width concerns

Layer 2: Block Model (internal/content/)
    Clean markdown --> []Block with email-aware types
    Also: ParsedHeaders, rendering via lipgloss

Layer 3: Consumers
    CLI filter: CleanHTML -> ParseBlocks -> RenderBody -> stdout
    Poplar viewer: same functions, called in View() with current width
```

### Content pipeline (internal/filter/)

The existing battle-tested cleanup stages stay. The pipeline
produces clean markdown and nothing else.

**Retained stages:**

| Function | Purpose |
|---|---|
| `prepareHTML` | Strip Mozilla attrs, hidden divs, tracking images |
| `convertHTML` | html-to-markdown with layout table + image strip plugins |
| `normalizeWhitespace` | NBSP cleanup, blank line collapse |
| `deduplicateBlocks` | Remove duplicate text blocks |
| `stripEmptyLinks` | Remove `[](url)` fragments |
| `unflattenQuotes` | Reconstruct blockquotes from inline patterns |
| `detectHTML` | Plain text HTML detection (first 50 lines) |

**Deleted stages** (rendering decisions that move to the block
renderer):

| Function | Replacement |
|---|---|
| `collapseShortBlocks` | Block renderer layout decisions |
| `compactLineRuns` | Block renderer layout decisions |
| `reflowMarkdown` + DP algorithm | `lipgloss.Style.Width()` |

**Relocated functions:**

| Function | New home | Purpose |
|---|---|---|
| `ToHTML` | `internal/content/` | Markdown to HTML for compose multipart send |
| `Markdown` | `internal/content/` | Clean markdown output for reply templates |

Both are rendering concerns -- they transform the block model
or clean markdown into a different output format.

**New exported API:**

```go
func CleanHTML(html string) string    // full cleanup pipeline
func CleanPlain(text string) string   // HTML detection + light normalization
```

### Named rule pipeline

Regex-based cleanup rules become named `Rule` structs for
independent testability and easy addition of new rules:

```go
type Rule struct {
    Name    string
    Phase   Phase  // PreConvert (HTML) or PostConvert (markdown)
    Apply   func(string) string
}
```

Two phases:
- **PreConvert** -- operates on raw HTML before markdown conversion
- **PostConvert** -- operates on markdown after conversion

Complex rules (`stripHiddenElements`, `unflattenQuotes`) stay as
built-in functions at fixed pipeline positions. Regex-based rules
(Mozilla class stripping, tracking image removal, empty link
stripping) become named `Rule` values.

Adding a new sender cleanup: write a function, add a `Rule` to
the list, write a table-driven test. Each rule is independently
testable -- takes a string, returns a string.

### Block model (internal/content/)

**Block types** -- email-aware semantic units, implemented as an
interface with a `blockType()` marker method:

| Type | Key fields |
|---|---|
| `Paragraph` | `Spans []Span` |
| `Heading` | `Spans []Span`, `Level int` |
| `Blockquote` | `Blocks []Block`, `Level int` |
| `QuoteAttribution` | `Spans []Span` |
| `Signature` | `Lines [][]Span` |
| `Rule` | (none) |
| `CodeBlock` | `Text string`, `Lang string` |
| `Table` | `Headers [][]Span`, `Rows [][][]Span` |
| `ListItem` | `Spans []Span`, `Ordered bool`, `Index int` |

**Span types** -- inline styled segments, also an interface:

| Type | Key fields |
|---|---|
| `Text` | `Content string` |
| `Bold` | `Content string` |
| `Italic` | `Content string` |
| `Code` | `Content string` |
| `Link` | `Text string`, `URL string` |

**ParsedHeaders** -- separate struct, not a block type:

| Field | Type |
|---|---|
| `From` | `[]Address` |
| `To` | `[]Address` |
| `Cc` | `[]Address` |
| `Bcc` | `[]Address` |
| `Date` | `string` |
| `Subject` | `string` |

Headers and body are separate types because they have different
width rules (headers fill available width, body caps at
min(78, terminal width)) and different rendering logic (headers
do address wrapping, body does paragraph reflow).

### Parsing

Two entry points, one block parser:

```go
func ParseHeaders(raw string) ParsedHeaders
func ParseBlocks(markdown string) []Block
```

`ParseBlocks` handles:
- Paragraph detection (blank-line separated text)
- Heading detection (`#` prefix)
- Blockquote detection (`>` prefix, with nesting level)
- Quote attribution detection ("On ... wrote:" pattern)
- Signature detection (`-- ` marker, everything after)
- Code fence detection
- Table detection
- List item detection
- Inline span parsing within each block

HTML and plain text share the same block parser. The HTML path
runs `CleanHTML` first; the plain text path runs `CleanPlain`
(which routes HTML-in-plain-text through the HTML pipeline).
Plain text is trivially valid markdown, so both paths produce
markdown that feeds into `ParseBlocks`.

### Rendering

```go
func RenderHeaders(h ParsedHeaders, t *theme.Theme, width int) string
func RenderBody(blocks []Block, t *theme.Theme, width int) string
```

Both return styled strings built with lipgloss. `RenderHeaders`
wraps addresses at `width`. `RenderBody` caps content at
`min(78, width)`.

**CLI filter path** (cmd/mailrender/):

```go
md := filter.CleanHTML(input)
blocks := content.ParseBlocks(md)
fmt.Fprint(os.Stdout, content.RenderBody(blocks, theme.Nord, cols))
```

**Poplar viewer path** (internal/ui/viewer.go):

```go
func (m ViewerModel) View() string {
    hdr := content.RenderHeaders(m.headers, m.theme, m.width)
    body := content.RenderBody(m.blocks, m.theme, m.width)
    return lipgloss.JoinVertical(lipgloss.Left, hdr, body)
}
```

On terminal resize, `View()` is called again with the new width.
Reflow is free -- no re-parsing, no re-fetching. The blocks are
the model; the styled string is always derived.

### Theme package (internal/theme/)

Themes are compiled Go values, not runtime-loaded TOML files.

**Theme struct:**

```go
type Theme struct {
    // Palette -- 16 semantic colors
    BgBase, BgElevated, BgSelection, BgBorder     lipgloss.Color
    FgBase, FgBright, FgBrightest, FgDim           lipgloss.Color
    AccentPrimary, AccentSecondary, AccentTertiary  lipgloss.Color
    ColorError, ColorWarning, ColorSuccess          lipgloss.Color
    ColorInfo, ColorSpecial                         lipgloss.Color

    // Composed styles -- built from palette by NewTheme()
    HeaderKey      lipgloss.Style
    HeaderValue    lipgloss.Style
    HeaderDim      lipgloss.Style
    Paragraph      lipgloss.Style
    Heading        lipgloss.Style
    Quote          lipgloss.Style
    DeepQuote      lipgloss.Style
    Attribution    lipgloss.Style
    Signature      lipgloss.Style
    Bold           lipgloss.Style
    Italic         lipgloss.Style
    Link           lipgloss.Style
    Code           lipgloss.Style
    Rule           lipgloss.Style
}
```

**Constructor:**

```go
func NewTheme(p Palette) *Theme
```

Takes 16 hex values, builds all composed styles. One place to
define colors, all styles derived.

**Three compiled themes:**

```go
var Nord = NewTheme(Palette{...})
var SolarizedDark = NewTheme(Palette{...})
var GruvboxDark = NewTheme(Palette{...})
```

**Styleset generator stays** as `GenerateStyleset(t *Theme) string`.
Becomes a `make stylesets` target rather than a runtime command.

**Deleted from internal/theme/:**

| What | Replacement |
|---|---|
| `Load()`, `FindPath()`, `FindConfigDir()` | Compiled theme values |
| `ANSI()`, `Raw()`, `Reset()` | `lipgloss.Style.Render()` |
| `GlamourStyle()` | Block renderer with lipgloss |
| TOML parsing | Go struct literals |
| Token resolution | Direct lipgloss.Style fields |

**Long-term:** themed binaries (`poplar-nord`, `poplar-solarized`)
or a single binary with a flag/build-tag selecting the compiled
theme. No config files, no parsers.

## Preview Subcommand

```
mailrender preview corpus/broken-email.html
mailrender preview corpus/broken-email.html --theme solarized-dark
mailrender preview corpus/broken-email.html --width 60
```

Takes a file argument, renders the full pipeline to stdout.
Defaults to Nord theme and 78 columns. Same code path as
`mailrender html` but without the stdin dance.

## /fix-render Skill

Claude-driven iteration loop for developing cleanup rules:

1. User saves a broken email to corpus or points at existing file
2. User says `/fix-render corpus/whatever.html`
3. Claude reads the raw HTML, runs the pipeline via `go test`,
   shows the rendered output
4. User describes what's wrong
5. Claude writes a rule + test case, re-runs, shows the diff
6. Iterate until clean
7. Claude runs `make check`
8. Claude runs `/simplify` on changed files
9. Claude updates docs if the rule addresses a new sender pattern
10. Claude commits the rule + test
11. Claude runs `make install`

The skill uses `go test` as the execution engine -- each iteration
compiles only the package under test, sub-second feedback. No
manual recompile during the iteration loop. Full `make install`
only at the end.

## Contributing: Improving Email Rendering

The `/fix-render` workflow and `mailrender preview` subcommand are
documented in the project's contributing docs so that anyone
(including future Claude sessions) can follow the same process to
improve mailrender and poplar's email rendering.

The contributing docs cover:

### Manual workflow (without Claude)

1. Save a broken email to `corpus/` using `aerc-save-email`
2. Run `mailrender preview corpus/<file>.html` to see current output
3. Add a `Rule` to the pipeline in `internal/filter/`
4. Write a table-driven test for the rule
5. Run `go test ./internal/filter/ -run TestRuleName` to iterate
6. Run `make check` to verify no regressions
7. Run `make install` to deploy locally

### Claude-assisted workflow

1. Save the broken email to `corpus/`
2. Run `/fix-render corpus/<file>.html`
3. Describe the rendering problem
4. Claude iterates: writes rule, tests, shows output, repeats
5. Claude runs quality gates, updates docs, commits, installs

### Adding a new cleanup rule

A cleanup rule is a named function with a phase:

- **PreConvert rules** operate on raw HTML before markdown
  conversion. Use for: stripping sender-specific HTML attributes,
  removing tracking elements, cleaning up broken markup.
- **PostConvert rules** operate on markdown after conversion. Use
  for: removing empty links, deduplicating content blocks,
  fixing broken quote structure.

Each rule is independently testable: it takes a string and returns
a string. Corpus emails serve as test fixtures.

### Problem sender patterns

The docs maintain a list of sender types that stress the pipeline
(marketing emails, GitHub notifications, Google Calendar invites,
etc.) with example corpus files. When a new pattern is fixed, the
sender type is added to this list for regression awareness.

## Migration Path

Incremental -- the aerc filters keep working throughout.

**Phase 1: Block model and parser.** Build `internal/content/`
with block types, span types, `ParsedHeaders`, `ParseBlocks()`.
Test against corpus emails. No rendering yet.

**Phase 2: Theme refactor.** Replace `internal/theme/` with
lipgloss `Theme` struct and three compiled themes. Keep
`GenerateStyleset()` working. Delete TOML-loading API.

**Phase 3: Lipgloss renderer.** Add `RenderHeaders()` and
`RenderBody()` to `internal/content/`. Test against golden files
for visual equivalence with current glamour output.

**Phase 4: Wire up CLI filters.** Replace current `filter.HTML()`,
`filter.Plain()`, `filter.Headers()` with the new pipeline.
Update `cmd/mailrender/` to call `CleanHTML` -> `ParseBlocks` ->
`RenderBody`. Update e2e golden tests. Add `mailrender preview`.

**Phase 5: Cleanup.** Delete glamour dependency, old TOML files,
`ColorSet`, dead code. Build `/fix-render` skill. Update
`aerc-setup.md` and `CLAUDE.md`.

Each phase produces a working binary. No big-bang switchover.

## What This Enables for Poplar

- **Resize reflow** -- `View()` renders at current width on every
  frame, no re-parsing needed
- **Lipgloss consistency** -- email body uses the same style system
  as the poplar chrome
- **Compiled themes** -- one `Theme` struct controls the entire
  visual experience (UI chrome + rendered content)
- **Semantic rendering** -- the viewer knows what a signature,
  quote attribution, or heading is and can style them distinctly
- **Future interactivity** -- block-level awareness enables
  collapsible quotes, dimmed signatures, link navigation
