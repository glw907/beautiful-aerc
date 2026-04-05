# beautiful-aerc

Themeable aerc email filters and configuration, distributed as a
single GNU Stow package.

## MANDATORY: Go Conventions

**Read and follow `~/.claude/docs/go-conventions.md` before writing
ANY Go code.** Every Go file, function, test, and error message must
conform. Key rules:

- No unnecessary interfaces, goroutines, builder patterns
- `cmd/` for CLI wiring only, `internal/` for business logic
- cobra with `SilenceUsage: true`, flags in a struct
- `fmt.Errorf("context: %w", err)` at every error boundary
- Table-driven tests, no assertion libraries
- `make check` (vet + test) must pass before any commit

## MANDATORY: Go Skill

**Use superpowers:go skill for all Go development tasks.**

## Project Structure

```
cmd/beautiful-aerc/    CLI wiring (cobra root + subcommands)
internal/palette/      Parse generated/palette.sh, expose color tokens
internal/filter/       Filter implementations (headers, html, plain) + footnote rendering
internal/picker/       Link picker UI (pick-link subcommand)
e2e/                   End-to-end tests (build binary, pipe fixtures)
e2e/testdata/          HTML email fixtures + golden output files
.config/aerc/          aerc configuration files
.config/aerc/themes/   Theme source files + generator script
.config/aerc/generated/ Generated palette.sh (produced by generator)
.config/aerc/stylesets/ Generated aerc stylesets
.config/aerc/filters/  pandoc Lua filter (unwrap-tables.lua)
.config/nvim-mail/     Neovim compose editor profile
.config/kitty/         kitty terminal profile for mail
.local/bin/            Launcher scripts (mail, nvim-mail)
```

## aerc Filter Protocol

aerc calls filters as shell commands. Each filter:
- Receives email content on **stdin**
- Writes ANSI-styled text to **stdout**
- Has access to `AERC_COLUMNS` env var (terminal width)
- `.headers` filter receives RFC 2822 headers (key: value, folded)
- `text/html` filter receives raw HTML body
- `text/plain` filter receives raw plain text body

## Theme System

Theme files (`.config/aerc/themes/*.sh`) define 16 semantic hex color
slots + markdown tokens. The generator (`themes/generate`) reads a
theme file and produces `generated/palette.sh` (ANSI tokens for the
Go binary) and `stylesets/<name>` (aerc UI colors).

The Go binary reads `palette.sh` at runtime for all color tokens.
It finds palette.sh by checking: `$AERC_CONFIG/generated/palette.sh`,
then relative to binary, then `~/.config/aerc/generated/palette.sh`.
If not found, it exits with a clear error.

## Link Picker

The `pick-link` subcommand provides an interactive URL picker for
the message viewer. It is invoked via `:pipe` so aerc feeds the raw
message on stdin.

**Pipeline:** raw HTML → `filter.HTML` (same filter the viewer uses)
→ extract URLs from footnotes and plain text → interactive picker
→ `xdg-open` selected URL.

**Keybinding** (in `[view]` section of `binds.conf`):
- `Tab` — open the link picker (`pick-link`)
- `Ctrl-l` — manually type a URL to open (`open-link`)

**Picker controls:**
- `1`-`9`, `0` — instant-select link by number (0 = 10th)
- `j`/`k` or arrows — move selection
- `Enter` — open selected link
- `q` or `Escape` — cancel

**Key design decisions:**
- Reads keyboard from `/dev/tty` (not stdin) since stdin is the
  piped message content.
- Runs the HTML filter internally to extract clean footnoted URLs
  rather than parsing raw HTML (avoids DTD, image, and tracking URLs).
- Opens URLs directly via `xdg-open` since `:pipe` cannot feed
  output back to aerc's `:open-link`.

## Footnote URLs

Long URLs in the footnote reference section are visually truncated
with `…` to fit within the terminal width. The full URL is embedded
in an OSC 8 hyperlink escape sequence so terminals that support it
can still make the truncated text clickable. The link picker extracts
full URLs from OSC 8 hrefs, so truncation does not affect link opening.

## Testing

- **Unit tests:** table-driven, same package, alongside source files
- **E2E tests:** build binary in TestMain, pipe HTML fixtures, compare
  against golden files in `e2e/testdata/golden/`
- **Live verification:** tmux-based aerc testing (see global CLAUDE.md)

## Build

```
make build     # build binary
make test      # run tests
make vet       # go vet
make check     # vet + test (gate before commits)
make install   # install to ~/.local/bin/
```

## Corpus

`corpus/` holds raw email parts (HTML or plain text) flagged for
rendering issues. Save emails from aerc with `b` in the viewer.
The `/fix-corpus` skill batch-processes accumulated corpus emails.

## Filter Testing

See `.claude/docs/tmux-testing.md` for patterns to render emails
through the filter and verify output via tmux.
