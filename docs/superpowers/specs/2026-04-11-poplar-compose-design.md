# Poplar Compose System Design

Spec for the poplar compose experience: inline editing in the right
panel with sidebar and chrome visible, Catkin (the built-in editor)
for v1, and neovim embedding via `--embed` RPC for v1.1.

## Motivation

Every terminal email client today shells out to an external editor
via full terminal takeover. The sidebar disappears, the chrome
vanishes, context is gone. Poplar keeps the compose experience
inline — the sidebar stays visible, the status bar shows compose
state, the footer shows compose keybindings. This is the
differentiating feature that makes poplar a showcase TUI email
client.

## Two-Editor Architecture

| Editor | Version | How it works | Target user |
|--------|---------|-------------|-------------|
| Catkin (default) | v1 | Extended textarea with pico-level editing | Everyone |
| Neovim | v1.1 | `nvim --embed` RPC grid renderer | Power users |

Both editors render in the right panel's editor region. The compose
panel, header region, lifecycle, and send pipeline are shared. An
`Editor` interface abstracts the boundary so the neovim
implementation slots in without touching any compose infrastructure.

Config selects the editor:

```toml
# ~/.config/poplar/config.toml
editor = "catkin"  # default; "nvim" enables neovim embedding (v1.1)
```

## Compose Panel Layout

When the user presses `c` (compose), `r` (reply), or `f` (forward),
the right panel switches from message list/viewer to the compose
view. The sidebar stays. All chrome (top line, status bar, footer)
stays and shows compose-appropriate content.

The compose panel has two vertical regions:

```
┌─ Header Region (bubbletea native) ──────────────┐
│ From:  geoff@907.life                            │
│ To:    ▏                                         │
│ Cc:                                              │
│ Subj:                                            │
│──────────────────────────────────────────────────│
├─ Editor Region ──────────────────────────────────│
│                                                   │
│  Body text goes here...                           │
│                                                   │
│  > quoted text from original message              │
│                                                   │
│  Inline reply here...                             │
│                                                   │
│  > more quoted text                               │
│                                                   │
│  --                                               │
│  **Geoffrey L. Wright**                           │
│                                                   │
└───────────────────────────────────────────────────┘
```

### Header Region

Native bubbletea component. Not part of the editor.

- **Fields:** From (pre-filled, read-only for single account), To,
  Cc, Bcc, Subject
- **Navigation:** Tab/Shift-Tab between fields
- **Contact picker:** Ctrl+K opens khard fuzzy picker via
  bubbletea (same UX as nvim-mail's Telescope picker, native
  implementation)
- **Auto comma:** appends `, ` when a field already has content
- **Visual continuity:** same background as the editor region,
  subtle separator line between them — looks like one continuous
  message but is structurally separate

### Editor Region

Holds the `Editor` interface implementation. Receives only body
text: user prose, quoted text (for reply/forward), and signature.
No headers.

### Focus

- **New compose / forward:** focus starts on To field (empty)
- **Reply:** focus starts in editor (To is filled)
- Keybinding moves focus between header region and editor region

## Editor Interface

The `Editor` interface is the seam between the compose panel and
the editor implementation. Designed in v1 to accept the neovim
RPC implementation in v1.1.

```go
type Editor interface {
    Update(tea.Msg) (Editor, tea.Cmd)
    View() string
    Content() string
    SetContent(string)
    Focus()
    Blur()
    Focused() bool
    Resize(width, height int)
}
```

Both Catkin and the future neovim editor satisfy this
interface. The compose panel calls only these methods — it never
reaches into implementation details.

## Catkin: The Built-in Editor (v1)

Catkin is poplar's built-in compose editor — a lightweight editor
highly optimized for email, built as a premier bubbletea component.
Named after the fuzzy flower clusters that poplar trees produce.
Works out of the box with no configuration.

### Design Philosophy

Catkin is a reusable bubbletea component, not a poplar internal.
It should be importable by any bubbletea application that needs a
text editor — the email-specific features (reflow, quote handling,
tidytext) are layered on top via poplar's compose panel, not
baked into the core editor.

**Package structure:**

- `catkin/` — core editor component (standalone, no poplar
  dependencies). Manages its own key sequences internally.
- Poplar's compose panel wraps Catkin and adds email-specific
  behavior (quote-aware reflow, tidytext, spellcheck, signature)

Catkin is not modal, but its command vocabulary is vim-flavored.
Users who know vim motions feel at home; users who don't can
discover commands through the footer hints and `?` help. The goal
is a non-modal editor where vim-literate users never reach for a
manual.

### Core Editing

- Insert, delete, backspace, word-delete (`dw`-style)
- Word-level navigation (`w`/`b` or Ctrl+Left/Right)
- Line start/end (`0`/`$` or Home/End)
- Paragraph-level cut/paste (`dd`-paragraph / `p`) — a
  "paragraph" is consecutive non-blank lines at the same quote
  depth
- Line-level cut (secondary binding)
- Selection support for cut operations
- Undo/redo (`u` / Ctrl+R)
- Search (`/`) with `n`/`N` for next/previous match
- Jump to top/bottom (`gg`/`G`)

### Mail-Specific Features

- **Markdown syntax highlighting:** headings, bold, italic, links,
  list markers, code spans — styled from the compiled theme
- **Insert/toggle signature:** keybinding inserts signature at
  bottom of body
- **Reflow paragraph:** re-wrap current paragraph to 72 characters
- **Reflow quoted block:** re-wrap respecting `> ` prefixes at
  each nesting level
- **Quote/unquote lines:** add or remove `> ` prefix from selected
  lines
- **Attach file:** path prompt, adds attachment
- **Attachment warning:** on send, warn if body mentions "attach"
  with no attachments
- **Jump to signature:** quick navigation to `-- ` separator
- **Jump between quote blocks:** next/previous quote boundary

### Prose Quality

- **Spellcheck:** inline highlights on misspelled words, navigate
  between errors. On send, exit-time prompt: "N misspelled words:
  (s)pellcheck, (y)es send, (n)o stay"
- **Tidytext:** built-in Claude Haiku prose tidier. Keybinding
  runs tidytext on prose blocks only (skips quoted text and
  signature). Changed regions get a subtle theme-derived highlight
  (underline). First keystroke anywhere clears all highlights.
  Toast: "Tidied: N changes"

### Status Indicators

- Word count in status bar
- Compose format (markdown/plaintext) from config
- Modified indicator

### Footer

Context-appropriate keybinding hints grouped by function. The `?`
help popover has the complete reference.

## Neovim Embedding (v1.1)

When the user sets `editor = "nvim"` in config, the compose panel's
editor region runs an embedded neovim instance.

### Architecture

1. Poplar spawns `nvim --embed` with poplar-embed plugin config
2. Connects via msgpack-RPC over stdin/stdout using
   `neovim/go-client`
3. Calls `AttachUI(width, height, {ext_linegrid: true, rgb: true})`
4. A goroutine runs `nv.Serve()`, forwarding redraw events to
   bubbletea via channel → `tea.Cmd`
5. Poplar loads compose body into a buffer via
   `nv.SetBufferLines()`

### Rendering

- `hl_attr_define` events map highlight IDs to `lipgloss.Style`
  (foreground, background, bold, italic, underline from RGB attrs)
- `grid_line` events update an `[][]Cell` grid (rune + highlight ID)
- `grid_cursor_goto` tracks cursor position
- On `flush`, `View()` walks the grid and renders styled text

### Input

- When editor is focused, `tea.KeyMsg` translates to neovim key
  notation and sends via `nv.Input()`
- When header region is focused, neovim receives nothing

### Resize

`nv.TryResizeUI(width, height)` on panel size changes.

### poplar-embed nvim plugin

Coordination plugin loaded by the embedded neovim instance:

- Sets filetype, loads syntax highlighting
- Signals save/abort back to poplar via RPC
- Suppresses nvim statusline/cmdline (poplar's chrome handles it)
- Exposes poplar commands (`:PoplarSend`, `:PoplarAttach`)

### Shutdown

On send or abort, poplar calls `nv.Command("qa!")` and closes the
RPC connection.

## Compose Format

Set in config, not toggled at compose time:

```toml
compose-format = "markdown"  # default; or "plaintext"
```

### Markdown Mode (default)

The editor shows markdown source with syntax highlighting. On send,
the body is converted to clean semantic HTML via goldmark and sent
as `multipart/alternative` (text/plain + text/html).

### Plain Text Mode

The editor shows raw text with no syntax highlighting. On send, the
body is re-wrapped with RFC 3676 `format=flowed` markers and sent
as `text/plain`.

## Inline Replying

The editor supports interleaved prose and quoted blocks at arbitrary
nesting depths:

```
> Original paragraph one about the budget.

My response to the budget point.

> Original paragraph two about the timeline.
> It spans multiple lines and needs to
> stay together as a block.

My response to the timeline point.

> > Nested quote from an earlier message.

And a response to that too.
```

Quote prefixes (`> `, `> > `) are literal characters in the text.
The user navigates freely, placing their cursor between any quoted
blocks to type responses. Both editors treat `> ` as plain text
during editing.

## Send Pipeline

### Block Parsing

On send, the body is parsed into an ordered sequence of blocks:

- **Prose blocks:** consecutive lines with no `> ` prefix
- **Quote blocks:** consecutive lines at the same `> ` nesting
  depth

The interleaved order is preserved exactly.

### Markdown Mode Output

Each prose block is converted to HTML via goldmark. All valid
markdown passes through without opinions — headings at any level,
bold, italic, links, lists, code, horizontal rules, bare URL
auto-linking.

Quoted blocks become nested `<blockquote><p>...</p></blockquote>`
elements matching the `> ` nesting depth.

Signature is separated by `<p>-- </p>`.

The HTML output is squeaky clean:

```html
<p>My response to the budget point.</p>

<blockquote>
<p>Original paragraph two about the timeline. It spans
multiple lines and stays together.</p>
</blockquote>

<p>My response to the timeline point.</p>

<blockquote>
<blockquote>
<p>Nested quote from an earlier message.</p>
</blockquote>
</blockquote>

<p>And a response to that too.</p>

<p>-- </p>
<p><strong>Geoffrey L. Wright</strong></p>
```

Principles:
- Semantic elements only: `<p>`, `<blockquote>`, `<strong>`, `<em>`,
  `<a>`, `<ul>`, `<ol>`, `<li>`, `<code>`, `<h1>`–`<h6>`, `<hr>`
- No `<div>`, `<span>`, `style=` attributes, or CSS classes
- Nested blockquotes via nesting elements, not visual hacks
- Clean indentation in the HTML source — human-readable
- UTF-8 throughout, no unnecessary HTML entities
- `multipart/alternative` with a faithful plain text part

### Plain Text Mode Output

Each prose block is re-wrapped with `format=flowed` markers per
RFC 3676. Quoted blocks are re-wrapped with `> ` prefixes preserved
at each nesting level. Sent as `text/plain; format=flowed`.

### Email Headers

Standards-compliant RFC 2822:

```
Content-Type: multipart/alternative; boundary="poplar-xxxx"
MIME-Version: 1.0
From: Name <email>
To: recipient@example.com
Subject: Re: Meeting Notes
Date: Fri, 11 Apr 2026 14:30:00 -0500
In-Reply-To: <original-message-id@example.com>
References: <original-message-id@example.com>
Message-ID: <generated-uuid@domain>
User-Agent: Poplar/0.1
```

Proper threading via `In-Reply-To` and `References`. Clean
`Message-ID` generation.

## The Virtuous Cycle

Poplar's clean HTML output combined with mailrender's HTML cleanup
on the receiving end creates a virtuous cycle:

1. Poplar sends clean semantic HTML
2. Recipient sees well-rendered email in any client
3. Recipient replies — their client wraps in `<blockquote>` (with
   typical cruft)
4. mailrender's CleanHTML strips cruft, normalizes blockquotes,
   flattens layout tables
5. Poplar compose opens — `reflow.go` re-wraps quoted text at
   consistent `> ` prefixes
6. User replies inline — clean interleaved prose and quotes
7. Poplar sends clean HTML again

Each round trip through poplar/mailrender scrubs away formatting
debris from other clients. Conversations get cleaner over time.

## Compose Lifecycle

### 1. Trigger

User presses `c` (compose), `r` (reply), or `f` (forward). Right
panel switches to compose view.

### 2. Prepare

- Header region populates from context (reply fills To/Subject,
  forward fills Subject, compose is blank)
- Body prepared: quoted text reflowed via `compose.Reflow()`,
  signature appended
- Editor loads prepared body text

### 3. Edit

User works freely in both regions. Tab between header fields,
keybinding to move between header region and editor. All editing
features available.

### 4. Send

- User triggers send
- Validation: warn if To empty, Subject empty, "attach" without
  attachments
- Process body per compose-format config
- `backend.Send()` transmits, copy to Sent folder
- Toast: "Message sent" — right panel returns to message list

### 5. Abort

- User triggers abort
- Confirmation prompt if body modified
- Toast: "Compose discarded" — right panel returns

### 6. Draft

- Postpone saves to Drafts folder
- Resumable: opening a draft loads headers and body back into
  compose view

## Implementation Sequencing

### Pass 9 (v1): Compose + Send

- Header region (bubbletea native, contact picker)
- `Editor` interface (designed for future neovim implementation)
- Catkin editor (extended textarea, all features listed above)
- Compose lifecycle (prepare, edit, validate, send, abort, draft)
- Send pipeline (block parser, goldmark HTML, format=flowed plain)
- Inline reply support
- Spellcheck and tidytext integration

### Post-v1 (1.1): Neovim Embedding

- `neovim/go-client` integration
- RPC grid renderer (ext_linegrid → lipgloss cell grid)
- Key translation layer (tea.KeyMsg → nvim notation)
- poplar-embed nvim plugin
- Config: `editor = "nvim"`

The `Editor` interface is the seam. Pass 9 builds Catkin and the
full compose infrastructure. The v1.1 pass adds the neovim
implementation behind the same interface.
