# tidytext Design Spec

A lightweight CLI tool that uses Claude Haiku to tidy prose before
sending. Fixes spelling, grammar, and punctuation errors without
altering meaning or interfering with the author's style. Lives in
the beautiful-aerc repo as a third binary.

## CLI Interface

```
tidytext                              # read stdin, write stdout
tidytext message.txt                  # read file, write stdout
tidytext --in-place message.txt       # modify file directly
tidytext --no-config                  # all defaults, skip config file
tidytext --config /path/to/alt.toml   # alternate config file
tidytext --rule spelling=false        # override a single rule
tidytext --style em_dash_spaces=true  # override a single style
```

Stdin/stdout is the primary mode. When given a file path argument,
reads that file instead of stdin. `--in-place` writes back to the
file via temp file + rename. Without `--in-place`, file mode writes
to stdout.

Flags override config file values. `--no-config` ignores the config
file entirely and uses built-in defaults.

### Command Help

Cobra generates help from the command metadata. The root command
should produce output along these lines:

```
Tidy prose with AI-powered spelling, grammar, and punctuation fixes.

Reads text from stdin or a file, fixes errors using Claude, and
writes the corrected text to stdout. Quoted lines (> prefixed) and
code blocks are preserved unchanged. Returns original text on any
error.

Config: ~/.config/tidytext/config.toml

Usage:
  tidytext [file] [flags]

Flags:
      --config string   config file (default ~/.config/tidytext/config.toml)
      --in-place        modify file in place
      --no-config       skip config file, use defaults
      --rule strings    override a rule (e.g. spelling=false)
      --style strings   override a style (e.g. em_dash_spaces=true)
  -h, --help            help for tidytext
  -v, --version         version for tidytext
```

## Core Flow

1. Read input (stdin or file)
2. Split into author lines and quoted lines, preserving positions
3. If no author text, write input back unchanged, print
   "tidytext: no author text found" to stderr
4. Load config from `~/.config/tidytext/config.toml` (unless
   `--no-config` or `--config` specified)
5. Build system prompt from enabled rules and style preferences
6. Send author text to Claude Haiku, get corrected text back
7. Reassemble corrected author text with original quoted text in
   original positions
8. Write to stdout (or file with `--in-place`)
9. Print summary to stderr

## Quoted Text Handling

Lines starting with `>` (with optional leading whitespace) are
quoted text. Consecutive quoted blocks and blank lines within them
are preserved verbatim and never sent to the API.

When invoked from nvim-mail, the keybinding only sends the author's
body text (excluding headers and signature). The tool also strips
quoted lines internally as a safety net.

## Config File

Location: `~/.config/tidytext/config.toml`

If the file does not exist, all rules are enabled with built-in
defaults. Missing keys fall back to defaults. Users only need to
specify what they want to change.

```toml
[api]
model = "claude-haiku-4-5-20251001"
# api_key = ""  # optional, falls back to ANTHROPIC_API_KEY

[rules]
spelling = true
grammar = true             # to/too, its/it's, affect/effect, etc.
punctuation = true         # dash conversion, ellipses
whitespace = true          # double spaces, trailing spaces
capitalization = true      # sentence starts, standalone "I"
repeated_words = true      # "the the"
missing_punctuation = true # missing period at end of final sentence
oxford_comma = "ignore"    # "insert", "remove", or "ignore"

[style]
em_dash_spaces = false     # true: " — ", false: "—"
ellipsis = "character"     # "character" (...) or "dots" (...)
custom_instructions = []   # additional rules passed to the model
```

### Defaults Rationale

All rules enabled by default to fix clear errors out of the box.
Style-opinion settings use the most broadly conventional defaults:

- `oxford_comma = "ignore"` -- style preference, don't impose
- `em_dash_spaces = false` -- matches AP, Chicago style guides
- `ellipsis = "character"` -- proper Unicode

## Prompt Construction

The system prompt is built dynamically from the config. Each enabled
rule maps to one or two instruction lines. Disabled rules are
omitted. Style preferences modify the wording.

### System Prompt Structure

```
You are a proofreader. Fix errors in the text below and return ONLY
the corrected text with no commentary, explanations, or markdown
formatting.

Rules:
- Fix misspelled words
- Fix grammar errors (to/too, its/it's, affect/effect, then/than,
  lose/loose)
- Convert dashes to proper em dash with no spaces around them
- [... only enabled rules appear ...]

Do NOT:
- Rephrase or restructure sentences
- Change tone or formality
- Expand or contract contractions
- Add or remove content
- Change the author's voice or style
- Modify text inside backtick code spans or code blocks
[... custom_instructions appended here ...]

If the text has no errors, return it exactly as-is.
```

### User Message

The author's text with no wrapping or markup.

## API Integration

Direct HTTP to `https://api.anthropic.com/v1/messages`. No SDK
dependency. Uses `ANTHROPIC_API_KEY` env var by default; config file
`api_key` field overrides.

Model defaults to `claude-haiku-4-5-20251001`, configurable in the
config file.

## Error Handling

The tool is always safe to run. Any failure returns original text
unchanged and prints a descriptive message to stderr.

| Condition | Stdout | Stderr |
|-----------|--------|--------|
| Success | corrected text | `tidytext: N corrections applied` |
| No changes | original text | `tidytext: no changes needed` |
| No author text | original text | `tidytext: no author text found` |
| API unreachable | original text | `tidytext: API unavailable, text unchanged` |
| API key missing | original text | `tidytext: ANTHROPIC_API_KEY not set, text unchanged` |
| API error | original text | `tidytext: API error (status), text unchanged` |
| Config parse error | original text | `tidytext: config error: detail` |

## nvim-mail Integration

### Keybinding

`<leader>t` in nvim-mail runs the tidy pass on the author's body
text.

### Implementation

1. Identify body range: everything after the header separator
   extmark, excluding signature (lines after `-- `)
2. Save the current buffer lines in that range
3. Pipe those lines through `tidytext` via `vim.fn.systemlist()`
4. Replace the buffer range with the output
5. Capture stderr, display summary via `vim.notify` at info level
6. Word-level diff between saved and new lines
7. Place `EmailTidyChange` extmark highlights on changed regions
8. Register one-shot `TextChanged`/`TextChangedI` autocmd to clear
   highlights on next edit

### Change Highlighting

Word-level diff (not line-level) so individual corrected words are
highlighted. Uses a dedicated extmark namespace for clean teardown.

Default highlight group:

```lua
vim.api.nvim_set_hl(0, "EmailTidyChange", {
  undercurl = true,
  sp = "#8fbcbb"
})
```

Teal undercurl, visually distinct from red spell-check squiggles.
Users override by setting `EmailTidyChange` in their nvim-mail
config.

Highlights persist until the next `TextChanged`/`TextChangedI`
event (next edit clears them).

## Project Structure

```
cmd/tidytext/
  main.go              # build root cmd, run, print error, exit
  root.go              # cobra root: read input, run tidy, write output

internal/tidy/
  config.go            # TOML config parsing, defaults
  config_test.go
  prompt.go            # build system prompt from config
  prompt_test.go
  tidy.go              # core: strip quotes, call API, reassemble
  tidy_test.go
  quote.go             # split author/quoted lines, reassemble
  quote_test.go
  api.go               # Claude API HTTP call
  api_test.go
```

## Build Changes

Makefile additions:

- `build`: add `go build -o tidytext ./cmd/tidytext`
- `install`: add `GOBIN=$(HOME)/.local/bin go install ./cmd/tidytext`
- `clean`: add `tidytext` to `rm -f`

## Dependencies

- TOML parsing: `BurntSushi/toml` or small internal parser
- HTTP: standard library `net/http`
- JSON: standard library `encoding/json`
- No new dependencies for nvim-mail (pure lua)

## Testing

- **Unit tests**: table-driven, same package. Config parsing, prompt
  building, quote splitting/reassembly all tested independently.
- **API tests**: mock HTTP response to test the assembly flow
  without hitting the real API.
- **E2E**: build binary, pipe sample text through it, verify:
  quoted lines untouched, stderr output format correct, original
  text returned on simulated error.

## Out of Scope

- Rephrasing or restructuring sentences
- Tone adjustment
- Expanding/contracting contractions
- Line wrapping or formatting (nvim-mail handles this)
- Offline operation
- Exposing or editing the system prompt directly
