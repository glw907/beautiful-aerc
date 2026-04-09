-- nvim-mail: Neovim profile for composing email in aerc
--
-- This is a dedicated Neovim configuration, completely isolated from
-- your regular Neovim setup via NVIM_APPNAME. It provides:
--
--   - Custom syntax highlighting for email headers and quoted text
--   - Hard-wrap at 72 characters with format-flowed support
--   - Spell check on body text (skips headers and quoted lines)
--   - Fuzzy contact picker via khard + Telescope
--   - Prose tidying via tidytext (optional, requires Anthropic API key)
--
-- Plugins install automatically on first launch — just open a compose
-- window and wait a few seconds. See docs/nvim-mail.md for the full
-- compose workflow.

vim.g.mapleader = " "

-- Plugin management: lazy.nvim
--
-- lazy.nvim is a modern Neovim plugin manager. The bootstrap block
-- below auto-installs it on first launch, then it installs all the
-- plugins listed in the setup() call. You don't need to do anything
-- manually.
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({
    "git", "clone", "--filter=blob:none",
    "https://github.com/folke/lazy.nvim.git",
    "--branch=stable", lazypath,
  })
end
vim.opt.rtp:prepend(lazypath)

require("lazy").setup({
  -- nord.nvim: Nord color scheme, matching the aerc theme.
  -- Loads first (priority=1000) so all UI elements use Nord colors.
  {
    "shaunsingh/nord.nvim",
    priority = 1000,
    config = function()
      vim.cmd.colorscheme("nord")
    end,
  },
  -- telescope.nvim: Fuzzy finder framework. Powers the contact picker —
  -- search your address book by typing a few letters of a name.
  -- plenary.nvim is a required utility library.
  {
    "nvim-telescope/telescope.nvim",
    dependencies = { "nvim-lua/plenary.nvim" },
  },
  -- which-key.nvim: Shows available keybindings when you press the
  -- leader key (Space) and wait. Helps you discover what's available
  -- without memorizing everything. Loads lazily to keep startup fast.
  {
    "folke/which-key.nvim",
    event = "VeryLazy",
  },
}, { ui = { border = "none" } })

-- Editor settings: optimized for prose composition, not code.
--
-- These create a comfortable writing environment: automatic line
-- wrapping at 72 characters, spell check, and a clean distraction-free
-- display with no line numbers, no status bar, and no mode indicator.
vim.opt.wrap = true
vim.opt.linebreak = true
vim.opt.textwidth = 72
vim.opt.formatoptions = "tcrqwn"
vim.opt.breakat = " \t"
vim.opt.spell = true
vim.opt.spelllang = "en_us"
vim.opt.number = false
vim.opt.relativenumber = false
vim.opt.signcolumn = "no"
vim.opt.showmode = false
vim.opt.laststatus = 0
vim.opt.cursorline = false
vim.opt.autoindent = true
vim.opt.smartindent = true
vim.opt.breakindent = true
vim.opt.breakindentopt = "shift:2"
vim.opt.swapfile = false
vim.opt.termguicolors = true

-- Custom filetype: "aercmail"
--
-- We use a custom filetype instead of the built-in "mail" type because
-- mail's syntax definitions conflict with our custom highlighting for
-- headers and quoted text.
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    vim.bo.filetype = "aercmail"
  end,
})

-- Buffer preparation on open
--
-- When aerc opens the compose editor, the raw buffer has RFC 2822
-- formatted headers (folded continuation lines, bare angle brackets,
-- etc.). This pipes the buffer through compose-prep to normalize
-- headers and reflow quoted text, then adds visual separator lines
-- and positions the cursor.
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    local raw_lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
    local result = vim.fn.systemlist("compose-prep", raw_lines)
    if vim.v.shell_error == 0 and #result > 0 then
      vim.api.nvim_buf_set_lines(0, 0, -1, false, result)
    else
      result = raw_lines
    end

    -- Insert blank lines at top for decorative separator extmarks
    table.insert(result, 1, "")
    table.insert(result, 1, "")
    vim.api.nvim_buf_set_lines(0, 0, -1, false, result)

    -- Find the blank line that separates headers from body
    local header_end = nil
    for i = 3, #result do
      if result[i] == "" then
        header_end = i
        break
      end
    end

    if header_end then
      -- Draw decorative separator lines above and below the headers
      local ns = vim.api.nvim_create_namespace("mail_header_sep")
      local sep = string.rep("─", 72)

      vim.api.nvim_buf_set_extmark(0, ns, 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })
      vim.api.nvim_buf_set_extmark(0, ns, header_end - 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })

      -- Add blank lines after headers for the cursor landing zone
      vim.api.nvim_buf_set_lines(0, header_end, header_end, false, { "", "", "" })

      -- Cursor placement:
      -- Empty To: (new compose/forward) → land on To: line, ready to
      --   type a recipient or press Ctrl-k for the contact picker
      -- Populated To: (reply) → land in the body, ready to type
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
        vim.api.nvim_buf_set_lines(0, to_line_nr - 1, to_line_nr, false, { "To: " })
        vim.api.nvim_win_set_cursor(0, { to_line_nr, 3 })
        vim.cmd("startinsert!")
      else
        vim.api.nvim_win_set_cursor(0, { header_end + 2, 0 })
        vim.cmd("startinsert")
      end
    else
      vim.cmd("startinsert")
    end
  end,
})

-- Save cleanup: strip decorative blank lines before headers
--
-- The buffer preparation above inserts blank lines for the separator
-- extmarks. This removes them before saving so aerc sees valid RFC 2822
-- headers starting on line 1. Without this, aerc fails with "no valid
-- From: address found".
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

-- Tidytext: Claude-powered prose tidier (optional)
--
-- Runs AI-powered spelling, grammar, and punctuation fixes on the email
-- body (skipping headers and signature). Changed words are highlighted
-- with teal undercurl marks that clear on the next edit.
--
-- Requires: tidytext binary (make install) and ANTHROPIC_API_KEY env var.
vim.api.nvim_set_hl(0, "EmailTidyChange", { undercurl = true, sp = "#8fbcbb" })

local function run_tidy()
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)

  -- Find body boundaries (between headers and signature)
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

  local original = {}
  for i = body_start, body_end do
    original[#original + 1] = lines[i]
  end

  local input = table.concat(original, "\n") .. "\n"
  local output_lines = vim.fn.systemlist("tidytext fix", input)

  if vim.v.shell_error ~= 0 then
    vim.notify("tidytext: command failed", vim.log.levels.WARN)
    return
  end

  vim.api.nvim_buf_set_lines(0, body_start - 1, body_end, false, output_lines)

  -- Highlight changed words with undercurl extmarks
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

  -- Clear highlights on the next edit
  vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
    buffer = 0,
    once = true,
    callback = function()
      vim.api.nvim_buf_clear_namespace(0, ns, 0, -1)
    end,
  })

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

vim.keymap.set("n", "<leader>t", run_tidy, { desc = "Tidy prose (tidytext)" })

-- Keybindings
--
-- Space is the leader key (set at the top of this file). Press Space
-- and wait to see all available bindings via which-key.

vim.keymap.set("n", "<leader>s", function()
  vim.opt.spell = not vim.opt.spell:get()
end, { desc = "Toggle spell check" })

-- Save and quit with spellcheck prompt
--
-- Before exiting, checks for misspelled words in the body (skipping
-- headers, quoted lines, and signature). If misspellings are found,
-- you get three options:
--   (s)pellcheck — jump to the first misspelled word
--   (y)es — send anyway
--   (n)o — stay in the editor
vim.keymap.set("n", "<leader>q", function()
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
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

-- Abort compose — exits with non-zero code so aerc closes the compose
-- tab immediately without sending.
vim.keymap.set("n", "<leader>x", "<cmd>cq<cr>", { desc = "Abort compose" })

-- Insert your email signature from ~/.config/aerc/signature.md
-- Copy signature.md.example to signature.md and customize it.
vim.keymap.set("n", "<leader>sig", function()
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

-- Undo breakpoints: pressing . , ! ? : creates an undo point so that
-- pressing 'u' undoes smaller chunks instead of the whole paragraph.
for _, ch in ipairs({ ".", ",", "!", "?", ":" }) do
  vim.keymap.set("i", ch, ch .. "<C-g>u", { desc = "Undo breakpoint at " .. ch })
end

-- Spell navigation
vim.keymap.set("n", "<leader>]", "]s", { desc = "Next misspelled word" })
vim.keymap.set("n", "<leader>[", "[s", { desc = "Prev misspelled word" })
vim.keymap.set("n", "<leader>z", "z=", { desc = "Spelling suggestions" })
vim.keymap.set("n", "<leader>r", "gqip", { desc = "Reflow paragraph" })

-- Contact picker: search your address book with fuzzy matching
--
-- Requires khard (https://github.com/lucc/khard) with contacts synced
-- via vdirsyncer. Press <leader>k in normal mode or Ctrl-k in insert
-- mode. Type to filter contacts, Enter to insert, Escape to cancel.
--
-- On To:/Cc:/Bcc: lines, automatically prepends ", " when the line
-- already has a recipient.
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
