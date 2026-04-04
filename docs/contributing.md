# Contributing

## Project layout

```
beautiful-aerc/
  cmd/beautiful-aerc/        CLI entry point (cobra wiring only)
    main.go                  calls newRootCmd().Execute()
    root.go                  cobra root command, adds subcommands
    headers.go               headers subcommand: flags -> filter.Headers()
    html.go                  html subcommand: flags -> filter.HTML()
    plain.go                 plain subcommand: flags -> filter.Plain()
  internal/palette/
    palette.go               load palette.sh, expose ANSI escape sequences
  internal/filter/
    headers.go               header parsing, reordering, colorization
    html.go                  pandoc invocation, cleanup, highlighting, links
    plain.go                 HTML detection, routing to html or wrap|colorize
  e2e/
    e2e_test.go              builds binary in TestMain, pipes fixtures
    testdata/                HTML email fixtures
      golden/                expected output files
  .config/aerc/
    themes/                  theme source files + generator script
    generated/               palette.sh output (gitignored for real installs)
    stylesets/               aerc styleset output
    filters/                 unwrap-tables.lua pandoc filter
```

The boundary between `cmd/` and `internal/` is strict: `cmd/` contains only cobra wiring and flag parsing. All logic lives in `internal/`. This makes the filter functions directly testable without subprocess invocation.

## How aerc calls the binary

aerc invokes filter commands as shell commands. The protocol is simple:

- Email content arrives on **stdin**
- Styled ANSI text goes to **stdout**
- `AERC_COLUMNS` environment variable carries the terminal width as a string
- Non-zero exit causes aerc to show an error

The binary reads `AERC_COLUMNS` in each subcommand and falls back to 80 if not set or not parseable.

For the `.headers` filter, aerc sends RFC 2822 headers (key: value, with continuation lines for folded headers). The blank line that separates headers from body is included in stdin.

For `text/html`, aerc sends the raw HTML body. For `text/plain`, it sends the raw plain text body.

## Build and test

```sh
make build    # build ./beautiful-aerc binary
make vet      # go vet ./...
make test     # go test ./...
make check    # vet + test (required before commits)
make install  # go install ./cmd/beautiful-aerc
make clean    # remove ./beautiful-aerc binary
```

`make check` is the gate. Both `vet` and `test` must pass before committing.

## Architecture: how data flows

**Headers filter:**

1. `parseHeaders(r)` reads RFC 2822 headers into a map, handling CRLF and folded continuation lines
2. Load palette from `palette.FindPath()`
3. Render headers in fixed order (From, To, Cc, Date, Subject), wrapping long address lines at `AERC_COLUMNS`
4. Print separator line
5. Write to stdout

**HTML filter:**

1. Load palette
2. Strip pre-pandoc HTML artifacts (moz attributes)
3. Call pandoc as subprocess: `pandoc -f html -t markdown --wrap=none -L unwrap-tables.lua`
4. Run post-pandoc cleanup (compiled regexes, applied in order)
5. Walk lines, applying markdown syntax highlighting
6. Style links according to mode (markdown or clean)
7. Write to stdout

**Plain filter:**

1. Read all of stdin
2. Check first 50 lines for HTML tags
3. If HTML: delegate to `filter.HTML()` with the same writer and palette
4. If plain: exec `wrap | colorize` via shell pipeline

## Adding a new filter stage

Filter stages in the HTML pipeline are sequential transformations on the markdown string. To add a new cleanup step:

1. Add a compiled `regexp.MustCompile(...)` at package level in `html.go` (alongside the existing package-level vars)
2. Apply it in the post-pandoc cleanup section of `HTML()`, in the right position relative to other cleanups
3. Add a table-driven test in `html_test.go` covering the new regex

Keep regexes package-level. Compiling inside a function is wasteful since the filter runs for every message.

Example: adding a stage that strips `{.class}` attribute syntax pandoc sometimes emits:

```go
// package-level
reClassAttr = regexp.MustCompile(`\{\.[\w-]+\}`)

// in HTML(), post-pandoc cleanup section
md = reClassAttr.ReplaceAllString(md, "")
```

## Adding a new theme

1. Copy an existing theme file as a starting point:

```sh
cp .config/aerc/themes/nord.sh .config/aerc/themes/my-theme.sh
```

2. Edit the 16 hex color slots. Do not rename the slot variables - they are referenced by name throughout the generator and styleset template.

3. Adjust the markdown tokens (`C_HEADING`, `C_LINK_TEXT`, etc.) to reference the appropriate slots.

4. Test the generator:

```sh
cd .config/aerc
themes/generate themes/my-theme.sh
```

5. Verify `generated/palette.sh` has the expected colors and `stylesets/my-theme` looks correct.

6. Launch aerc (or use tmux capture) to verify the visual result. See the global CLAUDE.md for the tmux testing pattern.

## Code conventions

This project follows the conventions in `~/.claude/docs/go-conventions.md`. Key rules that apply here:

- No unnecessary interfaces. `palette.Palette` is a concrete struct.
- No goroutines. Filters run sequentially - pandoc is called with `exec.Command`, not async.
- Cobra with `SilenceUsage: true` on the root command.
- Flags in a struct local to each `cmd/` file.
- `fmt.Errorf("context: %w", err)` at every error boundary.
- Table-driven tests with `t.Run()`. No assertion libraries.
- Compiled regexes at package level, not inside functions.

The project `CLAUDE.md` at the repo root includes the full mandatory conventions reference. Claude Code reads it automatically when working in this repo.

## E2E tests

The e2e tests build the binary once in `TestMain`, then pipe each fixture through a subcommand and compare output against golden files.

**Fixture categories in `e2e/testdata/`:**

| File | Category |
|------|----------|
| `marketing.html` | Zero-width preheader characters, layout tables, tracking URLs |
| `transactional.html` | Password resets, security alerts |
| `developer.html` | GitHub notifications, nested links, code blocks |
| `simple.html` | Plain conversation, quoted threads |
| `edge-links.html` | Empty link text, image-only links, multi-line links |

Golden files are in `e2e/testdata/golden/`. The filename convention is `<fixture>.<subcommand>.golden`.

**Running e2e tests:**

```sh
go test ./e2e/...
```

**Updating golden files** after an intentional rendering change:

```sh
go test ./e2e/... -update-golden
```

Review the diff carefully before committing updated golden files - they are the ground truth for expected output.

**Adding a new fixture:**

1. Save the raw HTML as `e2e/testdata/<name>.html`
2. Run with `-update-golden` to generate the initial golden file
3. Review the output to confirm it renders correctly
4. Commit both the fixture and the golden file

**Adding a new subcommand to e2e coverage:**

Add a test case in `e2e_test.go` following the existing table-driven pattern. The test builds a command, pipes the fixture to stdin, captures stdout, and compares against the golden file.
