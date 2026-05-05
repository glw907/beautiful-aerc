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

## Out of scope for Pass 9

Deferred to Pass 9.5 or post-1.0:

- **Selection-aware Ctrl+B / Ctrl+I** (Catkin v1 has no selection; bare commands wrap the word at cursor).
- **Undo / redo** (post-1.0; Catkin's snapshot ring buffer per ADR-0076 lands later).
- **Renumbering ordered lists** when items inserted mid-list.
- **Auto-close pairs** (`(` → `()`).
- **Smart quotes / em-dash** typography (`--` → `—`).
- **Syntax highlighting** inside fenced code blocks.
- **Focus mode / typewriter scroll**.
- **Spellcheck** (#5).
- **Tidytext** (#12).
- **Compose-side attachments inline-add UI** beyond Ctrl+T (#24 layers richer attachment management in Pass 9.5).
- **Neovim adapter** (`v1.1`).
- **Runtime editor swap** — config selects at startup only.
- **Receiving HTML mail** rendering (separate effort; viewer currently renders text/plain only).

## Implementation phasing

Plan-doc-level breakdown (writing-plans skill produces the executable plan):

1. **Catkin foundation** — package skeleton, textarea wrap, block classifier, reflow engine, basic `View()` with cursor (no live styling yet). Tests: classifier table, reflow round-trip.
2. **Catkin live rendering** — inline span styler, block styling, depth-graduated quotes, hard-break glyph, long-URL two-layer wrap. Tests: render-to-string golden files.
3. **Catkin dispatch** — Ctrl+key commands, smart Enter, Tab/Shift-Tab, word/char count.
4. **`internal/compose/`** — Editor interface, CatkinEditor adapter, Draft, AssembleMIME (goldmark integration), SeedReply/SeedReplyAll/SeedForward.
5. **Mail backend extensions** — `Send` + `Append` on the interface; JMAP impl (`Email/submission` + `Email/import`); IMAP impl (SMTP via `emersion/go-smtp` on a third connection; IMAP `APPEND` for append).
6. **Cache outbox dispatch** — `executeSend` / `executeAppend`, conflict matrix, no optimistic flip.
7. **ComposeTab UI** — header form, chip row, focus management, async reply seeding, Ctrl+T path prompt, send / save / cancel flows.
8. **`c` key wiring** — MessageList and Viewer emit `OpenComposeMsg`; App roots ComposeTab.
9. **Theme + config** — blockquote palette slots, `[account.smtp]` config block, provider preset SMTP defaults, `poplar config check` SMTP probe.
10. **Conflict overlay text** — extend the existing overlay's per-row text to render send/append failures cleanly.

Each phase ends with `make check` green and a tmux capture at 80×24 and 120×40.

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
