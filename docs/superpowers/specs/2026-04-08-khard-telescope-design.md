# Khard Telescope Picker + Compose Cursor Positioning

Date: 2026-04-08

## Problem

nvim-mail has a khard contact picker (`<C-k>` / `<leader>k`) that
uses `vim.ui.select` to present the full contact list. This requires
scrolling or mouse selection — there is no search or filtering. With
hundreds of contacts, finding the right address is slow.

Separately, new compose messages place the cursor in the body area
below the header separator. For replies this is correct (the
recipient is already populated), but for new compose the cursor
should start on the empty `To:` line since that's the first field
the user needs to fill.

## Solution

### 1. Replace vim.ui.select with Telescope

Add `telescope.nvim` (and its dependency `plenary.nvim`) to
nvim-mail's lazy.nvim plugin list. Replace the `khard_insert`
function's `vim.ui.select` call with a custom Telescope picker.

**Why Telescope over alternatives:**

- Standard neovim fuzzy finder — well-maintained, widely understood
- Live fuzzy filtering as you type — no two-step search-then-pick
- Handles large contact lists efficiently
- Familiar UI for neovim users (many already know the keybindings)
- Alternative considered: `fzf-lua` — lighter but adds an external
  fzf dependency; Telescope is pure Lua

**Picker behavior:**

- `khard email --parsable` is called once when the picker opens
  (same as today)
- Results displayed as `Name <email>` (same format as today)
- User types to fuzzy-filter the list in real time
- Enter inserts the selected contact at the current cursor position
- Escape cancels with no insertion
- Insert-mode binding (`<C-k>`) exits insert, opens Telescope,
  inserts the contact, then returns to insert mode (same flow as
  today)

**Keybindings (unchanged):**

| Key | Mode | Action |
|-----|------|--------|
| `<leader>k` | normal | Open khard Telescope picker |
| `<C-k>` | insert | Open khard Telescope picker (returns to insert) |

**Contact insertion with auto comma separation:**

Inserts at cursor position with smart comma handling. Before
inserting, the logic checks whether the current line already has
an address after the header key (`To:`, `Cc:`, `Bcc:`). If the
line has existing content beyond the header key and trailing
whitespace, `, ` is prepended to the inserted contact.

Examples:
- `To: ` + pick "Alice <a@x.com>" → `To: Alice <a@x.com>`
- `To: Alice <a@x.com>` + pick "Bob <b@x.com>" → `To: Alice <a@x.com>, Bob <b@x.com>`
- `Cc: ` + pick "Carol <c@x.com>" → `Cc: Carol <c@x.com>`

Detection: match the line against `^[A-Za-z-]+:%s*(.+)` — if the
capture is non-empty, prepend `, `. This only applies on header
lines (lines matching `^[A-Za-z-]+:`). In the body, the current
bare-insert behavior is preserved.

**Graceful degradation:**

If khard is not installed or returns no contacts, a warning is shown
via `vim.notify` (same as today). Telescope is a plugin dependency
managed by lazy.nvim — it will be installed automatically on first
launch.

### 2. Smart cursor positioning for new compose

Detect whether the compose buffer is a new message or a reply by
checking the `To:` header content. If the `To:` line is empty
(just `To:` or `To: ` with no address), place the cursor at the end
of that line in insert mode. If the `To:` line has content (reply or
forward), keep the current behavior: cursor in the body between the
header separator and the quoted text.

**Detection logic:**

After the VimEnter buffer preparation (unfold, strip brackets,
refold, reflow, insert separators), scan for the `To:` header line.
If the value after `To:` is empty or whitespace-only, this is a new
compose. Position the cursor at the end of that line and enter
insert mode (append). Otherwise, use the existing body positioning.

**Why check To: rather than quoted text:**

A forward has quoted text but an empty To: — the user still needs to
fill in the recipient first. Checking To: handles both new compose
and forward correctly.

## Files changed

| File | Change |
|------|--------|
| `.config/nvim-mail/init.lua` | Add telescope to lazy.nvim plugins, rewrite `khard_insert` to use Telescope picker, add cursor positioning logic in VimEnter |
| `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua` | Same changes (dual config) |

## Documentation updates

| File | Change |
|------|--------|
| `CLAUDE.md` | Update nvim-mail keybinding docs to note Telescope picker |
| `README.md` | Update nvim-mail section to mention Telescope contact picker, add telescope to prerequisites |
| `~/.claude/docs/aerc-setup.md` | Update Compose section with Telescope picker details and cursor positioning behavior |
| `~/.claude/docs/neovim-setup.md` | Add Telescope plugin to nvim-mail plugin list, document picker implementation details |

## Testing

Manual verification:

1. Open new compose (`C` in aerc) — cursor should be on the `To:`
   line in insert mode
2. Press `<C-k>` — Telescope picker opens with full contact list
3. Type partial name — list filters in real time
4. Press Enter — contact inserted at cursor on the `To:` line
5. Press `<C-k>` again — second contact appended with `, ` separator
6. Verify no comma prepended on first contact (empty `To:` line)
7. Verify no comma prepended when inserting in the body
6. Open a reply (`r` in aerc) — cursor should be in the body area
   as before
7. `<leader>k` in normal mode — same Telescope picker behavior
8. With khard not installed — warning message, no crash
