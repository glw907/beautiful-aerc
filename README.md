# beautiful-aerc

A themeable, productive email environment for the [aerc](https://aerc-mail.org/) email client.

<!-- screenshot: Hero shot — full aerc window with the Nord theme
     size: 140x48 terminal (match kitty-mail.conf dimensions)
     show: Message list on left with Nerd Font icons and thread prefixes.
           Rendered HTML email on right with colored headers, markdown
           body with footnote links, and the URL reference section at
           the bottom. Inbox folder, mix of read/unread messages.
     file: docs/images/hero.png
-->

## Table of contents

- [Why aerc?](#why-aerc)
- [The problem: email is a mess](#the-problem-email-is-a-mess)
- [What beautiful-aerc gives you](#what-beautiful-aerc-gives-you)
- [The markdown-forward design](#the-markdown-forward-design)
- [Components](#components)
- [Prerequisites](#prerequisites)
- [Install](#install)
- [How email renders](#how-email-renders)
- [Footnote-style links](#footnote-style-links)
- [Link picker](#link-picker)
- [Theme system](#theme-system)
- [Composing email with nvim-mail](#composing-email-with-nvim-mail)
- [Optional components](#optional-components)
- [Further reading](#further-reading)

> **Want to understand the internals?** See
> [docs/power-users.md](docs/power-users.md) for the full filter
> pipeline, theme token resolution, and architectural details.

## Why aerc?

There are several terminal email clients out there — mutt, nstrstreomutt, notmuch, himalaya — but aerc stands out for a few reasons that make it the right foundation for this project.

First, aerc is fast and keyboard-driven. It's written in Go, renders quickly, and gets out of your way. Navigation is vim-style, and the interface is clean — no chrome you don't need.

Second, and more importantly, aerc is *extensible in exactly the right way*. It has a simple, powerful filter protocol: any program that reads email content on stdin and writes styled text to stdout can be a filter. This is aerc's greatest design strength — it doesn't try to do everything itself. It gives you the hooks to build what you need.

The gap? Aerc provides the engine, but its out-of-the-box experience for HTML email is basic. The default approach is to shell out to `w3m` or `lynx` for HTML-to-text conversion. The result is functional but rough — you lose document structure, links are hard to follow, and there's no theming. The compose experience is similarly bare. beautiful-aerc fills that gap.

## The problem: email is a mess

This isn't just about marketing spam. *All* email HTML on the internet is a mess.

Every sender generates HTML differently. There are no real standards in practice — just decades of accumulated workarounds. Layout tables nested three levels deep. Tracking pixels spliced into the middle of hyperlink text. Invisible Unicode characters (zero-width joiners, soft hyphens, Mongolian vowel separators) scattered through the content. Hidden `display:none` divs containing duplicated copies of the entire message body. Thunderbird-specific CSS classes. Broken nesting. Inline styles that fight each other.

GUI email clients — Gmail, Outlook, Apple Mail — do an enormous amount of hidden work to clean all of this up and present you with a coherent reading experience. You never see the mess because they handle it for you.

CLI email clients traditionally don't. They either show you the raw HTML, or pipe it through a basic text converter like `w3m` or `lynx`. You get the words, more or less, but you lose the structure, the links are buried in the text, and the experience doesn't feel like *reading email* — it feels like reading a dump.

beautiful-aerc bridges that gap. It gives you the same clean, structured email experience that GUI users take for granted, but in your terminal.

## What beautiful-aerc gives you

<!-- screenshot: Before/after comparison
     size: Two panels side by side, each ~70 columns wide
     show: The same HTML email (a newsletter or marketing email with
           links, headings, and lists) rendered by stock aerc on the
           left (w3m/lynx output) and beautiful-aerc on the right
           (mailrender html output with colors and footnote links).
           Pick something that shows the difference dramatically.
     file: docs/images/before-after.png
-->

- **Clean rendering of HTML emails** — even the messy ones. An 8-stage pipeline cleans up the worst HTML the internet can throw at it and produces readable, styled markdown.

- **Numbered footnote-style links** that keep body text clean and readable. URLs are collected in a reference section at the bottom, not jammed inline.

<!-- screenshot: Footnote links in action
     size: 80x30 terminal
     show: A rendered email with colored link text, dimmed [^N] markers
           in the body, and the numbered URL reference section at the
           bottom with the separator line.
     file: docs/images/footnote-links.png
-->

- **An interactive link picker** for opening URLs from emails. Press Tab, pick a link by number, done.

<!-- screenshot: Link picker UI
     size: 80x30 terminal
     show: The pick-link UI in an alternate screen buffer, showing
           numbered URLs with the selection highlight on one of them.
     file: docs/images/link-picker.png
-->

- **A semantic theme system** with three built-in themes (Nord, Solarized Dark, Gruvbox Dark) that colors everything consistently — the UI, the message viewer, the link picker, and the compose editor.

<!-- screenshot: Theme comparison
     size: Three panels stacked vertically or side by side
     show: The same email rendered in Nord, Solarized Dark, and Gruvbox
           Dark. Same message, same layout, different color palettes.
     file: docs/images/themes.png
-->

- **A proper compose editor** in Neovim with spell check, fuzzy contact search, and prose tidying.

- **Consistent visual design** across the entire email experience — reading, composing, and navigating all feel like parts of the same tool.

This pipeline was built by processing real personal email over many hours of iteration. Every edge case fix came from an actual broken email in the author's inbox. The project is actively maintained — check back for ongoing improvements as new sender patterns surface.

## The markdown-forward design

Markdown is the core abstraction throughout beautiful-aerc, in both directions.

**Reading email:** HTML messages are converted to clean markdown, then styled with ANSI colors for the terminal. Headings, bold text, lists, and links all render as you'd expect from a markdown document. Links become numbered footnotes so the body text stays readable.

**Writing email:** You compose in markdown in the Neovim editor. When you send, aerc converts your markdown to HTML and sends both versions as a multipart message. Recipients with GUI clients see rich text; recipients with CLI clients see your clean plain text.

Why markdown? Because it's the natural language for terminal users. It's readable as-is — no markup noise cluttering your terminal. It gives you clean formatting options (headings, bold, lists, links) when composing. And it converts losslessly to HTML for recipients who expect rich email. Your reading and writing experiences use the same formatting language, which makes the whole system feel coherent.

## Components

aerc's filter protocol is simple and powerful: any program that reads stdin and writes styled text to stdout can be a filter. aerc ships with a handful of built-in shell-script filters (`colorize`, `plaintext`, `wrap`) that work fine for plain text email between technical users.

But most email on the internet is HTML. aerc's default approach is to shell out to `w3m` or `lynx`, which produces functional but rough output — you lose document structure, links are hard to follow, and there's no theming.

beautiful-aerc replaces these defaults with purpose-built Go binaries. Why Go instead of more shell scripts? For simple filters, shell works great — aerc's design encourages that. But the problems we're solving (multi-stage HTML cleanup, RFC 2822 header parsing, interactive terminal UIs, consistent theming across tools) need things that shell scripts struggle with: proper Unicode handling, structured error handling, shared code between tools, and the ability to build real TUI applications. Go gives us single compiled binaries with no runtime dependencies — easy to install, easy to maintain.

### Go binaries (core)

**mailrender** — The filter pipeline, and the heart of the project. This is where the hard work happens.

Email HTML is the messiest markup on the internet — every sender generates it differently, there are no real standards in practice, and the edge cases are endless. mailrender is an 8+ stage pipeline that tames all of this into clean, readable markdown. It has three subcommands: `headers` for header rendering, `html` for HTML-to-markdown conversion, and `plain` for plain text handling.

Here's a taste of what email actually looks like under the hood — and what mailrender handles so you don't have to:

```go
// Every email sender on the internet embeds invisible Unicode
// characters in their HTML: soft hyphens, zero-width joiners,
// Mongolian vowel separators, byte order marks, and more. These
// are invisible to you but wreak havoc on terminal rendering.
reNBSP      = regexp.MustCompile(`[\x{a0}\x{2000}-\x{200a}]+`)
reZeroWidth = regexp.MustCompile(`[\x{ad}\x{34f}\x{180e}\x{200b}-\x{200d}\x{2060}-\x{2064}\x{feff}]`)
```

```go
// Bank of America (and others) embed 1x1 tracking pixel <img> tags
// literally inside hyperlink text. This causes pandoc to split a
// single URL across multiple disconnected paragraphs.
reZeroImg = regexp.MustCompile(
    `(?i)<img[^>]*(?:width:\s*0|height:\s*0|width="0"|height="0")[^>]*/?>`)
```

```go
// Apple receipts and responsive emails embed a hidden copy of the
// entire email body inside a display:none div — often deeply nested.
// A simple regex would close at the first inner </div>, so we
// hand-track nesting depth to find the real closing tag.
func stripHiddenElements(body string) string {
    for {
        loc := reHiddenDivOpen.FindStringIndex(body)
        if loc == nil { break }
        start := loc[0]
        rest := body[loc[1]:]
        depth := 1
        pos := 0
        for depth > 0 && pos < len(rest) {
            nextOpen := strings.Index(rest[pos:], "<div")
            nextClose := strings.Index(rest[pos:], "</div>")
            if nextClose < 0 { pos = len(rest); break }
            if nextOpen >= 0 && nextOpen < nextClose {
                depth++
                pos += nextOpen + len("<div")
            } else {
                depth--
                pos += nextClose + len("</div>")
            }
        }
        body = body[:start] + body[loc[1]+pos:]
    }
    return body
}
```

**compose-prep** — Normalizes the compose buffer before Neovim opens it. Unfolds RFC 2822 header continuations, strips bare angle brackets from email addresses, folds long address lists at 120-column boundaries, and reflows quoted text paragraphs at 72 columns. Falls back gracefully if not installed.

**pick-link** — Interactive URL picker for the message viewer. Reads the raw message, runs the HTML filter internally to extract clean footnoted URLs, then opens a full-screen picker UI where you select a link by number or with j/k navigation. Opens URLs via `xdg-open`.

### Go binaries (optional)

**fastmail-cli** — Fastmail JMAP CLI for mail filter rules, masked email management, and folder listing. Designed to be called from aerc keybindings. *(Fastmail users only.)*

**tidytext** — Claude-powered prose tidier for the compose editor. Fixes spelling, grammar, and punctuation without altering meaning or style. *(Requires Anthropic API key.)*

### Configuration and scripts

The project also ships working configuration files and launcher scripts, installed via GNU Stow:

- **aerc config** (`aerc.conf`, `binds.conf`) — ready-to-use settings with the beautiful-aerc filter pipeline, theme system, Nerd Font icons, and keybindings. Heavily commented to explain what everything does and why.
- **nvim-mail** — A dedicated Neovim profile for composing email, with custom syntax highlighting, spell check, and a fuzzy contact picker. See [Composing email with nvim-mail](#composing-email-with-nvim-mail) below.
- **kitty terminal profile** — An example of how to customize your terminal emulator for a dedicated email experience: prose-optimized font, generous padding, matching colors, and a fixed window size. See [Customizing your terminal](#customizing-your-terminal-for-email) under Optional components.
- **Launcher scripts** — `mail` launches aerc in a dedicated kitty window. `aerc-mail.desktop` puts it in your application launcher. These are examples to adapt for your own setup.

## Prerequisites

- [aerc](https://aerc-mail.org/) — the email client itself
- [pandoc](https://pandoc.org/) — called at runtime by mailrender to convert HTML to markdown
- [Go](https://go.dev/) 1.25+ — needed to build the binaries (not needed at runtime)
- [GNU Stow](https://www.gnu.org/software/stow/) — symlink manager for installing the config files

Optional:

- [Neovim](https://neovim.io/) 0.10+ — for the nvim-mail compose editor. Plugins install automatically on first launch.
- [khard](https://github.com/lucc/khard) — for address book completion in the compose editor. Needs contacts synced via [vdirsyncer](https://github.com/pimutils/vdirsyncer).
- [kitty](https://sw.kovidgoyal.net/kitty/) — for the `mail` launcher script (any terminal works with aerc itself)
- A [Nerd Font](https://www.nerdfonts.com/) — for the folder and status icons in the message list. Without one, you'll see placeholder squares for the icons, but everything else works fine.
- Fastmail account with API token — for fastmail-cli
- Anthropic API key — for tidytext

## Install

**1. Clone the repo**

```sh
git clone https://github.com/glw907/beautiful-aerc.git
cd beautiful-aerc
```

**2. Build and install the binaries**

This builds all five Go binaries and installs them to `~/.local/bin/` (make sure that's on your `PATH`):

```sh
make build
make install
```

**3. Generate a styleset**

Pick one of the three built-in themes and generate the aerc styleset:

```sh
mailrender themes generate nord
```

This reads the theme file (`themes/nord.toml`) and writes an aerc styleset to `stylesets/Nord`. The three built-in themes are `nord`, `solarized-dark`, and `gruvbox-dark`.

**4. Install the config files with Stow**

From the repo directory:

```sh
stow beautiful-aerc
```

This creates symlinks from `~/.config/aerc/`, `~/.config/nvim-mail/`, `~/.config/kitty/kitty-mail.conf`, and `~/.local/bin/` scripts into your clone. If you keep repos in `~/Projects/`, stow from there. If you manage dotfiles in `~/.dotfiles/`, you can symlink the repo into your dotfiles directory first:

```sh
ln -s ~/Projects/beautiful-aerc ~/.dotfiles/beautiful-aerc
cd ~/.dotfiles && stow beautiful-aerc
```

**5. Configure your account**

```sh
cp ~/.config/aerc/accounts.conf.example ~/.config/aerc/accounts.conf
```

Edit `accounts.conf` with your mail server settings. See the comments in the file for guidance on JMAP, IMAP, and credential helpers.

**6. Launch aerc**

```sh
aerc
```

The first launch may take a moment as aerc connects to your mail server and downloads headers. If you installed nvim-mail, the first time you compose a message Neovim will auto-install the plugins — wait a few seconds and they'll be ready.

## How email renders

aerc routes every message through filters defined in `aerc.conf`:

```ini
.headers = mailrender headers
text/html = mailrender html
text/plain = mailrender plain
```

When you open an email, here's what happens:

- **headers** — Receives the raw RFC 2822 headers. Reorders them (From, To, Cc, Date, Subject), colorizes field names, wraps long address lines to fit the terminal width, and prints a separator line below.
- **html** — Receives the raw HTML body. Cleans up sender-specific junk, calls pandoc to convert to markdown, cleans pandoc artifacts, converts links to numbered footnotes, and applies ANSI syntax highlighting for headings, bold, italic, and rules.
- **plain** — Receives the raw plain text body. Checks whether it's actually HTML in disguise (some clients send full HTML in a text/plain MIME part) and routes it through the HTML pipeline if so. Otherwise uses aerc's built-in `wrap | colorize`.

## Footnote-style links

Links in HTML emails render as footnote references. The body text stays clean and readable; URLs are collected in a numbered reference section at the bottom:

```
If you don't recognize this account, remove[^1] it.

Check activity[^2]

See https://myaccount.google.com/notifications
----------------------------------------
[^1]: https://accounts.google.com/AccountDisavow?adt=...
[^2]: https://accounts.google.com/AccountChooser?Email=...
```

Link text is colored; footnote markers are dimmed. Self-referencing links (where the display text is the URL itself) render as plain URLs with no footnote — no point adding a reference that just repeats itself.

Long URLs in the reference section are visually truncated to fit the terminal width, but the full URL is embedded in an [OSC 8 hyperlink](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda) so terminals that support it (kitty, iTerm2, etc.) can still make the truncated text clickable.

## Link picker

Press **Tab** in the message viewer to open an interactive URL picker. All links from the current message are listed with numbered shortcuts:

- **1-9, 0** — instantly open that link (0 opens the 10th)
- **j/k** or arrow keys — move the selection
- **Enter** — open the selected link
- **q** or **Escape** — cancel

The picker runs the HTML filter internally to extract the same clean footnoted URLs you see in the viewer. Selected URLs open via `xdg-open` (your system's default browser).

## Theme system

beautiful-aerc uses a semantic theme system: 16 named color slots (background, foreground, accent, error, success, etc.) defined in a single TOML file. The Go binaries read this file directly at runtime, so changing a color in the theme file changes it everywhere — the message viewer, the link picker, the header rendering, and the UI.

Three themes are included:

| Theme | Style |
|-------|-------|
| **Nord** | Cool dark (Arctic Ice Studio) |
| **Solarized Dark** | Classic dark (Ethan Schoonover) |
| **Gruvbox Dark** | Warm dark (morhetz) |

To switch themes, edit `styleset-name` in `aerc.conf`, regenerate the styleset, and restart aerc:

```sh
# In aerc.conf: styleset-name=solarized-dark
mailrender themes generate solarized-dark
# Restart aerc
```

To create your own theme, copy one of the built-in `.toml` files, adjust the hex values, and run the generator. See [docs/themes.md](docs/themes.md) for the full color slot reference and token format.

## Composing email with nvim-mail

### Why Neovim for email?

Most CLI email clients use whatever `$EDITOR` you have set — usually vim or nano. That works, but it's a minimal experience: no syntax highlighting for email, no contact search, no spell check integration, no awareness of what you're actually doing (composing a message, not editing code).

nvim-mail is a dedicated Neovim profile that turns the compose window into a proper email editor. Because Neovim is programmable, we can add features that would be impossible in a plain text editor:

- **Fuzzy contact search** — type a few letters and pick from your address book (via Telescope + khard)
- **Inline spell check** — catches misspellings as you type, with a pre-send check that prompts you before sending with errors
- **Prose tidying** — one-key AI-powered grammar and spelling fixes (via tidytext, optional)
- **Smart formatting** — headers are automatically reformatted for readability, quoted text is reflowed

If you already use Neovim, the keybindings will feel familiar. If you're new to Neovim, nvim-mail is a gentle introduction — the compose workflow only uses a handful of keys.

### Plugins

nvim-mail uses five Neovim plugins, all managed by [lazy.nvim](https://github.com/folke/lazy.nvim). **Plugins install automatically on first launch** — you don't need to install anything manually. Just open a compose window and wait a few seconds.

- **lazy.nvim** — Plugin manager. Auto-bootstraps itself the first time nvim-mail runs, then installs and updates all other plugins.
- **nord.nvim** — The Nord color scheme, matching the default aerc theme. Keeps the compose editor visually consistent with the rest of the email experience.
- **telescope.nvim** — A fuzzy finder framework. Powers the contact picker — press `Ctrl-k` in insert mode to search your address book by typing a few letters.
- **which-key.nvim** — Shows available keybindings when you press the leader key (Space) and wait. Invaluable for discovering what's available without memorizing everything.
- **nvim-treesitter** — Provides syntax highlighting for the compose buffer via the custom `aercmail` filetype.

### The compose flow

1. **Open compose** — press `C` or `m` in the message list, or `rr` to reply. aerc opens nvim-mail.
2. **Headers are reformatted** — compose-prep normalizes the raw RFC 2822 headers: unfolds continuation lines, cleans up angle brackets, wraps long address lists. You see clean, readable headers.
3. **Write your message** in markdown. Text wraps automatically at 72 characters.
4. **Exit to review** — press `<Space>q` to save and exit. If there are misspelled words, you'll be prompted: fix them, send anyway, or go back.
5. **Review and send** — aerc shows a review screen. Press `y` to convert your markdown to HTML and send, `e` to re-edit, `n` to abort, or `p` to postpone as a draft.
6. **Abort anytime** — press `<Space>x` to immediately close the compose window without sending.

For the full walkthrough, keybindings reference, contact picker setup, and troubleshooting, see [docs/nvim-mail.md](docs/nvim-mail.md).

## Optional components

### fastmail-cli

For Fastmail users, `fastmail-cli` provides JMAP operations designed to be called from aerc keybindings. Set `FASTMAIL_API_TOKEN` in your environment, then uncomment the bindings in `binds.conf`:

- **ff / fs / ft** — create a filter rule from the sender, subject, or recipient
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
# Please review the attached document.
```

In nvim-mail, `<leader>t` runs tidytext on the compose body. Changed words are highlighted with teal undercurl marks that clear on the next edit.

### Customizing your terminal for email

You don't need a special terminal setup to use beautiful-aerc — aerc runs in any terminal. But if you want your email experience to feel like a dedicated app rather than another terminal window, a separate terminal profile can make a real difference.

The project includes an example kitty profile that shows what this looks like in practice. The ideas apply to any terminal emulator:

**A prose-optimized font.** Code fonts are designed for distinguishing similar characters (`0` vs `O`, `1` vs `l`). Prose fonts are designed for comfortable reading over long periods. The example uses iA Writer Mono.

**Generous padding.** Email doesn't need to fill every pixel. Adding padding around the edges (the example uses 15px top/bottom, 30px left/right) makes the whole experience feel more spacious and less like staring at a code terminal.

**Matching colors.** The terminal's color palette should match your aerc theme so everything looks intentional.

**A fixed window size.** A consistent width (the example uses 140 columns) gives mailrender a predictable canvas for text wrapping and link truncation.

**A hidden tab bar.** aerc manages its own tabs — showing the terminal's tab bar too is redundant and confusing.

Here's the launcher script that ties it together:

```bash
#!/usr/bin/env bash
exec kitty --class aerc-mail --config ~/.config/kitty/kitty-mail.conf --title Mail aerc
```

And a `.desktop` file so it appears in your application launcher:

```ini
[Desktop Entry]
Name=Mail (aerc)
Comment=Terminal email client
Exec=mail
Icon=mutt
Terminal=false
Type=Application
Categories=Network;Email;
StartupWMClass=aerc-mail
Keywords=mail;email;aerc;
```

See `.config/kitty/kitty-mail.conf` for the full profile with annotated design choices. Adapt these ideas for your own terminal emulator.

## Further reading

- [docs/themes.md](docs/themes.md) — Color slots, token format, custom themes, and styleset generation
- [docs/nvim-mail.md](docs/nvim-mail.md) — Full compose workflow, keybindings, contact picker, and troubleshooting
- [docs/power-users.md](docs/power-users.md) — Filter pipeline internals, theme token resolution, edge cases, and architecture
- [docs/contributing.md](docs/contributing.md) — Project layout, code conventions, adding filters, adding themes, testing
- [docs/styling.md](docs/styling.md) — Visual hierarchy, layout patterns, and color token usage for contributors
