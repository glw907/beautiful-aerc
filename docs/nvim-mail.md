# Composing Email with nvim-mail

A guide to the nvim-mail compose editor — from first launch to sending your first email.

For a quick overview, see the [Composing email](../README.md#composing-email-with-nvim-mail) section of the README. For aerc keybinding reference, see `binds.conf`.

## Table of contents

- [Why Neovim for email?](#why-neovim-for-email)
- [How it works](#how-it-works)
- [Plugins](#plugins)
- [The compose flow](#the-compose-flow)
- [Keybindings reference](#keybindings-reference)
- [Contact picker](#contact-picker)
- [Signature](#signature)
- [Tidytext integration](#tidytext-integration)
- [Troubleshooting](#troubleshooting)

## Why Neovim for email?

Most terminal email clients use whatever `$EDITOR` you have set. That works, but it's a bare-minimum experience — no syntax highlighting for headers, no contact search, no awareness that you're composing an email rather than editing code.

Neovim is programmable, which means we can build features into the compose editor that would be impossible otherwise:

- **Fuzzy contact search** — type a few letters and pick from your address book instead of typing email addresses from memory
- **Spell check with a safety net** — catches misspellings as you type, and prompts before sending if any remain
- **Prose tidying** — one-key AI-powered grammar and spelling fixes
- **Smart header formatting** — raw RFC 2822 headers are automatically cleaned up and reformatted for readability
- **Quoted text reflow** — jagged quoted text from different clients is reflowed to consistent 72-column paragraphs

If you already use Neovim, the keybindings will feel familiar. If you're new to Neovim, don't worry — the compose workflow only uses a handful of keys, and the which-key plugin shows you what's available.

## How it works

nvim-mail is a dedicated Neovim profile, completely isolated from your regular Neovim configuration. It uses Neovim's `NVIM_APPNAME` feature — a way to run multiple independent Neovim configurations side by side. Your regular Neovim plugins, settings, and keybindings are untouched.

When you compose, reply, or forward in aerc, it opens `nvim-mail` (a small wrapper script) instead of plain `nvim`:

```bash
#!/usr/bin/env bash
NVIM_APPNAME=nvim-mail exec nvim "$@"
```

This tells Neovim to use `~/.config/nvim-mail/` for its configuration and `~/.local/share/nvim-mail/` for its data (plugins, etc.), keeping everything separate.

The configuration lives in `.config/nvim-mail/init.lua` — a single file that sets up plugins, editor settings, and keybindings. It's heavily commented to explain what everything does.

## Plugins

nvim-mail uses five plugins, all managed by [lazy.nvim](https://github.com/folke/lazy.nvim). **Plugins install automatically on first launch.** The first time you open a compose window, lazy.nvim bootstraps itself (downloads from GitHub), then installs all plugins. This takes a few seconds — after that, startup is instant.

### lazy.nvim

The plugin manager itself. It handles downloading, updating, and loading all other plugins. The bootstrap code in `init.lua` auto-installs lazy.nvim if it's not already present, so there's nothing to set up manually.

### nord.nvim

The [Nord color scheme](https://www.nordtheme.com/) for Neovim. This matches the default aerc theme, keeping the compose editor visually consistent with the message list and viewer. If you switch to a different aerc theme, you may want to install a matching Neovim colorscheme — see [themes.md](themes.md) for details.

### telescope.nvim

A fuzzy finder framework (with its dependency plenary.nvim). In nvim-mail, it powers the **contact picker** — press `Ctrl-k` in insert mode or `<Space>k` in normal mode to open a searchable list of contacts from your address book. Type a few letters to filter, press Enter to insert. See [Contact picker](#contact-picker) below.

### which-key.nvim

Shows available keybindings when you press the leader key (Space) and wait about a second. A floating window appears with all available bindings and their descriptions. This is invaluable when you're learning the keybindings — you don't need to memorize everything, just press Space and look.

which-key loads lazily (only after startup) to keep the editor fast.

### nvim-treesitter

Provides syntax highlighting for the compose buffer via the custom `aercmail` filetype. Header keys (To:, From:, Subject:) are colored differently from header values and quoted text, making the buffer easier to read at a glance.

## The compose flow

Here's what happens step by step when you compose, reply to, or forward an email.

### 1. Open compose

In the aerc message list, press:
- `C` or `m` — compose a new message
- `rr` — reply-all (quoted)
- `Rr` — reply to sender only
- `f` — forward

aerc opens nvim-mail with the compose buffer.

### 2. Buffer preparation

When the editor opens, `mailrender compose` normalizes the raw buffer:

- **Unfolds** RFC 2822 continuation lines (headers that wrap with leading whitespace)
- **Strips bare angle brackets** — `<email@dom>` without a name becomes `email@dom`
- **Folds address headers** — long To/Cc/Bcc lines wrap at recipient boundaries, aligned at 120 columns
- **Reflows quoted text** — jagged quoted lines from the original sender are joined into paragraphs and re-wrapped at 72 columns

After normalization, visual separator lines (thin horizontal rules) appear above and below the headers.

### 3. Cursor placement

- **New compose or forward** (empty To: field) — cursor lands on the `To:` line in insert mode, ready to type a recipient or press `Ctrl-k` for the contact picker
- **Reply** (To: already populated) — cursor lands in the body between the headers and quoted text, ready to type your response

### 4. Write your message

Write in markdown. Text wraps automatically at 72 characters as you type (hard wrap, not soft wrap). This is the standard line length for polite email — aerc's `format-flowed` setting adds reflow markers on send so recipients' clients can reflow the text to fit their own display width.

You can use markdown formatting: `**bold**`, `*italic*`, headings (`#`, `##`), lists (`-`), and links. When you send (step 6), aerc converts your markdown to HTML so recipients with GUI clients see rich text.

### 5. Exit to review

Press `<Space>q` to save and exit the editor. Before exiting, nvim-mail checks the body for misspelled words (skipping headers, quoted lines, and signature). If misspellings are found, you get three options:

- **(s)pellcheck** — jump to the first misspelled word so you can fix it
- **(y)es** — send it anyway
- **(n)o** — stay in the editor

If there are no misspellings, the editor exits immediately to the aerc review screen.

### 6. Review and send

aerc shows a review screen with these options:

| Key | Action |
|-----|--------|
| `y` | Convert markdown to HTML multipart and send |
| `n` | Abort — discard the message |
| `e` | Re-edit — go back into nvim-mail |
| `v` | Preview — see what the message will look like |
| `p` | Postpone — save as a draft |
| `q` | Choose between discard or postpone |
| `a` | Attach a file |
| `d` | Detach (remove) an attachment |

Press `y` to send. aerc converts your markdown body to HTML using `mailrender to-html`, then sends both versions as a multipart message. Recipients with GUI clients see the HTML; recipients with CLI clients see your plain text.

### 7. Abort anytime

Press `<Space>x` at any point in the editor to abort immediately. This exits with a non-zero code, telling aerc to close the compose tab without sending.

## Keybindings reference

All keybindings use Space as the leader key. Press Space and wait to see available bindings via which-key.

### Compose actions

| Key | Mode | Action |
|-----|------|--------|
| `<Space>q` | normal | Save and exit (with spell check prompt) |
| `<Space>x` | normal | Abort compose immediately |
| `<Space>sig` | normal | Insert email signature |
| `<Space>t` | normal | Run tidytext on the body |
| `<Space>r` | normal | Reflow the current paragraph |

### Spell check

| Key | Mode | Action |
|-----|------|--------|
| `<Space>s` | normal | Toggle spell check on/off |
| `<Space>]` | normal | Jump to next misspelled word |
| `<Space>[` | normal | Jump to previous misspelled word |
| `<Space>z` | normal | Show spelling suggestions for word under cursor |

### Contact picker

| Key | Mode | Action |
|-----|------|--------|
| `<Space>k` | normal | Open contact picker |
| `Ctrl-k` | insert | Open contact picker (returns to insert mode) |

### Undo

Pressing `.`, `,`, `!`, `?`, or `:` creates an undo breakpoint, so pressing `u` undoes smaller chunks instead of reverting the entire paragraph.

## Contact picker

The contact picker uses [Telescope](https://github.com/nvim-telescope/telescope.nvim) to provide fuzzy search over your address book.

### Setup

The picker requires [khard](https://github.com/lucc/khard) — a command-line address book program that reads CardDAV contacts. You'll need:

1. **khard** installed (`pip install khard` or your package manager)
2. **vdirsyncer** configured to sync contacts from your provider (Fastmail, Google, iCloud, etc.) to a local CardDAV store
3. Contacts synced at least once (`vdirsyncer sync`)

If khard isn't installed or has no contacts, the picker shows a warning and nothing happens — it won't break your compose flow.

### Using the picker

Press `Ctrl-k` in insert mode (or `<Space>k` in normal mode) to open the picker. A Telescope window appears with all your contacts listed as `Name <email>`. Type to filter — Telescope does fuzzy matching, so typing "joh" will match "John Smith", "Johnson", etc.

Press Enter to insert the selected contact at the cursor position. Press Escape to cancel.

**Auto-comma:** On header lines (To:, Cc:, Bcc:), if the line already has a recipient, the picker automatically prepends `, ` before the new contact. In the body, contacts are inserted without any prefix.

## Signature

Press `<Space>sig` to insert your email signature at the current cursor position.

The signature is read from `~/.config/aerc/signature.md`. To set it up:

```sh
cp ~/.config/aerc/signature.md.example ~/.config/aerc/signature.md
```

Edit the file with your name and contact info. The signature is inserted with a standard `-- ` delimiter line above it.

**Markdown in signatures:** The signature supports markdown formatting. For example, `**Your Name**` renders as bold in the HTML multipart when the message is sent. This is a nice touch for recipients with GUI clients.

The signature is inserted manually (not auto-appended) so you can choose when and where to include it.

## Tidytext integration

Press `<Space>t` to run [tidytext](../README.md#tidytext) on the email body. tidytext uses Claude Haiku to fix spelling, grammar, and punctuation without altering your meaning or writing style.

**What it processes:** Only the body text — headers and signature (everything after `-- `) are excluded.

**Visual feedback:** Changed words are highlighted with teal undercurl marks (wavy underline). These highlights clear automatically on the next edit, so they don't interfere with your writing.

**Requirements:** tidytext binary installed (`make install`) and `ANTHROPIC_API_KEY` environment variable set.

## Troubleshooting

### Plugins aren't installing on first launch

Check that you have internet access and git installed. lazy.nvim bootstraps itself by cloning from GitHub. If it fails, you'll see an error in the Neovim command line.

You can also try installing manually:

```sh
git clone --filter=blob:none --branch=stable \
  https://github.com/folke/lazy.nvim.git \
  ~/.local/share/nvim-mail/lazy/lazy.nvim
```

Then reopen a compose window — lazy.nvim should install the remaining plugins.

### Contact picker says "No khard contacts found"

Either khard isn't installed, or it has no contacts. Check:

```sh
khard email --parsable
```

If this returns nothing, you need to set up vdirsyncer to sync your contacts. See the [khard documentation](https://khard.readthedocs.io/) and [vdirsyncer documentation](https://vdirsyncer.pimutils.org/).

### Spell check is marking everything as misspelled

Check that the spell check language matches your writing:

```lua
-- In .config/nvim-mail/init.lua
vim.opt.spelllang = "en_us"
```

Change to `en_gb`, `de`, `fr`, etc. as needed. Neovim downloads spell files automatically on first use.

### Buffer normalization isn't running / headers look raw

`mailrender compose` falls back gracefully — if it's not installed or fails, the raw buffer is shown unchanged. Verify it's installed:

```sh
which mailrender
mailrender compose --help
```

If missing, run `make install` from the project directory.

### "No valid From: address found" when sending

This usually means the BufWritePre cleanup didn't run — blank lines appear before the first header. Check that `init.lua` hasn't been modified in a way that breaks the BufWritePre autocmd.

You can also check the saved file manually — the first line should be a header like `From:` or `To:`, not a blank line.
