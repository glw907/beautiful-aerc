# Styling

Guidelines for visual styling across all beautiful-aerc components.
Every color comes from the theme — nothing is hardcoded in Go source.

## Principle

All styling flows from compiled themes through two rendering paths:

1. **Content pipeline** — `internal/filter/` converts raw email to
   normalized markdown. `internal/content/` parses it into blocks and
   renders with lipgloss styles from `CompiledTheme`.
2. **aerc styleset** — `mailrender themes generate` writes an aerc
   styleset file from the compiled theme's palette hex values.

Go code never assembles ANSI codes manually or hardcodes color values.

See [themes.md](themes.md) for the compiled theme reference.

## Color Token Usage

### Header tokens

Used by `filter.Headers()` via the `ColorSet` struct:

| Token | Usage |
|-------|-------|
| `hdr_key` | Header field names (From, Subject, etc.) — bold accent |
| `hdr_value` | Header field values (names, text) — foreground |
| `hdr_dim` | Angle brackets, separator line — dim |

### Glamour tokens

Used by `theme.GlamourStyle()` to configure Glamour's renderer:

| Token | Usage |
|-------|-------|
| `heading` | Markdown headings |
| `bold` | Bold text |
| `italic` | Italic text |
| `link_text` | Link display text |
| `link_url` | Link URLs |
| `rule` | Horizontal rules |

### Message UI tokens

Used by standalone screens (confirmation dialogs, etc.):

| Token | Usage |
|-------|-------|
| `msg_marker` | Heading `#` marker |
| `msg_title_success` | Success headings (confirmations) |
| `msg_title_accent` | Interactive headings (prompts) |
| `msg_detail` | Detail text (filenames, labels) |
| `msg_dim` | Secondary text (counts, hints) |

### Picker tokens

Used by interactive TUI overlays:

| Token | Usage |
|-------|-------|
| `picker_num` | Shortcut digits |
| `picker_label` | Item label text |
| `picker_url` | URL text |
| `picker_sel_bg` | Selected row background |
| `picker_sel_fg` | Selected row foreground |

## ANSI Output Conventions

- Always pair color sequences with a `\033[0m` reset
- Header filter: wraps styled spans as `\033[<sgr>m<text>\033[0m`
- HTML filter: Glamour handles all escape sequences internally

## aerc Constraints

aerc has no overlay modal API. The two feedback mechanisms are:

- **`:pipe`** — runs a command, shows stdout in a terminal widget,
  then "Process complete, press any key to close." Best for
  confirmations.
- **`:pipe -b`** — runs in background, shows "completed with
  status 0" briefly in the status bar. Too brief for user-facing
  messages; avoid.
