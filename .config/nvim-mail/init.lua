-- nvim-mail: neovim profile for aerc email composing.
-- Minimal visual style with markdown support and spell check.

-- Leader key
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
require("lazy").setup({
  {
    "shaunsingh/nord.nvim",
    priority = 1000,
    config = function()
      vim.cmd.colorscheme("nord")
    end,
  },
}, { ui = { border = "none" } })

-- Editor settings
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

-- Quote reflow: join consecutive quoted lines into paragraphs, re-wrap
local function get_quote_prefix(line)
  return line:match("^(>[ >]*%s?)")
end

-- Normalize prefix to just the ">" characters for comparison
local function normalize_prefix(prefix)
  if not prefix then return nil end
  return prefix:gsub("[^>]", "")
end

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



-- Set custom filetype for our syntax highlighting
vim.api.nvim_create_autocmd("VimEnter", {
  callback = function()
    vim.bo.filetype = "aercmail"
  end,
})

-- Prepare compose buffer: unfold headers, add separator, position cursor
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

    -- Strip angle brackets from bare <email> (no name) in address headers
    -- Bare = preceded by ":" or "," (plus whitespace), not by a name
    for i, line in ipairs(unfolded) do
      if line:match("^[A-Za-z-]+:") then
        unfolded[i] = line:gsub("([,:])(%s*)<([^>]+)>", "%1%2%3")
      end
    end

    -- Re-fold address headers at recipient boundaries (fill to ~72 cols)
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

    -- Hard-wrap quoted text for display
    result = reflow_quoted(result, 72)

    -- Add blank line + separator line above headers
    table.insert(result, 1, "")  -- will be overlaid with separator
    table.insert(result, 1, "")  -- blank line at top

    vim.api.nvim_buf_set_lines(0, 0, -1, false, result)
    lines = result

    -- Find first blank line after headers (end of header block)
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

      -- Top separator (overlay on second line, after blank)
      vim.api.nvim_buf_set_extmark(0, ns, 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })

      -- Bottom separator (overlay on blank line after headers)
      vim.api.nvim_buf_set_extmark(0, ns, header_end - 1, 0, {
        virt_text = { { sep, "mailHeaderKey" } },
        virt_text_pos = "overlay",
      })

      -- Ensure: blank, cursor, blank, then body
      local body_start = header_end + 1
      if body_start <= #lines and lines[body_start] ~= "" then
        vim.api.nvim_buf_set_lines(0, header_end, header_end, false, { "", "", "" })
        vim.api.nvim_win_set_cursor(0, { header_end + 2, 0 })
      else
        vim.api.nvim_buf_set_lines(0, header_end, header_end, false, { "", "", "" })
        vim.api.nvim_win_set_cursor(0, { header_end + 2, 0 })
      end
    end

    vim.cmd("startinsert")
  end,
})

-- Strip decorative blank lines before headers on save so aerc sees
-- valid RFC 2822 (headers must start on line 1).
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

-- Highlight group for tidytext changes (teal undercurl)
vim.api.nvim_set_hl(0, "EmailTidyChange", { undercurl = true, sp = "#8fbcbb" })

-- tidytext: run prose tidier on author's body text
local function run_tidy()
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)

  -- Find body start: first blank line after headers
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

  -- Find body end: exclude signature (line starting with "-- ")
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

  -- Save original lines for diff
  local original = {}
  for i = body_start, body_end do
    original[#original + 1] = lines[i]
  end

  -- Pipe body through tidytext fix
  local input = table.concat(original, "\n") .. "\n"
  local output_lines = vim.fn.systemlist("tidytext fix", input)

  -- If the command failed, notify and return
  if vim.v.shell_error ~= 0 then
    vim.notify("tidytext: command failed", vim.log.levels.WARN)
    return
  end

  -- Replace body lines with output
  vim.api.nvim_buf_set_lines(0, body_start - 1, body_end, false, output_lines)

  -- Word-level diff: highlight changes
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

  -- Clear highlights on next edit
  vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
    buffer = 0,
    once = true,
    callback = function()
      vim.api.nvim_buf_clear_namespace(0, ns, 0, -1)
    end,
  })

  -- Notify user
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
vim.keymap.set("n", "<leader>s", function()
  vim.opt.spell = not vim.opt.spell:get()
end, { desc = "Toggle spell check" })

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
  -- Check only non-quoted, non-empty body lines
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
vim.keymap.set("n", "<leader>x", "<cmd>cq<cr>", { desc = "Abort compose" })


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
        if line ~= "" or sig_lines then
          sig_lines[#sig_lines + 1] = line
        end
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
for _, ch in ipairs({ ".", ",", "!", "?", ":" }) do
  vim.keymap.set("i", ch, ch .. "<C-g>u", { desc = "Undo breakpoint at " .. ch })
end

-- Spellcheck navigation (leader aliases for built-ins)
vim.keymap.set("n", "<leader>]", "]s", { desc = "Next misspelled word" })
vim.keymap.set("n", "<leader>[", "[s", { desc = "Prev misspelled word" })
vim.keymap.set("n", "<leader>z", "z=", { desc = "Spelling suggestions" })

-- Paragraph reflow to textwidth=72
vim.keymap.set("n", "<leader>r", "gqip", { desc = "Reflow paragraph" })

-- khard contact picker: <leader>k inserts a contact address at the cursor.
-- Works in normal and insert mode. In insert mode, returns to insert after selection.
local function khard_insert(reenter_insert)
  local raw = vim.fn.systemlist("khard email --parsable 2>/dev/null")
  local entries = {}
  for _, line in ipairs(raw) do
    local email, name = line:match("^([^\t]+)\t([^\t]*)")
    if email then
      name = name and name:match("^%s*(.-)%s*$") or ""
      local label = name ~= "" and (name .. " <" .. email .. ">") or email
      entries[#entries + 1] = { label = label, text = label }
    end
  end
  if #entries == 0 then
    vim.notify("No khard contacts found", vim.log.levels.WARN)
    if reenter_insert then vim.cmd("startinsert") end
    return
  end
  vim.ui.select(entries, {
    prompt = "Insert contact: ",
    format_item = function(e) return e.label end,
  }, function(choice)
    if not choice then
      if reenter_insert then vim.cmd("startinsert") end
      return
    end
    local pos = vim.api.nvim_win_get_cursor(0)
    local buf_line = vim.api.nvim_buf_get_lines(0, pos[1] - 1, pos[1], false)[1]
    local new_line = buf_line:sub(1, pos[2]) .. choice.text .. buf_line:sub(pos[2] + 1)
    vim.api.nvim_buf_set_lines(0, pos[1] - 1, pos[1], false, { new_line })
    vim.api.nvim_win_set_cursor(0, { pos[1], pos[2] + #choice.text })
    if reenter_insert then vim.cmd("startinsert") end
  end)
end

vim.keymap.set("n", "<leader>k", function() khard_insert(false) end,
  { desc = "Insert khard contact" })
vim.keymap.set("i", "<C-k>", function()
  vim.cmd("stopinsert")
  vim.schedule(function() khard_insert(true) end)
end, { desc = "Insert khard contact (insert mode)" })
