-- nvim-mail: dedicated Neovim profile for composing email in aerc.
--
-- This profile is isolated via NVIM_APPNAME=nvim-mail, which gives it its own
-- config, data, and plugin directories separate from any general-purpose Neovim
-- setup. Launched by the nvim-mail wrapper script (~/.local/bin/nvim-mail).
-- aerc calls nvim-mail as its compose editor (set in aerc.conf: editor=nvim-mail).
-- The buffer opens with the RFC 2822 message (headers + body) pre-populated by aerc.

-- Leader key — used as the prefix for all custom keybindings below
vim.g.mapleader = " "

-- Bootstrap lazy.nvim
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({
    "git", "clone", "--filter=blob:none",
    "https://github.com/folke/lazy.nvim.git",
    "--branch=stable", lazypath,
  })
end
vim.opt.rtp:prepend(lazypath)

-- Plugins
-- Only nord.nvim is needed. Treesitter for markdown is built into Neovim 0.10+
-- as a bundled parser, so no treesitter plugin is required.
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
  {
    "folke/which-key.nvim",
    event = "VeryLazy",
  },
}, { ui = { border = "none" } })

-- Editor settings
-- These are tuned for prose email composition, not code editing.

-- Hard wrap at 72 columns — the standard for email body text.
-- Auto-wrap and reflow; no 'a' flag (avoids reformatting on every keystroke).
-- Pairs with aerc's format-flowed=true for RFC 3676 reflow on send.
vim.opt.wrap = true
vim.opt.linebreak = true
vim.opt.textwidth = 72
vim.opt.formatoptions = "tcrqwn"

-- Only break at spaces and tabs, never at punctuation characters.
-- The default breakat includes !@*-+;:,./?  which would split words at
-- characters like "!" — e.g., "morning!" would break between "n" and "!".
vim.opt.breakat = " \t"

-- Spell check enabled by default for email composition.
-- Excluded from headers and quoted text via aercmail.vim syntax clusters
-- so the spell checker only flags words in the author's own body text.
vim.opt.spell = true
vim.opt.spelllang = "en_us"

-- Minimal UI — no line numbers, sign column, mode indicator, or status line.
-- These are noise for a focused compose window.
vim.opt.number = false
vim.opt.relativenumber = false
vim.opt.signcolumn = "no"
vim.opt.showmode = false
vim.opt.laststatus = 0
vim.opt.cursorline = false

-- Auto-indent keeps indentation level when starting a new line.
vim.opt.autoindent = true
vim.opt.smartindent = true

-- breakindent: visually indent soft-wrapped continuation lines.
-- shift:2 indents them by 2 spaces, which visually aligns wrapped lines
-- under the text of quoted blocks ("> ..." lines).
vim.opt.breakindent = true
vim.opt.breakindentopt = "shift:2"

-- No swap file — email compose buffers are short-lived and ephemeral.
vim.opt.swapfile = false

-- Enable 24-bit color so Nord theme hex values render correctly.
vim.opt.termguicolors = true

-- Quote reflow
--
-- When aerc opens a reply, the quoted text from the original message often
-- has irregular line breaks from the sender's client. reflow_quoted() fixes
-- this by joining consecutive quoted lines at the same quote level into
-- paragraphs, then re-wrapping each paragraph at 72 columns with a
-- consistent "> " prefix. This runs once on buffer open via VimEnter.
--
-- Blank quoted lines are preserved as paragraph breaks. Lines with only
-- decorative characters (e.g., "--- Original Message ---") are truncated to
-- the terminal width and emitted as-is without joining.

-- Extract the raw quote prefix from a line (e.g., "> > " or ">").
local function get_quote_prefix(line)
  return line:match("^(>[ >]*%s?)")
end

-- Normalize a prefix to just its ">" characters for level comparison.
-- ">> " and "> > " are both level 2 — this strips spaces for comparison.
local function normalize_prefix(prefix)
  if not prefix then return nil end
  return prefix:gsub("[^>]", "")
end

-- Word-wrap `text` to fit within `width` columns, prefixed by `prefix`.
-- Returns a list of wrapped lines. Each line starts with `prefix`.
-- Breaks only at spaces — never mid-word.
local function wrap_text(text, prefix, width)
  local result = {}
  local avail = width - vim.fn.strdisplaywidth(prefix)
  while vim.fn.strdisplaywidth(text) > avail do
    local chars = vim.fn.split(text, [[\zs]])
    local break_idx = nil
    local w = 0
    for ci, ch in ipairs(chars) do
      w = w + vim.fn.strdisplaywidth(ch)
      if w > avail then break end
      if ch == " " then break_idx = ci end
    end
    if not break_idx then break end
    local first = table.concat(chars, "", 1, break_idx)
    result[#result + 1] = prefix .. first:gsub("%s+$", "")
    text = table.concat(chars, "", break_idx + 1):gsub("^%s+", "")
  end
  if #text > 0 then
    result[#result + 1] = prefix .. text
  end
  return result
end

-- Reflow all quoted blocks in `lines`, returning the updated line list.
-- Consecutive quoted lines at the same level are joined into one paragraph
-- and re-wrapped. Quote levels are identified by the number of ">" characters.
local function reflow_quoted(lines, width)
  local result = {}
  local i = 1
  while i <= #lines do
    local prefix = get_quote_prefix(lines[i])
    if prefix then
      local level = normalize_prefix(prefix)
      -- Build canonical prefix: "> " for single, "> > " for nested, etc.
      local canon = string.rep("> ", #level - 1) .. "> "
      local text = lines[i]:sub(#prefix + 1)
      -- Blank quoted line = paragraph break, emit and advance
      if text:match("^%s*$") then
        result[#result + 1] = canon:gsub("%s+$", "")
        i = i + 1
      -- Decorative line (no letters/digits) = truncate to width, emit standalone
      elseif not text:match("[%w]") then
        local chars = vim.fn.split(text, [[\zs]])
        local avail = width - vim.fn.strdisplaywidth(canon)
        local truncated = {}
        local w = 0
        for _, ch in ipairs(chars) do
          w = w + vim.fn.strdisplaywidth(ch)
          if w > avail then break end
          truncated[#truncated + 1] = ch
        end
        result[#result + 1] = canon .. table.concat(truncated)
        i = i + 1
      else
        -- Join consecutive lines at the same quote level
        local j = i + 1
        while j <= #lines do
          local next_prefix = get_quote_prefix(lines[j])
          local next_level = normalize_prefix(next_prefix)
          if next_level ~= level then break end
          local next_text = lines[j]:sub(#next_prefix + 1)
          -- Blank quoted line = paragraph break
          if next_text:match("^%s*$") then break end
          -- Decorative line = paragraph break
          if not next_text:match("[%w]") then break end
          text = text:gsub("%s+$", "") .. " " .. next_text:gsub("^%s+", "")
          j = j + 1
        end
        -- Re-wrap the joined paragraph
        text = text:gsub("^%s+", ""):gsub("%s+$", "")
        local wrapped = wrap_text(text, canon, width)
        for _, wl in ipairs(wrapped) do
          result[#result + 1] = wl
        end
        i = j
      end
    else
      result[#result + 1] = lines[i]
      i = i + 1
    end
  end
  return result
end


-- Custom filetype
--
-- We use a custom filetype "aercmail" instead of Neovim's built-in "mail"
-- filetype. The built-in mail filetype defines many highlight groups
-- (mailHeaderEmail, mailHeader, mailQuoted*, etc.) that conflict with our
-- Nord color scheme. Using a custom filetype gives us full control with
-- no interference. The syntax rules are in syntax/aercmail.vim.
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    vim.bo.filetype = "aercmail"
  end,
})

-- Buffer preparation (VimEnter)
--
-- Runs once when the compose buffer opens. Normalizes the RFC 2822 headers
-- that aerc wrote into the buffer, then positions the cursor for writing.
--
-- Steps:
--   1. Unfold RFC 2822 continuation lines (lines starting with whitespace
--      are joined to the preceding header line — RFC 2822 allows long header
--      values to be folded across multiple lines with leading whitespace).
--   2. Strip bare angle brackets from address headers. Addresses like
--      "<email@dom>" with no preceding name become "email@dom". Named
--      addresses like "Name <email@dom>" are left unchanged.
--   3. Re-fold To/Cc/Bcc lines at recipient boundaries, filling to 72 columns.
--      Each "Name <email>" stays as a unit; continuation lines are indented
--      to align under the first address.
--   4. Reflow quoted text (see reflow_quoted above).
--   5. Insert two blank buffer lines before the headers and render visual
--      separator lines (─ × 72) above and below the header block using
--      extmark overlays. These are display-only; BufWritePre strips them
--      before saving (see below).
--   6. Position the cursor between the bottom separator and the quoted text,
--      with blank lines above and below, and enter insert mode.
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)

    -- Unfold RFC 2822 continuation lines (leading whitespace) into single lines
    local unfolded = {}
    for _, line in ipairs(lines) do
      if line:match("^[ \t]") and #unfolded > 0 then
        unfolded[#unfolded] = unfolded[#unfolded] .. " " .. line:match("^%s*(.*)")
      else
        unfolded[#unfolded + 1] = line
      end
    end

    -- Strip angle brackets from bare <email> (no name) in address headers.
    -- Pattern ([,:])(%s*)<([^>]+)> matches addresses preceded by ":" or ","
    -- (i.e., no name before the angle bracket). "Name <email>" is unchanged
    -- because the character before "<" is a letter, not ":" or ",".
    for i, line in ipairs(unfolded) do
      if line:match("^[A-Za-z-]+:") then
        unfolded[i] = line:gsub("([,:])(%s*)<([^>]+)>", "%1%2%3")
      end
    end

    -- Re-fold address headers at recipient boundaries (fill to ~72 cols).
    -- Only To/Cc/Bcc are re-folded; other headers (From, Subject, etc.)
    -- are left as-is.
    local result = {}
    local max_width = 72
    for _, line in ipairs(unfolded) do
      local key, value
      for _, k in ipairs({ "To", "Cc", "Bcc" }) do
        local pattern = "^(" .. k .. "):%s*(.*)"
        key, value = line:match(pattern)
        if key then break end
      end
      if key and value and value:find(",") then
        local indent = string.rep(" ", #key + 2)
        local recipients = {}
        for addr in (value .. ","):gmatch("(.-),%s*") do
          if addr ~= "" then
            recipients[#recipients + 1] = addr
          end
        end
        local cur = key .. ": " .. recipients[1]
        for j = 2, #recipients do
          local candidate = cur .. ", " .. recipients[j]
          if #candidate <= max_width then
            cur = candidate
          else
            result[#result + 1] = cur .. ","
            cur = indent .. recipients[j]
          end
        end
        result[#result + 1] = cur
      else
        result[#result + 1] = line
      end
    end

    -- Reflow quoted text (joins jagged lines, re-wraps at 72 columns)
    result = reflow_quoted(result, 72)

    -- Add blank line + separator line above headers.
    -- Two blank lines are inserted at the top; extmarks will overlay them
    -- with visual separator lines. The actual buffer text stays blank so
    -- BufWritePre can strip these lines cleanly on save.
    table.insert(result, 1, "")  -- will be overlaid with separator
    table.insert(result, 1, "")  -- blank line at top

    vim.api.nvim_buf_set_lines(0, 0, -1, false, result)
    lines = result

    -- Find the first blank line after the headers (marks end of header block)
    local header_end = nil
    for i = 3, #lines do  -- skip the two lines we added
      if lines[i] == "" then
        header_end = i
        break
      end
    end

    if header_end then
      local ns = vim.api.nvim_create_namespace("mail_header_sep")
      local sep = string.rep("─", 72)

      -- Top separator: overlay on the second line (after the blank line at top)
      vim.api.nvim_buf_set_extmark(0, ns, 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })

      -- Bottom separator: overlay on the blank line that follows the headers
      vim.api.nvim_buf_set_extmark(0, ns, header_end - 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })

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
        -- New compose or forward: append at end of To: line
        vim.api.nvim_win_set_cursor(0, { to_line_nr, 0 })
        vim.cmd("normal! A")
      else
        -- Reply: cursor in body between separator and quoted text
        vim.api.nvim_win_set_cursor(0, { header_end + 2, 0 })
        vim.cmd("startinsert")
      end
    else
      vim.cmd("startinsert")
    end
  end,
})

-- BufWritePre: strip decorative blank lines before headers on save.
--
-- VimEnter inserts two blank lines at the top of the buffer for the visual
-- separator extmarks. If those blank lines are saved to disk, aerc fails with:
--   "PrepareHeader: no valid From: address found"
-- because RFC 2822 requires headers to start on line 1 of the file.
--
-- This autocmd finds the first real header line (matching "Key:") and removes
-- everything before it on every save. The visual separators are display-only
-- extmarks, so stripping the blank lines does not affect the display.
vim.api.nvim_create_autocmd("BufWritePre", {
  callback = function()
    local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
    local first_header = nil
    for i, line in ipairs(lines) do
      if line:match("^[A-Za-z-]+:") then
        first_header = i
        break
      end
    end
    if first_header and first_header > 1 then
      vim.api.nvim_buf_set_lines(0, 0, first_header - 1, false, {})
    end
  end,
})

-- Tidytext integration
--
-- tidytext is a Claude-powered prose tidier that fixes spelling, grammar,
-- and punctuation without altering meaning or style. It is a separate binary
-- that must be installed (~/.local/bin/tidytext) and requires the environment
-- variable ANTHROPIC_API_KEY to be set.
--
-- <leader>t runs tidytext on the author's body text only — it excludes headers
-- (above the first blank line after "Key:" lines) and the signature (a line
-- starting with "-- ").
--
-- After tidying, changed words are highlighted with a teal undercurl using
-- the EmailTidyChange highlight group. The highlights clear automatically on
-- the next edit. You can customize the highlight color by redefining this
-- group in your own config.
--
-- Config: ~/.config/tidytext/config.toml (run `tidytext config init` to create)

-- Highlight group for tidytext changes: teal undercurl (sp = underline color)
vim.api.nvim_set_hl(0, "EmailTidyChange", { undercurl = true, sp = "#8fbcbb" })

-- Run tidytext fix on the compose body and highlight changed words
local function run_tidy()
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)

  -- Find body start: first blank line after headers (lines matching "Key: ...")
  local body_start = 1
  local in_headers = false
  for i, line in ipairs(lines) do
    if line:match("^[A-Za-z-]+:") then
      in_headers = true
    elseif in_headers and line == "" then
      body_start = i + 1
      break
    end
  end

  -- Find body end: exclude signature block (delimiter line "-- ")
  local body_end = #lines
  for i = body_start, #lines do
    if lines[i] == "-- " then
      body_end = i - 1
      break
    end
  end

  if body_start > body_end then
    vim.notify("tidytext: no body text found", vim.log.levels.INFO)
    return
  end

  -- Save original lines for the word-level diff after tidying
  local original = {}
  for i = body_start, body_end do
    original[#original + 1] = lines[i]
  end

  -- Pipe body text through tidytext fix; it reads from stdin and writes fixed
  -- text to stdout. The API call is synchronous — expect a short pause.
  local input = table.concat(original, "\n") .. "\n"
  local output_lines = vim.fn.systemlist("tidytext fix", input)

  -- If the command failed (binary not found, API key missing, etc.), bail out
  if vim.v.shell_error ~= 0 then
    vim.notify("tidytext: command failed", vim.log.levels.WARN)
    return
  end

  -- Replace body lines with the tidied output
  vim.api.nvim_buf_set_lines(0, body_start - 1, body_end, false, output_lines)

  -- Word-level diff: highlight each changed word with EmailTidyChange.
  -- Compares old vs new line by line and word by word; marks changed words
  -- with extmarks in the tidytext_changes namespace.
  local ns = vim.api.nvim_create_namespace("tidytext_changes")
  vim.api.nvim_buf_clear_namespace(0, ns, 0, -1)

  for i, new_line in ipairs(output_lines) do
    local old_line = original[i] or ""
    if new_line ~= old_line then
      local old_words = vim.split(old_line, "%s+")
      local new_words = vim.split(new_line, "%s+")
      local col = 0
      for j, nw in ipairs(new_words) do
        local ow = old_words[j] or ""
        local word_start = new_line:find(nw, col + 1, true)
        if word_start and nw ~= ow then
          vim.api.nvim_buf_set_extmark(0, ns, body_start - 1 + i - 1, word_start - 1, {
            end_col = word_start - 1 + #nw,
            hl_group = "EmailTidyChange",
          })
        end
        if word_start then
          col = word_start + #nw - 1
        end
      end
    end
  end

  -- Clear the change highlights on the next edit so they don't linger
  vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
    buffer = 0,
    once = true,
    callback = function()
      vim.api.nvim_buf_clear_namespace(0, ns, 0, -1)
    end,
  })

  -- Report how many lines were changed
  local changed = 0
  for i, new_line in ipairs(output_lines) do
    if new_line ~= (original[i] or "") then
      changed = changed + 1
    end
  end
  if changed > 0 then
    vim.notify("tidytext: " .. changed .. " line(s) changed", vim.log.levels.INFO)
  else
    vim.notify("tidytext: no changes needed", vim.log.levels.INFO)
  end
end

-- <leader>t — run tidytext on the compose body (normal mode only)
vim.keymap.set("n", "<leader>t", run_tidy, { desc = "Tidy prose (tidytext)" })

-- Keybindings

-- <leader>s — toggle spell check on/off (useful when quoting technical content)
vim.keymap.set("n", "<leader>s", function()
  vim.opt.spell = not vim.opt.spell:get()
end, { desc = "Toggle spell check" })

-- <leader>q — save and quit (normal exit to aerc's review screen).
-- Before exiting, checks the body for misspelled words. If any are found,
-- prompts with three options:
--   (s)pellcheck — jump to the first misspelled word
--   (y)es        — send anyway (runs :wq)
--   (n)o         — stay in the editor
-- Skips quoted lines (starting with ">") and the signature delimiter ("--").
vim.keymap.set("n", "<leader>q", function()
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
  -- Find body start: first blank line after a header (Key: value) line
  local body_start = 1
  local in_headers = false
  for i, line in ipairs(lines) do
    if line:match("^[A-Za-z-]+:") then
      in_headers = true
    elseif in_headers and line == "" then
      body_start = i + 1
      break
    end
  end
  -- Check only non-quoted, non-empty body lines for misspellings
  local misspelled = {}
  for i = body_start, #lines do
    local line = lines[i]
    if line ~= "" and not line:match("^>") and not line:match("^%-%-") then
      for _, entry in ipairs(vim.spell.check(line)) do
        if entry[2] == "bad" or entry[2] == "rare" then
          misspelled[#misspelled + 1] = entry[1]
        end
      end
    end
  end
  if #misspelled > 0 then
    local prompt = #misspelled .. " misspelled word"
    if #misspelled > 1 then prompt = prompt .. "s" end
    prompt = prompt .. " - (s)pellcheck, (y)es send, (n)o? "
    vim.ui.input({ prompt = prompt }, function(input)
      if not input then return end
      if input:lower() == "y" then
        vim.cmd("wq")
      elseif input:lower() == "s" then
        vim.cmd("normal! ]s")
      end
    end)
  else
    vim.cmd("wq")
  end
end, { desc = "Save and quit" })

-- <leader>x — abort compose immediately (non-zero exit so aerc discards the draft)
vim.keymap.set("n", "<leader>x", "<cmd>cq<cr>", { desc = "Abort compose" })

-- <leader>sig — insert email signature at the current cursor position.
-- Reads from ~/.config/aerc/signature.md. The signature delimiter "-- " is
-- prepended automatically (this is the standard RFC 3676 sig delimiter that
-- mail clients use to identify and optionally hide signatures).
-- Signature file not found? Create ~/.config/aerc/signature.md with your name.
vim.keymap.set("n", "<leader>sig", function()
  -- Read signature from file. Look for signature.md in the aerc config
  -- directory, falling back to a default if not found.
  local sig_paths = {
    vim.fn.expand("~/.config/aerc/signature.md"),
  }
  local sig_lines = nil
  for _, path in ipairs(sig_paths) do
    local f = io.open(path, "r")
    if f then
      local content = f:read("*a")
      f:close()
      sig_lines = { "-- " }
      for line in content:gmatch("([^\n]*)\n?") do
        sig_lines[#sig_lines + 1] = line
      end
      -- Trim trailing empty lines
      while #sig_lines > 0 and sig_lines[#sig_lines] == "" do
        sig_lines[#sig_lines] = nil
      end
      break
    end
  end
  if not sig_lines then
    vim.notify("No signature.md found in ~/.config/aerc/", vim.log.levels.WARN)
    return
  end
  local row = vim.api.nvim_win_get_cursor(0)[1]
  vim.api.nvim_buf_set_lines(0, row, row, false, sig_lines)
end, { desc = "Insert email signature" })

-- Insert-mode undo breakpoints: pressing punctuation ends the current undo chunk.
-- Without these, `u` undoes the entire insert session (a paragraph or more).
-- With these, `u` undoes only back to the last punctuation character typed.
for _, ch in ipairs({ ".", ",", "!", "?", ":" }) do
  vim.keymap.set("i", ch, ch .. "<C-g>u", { desc = "Undo breakpoint at " .. ch })
end

-- Spellcheck navigation aliases — shorter than the built-in ]s / [s / z=
vim.keymap.set("n", "<leader>]", "]s", { desc = "Next misspelled word" })
vim.keymap.set("n", "<leader>[", "[s", { desc = "Prev misspelled word" })
vim.keymap.set("n", "<leader>z", "z=", { desc = "Spelling suggestions" })

-- <leader>r — reflow the current paragraph to textwidth=72 (gqip built-in)
vim.keymap.set("n", "<leader>r", "gqip", { desc = "Reflow paragraph" })

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

        -- Auto-prepend ", " on address header lines that already have a recipient
        local on_address_header = buf_line:match("^To:%s*.+")
          or buf_line:match("^Cc:%s*.+")
          or buf_line:match("^Bcc:%s*.+")
        if on_address_header then
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
