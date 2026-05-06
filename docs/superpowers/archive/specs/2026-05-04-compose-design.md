# Compose Framing — Pass 9 Design

**Spec date:** 2026-05-04
**Pass:** 9 (framing) → 9.5 (compose enhancements: #5 #12 #24)
**Status:** drafted, awaiting review

## Purpose

Land the framing for outbound mail in poplar. Pass 9 ships:

1. **Catkin** — a markdown-first bubbletea editor, library-pure, the
   first iA-Writer-shaped TUI editor in the bubbles ecosystem.
2. **The `Editor` interface** (ADR-0033) — the seam future neovim
   adapters plug into.
3. **`internal/compose/`** — the poplar-side compose surface: header
   form, attachment chip row, draft assembler, reply/forward seeders,
   markdown→HTML render.
4. **SMTP backend** via `emersion/go-smtp` for IMAP accounts; JMAP
   uses `Email/submission`. Auth re-uses the account's `password-cmd`.
5. **Cache outbox wiring** — `cache.OpKind{KindSend, KindAppend}` and
   `SendArgs` / `AppendArgs` flesh out their reserved slots.

Pass 9 does **not** ship: neovim adapter (Pass 1.1), attachment-add
inside compose (Pass 9.5 layers attachments per #24), tidytext or
spellcheck overlays, link-preview chips, runtime editor swap.

## Settled

The Pass 9 starter prompt called four open questions; brainstorming
on 2026-05-04 closed them and surfaced two more.

| # | Question | Decision |
|---|---|---|
| 1 | Catkin's text model — rune buffer vs. line buffer | `bubbles/textarea` as buffer + cursor + edit-op primitive. Catkin owns the renderer (live markdown styling) and the reflow engine. ADR-0076 already commits the foundation libs. |
| 2 | Attachment-add UI inside compose | `Ctrl+T` opens a path prompt at the bottom of the compose surface; a chip row renders above the body when `len(attachments) > 0` (mirrors the viewer's chip pattern from ADR-0138). No persistent header field. |
| 3 | Reply / Reply-all quoting depth | Full body quoted, **preserving nested depth**. Walk existing `>` prefixes and add one more level. Convention every major reference client uses (Thunderbird, Apple Mail, mutt, aerc, Geary, Evolution, alpine). |
| 4 | SMTP failure surface | Outbox conflict overlay (existing Cache III pattern, ADR-0132/0133/0134). `KindSend`/`KindAppend` failures land on the same `!` overlay and `⚠N` status segment. No new toast surface. |
| 5 (new) | Markdown live styling | iA-Writer-shaped: source stays raw markdown, `View()` overlays inline span styling on `**bold**`, `*italic*`, `~~strike~~`, `` `code` ``, `[text](url)`, headings, hard-break glyph. Block styling on quotes, code fences, lists. **Depth-graduated blockquote color** in v1, capped at depth 4. |
| 6 (new) | Long URL display | Two-layer wrap: source-level reflow never breaks tokens (URL stays one markdown line); display-level wrap soft-breaks long lines mid-token for the editor view only. Buffer unchanged either way. |
| 7 (new) | QoL batch surfaced from prior-art survey | Twelve additions (§ Catkin QoL Additions) folding in: undo/redo, find/replace, markdown auto-pair, smart URL paste, word-level nav, scroll-off, bracket/span match highlight, task lists, typewriter scroll, focus mode, trailing-whitespace cleanup. Smart quotes deferred (carve-out complexity). |
| 8 (new) | Annotation pipeline + spellcheck | `Editor.SetAnnotations` is the generic seam. Spellcheck ships in v1 via subprocess to system `hunspell -a` (pure-Go landscape lacks working suggestions). Multilingual via `[ui] spellcheck_lang`. Graceful degrade when binary absent. |
| 9 (new) | Tidy seam | Catkin stays Claude-unaware. `compose.Tidy` interface + `NoopTidy` ship with Pass 9 framing; full Anthropic-API impl ships in a follow-up sub-pass. Paragraph-level rewrite with review-mode diff UI in ComposeTab. |

## Architecture

### Package map

Three new internal packages plus targeted edits to existing ones.

```
internal/
  catkin/              new — markdown-first bubbletea editor (library-pure)
  compose/             new — Editor interface, Catkin adapter, draft assembler
  ui/compose_tab.go    new — ComposeTab tea.Model
  mail/                edit — Send + Append on Backend interface
  mailjmap/            edit — Email/submission, Email/import
  mailimap/            edit — third connection for SMTP via emersion/go-smtp
  cache/               edit — OpKind{KindSend,KindAppend}, executeSend/Append
  theme/               edit — blockquote_2, blockquote_3, blockquote_4 palette slots
  config/              edit — [account.smtp] block; provider presets fill defaults
```

### `internal/catkin/` — the editor

Bubbletea component, no poplar imports. Future home: `github.com/glw907/catkin`. v1 dependency floor: `bubbletea`, `bubbles/textarea`, `lipgloss`, `muesli/reflow`. ADR-0031 + ADR-0076 already lock the shape.

Public surface:

```go
type Model struct { /* ... */ }

func New(opts ...Option) Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string

func (m *Model) SetSize(w, h int)
func (m *Model) SetWidth(w int)        // body wrap width; reflow re-runs
func (m *Model) Value() string         // raw markdown source
func (m *Model) SetValue(s string)     // for reply/forward seeding

func (m Model) Focus() tea.Cmd
func (m *Model) Blur()
func (m Model) Focused() bool

func (m Model) WordCount() int
func (m Model) CharCount() int
```

Catkin uses `bubbles/textarea` as its buffer + cursor + edit-op primitive only — Catkin's `View()` reads the raw buffer, runs the block classifier, applies inline span styling, and draws the cursor at the wrapped (line, col). Textarea's own `View()` is **not** used (parsing its ANSI escape stream would be fragile and version-coupled).

Internal anatomy:

- `internal/catkin/buffer.go` — wraps `textarea.Model`. All edit ops route through here.
- `internal/catkin/blocks.go` — line-by-line block classifier. Produces a context stack per line (`[quote(2), list, code]` etc.).
- `internal/catkin/reflow.go` — block-aware paragraph reflow. Quote-prefix preserving, code-fence skipping, list-marker aware. Operates on the raw buffer, fires on edit and on `SetWidth`.
- `internal/catkin/render.go` — Catkin's `View()` implementation. Produces styled, display-wrapped lines for the visible viewport.
- `internal/catkin/dispatch.go` — Ctrl+key command table (ADR-0032). Markdown helpers + standard editing commands.
- `internal/catkin/styles.go` — pluggable `Styles` struct. Compose constructs it from `theme.CompiledTheme`.

### `internal/compose/` — the poplar-side surface

```go
type Editor interface {
    tea.Model               // Init, Update, View
    SetSize(w, h int)
    Focus() tea.Cmd
    Blur()
    Focused() bool
    Value() string
    SetValue(s string)
}

type CatkinEditor struct {
    inner catkin.Model
}
// CatkinEditor implements Editor.

type Mode int
const (
    ModeNew Mode = iota
    ModeReply
    ModeReplyAll
    ModeForward
)

type Draft struct {
    From        mail.Address
    To, Cc, Bcc []mail.Address
    Subject     string
    Body        string         // markdown source
    InReplyTo   string         // Message-Id of parent (Reply/Reply-all)
    References  []string       // chain
    Attachments []string       // filesystem paths
}

// AssembleMIME renders Draft into a multipart/alternative RFC 5322
// message: text/plain (markdown source verbatim) + text/html
// (rendered via goldmark with autolink + tables extensions).
// Attachments become multipart/mixed siblings under the alternative
// container.
func AssembleMIME(d Draft, now time.Time) ([]byte, error)

// SeedReply/SeedReplyAll/SeedForward build a Draft with headers
// populated and the parent body quoted (depth-preserving for reply).
func SeedReply(parent mail.MessageInfo, body []byte, self mail.Address) Draft
func SeedReplyAll(parent mail.MessageInfo, body []byte, self mail.Address) Draft
func SeedForward(parent mail.MessageInfo, body []byte) Draft
```

`AssembleMIME` is a pure function — given a `Draft` and a clock, it returns the bytes the cache outbox will hold and the SMTP backend will ship. Markdown→HTML rendering uses `yuin/goldmark` with the `autolink` and `table` extensions; output is wrapped in a minimal HTML skeleton (no inline CSS — recipients' clients style).

### `internal/ui/compose_tab.go` — the UI tab

`ComposeTab` is a tea.Model rooted under `App`. Owns:

- Header textinputs: `to`, `cc`, `bcc`, `subject` — all `bubbles/textinput.Model`. Each owns its buffer; ComposeTab reads via `.Value()` at send.
- `editor compose.Editor` — body. v1 always `*compose.CatkinEditor`.
- `attachments []string` — paths.
- `focusIndex int` — which field is active.
- `mode compose.Mode`, `parentUID uint32`, `parentLoading bool` — reply/forward state.

State ownership follows ADR-0036 / ADR-0042: each child owns its buffer, ComposeTab assembles a `compose.Draft` only at the send moment by reading children. No mirrored state.

Routing:

- `c` from MessageList or Viewer emits `OpenComposeMsg{Mode, ParentUID}`. `App` mounts a new `ComposeTab` and switches focus.
- Reply mode is async: ComposeTab fires `loadParentBodyCmd` on init → `ReplySeededMsg{header, quotedBody}` → ComposeTab calls `editor.SetValue(...)`. Editor stays empty until the seed arrives — no synchronous fetch.
- `Ctrl+Enter` → ComposeTab returns a `tea.Cmd` that calls `cache.Account.QueueOp(SendArgs{...})` and `QueueOp(AppendArgs{folder=Sent, ...})`. Drainer emits `CacheEvent` on the existing channel.
- `Esc` with unsaved changes → `ConfirmModalRequestMsg` (existing pattern, ADR-0128). `App` mounts ConfirmModal; on Yes → `ConfirmModalYesMsg` → ComposeTab emits `CloseComposeMsg`.
- `Ctrl+S` → queue `AppendArgs{folder=Drafts, ...}` only; do not queue the Send op.
- `Ctrl+T` → opens a path prompt at the bottom of the compose surface. Enter attaches; Esc cancels.

Focus rules:

- Header field focused: `Tab` → next field, `Shift+Tab` → previous.
- Body focused: `Tab` and `Shift+Tab` are Catkin commands (list indent / outdent, or 2-space insert on non-list lines).
- `Shift+Tab` at body cursor `(row 0, col 0)` returns focus to Subject (only at that exact position, so it doesn't fight the outdent command). Forward escape from body has nowhere to go — `Ctrl+Enter` sends.
- All header textinputs have `Tab`/`Shift+Tab` handled at ComposeTab level (intercepted before the textinput sees them).

Window sizing:

ComposeTab on `WindowSizeMsg` stores w/h, computes child sizes, calls `SetSize` on each textinput and `editor.SetSize(bodyW, bodyH)` where `bodyW = min(72, w - chrome)`, then forwards the msg per ADR-0035. Narrowing the terminal triggers Catkin's reflow on the next `SetSize`.

Width math throughout: `lipgloss.Width` for icon-free strings; `displayCells` for any string that may contain icon-bearing runes. Row composition uses `strings.Join` over pre-padded children (icon-mode safe per ADR-0084) — never `lipgloss.JoinHorizontal`.

### `internal/mail/` — backend extensions

Two new methods on the existing `Backend` interface:

```go
type Backend interface {
    // ... existing ...
    Send(ctx context.Context, raw []byte) error
    Append(ctx context.Context, folder string, raw []byte, flags []string) error
}
```

`Send` ships the assembled MIME bytes to the recipient(s) over SMTP (IMAP backend) or JMAP `Email/submission` (JMAP backend). `Append` writes the same bytes (typically with `\Seen` flag) to the Sent folder so the user has a copy.

JMAP's `Email/submission` collapses Send + Append-to-Sent into a single atomic operation; the JMAP `Send` impl performs both. The IMAP `Send` impl ships via SMTP only; the cache drainer queues a separate `KindAppend` op for the Sent-folder copy.

### `internal/mailimap/` — SMTP sibling

Generic IMAP gains a third connection: an SMTP client via `emersion/go-smtp`. Lifetime: dial on first `Send`, keep open with the same backoff and reconnect rules as the IDLE connection (ADR-0107 lineage), close on backend `Close`. SMTP host/port come from `[account.smtp]` in `config.toml`; provider presets (`fastmail`, `gmail`, `yahoo`, `icloud`, `zoho`, `outlook`, `mailbox-org`, `posteo`, `runbox`, `gmx`, `protonmail`) fill canonical defaults at decode time, mirroring the existing IMAP preset path.

Auth: re-uses `password-cmd` resolved at backend `Connect` (ADR-0102). XOAUTH2 path for Gmail mirrors the IMAP-side XOAUTH2 helper (ADR-0108). No internal token refresh in v1.

### `internal/mailjmap/` — submission

`Email/submission` request includes `emailId` (from `Email/import` of the assembled MIME) plus `envelope` (mailFrom + rcptTo). On success, JMAP also performs `onSuccessUpdateEmail` to set `\Seen` on the Sent copy.

`FetchHeaders` chunking (500 per `Email/get`, ADR-0143) is unaffected — submission is its own method call.

### `internal/cache/` — outbox extensions

`OpKind` and `OpArgs` already reserve `KindSend` / `KindAppend` / `SendArgs` / `AppendArgs` (per Cache III invariants). Pass 9 fleshes them out:

```go
type SendArgs struct {
    MIME []byte               // assembled message bytes
    To   []string             // envelope recipients (flat)
}

type AppendArgs struct {
    Folder string             // canonical folder name (e.g., "Sent", "Drafts")
    MIME   []byte
    Flags  []string           // e.g., ["\Seen"]
}
```

Drainer dispatch: `executeSend(ctx, args)` calls `Backend.Send(ctx, args.MIME)`; `executeAppend(ctx, args)` calls `Backend.Append(ctx, args.Folder, args.MIME, args.Flags)`. Conflict matrix follows ADR-0116:

- `mail.ErrAuth` → `OpConflict auth-failure` (user resolves via `!`).
- Transient (timeout, network, 4xx-retryable) → `OpFailed` with exponential `next_eligible_at`.
- `attempts >= max` → `OpConflict max-attempts-exceeded`.
- Crash recovery on `OpExecuting` for `KindSend` / `KindAppend` is **not** auto-resumed. They land in `OpConflict crashed-mid-execute` (ADR-0116) — both ops are not provably idempotent (a Send that crashed mid-write may or may not have been delivered; an Append that crashed may or may not have created the Sent copy). User resolves via `!`.

No optimistic flip on `KindSend` / `KindAppend` — neither op edits a row's `ui_flags`. The compose tab closes immediately on dispatch; success is silent (ops vanish from the outbox); failure surfaces via `⚠N` + `!`.

### `internal/theme/` — blockquote palette

Three new slots in `internal/theme/palette.go`:

- `blockquote_1` (alias of existing `blockquote` slot — single-depth quotes)
- `blockquote_2`, `blockquote_3`, `blockquote_4` — graduated tones for nested-quote depth.

Depth > 4 collapses to `blockquote_4`. Each of the 15 themes fills the four slots; `docs/poplar/styling.md` adds the surface→slot mapping per the styling-discipline rule.

### `internal/config/` — `[account.smtp]` block

```toml
[[account]]
name = "personal"
provider = "fastmail"
# JMAP fills SMTP defaults from the preset.

[[account]]
name = "work"
provider = "imap"
host = "mail.example.com"
port = 993
# Explicit SMTP for self-hosted:
[account.smtp]
host = "mail.example.com"
port = 465
starttls = false   # implicit TLS on 465; STARTTLS on 587
```

Provider presets fill `[account.smtp]` defaults at decode time. Validation errors carry the same `account "<name>" (provider = "<p>"): ...` prefix as the IMAP path. The `poplar config check` subcommand learns to verify SMTP connectivity (sequential per-account check, parallel to the existing IMAP/JMAP probe).

## Catkin internals

### Block classifier

`internal/catkin/blocks.go` walks the buffer line-by-line and produces a `LineContext` per line:

```go
type BlockKind int
const (
    BlockParagraph BlockKind = iota
    BlockHeading
    BlockQuote
    BlockListItem
    BlockCodeFence
    BlockCodeIndent
    BlockTable
    BlockBlankLine
)

type LineContext struct {
    Kind         BlockKind
    QuoteDepth   int        // 0 = not in quote
    ListMarker   string     // "-", "*", "+", "1.", etc.; empty if not a list
    HeadingLevel int        // 1..6; 0 if not heading
    InsideFence  bool       // line is between two ``` markers
    PrefixWidth  int        // quote+list marker prefix in cells
}
```

Classification is single-pass with a small state machine for fenced regions. Quote depth counts leading `>` runs (with optional spaces). List marker detection runs on the post-quote-prefix content. Tables detected by header-row + separator-row pattern (`| --- |`). Headings: ATX (`# `) and Setext (`text\n===`).

The classifier is **deterministic per line in isolation, plus fence state**. Fence state is the only cross-line context; everything else (quote depth, list, heading) reads from the line itself.

### Reflow engine

`internal/catkin/reflow.go` operates on the **raw buffer** (mutates it). Triggers:

1. On `SetWidth(w)` if `w` changed.
2. On every commit boundary in `Update` (paragraph break, Enter, command boundary).

Algorithm per paragraph:

1. Walk line-by-line; classifier produces `LineContext` per line.
2. Group consecutive lines with the same `(QuoteDepth, ListMarker, InsideFence)` into a **paragraph block**.
3. For each paragraph block:
   - If `InsideFence` or `Kind == BlockTable` or `Kind == BlockHeading`: skip (preserve verbatim).
   - Else: tokenize by whitespace, never breaking a single token. Emit lines with the original quote/list prefix at width `wrapWidth = SetWidth - PrefixWidth`.
4. Replace the paragraph's lines in the buffer with the rewrapped lines.
5. Adjust cursor position: re-locate by absolute character offset within the paragraph; if the cursor was at the end, snap to new end.

Long-token escape: if a single token exceeds `wrapWidth`, place it on its own line — the resulting line is longer than the body width. Display-level wrap (next section) handles this visually without mutating the buffer.

### Display renderer

`internal/catkin/render.go` is Catkin's `View()`. Produces a styled string of the visible viewport:

1. Read the raw buffer.
2. Run the block classifier (reuse output if buffer unchanged since last render).
3. For each line in the visible vertical range:
   - Apply block-level style (heading color, blockquote depth color, code-fence background).
   - Run the inline span styler on the post-prefix content.
   - If the styled line exceeds the display width, soft-wrap mid-token (display only — buffer untouched).
4. Draw the cursor at the (line, col) inside the wrapped output. Cursor styling preserves `displayCells` invariants.

The inline span styler handles:

| Pattern | Style |
|---|---|
| `**text**`, `__text__` | bold weight on `text` (markers visible) |
| `*text*`, `_text_` | italic on `text` |
| `~~text~~` | strikethrough on `text` |
| `` `text` `` | inline-code style on `text` |
| `[text](url)` | link style on `text`; URL portion dimmed |
| `https://...`, `http://...` (bare) | link style on whole token |
| Trailing `␣␣` at line end | append dimmed `↵` glyph (display only) |

Style application uses lipgloss only — no character substitution — so `displayCells` math is preserved exactly. Cursor positioning math reads the **raw buffer**, not the styled output, to avoid escape-sequence parsing.

### Command dispatch

`internal/catkin/dispatch.go` declares `key.Binding`s and a dispatcher in Catkin's `Update`. v1 vocabulary:

| Key | Command |
|---|---|
| `Ctrl+B` | wrap word at cursor in `**…**` |
| `Ctrl+I` | wrap word at cursor in `*…*` |
| `Ctrl+K` | insert `[](url)` skeleton, cursor inside `[]` |
| `Ctrl+L` | toggle list prefix on current line (`- ` on/off) |
| `Ctrl+Q` | toggle quote prefix on current line (`> ` on/off) |
| `Ctrl+A` | cursor to line start |
| `Ctrl+E` | cursor to line end |
| `Ctrl+U` | delete to line start |
| `Ctrl+K` (when not on link skeleton) | delete to line end (overloaded — disambiguated by cursor context) |
| `Ctrl+W` | kill word backward |
| `Ctrl+Y` | yank kill ring |
| `Tab` | list indent (on list line) / 2-space insert (otherwise) |
| `Shift+Tab` | list outdent (on list line) / focus-up at `(0, 0)` (otherwise — handled by ComposeTab) |
| `Enter` | smart-newline (continue list/quote prefix; double-Enter on empty prefix ends the block) |

**Note on `Ctrl+K` overload:** the link-skeleton form fires when the cursor is on a non-empty word; the delete-to-EOL form fires on an empty cursor position. This matches GNU readline's `Ctrl+K` (kill-line) by context. If the disambiguation proves confusing in dogfooding, we'll split them in a follow-up — but starting overloaded preserves familiar shortcuts.

`Ctrl+T` (attachment-add) and `Ctrl+S` (save draft) and `Ctrl+Enter` (send) are **not** Catkin commands — they live at ComposeTab level and are intercepted before Catkin sees them.

### Smart Enter

Pressing Enter inside Catkin runs the smart-newline rule:

1. Find current line's prefix (quote `>` runs + optional list marker).
2. If line content (post-prefix) is empty and prefix is non-empty: strip the prefix from current line (end the block), insert blank line, cursor on blank line. Empty `> ` and empty `- ` both terminate.
3. Else: insert newline; new line begins with the same prefix. For ordered lists (`1. `), increment the number (`2. `).
4. Else (no prefix): plain newline.

Renumbering ordered lists on insert in the **middle** is out of scope for v1 (post-1.0 polish).

### Tab / Shift-Tab on list items

- `Tab` on a list line: prepend 2 spaces to the line's prefix (deepens nesting level by one).
- `Shift+Tab` on a list line: strip leading 2 spaces if present (outdent); if already at depth 0, no-op (so Shift+Tab at body `(0, 0)` falls through to ComposeTab, which routes focus to Subject).
- `Tab` on a non-list line: insert 2 spaces at cursor.
- `Shift+Tab` on a non-list line: no-op.

### Word + char count

`Catkin.WordCount()` and `CharCount()` walk the raw buffer. Word count uses `unicode.IsSpace`-delimited splitting; char count is `utf8.RuneCountInString`. ComposeTab renders both in its footer at the right edge: `123 words · 567 chars`. Live update — both methods are O(n) over the buffer; on a 10K-char compose this is well under a render budget.

## Catkin QoL additions

Eleven additions surfaced from a prior-art survey of micro / nano / helix / iA Writer / Typora / Obsidian / VS Code-markdown / aerc-compose. All v1, all in service of "effortless markdown email."

1. **Undo / redo (`Ctrl+Z` / `Ctrl+Y`).** ~50-step linear ring buffer of buffer + cursor snapshots. Without it, no recovery from a bad edit (no selection in v1). Promoted from post-1.0 — Catkin core feature.
2. **Find / replace (`Ctrl+F` / `Ctrl+R`).** A 1-row prompt at the bottom of Catkin's render area. Literal substring; case-insensitive toggle; `Enter` next match; `Esc` cancels. Replace mode adds a second row; `y`/`n` per match, `a` for all. No regex in v1.
3. **Markdown auto-pair.** Type `**`, get `**▌**`. Same for `_…_`, `` `…` ``, `[…]`, ` ``` `…` ``` `. **Carve-out:** disabled inside inline code spans and fenced blocks (block classifier already provides context). Generic bracket-pairing (`(`, `{`) **not** included — would be intrusive in markdown prose.
4. **Smart URL paste → `[word](url)`.** When the clipboard contains a URL and the cursor is on a non-empty word, paste wraps the word as link text. Mirrors VS Code's `markdown.editor.pasteUrlAsFormattedLink`. The plain `Ctrl+K` link-skeleton command still handles the empty-cursor case.
5. **Word-level navigation.** `Ctrl+Left` / `Ctrl+Right` jump by word; `Ctrl+Backspace` / `Ctrl+Delete` delete by word. Bubbles textarea may already provide some — verify and supplement during Pass 9.
6. **Scroll-off (3 lines).** Cursor never sits within 3 lines of viewport top or bottom. One constant in the scroll-position calculation.
7. **Bracket / span match highlight.** When the cursor is on `**`, `_`, `` ` ``, `[`, or `]`, the matching delimiter dims+underlines. Same scanner the inline span styler runs. Catches unbalanced markdown immediately.
8. **Task lists.** `- [ ] ` and `- [x] ` recognized as a list item subtype. Smart Enter on `- [ ] foo` continues with `- [ ] `. `Ctrl+Space` on a task line toggles `[ ]` ↔ `[x]`. Render: `☐` / `☑` (Fancy icon mode) or `[ ]` / `[x]` (Simple). Goldmark renders as HTML `<input type=checkbox>` in the alternative.
9. **Typewriter scroll mode.** Cursor stays vertically centered. Toggleable via `Ctrl+\`; default off; footer mode indicator (`▮ typewriter`). One offset adjustment in the scroll-position math.
10. **Focus mode.** Surrounding paragraphs render at `FgDim`; the active paragraph at full color. Toggleable on the same `Ctrl+\` cycle (`off → typewriter → focus → typewriter+focus → off`). Block classifier already identifies paragraph boundaries.
11. **Trailing-whitespace cleanup on commit boundary.** At the same boundary as reflow (paragraph break / Enter), strip trailing single space and 3+ trailing spaces. **Preserve exactly `␣␣`** (CommonMark hard break — also rendered with the dimmed `↵` glyph).

**Smart quotes — deferred to post-1.0.** Carve-out complexity (skip inside code spans / fenced blocks / URLs) plus directional inference plus apostrophe edge cases (`don't`, `'twas`) make it more work than the user-visible payoff justifies for v1.

## Annotation pipeline & spellcheck

The annotation pipeline is the seam future linters plug into — spellcheck in v1, grammar check post-1.0 (likely indefinitely deferred; see "Tidy obviates grammar check" below).

### Editor interface extension

```go
type AnnotationKind int
const (
    AnnotationSpelling AnnotationKind = iota
    AnnotationGrammar          // post-1.0
    AnnotationStyle            // reserved
)

type Annotation struct {
    Start, End  int             // rune offsets into the buffer
    Kind        AnnotationKind
    Message     string          // shown in suggestion popover
    Suggestions []string
}

type Editor interface {
    // ... existing ...
    SetAnnotations(spans []Annotation)
}
```

CatkinEditor implements it. The v1.1 neovim adapter will map annotations to neovim's diagnostic signs.

### Catkin annotation rendering

Catkin's renderer applies `lipgloss` underline styling per `Kind`:

- **`AnnotationSpelling`** — red-dim underline on the span. Terminals can't do squiggle; underline is the standard fallback (VS Code's low-color mode does this too).
- **`AnnotationGrammar`** — blue-dim underline (post-1.0).
- **`AnnotationStyle`** — green-dim underline (reserved).

Annotations are pure overlay: zero buffer mutation, zero `displayCells` impact (lipgloss styling adds no display cells).

### `AnnotationPicker` overlay

App-owned, mirrors `LinkPicker` (ADR-0087). Opens on `Ctrl+;` when the cursor sits on an annotated span. Renders the `Message` + numbered suggestions. `1`–`9` apply suggestion N; `j/k` + `Enter` apply selected; `Esc` / `Ctrl+;` close.

### Spellcheck — v1

Spellcheck is a core feature, not an opt-in. Two-track design ensures it **works out of the box** for the supermajority of users without any system installation:

**Track 1: English (default) — bundled pure-Go SymSpell.**

- **Library:** pure-Go SymSpell implementation (vendor or use `eskriett/spell`). SymSpell is ~500 lines; vendoring is acceptable if external maintenance is suspect. MIT.
- **Word list:** SCOWL or the Hunspell-derived `en_US.dic` word list, MPL/MIT-compatible. Covers ~500K words including normal inflections (run/runs/running/ran).
- **Embedding:** `go:embed` at compile time. Binary impact ~600KB — acceptable.
- **Suggestions:** edit-distance ranked + word-frequency weighted. Quality approximates Hunspell for the validity check; suggestion quality is competitive (production tools use SymSpell exactly this way).
- **Zero runtime dependency.** Works the moment `poplar` is installed on any platform.

**Track 1 expanded: bundle eight Latin-script European languages.**

To cover near-99% of poplar's realistic user base (technical, Linux/macOS, Western), Track 1 ships pre-embedded word lists for:

| Language | Code(s) | Notes |
|---|---|---|
| English | `en_US`, `en_GB` | Default + UK variant |
| German | `de_DE` | Heavy due to compound morphology (~3MB) |
| French | `fr_FR` | ~1.5MB |
| Spanish | `es_ES`, `es_MX` | Spain + Mexico variants |
| Portuguese | `pt_BR`, `pt_PT` | Brazilian dominant by volume |
| Italian | `it_IT` | ~700KB |
| Dutch | `nl_NL` | Strong Linux/dev representation |
| Polish | `pl_PL` | Rich morphology (~3MB) |

Total binary size impact: ~12MB. Each list is selected at startup based on `[ui] spellcheck_lang`. Only the configured language's list is loaded into memory.

CJK languages (Chinese / Japanese / Korean) are deliberately **not** bundled — they don't use traditional spellcheck (input is via IME; errors are character-selection mistakes no algorithm catches). RTL scripts (Arabic / Hebrew) are out of v1 scope on a separate axis (require bidi rendering work in Catkin).

**Track 2: Other languages — subprocess hunspell fallback.**

- Triggered when `[ui] spellcheck_lang` is set to a language **outside** the bundled eight.
- Subprocess `hunspell -a -d <lang>` in pipe mode. One pooled long-lived process per session.
- Future work: promote additional bundled languages on demonstrated demand.

**Surfacing instructions to non-English / non-bundled speakers.**

Three places carry the install guidance, scaling from "user is mid-task" to "user is reading docs":

1. **Inline error banner on first failure.** When `[ui] spellcheck_lang = "cs_CZ"` (or any non-bundled language) is set and the subprocess fails (binary or dictionary missing), the next attempt to spellcheck emits a one-time per-session `ErrorMsg` to the existing chrome banner. The text platform-detects:

   - Linux (apt-family): `spellcheck: cs_CZ unavailable. Install: sudo apt install hunspell hunspell-cs`
   - Linux (dnf/rpm-family): `spellcheck: cs_CZ unavailable. Install: sudo dnf install hunspell hunspell-cs`
   - macOS: `spellcheck: cs_CZ unavailable. Install: brew install hunspell, then drop the .aff/.dic files into ~/Library/Spelling/`
   - Other / detection-failed: `spellcheck: cs_CZ unavailable. Install hunspell + the cs_CZ dictionary, then restart poplar.`

   The banner is fire-and-forget — it doesn't block typing, doesn't requeue, doesn't repeat in the same session. Detection is via `runtime.GOOS` plus a probe of `/etc/os-release` for distro family. Falls back to the generic message if probe fails.

2. **`poplar config check` reports it.** The existing `config check` subcommand learns a spellcheck probe: prints `spellcheck (cs_CZ): not installed — see README.md#spellcheck` (or `ok` on success) per account. Allows users to verify before writing email.

3. **README documentation.** A dedicated `README.md#spellcheck` section lists:
   - The eight bundled languages by name + code.
   - Per-platform install commands for the major non-bundled languages (Czech, Russian, Turkish, Greek, Norwegian, Swedish, Danish, Finnish, Hungarian — covering most other significant European email locales).
   - Where to drop dictionary files when packaging isn't available.
   - How to set `[ui] spellcheck_lang` in `config.toml`.

This three-tier surfacing ensures users discover the path regardless of how they arrive at the problem — whether they're mid-compose and hit the banner, doing a config sanity-check, or reading docs before setup.

**Why not subprocess-only?** Linux distros (including Linux Mint, the development workstation) don't ship `hunspell` by default. macOS doesn't make its native `NSSpellChecker` accessible via subprocess. Windows has no equivalent. "Graceful degrade silent" would mean most users get no spellcheck — not viable for a core feature.

**Why not pure-Go-only?** Pure-Go Hunspell-format parsers (`shuLhan/share/lib/hunspell`) exist but lack suggestion APIs — and re-implementing Hunspell's morphological-analysis suggestion engine is months of work for marginal gains over SymSpell. SymSpell + a comprehensive word list is the pragmatic equivalent.

**Behavior (both tracks):**

- Debounce 400ms after last edit.
- Consults the block classifier; **skips** spans inside inline code spans, fenced code blocks, table cells, and URL tokens.
- ComposeTab on each Catkin update fires `tea.Cmd` running the spellchecker on debounce.
- Returns `AnnotationsReadyMsg` → ComposeTab calls `editor.SetAnnotations(spans)`.
- Standard tea.Cmd off-loop pattern; no goroutine survives across `Update` calls.

## Claude Tidy

Killer feature, exclusive to poplar. Paragraph-level rewrite via the Anthropic API, presented as a unified-diff review the user accepts or rejects. Catkin stays Claude-unaware — all API logic lives in `internal/compose/` and `internal/ui/compose_tab.go`.

### Seam (Pass 9 framing)

```go
// internal/compose/tidy.go
type TidyRequest struct {
    Paragraph string         // current paragraph text
    Context   string         // surrounding draft for tone/voice
    Audience  string         // optional ("colleague", "customer")
}

type TidyResponse struct {
    Rewrite string
    Notes   []string         // brief explanations
}

type Tidy interface {
    Suggest(ctx context.Context, req TidyRequest) (TidyResponse, error)
}
```

Pass 9 ships:

- The `Tidy` interface + `NoopTidy{}` (always returns "no changes"). Compiles, no UI surface yet.
- `ComposeTab` reserves `Ctrl+G` and an empty handler that calls `tidy.Suggest` if a non-noop impl is wired.
- Review-mode UI shape sketched but not built.

### Implementation (Pass 9i — see passes table below)

- `internal/anthropic/` — new package wrapping the Anthropic Go SDK. Auth via `$ANTHROPIC_API_KEY` or `[ui] tidy_api_key_cmd`.
- `compose.ClaudeTidy` impl. Prompt engineering: system prompt that respects the user's voice, preserves markdown structure, never invents facts, never adds salutations or sign-offs.
- Session caching: identical `(Paragraph, Context, Audience)` triples cached for the session lifetime.
- `Ctrl+G` on a paragraph dispatches; spinner in the review pane while the request is in flight; `Esc` cancels mid-flight.
- Review-mode UI: ComposeTab enters a "tidy review" state. Renders the current paragraph and the proposed rewrite as a unified diff in a panel below the body. Keys: `y`/`Enter` accepts (calls `editor.SetValue(newFullBuffer)`); `n`/`Esc` rejects; `e` edits the suggestion before accepting.

### Tidy obviates grammar check

Local grammar check is indefinitely deferred. The Go grammar-checking landscape is barren: no maintained pure-Go library exists; LanguageTool is a Java daemon; `vale` is style-only and not embeddable. Crucially, Claude tidy already catches grammar, clarity, tone, and structural issues in one pass — and respects markdown, where rule-based grammar tools struggle. Grammar check stays in the "post-1.0, only if needed" bucket; for the foreseeable future, tidy fills the role.

## Reply / Forward seeding

`compose.SeedReply(parent, body, self)`:

1. **Subject**: prefix `Re: ` if not already present; collapse `Re: Re: ` → `Re: ` (case-insensitive on the prefix).
2. **To**: parent's `From`.
3. **In-Reply-To**: parent's `Message-Id`.
4. **References**: parent's `References` chain + parent's `Message-Id`.
5. **Body**: the parent's text/plain body (markdown-friendly), prefix-walked: every line gets one additional `> ` level. Existing `>` prefixes preserved (depth+1). Blank attribution line above: `> On <date>, <sender> wrote:`.
6. **Cursor lands above the attribution.** Bottom-posting under properly attributed quote is the expected idiom for "better Pine."

`SeedReplyAll`: same as `SeedReply` plus parent's `To` and `Cc` minus `self` go into `Cc`.

`SeedForward`:

1. **Subject**: prefix `Fwd: ` if not already present.
2. **To** / **Cc**: empty (user fills).
3. **In-Reply-To** / **References**: empty (forwards start a new thread).
4. **Body**: attribution block + parent body quoted with one level of `> `.

Quoted markdown stays markdown — no transformation. Catkin's renderer applies live styling inside quoted regions per the block classifier (see Quoted Markdown below).

## Quoted markdown rendering

The block classifier produces a context **stack** per line, not a single block kind. A line `> - **important**` resolves to `[quote(1), list]` with inline `**important**` styling on the post-marker text.

Concrete rules:

- **Inline spans inside quotes.** All v1 inline styling (bold, italic, strike, inline code, links, hard-break glyph) applies to the post-prefix content of every quoted line.
- **Block forms inside quotes.** Code fences inside a quote (`> ` followed by ` ``` `, content lines prefixed `> `, then `> ` + ` ``` ` close) render as code-block style on the post-prefix content. Lists inside quotes get list-marker styling on the post-prefix marker. Headings inside quotes (`> # H1`) render heading-style.
- **Reflow inside quotes.** Walks per-line, preserves prefix depth (count of `>` plus their spacing), re-wraps post-prefix content. Fence state is prefix-aware (a fence opened with `> ` ``` ` is closed only by another `> ` ``` `; a non-quoted ` ``` ` does not close it).
- **Depth coloring.** `blockquote_N` palette slot for `N = min(depth, 4)`. The `>` prefix runes themselves render in the depth color; post-prefix text inherits the color faintly so eyes can track depth across paragraphs.

## Long URL display

Two-layer wrap:

- **Source-level reflow** never breaks a single token. A 200-char URL stays on one markdown line, even if that line exceeds `wrapWidth`. Receiving clients parse the URL as one token — correctness over visual.
- **Display-level wrap** soft-breaks long source lines mid-token in `View()` only. Buffer unchanged. Cursor positions interpolate across the soft-wrap.

Bare URL styling: bare URLs (anywhere — paragraph, list item, quoted line) render with link style for the whole URL token. No truncation, no auto-shorten. CommonMark autolink semantics on send: goldmark runs with the autolink extension enabled so the HTML alternative renders bare URLs as clickable `<a>` links.

## Failure surface

Outbox conflict, no toast. Existing infrastructure handles it:

- Drainer marks `OpSend` / `OpAppend` rows as `OpConflict` per the matrix above.
- `cache.Account.Events()` emits `CacheEvent`.
- `App.pumpUpdatesCmd` already handles cache events; `⚠N` segment in the status bar updates.
- `!` opens the conflict overlay; user resolves via `r` (retry — `cache.RetryOp`) or `d` (discard — `cache.DiscardOp`).

For send-specific UX, the conflict overlay row text reads `Send to <recipient>: <error>` for `KindSend`, and `Append to <folder>: <error>` for `KindAppend`. The overlay's existing `r`/`d` semantics apply unchanged.

Connection-state Offline + non-empty outbox emits the existing one-shot ErrorMsg ("offline — queued ops will sync on reconnect"). No new banner type.

## Wireframes

### Compose, body focus, with attachments

```
╭─ poplar ───────────────────────┬─────────────────────────────────────╮
│  Inbox            3            │ Compose — New message                │
│  Drafts                        │                                      │
│  Sent                          │ To:      alice@example.com           │
│  ...                           │ Cc:                                  │
│                                │ Bcc:                                 │
│                                │ Subject: Project update              │
│                                │                                      │
│                                │ ┌ 󰈙 report.pdf  󰈙 chart.png ─────┐  │
│                                │ └─────────────────────────────────┘  │
│                                │                                      │
│                                │ Hi Alice,                            │
│                                │                                      │
│                                │ Quick update on **Q2**:              │
│                                │                                      │
│                                │ - Backend cutover complete           │
│                                │ - Frontend pass starts next sprint   │
│                                │                                      │
│                                │ See [the doc](https://example.com…)  │
│                                │ for details.                         │
│                                │                                      │
│                                │ Geoff                                │
│                                │                                      │
│                                │                       7 words · 89   │
├─ ⇅0  ●  Online  ───────────────┴──────────────────────────────────────┤
│ ^Enter:send  ^S:save  ^T:attach  ^B:bold  ^I:italic  ^K:link  Esc:cancel │
╰──────────────────────────────────────────────────────────────────────╯
```

### Compose, reply mode, after seeding

```
                                  │ Compose — Re: Project update         │
                                  │                                      │
                                  │ To:      alice@example.com           │
                                  │ Cc:                                  │
                                  │ Bcc:                                 │
                                  │ Subject: Re: Project update          │
                                  │                                      │
                                  │ ▌                                    │  cursor here
                                  │                                      │
                                  │ > On 2026-05-04, Alice wrote:        │  blockquote_1
                                  │ >                                    │
                                  │ > > Backend cutover complete         │  blockquote_2
                                  │ > > Frontend pass starts next sprint │
                                  │ >                                    │
                                  │ > Sounds good. Let me know if you    │
                                  │ > need anything from my side.        │
                                  │                                      │
                                  │                       0 words · 0    │
```

### Conflict overlay row for send failure

```
                  ╭─ Conflicts (2) ─────────────────────────────────────╮
                  │                                                     │
                  │  ▶ Send to alice@example.com:                       │
                  │      authentication failed                          │
                  │                                                     │
                  │    Move 4 → Archive: connection refused             │
                  │                                                     │
                  │  r:retry   d:discard   Esc:close                    │
                  │                                                     │
                  ╰─────────────────────────────────────────────────────╯
```

## Out of scope for Pass 9 (the framing pass — ties to sub-passes below)

Deferred entirely to post-1.0 or v1.1:

- **Selection-aware `Ctrl+B` / `Ctrl+I`** (Catkin v1 has no selection model; bare commands wrap the word at cursor).
- **Renumbering ordered lists** when items inserted mid-list.
- **Generic auto-close pairs** (`(` → `()`, `{` → `{}`). Markdown-only auto-pair ships in 9c.
- **Smart quotes / em-dash typography** (`'` → `'`, `--` → `—`). Carve-out + directional inference + apostrophe edge cases push past the v1 budget.
- **Syntax highlighting inside fenced code blocks.**
- **Local grammar check** — no usable pure-Go library; daemon-based options violate constraints. **Claude tidy fills the role**; rule-based grammar check effectively retired from the roadmap.
- **Multi-cursor / undo tree / Hemingway-mode** — wrong shape for prose email compose.
- **Neovim adapter** (v1.1).
- **Runtime editor swap** — config selects at startup only.
- **Receiving HTML mail rendering** (separate effort; viewer currently handles text/plain).

## Right-sized passes

Compose framing is too large for a single poplar pass. Breaking into ten sub-passes mirroring the 8.4a/b/c rhythm. Each sub-pass ends with `make check` green, a tmux capture at 80×24 and 120×40, the standard pass-end ritual (ADRs, invariants update, plan archival, commit + push + install), and produces its own STATUS row.

| Pass | Goal | Rough scope |
|------|------|-------------|
| **9** | **Catkin core** — package skeleton, `bubbles/textarea` wrap, block classifier, reflow engine, basic cursor render (no styling), word-level navigation, scroll-off (3 lines), unit tests for classifier table + reflow round-trip | ~600 LOC |
| **9a** | **Catkin live rendering** — inline span styler (bold/italic/strike/inline-code/link/bare URL/hard-break glyph), block styling (headings/blockquote/code-fence/list markers), depth-graduated blockquote palette (4 slots), long-URL two-layer wrap, golden render-to-string tests | ~400 LOC |
| **9b** | **Catkin markdown commands** — `Ctrl+B`/`I`/`K`/`L`/`Q`, smart Enter (list/quote/task continuation; double-Enter ends block), Tab/Shift-Tab list indent + 2-space insert, task list `Ctrl+Space` toggle, word + char count footer, trailing-WS cleanup on commit | ~500 LOC |
| **9c** | **Catkin power-user QoL** — undo/redo ring buffer (50-step), find/replace overlay (literal + case toggle), markdown auto-pair (six pairs), smart URL paste → `[word](url)`, bracket/span match highlight, typewriter scroll mode, focus mode, `Ctrl+\` mode-cycle key | ~600 LOC |
| **9d** | **Annotation pipeline + spellcheck** — `Editor.SetAnnotations`, Catkin annotation rendering (kind→underline-color), `AnnotationPicker` overlay, `internal/compose/spellcheck.go` (Track 1: pure-Go SymSpell + bundled word lists for `en_US`, `en_GB`, `de_DE`, `fr_FR`, `es_ES`, `es_MX`, `pt_BR`, `pt_PT`, `it_IT`, `nl_NL`, `pl_PL` embedded via `go:embed`, ~12MB; Track 2: subprocess `hunspell -a` for languages outside the bundled set with explicit install-instruction banner), debounce 400ms, span filtering, language-driven startup load | ~700 LOC (binary +~12MB for embeds) |
| **9e** | **`internal/compose/` package** — `Editor` interface, `CatkinEditor` adapter, `Draft` struct, `AssembleMIME` (goldmark integration: text/plain + text/html + multipart/mixed for attachments), `SeedReply` / `SeedReplyAll` / `SeedForward`, unit tests against fixture parents | ~400 LOC |
| **9f** | **Mail backend Send + Append** — `Send` + `Append` on `mail.Backend`; JMAP impl (`Email/submission` + `Email/import`); IMAP-side SMTP via `emersion/go-smtp` on a third connection + IMAP `APPEND`; `[account.smtp]` config block; provider preset SMTP defaults; `poplar config check` SMTP probe | ~700 LOC |
| **9g** | **Cache outbox Send/Append dispatch** — `cache.SendArgs`/`AppendArgs` flesh out, `executeSend`/`executeAppend` in drainer, conflict matrix (no optimistic flip; crashed-mid-execute → conflict per ADR-0116), conflict overlay row text extension for send/append failures | ~300 LOC |
| **9h** | **ComposeTab UI + `c` wiring + tidy seam** — `ComposeTab` tea.Model (header form via `bubbles/textinput`, chip row, focus management, async reply seeding, `Ctrl+T` attachment-add prompt, `Ctrl+Enter`/`Ctrl+S`/`Esc` flows), App-level `OpenComposeMsg` routing from MessageList and Viewer, `compose.Tidy` interface + `NoopTidy` + `Ctrl+G` stubbed handler | ~600 LOC |
| **9i** | **Claude Tidy implementation** — `internal/anthropic/` package (Anthropic SDK wrapper, auth via `$ANTHROPIC_API_KEY` / `[ui] tidy_api_key_cmd`), `compose.ClaudeTidy` impl with prompt engineering (preserve voice, preserve markdown, no fact invention, no salutations), session-scoped response caching, ComposeTab review-mode UI (unified-diff panel, `y`/`n`/`e`/`Esc` keys, in-flight spinner) | ~500 LOC |

**STATUS reconciliation.** The existing Pass 9.5 row in `STATUS.md` previously bundled `#5 (spellcheck) + #12 (tidytext) + #24 (attachments-richer)`. Spellcheck moves to 9d, tidy to 9i. Pass 9.5 collapses to a single coherent goal: **attachments-richer compose UI** (drag-and-drop equivalent, multi-attach, attach-from-cache for #24). Schedule: after 9i.

## Open risks

- **Catkin's render-from-raw approach** is the biggest deviation from "use bubbles' View() output." Mitigation: the classifier and styler are pure functions over the raw buffer, fully unit-testable; no escape-sequence parsing anywhere.
- **`Ctrl+K` overload** (link-skeleton vs. delete-to-EOL) may confuse. Mitigation: dogfood for two weeks post-merge; split if friction emerges.
- **Goldmark size** — adds `yuin/goldmark` as a direct dep. Already lightweight (no CGO, no transitive surprises). Acceptable.
- **SMTP on a third connection** doubles the IMAP backend's connection footprint. Acceptable per the per-account scale poplar targets (one account = one user; not a multi-tenant relay).

## ADRs to write at pass end

- **Catkin renderer ownership** — Catkin owns its `View()` instead of delegating to `bubbles/textarea`. Rationale + boundary.
- **Markdown-first compose** — buffer is CommonMark; live-rendered iA-Writer-style; HTML alternative on send via goldmark.
- **`Editor` interface seam shape** — codify the `tea.Model` + accessor surface so the v1.1 neovim adapter has a contract.
- **Compose attachment model** — Ctrl+T add, chip row when non-empty, no persistent header field. Pass 9.5 will revisit for #24.
- **SMTP backend on third IMAP connection** — connection lifecycle, auth source, reconnect rules.
- **Send / Append outbox semantics** — no optimistic flip, conflict surface, crashed-mid-execute → conflict (extends ADR-0116).
- **Depth-graduated blockquote palette** — 4 slots, post-1.0 may extend.

## Decision-record dependencies

This spec inherits and extends:

- **ADR-0031** — Catkin reusable built-in editor.
- **ADR-0032** — Catkin Ctrl+key, no multi-key.
- **ADR-0033** — two-editor architecture, Editor interface.
- **ADR-0068** — modifier-free reading-UI keybindings (compose is text-entry, exempt per ADR-0076).
- **ADR-0076** — Catkin library foundation.
- **ADR-0084** — icon-mode SPUA-A row composition.
- **ADR-0102** — `password-cmd` resolution on first Connect.
- **ADR-0107** — IMAP idle reconnect / backoff (SMTP reuses).
- **ADR-0108** — Gmail XOAUTH2 via password-cmd.
- **ADR-0116** — outbox terminal classification.
- **ADR-0128** — ConfirmModalYesMsg pattern (replacing callbacks).
- **ADR-0132 / 0133 / 0134** — outbox visibility (`Q`, `!`, status segment).
- **ADR-0138** — viewer attachment chip row pattern.
- **ADR-0143** — JMAP per-folder baseline pull.
