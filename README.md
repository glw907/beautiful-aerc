# beautiful-aerc

beautiful-aerc is a themeable, distributable email setup for the [aerc](https://aerc-mail.org/) email client. It ships a Go filter binary that replaces aerc's default shell-based filter pipeline, a theme system that generates consistent colors across aerc's UI and message viewer, and optional integration configs for kitty terminal and nvim-mail compose editor. The whole thing installs as a single GNU Stow package.

## What's included

- **Go binary** - three subcommands (`headers`, `html`, `plain`) that aerc calls to render every message. Replaces a tangle of shell scripts, awk, sed, and perl.
- **Theme system** - 16-slot semantic color definitions that generate both an aerc styleset (UI colors) and a palette file (message rendering colors) from one source file.
- **Three built-in themes** - Nord, Solarized Dark, and Gruvbox Dark.
- **aerc config** - `aerc.conf` and `binds.conf` ready to use, with comments. `accounts.conf.example` as a starting point.
- **nvim-mail config** - Neovim profile for composing messages in aerc, with a custom `aercmail` syntax file.
- **kitty config** - Terminal profile for launching aerc in a dedicated kitty window.
- **Launcher scripts** - `mail` (kitty + aerc) and `nvim-mail` (Neovim with the mail profile).

## Prerequisites

- [aerc](https://aerc-mail.org/) (email client)
- [pandoc](https://pandoc.org/) (HTML-to-markdown conversion, called at runtime)
- Go 1.23+ (build only)
- GNU Stow (install only)

Optional:

- [kitty](https://sw.kovidgoyal.net/kitty/) for the `mail` launcher script
- [Neovim](https://neovim.io/) for the nvim-mail compose editor
- [khard](https://github.com/lucc/khard) for address book completion

## Install

**1. Clone the repo**

```sh
git clone https://github.com/glw907/beautiful-aerc.git
cd beautiful-aerc
```

**2. Build the binary**

```sh
make build
make install   # installs to $GOPATH/bin or $GOBIN
```

`beautiful-aerc` must be on your `$PATH` before aerc can call it.

**3. Generate a theme**

Pick one of the three built-in themes and run the generator from inside `.config/aerc/`:

```sh
cd .config/aerc
themes/generate themes/nord.sh
```

This writes two files:
- `generated/palette.sh` - color tokens for the Go binary
- `stylesets/nord` - aerc styleset with hex values

**4. Install with Stow**

From your dotfiles directory (or directly):

```sh
stow beautiful-aerc
```

Or, if you're symlinking from `~/Projects/`:

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

Open `~/.config/aerc/aerc.conf` and set `styleset-name` to match the theme you generated:

```ini
styleset-name=nord
```

## How email renders

aerc routes every message through the filters defined in `aerc.conf`:

```ini
.headers=beautiful-aerc headers
text/html=beautiful-aerc html
text/plain=beautiful-aerc plain
```

- **headers** - reorders headers (From, To, Date, Subject), colorizes field names, wraps long address lines, and prints a separator line.
- **html** - calls pandoc to convert HTML to markdown, cleans up pandoc artifacts, then applies syntax highlighting for headings, bold, italic, and links.
- **plain** - detects HTML-in-plain-text MIME parts (common with some clients) and routes them through the HTML pipeline. Otherwise pipes through aerc's built-in `wrap | colorize`.

See [docs/filters.md](docs/filters.md) for the full pipeline description.

## Link display modes

The `html` subcommand supports two modes for rendering links:

**Markdown links (default)** - shows link text and URL:

```
[Check activity](https://github.com/notifications/...)
```

**Clean links** - shows link text only, URLs hidden:

```
Check activity
```

To switch to clean links, edit `aerc.conf`:

```ini
text/html=beautiful-aerc html --clean-links
```

## Switching themes

Re-run the generator with a different theme file:

```sh
cd ~/.config/aerc
themes/generate themes/solarized-dark.sh
```

Then update `styleset-name` in `aerc.conf` to match:

```ini
styleset-name=solarized-dark
```

Any customizations you made below the override marker in `generated/palette.sh` or the styleset file are preserved across regeneration.

See [docs/themes.md](docs/themes.md) for the full theme system, including how to create your own theme.

## Optional components

### nvim-mail

The `nvim-mail` Neovim profile is configured as the compose editor in `aerc.conf`. It provides a focused writing environment with syntax highlighting for the `aercmail` format (headers + body) and a dedicated set of plugins.

To use it, Neovim must be installed and `nvim-mail` must be on your `$PATH` (the stow package puts it at `~/.local/bin/nvim-mail`).

Edit `~/.config/nvim-mail/init.lua` to set your signature.

### kitty terminal

The `mail` launcher script opens aerc in a dedicated kitty window with a separate profile (`kitty-mail.conf`). Run it from a launcher or bind it to a keyboard shortcut.

The kitty color block in `kitty-mail.conf` should match your chosen theme. See [docs/themes.md](docs/themes.md) for details on keeping kitty and nvim-mail colors in sync.

## Further reading

- [docs/themes.md](docs/themes.md) - color slots, custom themes, the generator, and override mechanism
- [docs/filters.md](docs/filters.md) - full pipeline description, link modes, troubleshooting
- [docs/contributing.md](docs/contributing.md) - project layout, adding filters, adding themes, testing
