# beautiful-aerc Design Spec

A themeable, distributable aerc email setup with a Go filter binary,
multiple color themes, and optional nvim-mail and kitty integration.

## Goals

- Single `stow beautiful-aerc` installs the full mail stack
- Easy theme switching: change one file, regenerate, done
- Ships with 3 themes (Nord, Solarized Dark, Gruvbox Dark) as equal options
- Users can `git pull` for upstream updates while keeping local overrides
- Minimal runtime dependencies: aerc, pandoc, Go binary

## Repository Structure

```
beautiful-aerc/
  CLAUDE.md                        # project conventions for Claude Code
  README.md
  go.mod                           # go 1.23 (matches aerc)
  go.sum
  Makefile                         # build + install Go binary
  .golangci.yml
  cmd/
    beautiful-aerc/
      main.go
  internal/
    palette/
      palette.go
    filter/
      headers.go
      html.go
      plain.go
      footnotes.go
    picker/
      picker.go
  e2e/
    testdata/                      # HTML email fixtures + golden files
  .config/aerc/
    aerc.conf
    binds.conf
    accounts.conf.example
    themes/
      nord.sh
      solarized-dark.sh
      gruvbox-dark.sh
      generate
    generated/
      palette.sh
    stylesets/
      nord-custom
    filters/
      unwrap-tables.lua
  .config/nvim-mail/
    init.lua
    syntax/aercmail.vim
  .config/kitty/
    kitty-mail.conf
  .local/bin/
    mail
    nvim-mail
  .local/share/applications/
    aerc-mail.desktop
```

## Go Binary

Single binary with three cobra subcommands. Each reads stdin,
writes styled ANSI text to stdout - aerc's filter protocol.

### aerc.conf integration

```ini
.headers=beautiful-aerc headers
text/html=beautiful-aerc html
text/plain=beautiful-aerc plain
```

### Project layout

```
cmd/beautiful-aerc/main.go        # cobra root, SilenceUsage: true
internal/palette/palette.go        # parse palette.sh, expose tokens
internal/filter/headers.go         # reorder, colorize, wrap, separator
internal/filter/html.go            # cleanup, exec pandoc, highlight, links
internal/filter/plain.go           # detect HTML-in-plain, route or wrap
internal/filter/footnotes.go       # convertToFootnotes, styleFootnotes
internal/picker/picker.go          # interactive URL picker (pick-link subcommand)
```

### Subcommands

| Subcommand | Replaces | Calls pandoc? |
|---|---|---|
| `headers` | format-headers (sh) + format-headers.awk | No |
| `html` | html-to-text (sh + perl + sed) | Yes, as subprocess |
| `plain` | wrap-plain (sh) | Delegates to `html` when HTML detected, otherwise execs `wrap \| colorize` |
| `pick-link` | (new) | No - reads URLs from stdin, presents interactive picker |

### Link rendering

The `html` subcommand renders links as footnote references. Body text
shows colored link text with a dimmed `[^N]` marker inline; a numbered
reference section at the bottom of the message lists all URLs.

Self-referencing links (where the display text equals the URL) render
as plain URLs with no footnote number.

pandoc is called with `--reference-links` to produce reference-style
markdown output. `convertToFootnotes` in `internal/filter/footnotes.go`
converts it to footnote syntax - handling image labels, emphasis stripping
from link text, and unresolved reference cleanup - before `styleFootnotes`
applies ANSI colors. `styleFootnotes` returns `[]footnoteRef` structs
directly to avoid re-parsing formatted strings.

Example output:

```
If you don't recognize this account, remove[^1] it.

Check activity[^2]

See https://myaccount.google.com/notifications
----------------------------------------
[^1]: https://accounts.google.com/AccountDisavow?adt=...
[^2]: https://accounts.google.com/AccountChooser?Email=...
```

### Link picker

The `pick-link` subcommand (`internal/picker/`) provides keyboard-driven
URL selection. aerc's `:menu` command pipes the current message through
`beautiful-aerc pick-link` and passes the selected URL to `:open-link`.

Keybinding in `binds.conf`:

```ini
[view]
<Tab> = :menu -dc 'beautiful-aerc pick-link' :open-link
```

Navigation: 1-9 instantly select that link, 0 selects the 10th, j/k
or arrows to navigate, Enter to confirm, q/Escape to cancel.

Picker colors are read from palette.sh at runtime to match the active theme.

### What the Go binary absorbs

- All sed/perl regex cleanup (pandoc artifacts, zero-width chars, whitespace)
- Markdown syntax highlighting (headings, bold, italic, rules)
- Link rendering (footnote-style references: colored text, dimmed markers, numbered URL section)
- Interactive link picker (pick-link subcommand with vim-style navigation)
- Header formatting (reorder, colorize, address wrapping, separator)
- Palette loading and hex-to-ANSI conversion

### What stays external

- `pandoc` - HTML-to-markdown conversion, called as subprocess
- `unwrap-tables.lua` - pandoc Lua filter, passed via `-L` flag
- `colorize`, `wrap`, `linkify` - aerc built-ins, used by `plain` subcommand

### Palette loading

The binary finds `palette.sh` by checking in order:
1. `$AERC_CONFIG/generated/palette.sh`
2. Relative to binary: `../../.config/aerc/generated/palette.sh`
3. `~/.config/aerc/generated/palette.sh`
4. Error: "palette not found - run themes/generate to set up your theme"

Parsing is simple: `KEY=value` and `KEY="value"` lines. Ignores
comments and blank lines.

## Theme System

### Theme file format

Each theme file defines 16 semantic color slots (hex) plus markdown
tokens that reference those slots:

```sh
# Semantic color slots
BG_BASE="#2e3440"
BG_ELEVATED="#3b4252"
BG_SELECTION="#394353"
BG_BORDER="#49576b"
FG_BASE="#d8dee9"
FG_BRIGHT="#e5e9f0"
FG_BRIGHTEST="#eceff4"
FG_DIM="#616e88"
ACCENT_PRIMARY="#81a1c1"
ACCENT_SECONDARY="#88c0d0"
ACCENT_TERTIARY="#8fbcbb"
COLOR_ERROR="#bf616a"
COLOR_WARNING="#d08770"
COLOR_SUCCESS="#a3be8c"
COLOR_INFO="#ebcb8b"
COLOR_SPECIAL="#b48ead"

# Markdown tokens (reference slots + style modifiers)
C_HEADING="$COLOR_SUCCESS bold"
C_BOLD="bold"
C_ITALIC="italic"
C_LINK_TEXT="$ACCENT_SECONDARY"
C_LINK_URL="$FG_DIM"
C_RULE="$FG_DIM"
```

### Generator

`themes/generate` is a POSIX shell script. No nvim dependency.

- Reads one theme file as input
- Resolves slot references in markdown tokens
- Converts hex to ANSI escape parameters
- Produces two artifacts:
  - `generated/palette.sh` - hex colors + resolved ANSI tokens
  - `stylesets/<theme-name>` - aerc styleset with hex values

Usage:

```sh
themes/generate themes/nord.sh             # generate Nord theme
themes/generate themes/solarized-dark.sh   # switch to Solarized
```

### Override mechanism

Both generated files include a marker line:
```
# --- overrides below this line are preserved across regeneration ---
```
User customizations below this line survive re-running the generator.

### Shipped themes

- `nord.sh` - cool dark theme (Arctic Ice Studio)
- `solarized-dark.sh` - classic dark theme (Ethan Schoonover)
- `gruvbox-dark.sh` - warm dark theme (morhetz)

## Config Files

### Tracked (users can fork or use as reference)

- `aerc.conf` - working config with comments. Personal hooks removed.
  References `beautiful-aerc` binary for filters, `nvim-mail` for editor.
  Styleset name is set by the user after running the generator.
- `binds.conf` - fully generic keybindings, ships as-is.

### Template

- `accounts.conf.example` - placeholder values, user copies and edits.

### Gitignored

- `accounts.conf` - user's real account config
- Personal credential files

## nvim-mail

Ships as-is with two changes:
- Signature block in init.lua replaced with placeholder
- Colors in `syntax/aercmail.vim` should be updated when changing
  themes (vim syntax files don't support variable colors)

## kitty-mail

Ships as-is. Terminal color block (color0-color15) should be updated
to match the chosen theme. kitty and nvim have established theme
ecosystems -- beautiful-aerc covers the aerc layer.

## Launcher scripts

- `mail` - `exec kitty --class aerc-mail --config ... aerc`
- `nvim-mail` - `NVIM_APPNAME=nvim-mail exec nvim "$@"`

Both generic, ship as-is.

## What gets deleted from current setup

- `filters/format-headers` (shell wrapper) - absorbed into Go
- `filters/format-headers.awk` - absorbed into Go
- `filters/html-to-text` (shell/perl pipeline) - absorbed into Go
- `filters/wrap-plain` (shell) - absorbed into Go
- `filters/palette.sh` - moves to `generated/palette.sh`
- `filters/generate-palette` - becomes `themes/generate`
- `mailrules.json` - personal, not shipped
- `fastmail-*.age` - personal, not shipped
- `fastmail-password`, `fastmail-dav-password` - personal, not shipped

## Testing

### Unit tests

Table-driven tests alongside each Go source file. Cover palette
parsing, header formatting, whitespace cleanup, link styling,
markdown highlighting.

### E2E test fixtures

Real-world email HTML saved as fixtures in `e2e/testdata/`. The Go
binary is built once and each test pipes a fixture through a
subcommand, comparing output against golden files. Fixture categories:

- **Marketing spam** - zero-width preheader characters, layout tables,
  tracking URLs (e.g., Reebelo newsletters)
- **Transactional** - Google security alerts, password resets
- **Developer** - GitHub notification emails with nested links,
  code blocks, diff content
- **Plain conversation** - simple text replies, quoted threads
- **Edge cases** - empty link text, image-only links, deeply nested
  tables, HTML-in-plain-text MIME parts

Golden files capture expected rendered output for each fixture.
`--update-golden` flag regenerates them.

### Live verification

After the Go binary is integrated, verify rendering in aerc via tmux
capture on a selection of real messages across the fixture categories.

## Runtime dependencies

- `aerc` (email client)
- `pandoc` (HTML-to-markdown, called by Go binary)
- `beautiful-aerc` (Go binary, built from this repo)

## Optional dependencies

- `kitty` (terminal, for `mail` launcher)
- `nvim` (editor, for compose via nvim-mail)
- `khard` (address book completion)

## Build dependencies

- Go 1.23+ (matches aerc's go.mod)
- `make`

## Documentation

Four docs, each with a clear audience:

### README.md (repo root)

Audience: anyone who finds the repo. Covers:
- What beautiful-aerc is (one paragraph + screenshot)
- What it looks like (2-3 screenshots showing themes)
- Prerequisites (aerc, pandoc, Go)
- Install steps (clone, build, generate theme, stow, configure account)
- Quick usage overview
- Links to other docs

### docs/themes.md

Audience: users who want to customize or create themes. Covers:
- The 16 semantic color slots and what each controls
- Markdown token format (slot references + style modifiers)
- How to create a custom theme file
- Running the generator and what it produces
- Override mechanism (editing below the marker line)
- Updating kitty and nvim-mail colors to match

### docs/filters.md

Audience: users who want to understand how email renders. Covers:
- The three subcommands and what each does
- HTML pipeline stages (pandoc conversion, artifact cleanup,
  markdown highlighting, footnote link rendering)
- Footnote-style link rendering and the pick-link subcommand
- Header formatting (reorder, colorize, address wrapping, separator)
- Plain text handling (HTML detection, reflow, colorize)
- How palette.sh tokens map to visual output
- Troubleshooting (missing colors, broken rendering)

### docs/contributing.md

Audience: developers who want to improve or extend the project. Covers:
- Go project layout (cmd/, internal/, why each package exists)
- How to build and test (`make check`)
- Architecture: how aerc calls the binary, stdin/stdout protocol
- How to add a new filter stage
- How to add a new theme
- Code conventions (matches the project's Go style)

## Project CLAUDE.md

The repo includes a CLAUDE.md so that Claude Code has full project
context when working on the codebase. Contents:

- **Go conventions** - `MANDATORY: Read and follow
  ~/.claude/docs/go-conventions.md before writing ANY Go code.`
  This is the most important line. It pulls in the full conventions
  (no unnecessary interfaces/goroutines, cobra with SilenceUsage,
  table-driven tests, error wrapping, atomic writes, etc.)
- **Superpowers Go skill** - `MANDATORY: Use superpowers:go skill
  for all Go development tasks.` Ensures the Go-specific skill is
  invoked for implementation work.
- **Project overview** - what beautiful-aerc is, the aerc filter
  protocol (stdin/stdout, ANSI escape codes, AERC_COLUMNS env var)
- **Project structure** - what lives in cmd/, internal/, .config/,
  themes/, generated/, e2e/
- **Theme system** - how themes work, the generator, palette.sh format
- **Testing** - unit tests (table-driven), e2e golden file tests,
  tmux live verification for visual rendering
- **Build** - `make build`, `make check`, `make install`
- **Dependencies** - pandoc (runtime), aerc built-ins (colorize, wrap)

## Consumer setup (dotfiles integration)

The project is developed at `~/Projects/beautiful-aerc/`. To consume
it from the dotfiles repo:

- `~/.dotfiles/beautiful-aerc` is a symlink to
  `~/Projects/beautiful-aerc/`
- `cd ~/.dotfiles && stow beautiful-aerc` installs everything
- The old stow packages (`aerc`, `nvim-mail`, mail-related files in
  `kitty` and `bin`) are retired once beautiful-aerc is working
- Development happens in `~/Projects/beautiful-aerc/` with its own
  git history; dotfiles repo just points to it

This means:
- `git pull` in the project repo gets upstream updates
- `stow -R beautiful-aerc` re-links after updates
- Personal overrides (accounts.conf, palette override sections) are
  unaffected by stow operations
- The project repo is both the development workspace and the
  distributable artifact

## Future ideas (not in scope)

- Install script that asks questions and personalizes accounts.conf,
  editor choice, etc.
- Blog post: "beautiful aerc" detailing the setup
- Additional themes contributed by users
