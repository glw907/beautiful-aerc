# Khard Telescope Picker + Compose Cursor Positioning

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the khard contact picker's scroll-and-select UI with a fuzzy-searchable Telescope picker, add smart comma separation for multiple recipients, and position the cursor on the To: line for new compose messages.

**Architecture:** All changes are in nvim-mail's `init.lua` (Lua config, not Go). Telescope is added as a lazy.nvim plugin. The existing `khard_insert` function is rewritten to use a custom Telescope picker. Cursor positioning logic is added to the existing VimEnter autocmd. Both the project repo and personal dotfiles copies must be updated.

**Tech Stack:** Neovim 0.10+, Lua, lazy.nvim, telescope.nvim, plenary.nvim, khard CLI

**Spec:** `docs/superpowers/specs/2026-04-08-khard-telescope-design.md`

**Dual config requirement:** Every change to `.config/nvim-mail/init.lua` must be applied to BOTH:
- Project repo: `.config/nvim-mail/init.lua`
- Personal dotfiles: `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua`

The dotfiles copy has minor comment differences but the functional code is identical. Apply the same logical change to both. Line numbers in this plan refer to the project repo copy; the dotfiles copy is offset by ~15 lines (more comments at the top).

---

### Task 1: Add Telescope plugin to lazy.nvim

**Files:**
- Modify: `.config/nvim-mail/init.lua:26-34`
- Modify: `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua:34-42`

- [ ] **Step 1: Add telescope.nvim to the lazy.nvim plugin list**

In `.config/nvim-mail/init.lua`, replace the `require("lazy").setup(...)` block (lines 26–34) with:

```lua
require("lazy").setup({
  {
    "shaunsingh/nord.nvim",
    priority = 1000,
    config = function()
      vim.cmd.colorscheme("nord")
    end,
  },
  {
    "nvim-telescope/telescope.nvim",
    dependencies = { "nvim-lua/plenary.nvim" },
  },
}, { ui = { border = "none" } })
```

- [ ] **Step 2: Apply same change to dotfiles copy**

Apply the identical plugin list change to `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua` (lines 34–42). The `require("lazy").setup(...)` block is identical in both files.

- [ ] **Step 3: Verify Telescope loads**

Open nvim-mail manually to confirm lazy.nvim installs Telescope on first launch:

```bash
NVIM_APPNAME=nvim-mail nvim /tmp/test-mail.txt
```

Run `:Telescope` in command mode — it should open the Telescope builtin picker. Close with `:q!`.

- [ ] **Step 4: Commit**

```bash
cd ~/Projects/beautiful-aerc
git add .config/nvim-mail/init.lua
git commit -m "Add telescope.nvim to nvim-mail plugin list"
```

---

### Task 2: Rewrite khard picker to use Telescope

**Files:**
- Modify: `.config/nvim-mail/init.lua:605-658` (khard section)
- Modify: `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua:621-678` (same section, offset)

- [ ] **Step 1: Replace the khard_insert function and keybindings**

Replace lines 605–658 in `.config/nvim-mail/init.lua` (the entire khard section from the comment block through the keybinding definitions) with:

```lua
-- khard contact picker (Telescope)
--
-- khard is a CLI address book that syncs contacts from CardDAV (e.g.,
-- Fastmail). The picker calls `khard email --parsable` to get a tab-separated
-- list of addresses and presents them via Telescope for fuzzy filtering.
--
-- On header lines (To:, Cc:, Bcc:), if the line already has an address,
-- ", " is prepended to the inserted contact for proper RFC 2822 formatting.
-- In the body, the contact is inserted bare at the cursor position.
--
-- This is optional — if khard is not installed or has no contacts, a warning
-- is shown and nothing is inserted. To set up khard:
--   apt install khard vdirsyncer
--   configure vdirsyncer to sync your CardDAV contacts
--   run `vdirsyncer sync && khard` to verify
--
-- Keybindings:
--   <leader>k — insert contact address at cursor (normal mode)
--   <C-k>     — insert contact address at cursor (insert mode; returns to insert)
local function khard_pick(reenter_insert)
  local raw = vim.fn.systemlist("khard email --parsable 2>/dev/null")
  local entries = {}
  for _, line in ipairs(raw) do
    local email, name = line:match("^([^\t]+)\t([^\t]*)")
    if email then
      name = name and name:match("^%s*(.-)%s*$") or ""
      local label = name ~= "" and (name .. " <" .. email .. ">") or email
      entries[#entries + 1] = label
    end
  end
  if #entries == 0 then
    vim.notify("No khard contacts found", vim.log.levels.WARN)
    if reenter_insert then vim.cmd("startinsert") end
    return
  end

  local pickers = require("telescope.pickers")
  local finders = require("telescope.finders")
  local conf = require("telescope.config").values
  local actions = require("telescope.actions")
  local action_state = require("telescope.actions.state")

  pickers.new({}, {
    prompt_title = "Insert contact",
    finder = finders.new_table({ results = entries }),
    sorter = conf.generic_sorter({}),
    attach_mappings = function(prompt_bufnr)
      actions.select_default:replace(function()
        local selection = action_state.get_selected_entry()
        actions.close(prompt_bufnr)
        if not selection then
          if reenter_insert then vim.cmd("startinsert") end
          return
        end

        local contact = selection[1]
        local pos = vim.api.nvim_win_get_cursor(0)
        local buf_line = vim.api.nvim_buf_get_lines(0, pos[1] - 1, pos[1], false)[1]

        -- Auto-prepend ", " on header lines that already have an address
        local header_value = buf_line:match("^[A-Za-z-]+:%s*(.+)")
        if header_value then
          contact = ", " .. contact
        end

        local new_line = buf_line:sub(1, pos[2]) .. contact .. buf_line:sub(pos[2] + 1)
        vim.api.nvim_buf_set_lines(0, pos[1] - 1, pos[1], false, { new_line })
        vim.api.nvim_win_set_cursor(0, { pos[1], pos[2] + #contact })
        if reenter_insert then vim.cmd("startinsert") end
      end)
      return true
    end,
  }):find()
end

vim.keymap.set("n", "<leader>k", function() khard_pick(false) end,
  { desc = "Insert khard contact" })
vim.keymap.set("i", "<C-k>", function()
  vim.cmd("stopinsert")
  vim.schedule(function() khard_pick(true) end)
end, { desc = "Insert khard contact (insert mode)" })
```

- [ ] **Step 2: Apply same change to dotfiles copy**

Replace the corresponding khard section in `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua` (lines 621–678) with the identical code from Step 1.

- [ ] **Step 3: Test the Telescope picker manually**

Open a compose in aerc. On the `To:` line, press `<C-k>`. Verify:
- Telescope picker opens with full contact list
- Typing filters the list in real time
- Enter inserts the selected contact
- Escape cancels with no insertion

- [ ] **Step 4: Test comma separation**

On the `To:` line with an existing contact, press `<C-k>` and pick a second contact. Verify:
- `, ` is prepended before the second contact
- No comma on first contact (empty `To:` line)
- No comma when inserting in the message body via `<leader>k`

- [ ] **Step 5: Test graceful degradation**

Temporarily rename the khard binary and press `<C-k>`. Verify a warning appears and no crash occurs. Restore the binary.

- [ ] **Step 6: Commit**

```bash
cd ~/Projects/beautiful-aerc
git add .config/nvim-mail/init.lua
git commit -m "Replace khard vim.ui.select with Telescope fuzzy picker

Add auto comma separation when inserting multiple recipients on
header lines. Body insertions remain bare."
```

---

### Task 3: Smart cursor positioning for new compose

**Files:**
- Modify: `.config/nvim-mail/init.lua:328-340` (cursor positioning in VimEnter)
- Modify: `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua:343-355` (same section, offset)

- [ ] **Step 1: Replace cursor positioning logic**

In `.config/nvim-mail/init.lua`, replace lines 328–340 (from the `-- Insert blank lines` comment through `vim.cmd("startinsert")`) with:

```lua
      -- Insert blank lines after the header block so the cursor lands in
      -- empty space between the bottom separator and any quoted text.
      vim.api.nvim_buf_set_lines(0, header_end, header_end, false, { "", "", "" })

      -- Smart cursor placement: new compose (empty To:) lands on the To:
      -- line; replies and forwards (To: populated) land in the body.
      local to_line_nr = nil
      local to_empty = false
      for i = 3, header_end - 1 do
        local l = vim.api.nvim_buf_get_lines(0, i - 1, i, false)[1]
        if l:match("^To:") then
          to_line_nr = i
          to_empty = not l:match("^To:%s*%S")
          break
        end
      end

      if to_empty and to_line_nr then
        -- New compose or forward: cursor at end of To: line
        local to_line = vim.api.nvim_buf_get_lines(0, to_line_nr - 1, to_line_nr, false)[1]
        vim.api.nvim_win_set_cursor(0, { to_line_nr, #to_line })
      else
        -- Reply: cursor in body between separator and quoted text
        vim.api.nvim_win_set_cursor(0, { header_end + 2, 0 })
      end
    end

    vim.cmd("startinsert")
```

This replaces the vestigial if/else that did the same thing in both branches.

- [ ] **Step 2: Apply same change to dotfiles copy**

Apply the identical cursor positioning change to `~/.dotfiles/beautiful-aerc/.config/nvim-mail/init.lua` (lines 343–355). The code is identical; only the line numbers differ.

- [ ] **Step 3: Test new compose cursor positioning**

In aerc, press `C` to compose a new message. Verify:
- Cursor is on the `To:` line at the end (after `To: `)
- Neovim is in insert mode
- `<C-k>` immediately opens the Telescope contact picker
- After picking a contact, cursor moves to end of the inserted text

- [ ] **Step 4: Test reply cursor positioning**

In aerc, press `r` to reply to a message. Verify:
- Cursor is in the body area between the header separator and quoted text (existing behavior preserved)
- Neovim is in insert mode

- [ ] **Step 5: Test forward cursor positioning**

In aerc, press `f` to forward a message. Verify:
- The `To:` line is empty, so cursor lands on the `To:` line (same as new compose)

- [ ] **Step 6: Commit**

```bash
cd ~/Projects/beautiful-aerc
git add .config/nvim-mail/init.lua
git commit -m "Position cursor on To: line for new compose and forward

Replies land in the body as before. Detection checks whether the
To: header has content after buffer preparation."
```

---

### Task 4: Update documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `~/.claude/docs/aerc-setup.md`
- Modify: `~/.claude/docs/neovim-setup.md`

- [ ] **Step 1: Update CLAUDE.md**

The CLAUDE.md does not currently document nvim-mail keybindings in detail, but it does reference nvim-mail in the tidytext section. No change needed unless the Compose Settings or Custom Keybindings sections mention khard. Search for "khard" in CLAUDE.md — if absent, skip.

- [ ] **Step 2: Update README.md**

In the `### nvim-mail` section (around line 186), add Telescope contact picker to the feature list. Replace the current bullet list:

```markdown
A dedicated Neovim profile for composing email in aerc. It provides:

- Custom `aercmail` syntax highlighting (header keys, address fields, quoted text)
- Hard-wrap at 72 characters with RFC 3676 format=flowed support
- Spell check on body text, skipping headers and quoted lines
- Telescope-powered contact picker with fuzzy search (`<C-k>` in insert mode, `<leader>k` in normal mode) — requires [khard](https://github.com/lucc/khard) with CardDAV contacts synced via vdirsyncer
- tidytext integration via `<leader>t`
- Address header reformatting and quoted text reflow on buffer open
- Smart cursor positioning: new compose and forward land on the `To:` line; replies land in the body
- Signature insertion via `<leader>sig` — copy `signature.md.example` to `signature.md` and edit it
```

Also add `telescope.nvim` to the Optional prerequisites section. It is a lazy.nvim dependency so it auto-installs, but it's worth noting. No — actually, lazy.nvim handles it transparently. Don't add it to prerequisites. The khard prerequisite already exists on line 29.

- [ ] **Step 3: Update `~/.claude/docs/aerc-setup.md`**

Find the "Compose Tab Title" or "Compose Settings" section. Add a new subsection after the keybindings:

```markdown
### Contact Picker (Telescope + khard)

nvim-mail provides a fuzzy-searchable contact picker powered by
Telescope and khard. Press `<C-k>` in insert mode (or `<leader>k`
in normal mode) to open the picker. Type to filter contacts in
real time, Enter to insert, Escape to cancel.

On header lines (To:, Cc:, Bcc:), the picker auto-prepends `, `
when the line already has a recipient. In the body, the contact
is inserted bare at the cursor position.

Requires khard with contacts synced via vdirsyncer. If khard is
not installed or returns no contacts, a warning is shown.

### Cursor Positioning

New compose and forward messages place the cursor on the empty
`To:` line in insert mode, ready for recipient entry. Replies
place the cursor in the body between the header separator and
quoted text, ready to type the response.

Detection: after buffer preparation, the VimEnter autocmd checks
whether the `To:` header has content. Empty `To:` = new compose
or forward; populated `To:` = reply.
```

- [ ] **Step 4: Update `~/.claude/docs/neovim-setup.md`**

Find the `### Plugins` subsection under `## nvim-mail Profile`. Update it:

```markdown
### Plugins

- **nord.nvim** — colorscheme
- **telescope.nvim** — fuzzy contact picker for khard address book
  (with plenary.nvim dependency, auto-installed by lazy.nvim)
- **nvim-treesitter** — markdown highlighting
```

Also find the `### Keybindings summary` table and update the khard entries:

```markdown
| `<leader>k` | n | Open khard Telescope contact picker | 653 |
| `<C-k>` | i | Open khard Telescope contact picker (returns to insert) | 655–658 |
```

(Update line numbers after implementation is complete.)

Add a new subsection after the keybindings:

```markdown
### Contact Picker Implementation

The khard contact picker uses Telescope for fuzzy filtering. The
flow:

1. `khard email --parsable` is called to get a tab-separated list
   of email addresses and names
2. Results are formatted as `Name <email>` and passed to a custom
   Telescope picker
3. User types to fuzzy-filter, Enter to select, Escape to cancel
4. On selection, the contact is inserted at the cursor position

**Auto comma separation:** On header lines (To:, Cc:, Bcc:), if
the line already has content after the header key, `, ` is
prepended to the inserted contact. Detection uses
`^[A-Za-z-]+:%s*(.+)` — if the capture is non-empty, the line
already has an address. Body insertions are always bare.

**Graceful degradation:** If khard is not installed or has no
contacts, `vim.notify` shows a warning and no insertion occurs.
Telescope is a lazy.nvim dependency that auto-installs on first
launch.

### Cursor Positioning Logic

The VimEnter autocmd positions the cursor differently for new
compose vs replies:

1. After buffer preparation (unfold, strip brackets, refold,
   reflow, insert separators), the autocmd scans for the `To:`
   header line
2. If `To:` is empty or whitespace-only (new compose or forward),
   the cursor is placed at the end of the `To:` line in insert
   mode — ready for recipient entry or `<C-k>` contact picking
3. If `To:` has content (reply), the cursor is placed in the body
   between the header separator and quoted text (existing behavior)

**Why check To: rather than quoted text:** A forward has quoted
text but an empty To: — the user still needs to fill in the
recipient first. Checking To: handles both new compose and
forward correctly.
```

- [ ] **Step 5: Commit**

```bash
cd ~/Projects/beautiful-aerc
git add README.md CLAUDE.md
git commit -m "Update docs for Telescope contact picker and cursor positioning"
```

Note: `~/.claude/docs/aerc-setup.md` and `~/.claude/docs/neovim-setup.md` are outside the repo — they are personal reference docs, not committed.
