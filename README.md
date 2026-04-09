# beautiful-aerc

A themeable, productive email environment for the [aerc](https://aerc-mail.org/) email client.

aerc is a powerful terminal email client, but out of the box it relies on shell scripts and external tools for message rendering, has limited theming support, and requires significant configuration work to reach daily-driver quality. beautiful-aerc provides a complete, cohesive setup: fast Go-based message rendering, a semantic theme system, an interactive link picker, and optional integrations for Fastmail, prose editing, and Neovim — everything needed to make aerc a polished, productive environment.

## Components

- **mailrender** — message rendering filters (`headers`, `html`, `plain`). Replaces a tangle of shell scripts, awk, sed, and perl with a single fast Go binary. Noticeably faster rendering on complex HTML emails.
- **pick-link** — interactive URL picker for the message viewer. Press Tab to open, use 1–9 for instant selection or j/k to navigate.
- **fastmail-cli** — Fastmail JMAP CLI for mail filter rules, masked email management, and folder listing. Designed to be called from aerc keybindings. *(Optional — Fastmail users only.)*
- **tidytext** — Claude-powered prose tidier for the compose editor. Fixes spelling, grammar, and punctuation without altering meaning or style. *(Optional — requires Anthropic API key.)*
- **aerc-save-email** — dev utility for saving raw email parts to a test corpus. *(Optional — development use.)*
- **compose-prep** — Compose buffer normalizer: RFC 2822 header unfolding, bare bracket stripping, address folding at 72 columns, Cc/Bcc header injection, and quoted text reflow. Falls back gracefully if not installed.
- **nvim-mail** — Neovim compose editor profile with custom `aercmail` syntax highlighting, hard-wrap at 72 characters, spell check, and tidytext integration.
- **aerc config** — `aerc.conf` and `binds.conf` ready to use. Includes a semantic theme system with three built-in themes, aerc stylesets, Nerd Font icons for message flags and folder names, and clean thread display.
- **kitty config** — Terminal profile for launching aerc in a dedicated kitty window.

## Prerequisites

- [aerc](https://aerc-mail.org/)
- [pandoc](https://pandoc.org/) (called at runtime for HTML conversion)
- Go 1.25+ (build only)
- GNU Stow (install only)

Optional:

- [kitty](https://sw.kovidgoyal.net/kitty/) — for the `mail` launcher script
- [Neovim](https://neovim.io/) 0.10+ — for the nvim-mail compose editor
- [khard](https://github.com/lucc/khard) — for address book completion in the compose editor
- Fastmail account with API token — for fastmail-cli
- Anthropic API key — for tidytext

## Install

**1. Clone the repo**

```sh
git clone https://github.com/glw907/beautiful-aerc.git
cd beautiful-aerc
```

**2. Build and install the five binaries**

```sh
make build
make install   # installs mailrender, pick-link, fastmail-cli, tidytext, compose-prep to ~/.local/bin/
```

**3. Generate a styleset**

Pick one of the three built-in themes and generate the aerc styleset:

```sh
mailrender themes generate nord
```

This produces `stylesets/Nord` in your aerc config directory.

**4. Install with Stow**

```sh
stow beautiful-aerc
```

Or, if symlinking from `~/Projects/`:

```sh
ln -s ~/Projects/beautiful-aerc ~/.dotfiles/beautiful-aerc
cd ~/.dotfiles && stow beautiful-aerc
```

**5. Configure your account**

```sh
cp ~/.config/aerc/accounts.conf.example ~/.config/aerc/accounts.conf
# Edit accounts.conf with your mail server settings
```

**6. Set the styleset name in aerc.conf**

```ini
styleset-name=nord
```

## How email renders

aerc routes every message through filters defined in `aerc.conf`:

```ini
.headers=mailrender headers
text/html=mailrender html
text/plain=mailrender plain
```

- **headers** — reorders headers (From, To, Cc, Bcc, Date, Subject), colorizes field names, wraps long address lines, and prints a separator line.
- **html** — calls pandoc to convert HTML to markdown, cleans up pandoc artifacts, renders links as footnote references with a numbered URL section at the bottom, and applies syntax highlighting for headings, bold, and italic.
- **plain** — detects HTML-in-plain-text MIME parts (sent by some clients) and routes them through the HTML pipeline. Otherwise pipes through aerc's built-in `wrap | colorize`.

See [docs/filters.md](docs/filters.md) for the full pipeline description.

## Footnote-style links

Links in HTML emails render as footnote references. Body text stays clean and readable; URLs are collected in a numbered reference section at the bottom:

```
If you don't recognize this account, remove[^1] it.

Check activity[^2]

See https://myaccount.google.com/notifications
----------------------------------------
[^1]: https://accounts.google.com/AccountDisavow?adt=...
[^2]: https://accounts.google.com/AccountChooser?Email=...
```

Link text is colored; footnote markers are dimmed. Self-referencing links (where the display text is the URL) render as plain URLs with no footnote.

## Link picker

Press Tab in the message viewer to open an interactive URL picker. All URLs from the current message are listed with numbered shortcuts:

- **1–9, 0** — instantly open that link (0 opens the 10th)
- **j/k or arrows** — move selection
- **Enter** — open selected link
- **q or Escape** — cancel

The keybinding in `binds.conf`:

```ini
[view]
<Tab> = :pipe pick-link<Enter>
```

## Theme system

Themes are defined as 16 semantic color slots in a TOML file under `.config/aerc/themes/`. Go binaries read the active theme directly at runtime. A separate command generates the aerc styleset (UI colors):

```sh
mailrender themes generate nord
```

Three themes are included: **Nord**, **Solarized Dark**, and **Gruvbox Dark**.

To switch themes, set `styleset-name` in `aerc.conf` to the theme name and run `mailrender themes generate`.

To create your own theme, copy one of the built-in `.toml` files, adjust the hex values, and run the generator. The color slots map to semantic roles (primary background, selection, accent, error, etc.) so changes propagate consistently across the entire UI.

See [docs/themes.md](docs/themes.md) for the full token reference and theme file format.

## Optional components

### fastmail-cli

For Fastmail users, `fastmail-cli` provides JMAP operations designed to be called from aerc keybindings. Set `FASTMAIL_API_TOKEN` in your environment, then use the bindings in `binds.conf` (commented out by default):

- **ff / fs / ft** — create a filter rule from sender address, subject, or recipient
- **md** — delete a masked email address and the message that used it

Rules can also be managed from the command line:

```sh
fastmail-cli rules add --search "from:news@example.com" --folder Newsletters
fastmail-cli rules sweep --search "from:news@example.com" --folder Newsletters
fastmail-cli folders   # list custom mailboxes
```

### tidytext

`tidytext` pipes text through Claude Haiku to fix spelling, grammar, and punctuation without altering meaning or style. Set `ANTHROPIC_API_KEY` in your environment.

```sh
echo "Plese review the attatchd documnt." | tidytext fix
# → Please review the attached document.
```

In nvim-mail, `<leader>t` runs tidytext on the compose buffer body. Changed words are highlighted with undercurl extmarks that clear on the next edit.

The keybinding in `binds.conf` is commented out by default — uncomment it if you install tidytext and set the API key.

### nvim-mail

A dedicated Neovim profile for composing email in aerc. It provides:

- Custom `aercmail` syntax highlighting (header keys, address fields, quoted text)
- Hard-wrap at 72 characters with RFC 3676 format=flowed support
- Spell check on body text, skipping headers and quoted lines
- Telescope-powered contact picker with fuzzy search (`<C-k>` in insert mode, `<leader>k` in normal mode) — requires [khard](https://github.com/lucc/khard) with CardDAV contacts synced via vdirsyncer
- tidytext integration via `<leader>t`
- Compose buffer normalization via `compose-prep` — RFC 2822 header processing and quoted text reflow on buffer open
- Smart cursor positioning: new compose and forward land on the `To:` line; replies land in the body
- Signature insertion via `<leader>sig` — copy `signature.md.example` to `signature.md` and edit it

Requires Neovim 0.10+. The stow package puts `nvim-mail` at `~/.local/bin/nvim-mail`.

### kitty terminal

The `mail` launcher script opens aerc in a dedicated kitty window using `kitty-mail.conf`. Bind it to a keyboard shortcut or application launcher for quick access.

The kitty color block in `kitty-mail.conf` uses the same Nord hex values as the theme. If you switch themes, update the kitty color block to match. See [docs/themes.md](docs/themes.md) for details.

## Further reading

- [docs/themes.md](docs/themes.md) — color slots, custom themes, and theme management
- [docs/filters.md](docs/filters.md) — full pipeline description, link modes, troubleshooting
- [docs/styling.md](docs/styling.md) — visual hierarchy, layout patterns, color token usage
- [docs/contributing.md](docs/contributing.md) — project layout, adding filters, adding themes, testing
