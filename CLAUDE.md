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
cmd/mailrender/        CLI wiring: filters (cobra)
cmd/pick-link/         CLI wiring: interactive URL picker (cobra)
cmd/fastmail-cli/      CLI wiring: rules, masked, folders (cobra)
cmd/tidytext/          CLI wiring: fix, config (cobra)
cmd/compose-prep/      CLI wiring: compose buffer normalizer (cobra)
internal/compose/      Compose buffer normalization pipeline
internal/theme/        Load TOML theme files, resolve color tokens to ANSI
internal/filter/       Filter implementations (headers, html, plain) + footnote rendering
internal/picker/       Link picker UI
internal/tidy/         Prose tidying: config, prompt, API, quote handling
internal/jmap/         JMAP session auth, mail operations, masked email operations
internal/header/       RFC 2822 header parsing (from, subject, to/cc)
internal/rules/        Local JSON rule file operations
e2e/                   End-to-end tests for mailrender (build binary, pipe fixtures)
e2e/testdata/          HTML email fixtures + golden output files
e2e-fastmail/          End-to-end tests for fastmail-cli
e2e-tidytext/          End-to-end tests for tidytext
.config/aerc/          aerc configuration files
.config/aerc/themes/   TOML theme files (nord.toml, solarized-dark.toml, gruvbox-dark.toml)
.config/aerc/stylesets/ Generated aerc stylesets
.config/nvim-mail/     Neovim compose editor profile
.config/kitty/         kitty terminal profile for mail
.local/bin/            Launcher scripts (mail, nvim-mail, aerc-save-email)
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

Theme files (`.config/aerc/themes/*.toml`) define 16 semantic hex color
slots + TOML token definitions. Go binaries read `.toml` theme files
directly at runtime, resolving color references and style modifiers at
startup. The active theme is determined by `styleset-name` in
`aerc.conf`.

To generate an aerc styleset from a theme: `mailrender themes generate`.
To generate for a specific theme: `mailrender themes generate nord`.

**Never hardcode ANSI color codes or style modifiers (bold, italic,
underline) in Go source.** All text styling must use composite
tokens defined in the theme file. If a UI element needs styling,
add a token to the theme and reference it through the theme package.

## Link Picker

`pick-link` is a standalone binary that provides an interactive URL
picker for the message viewer. It is invoked via `:pipe` so aerc feeds
the raw message on stdin.

**Pipeline:** raw HTML → `filter.HTML` (same filter the viewer uses)
→ extract URLs from footnotes and plain text → interactive picker
→ `xdg-open` selected URL.

**Keybinding** (in `[view]` section of `binds.conf`):
- `Tab` — open the link picker (`:pipe pick-link<Enter>`)
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

## Styling

**Read `docs/styling.md` before building any UI element.** It
defines the visual hierarchy, layout patterns, color token usage,
and aerc integration patterns. See `docs/themes.md` for the token
reference and theme file format.

## Testing

- **Unit tests:** table-driven, same package, alongside source files
- **E2E tests:** build binary in TestMain, pipe HTML fixtures, compare
  against golden files in `e2e/testdata/golden/`
- **Live verification:** tmux-based aerc testing (see global CLAUDE.md)

## fastmail-cli

Fastmail JMAP CLI, built as a second binary from the same module.

### Command Structure

    fastmail-cli
      rules         Manage mail filter rules
        interactive   Full interactive filter creation flow
        add           Add a filter rule
        sweep         Move matching messages
        count         Count matching messages
        export        Copy rules to export destination
        export-check  Check if export is needed
        extract       Extract header fields from a message
      masked        Manage masked email addresses
        delete        Delete a masked email address
      folders       List custom mailboxes
      version       Print version

### Environment Variables

    FASTMAIL_API_TOKEN       Fastmail API token (required for JMAP commands)
    AERC_RULES_FILE          Path to rules file (default: ~/.config/aerc/mailrules.json)
    AERC_RULES_EXPORT_DEST   Export destination (default: ~/Documents/mailrules.json)

## tidytext

Claude-powered prose tidier, built as a third binary from the same
module. Fixes spelling, grammar, and punctuation without altering
meaning or the author's style.

### Command Structure

    tidytext
      fix           Fix spelling, grammar, and punctuation
      config        Show effective configuration
        init          Create default config file

### Usage

    tidytext                              # show help
    echo "text" | tidytext fix            # piped stdin
    tidytext fix message.txt              # read file
    tidytext fix --in-place message.txt   # modify file directly
    tidytext fix --no-config              # skip config, use defaults
    tidytext fix --rule spelling=false    # override a rule
    tidytext fix --style em_dash_spaces=true
    tidytext fix --style time_format=uppercase
    tidytext config                       # show current config
    tidytext config init                  # create default config

### Config

Location: `~/.config/tidytext/config.toml`

If the file does not exist, all rules are enabled with defaults.
Users only need to specify what they want to change.

### Environment Variables

    ANTHROPIC_API_KEY    Claude API key (required for fix command)
    TIDYTEXT_API_URL     Override API endpoint (testing only)

### nvim-mail Integration

`<leader>t` runs tidytext on the compose buffer body (excluding
headers and signature). Changed words are highlighted with teal
undercurl extmarks (`EmailTidyChange` highlight group) that clear
on next edit.

## compose-prep

Stdin/stdout buffer normalizer for the nvim-mail compose editor.
Reads an aerc compose buffer from stdin, normalizes headers and
reflows quoted text, writes the result to stdout. Called by
nvim-mail's VimEnter autocmd via `vim.fn.systemlist`.

### Pipeline

1. **Unfold** RFC 2822 continuation lines (space/tab prefix)
2. **Strip brackets** from bare `<email>` addresses (uses `net/mail`)
3. **Fold** To/Cc/Bcc at 72-column recipient boundaries
4. **Inject** empty Cc:/Bcc: headers when absent
5. **Reflow** quoted text paragraphs at 72 columns

### Flags

    --no-cc-bcc    Suppress empty Cc/Bcc header injection
    --debug        Write diagnostic messages to stderr

### Error Behavior

On any processing error, the original input is passed through
unchanged. Exit code is always 0. The compose window always opens.

## Build

```
make build     # build all five binaries
make test      # run tests
make vet       # go vet
make check     # vet + test (gate before commits)
make install   # install all five to ~/.local/bin/
```

## Corpus

`corpus/` holds raw email parts (HTML or plain text) flagged for
rendering issues. Save emails from aerc using the `aerc-save-email`
shell script. The `/fix-corpus` skill batch-processes accumulated
corpus emails.

## Personal Config

This project ships working defaults that any user can stow directly
from their clone. The author's personal configs live in
`~/.dotfiles/beautiful-aerc/` (workstation repo) as a real stow
package — not a symlink to this project. Personal differences from
repo defaults:

- `binds.conf` — all optional bindings enabled (fastmail-cli,
  aerc-save-email)
- `signature.md` — real signature (repo ships `.example` only)
- `mailrules.json` — personal mail rules (not in this repo)
- `accounts.conf` — personal credentials (repo ships `.example` only)

When configs change in this project, the corresponding personal
configs in `~/.dotfiles/beautiful-aerc/` need manual sync. The Go
binaries are installed via `make install` (not stowed).

## Filter Testing

See `.claude/docs/tmux-testing.md` for patterns to render emails
through the filter and verify output via tmux.
