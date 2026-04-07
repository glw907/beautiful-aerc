# Binary Split and Documentation Overhaul — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the `beautiful-aerc` binary into `mailrender` (filters), `pick-link` (URL picker), and `aerc-save-email` (shell script). Separate personal config from repo defaults. Overhaul all documentation for public release.

**Architecture:** The Go module stays as one repo with four binaries (`cmd/mailrender`, `cmd/pick-link`, `cmd/fastmail-cli`, `cmd/tidytext`). The `save` subcommand becomes a standalone shell script. Configs ship as working defaults with optional bindings commented out. Personal configs move to the workstation dotfiles repo.

**Tech Stack:** Go 1.25, cobra, GNU Stow, aerc, Neovim (Lua), bash

---

### Task 1: Rename cmd/beautiful-aerc to cmd/mailrender

**Files:**
- Rename: `cmd/beautiful-aerc/` -> `cmd/mailrender/`
- Modify: `cmd/mailrender/root.go` (update Use field, remove pick-link and save)
- Delete: `cmd/mailrender/picklink.go` (will be recreated in Task 2)
- Delete: `cmd/mailrender/save.go` (replaced by shell script in Task 4)

- [ ] **Step 1: Rename the directory**

```bash
cd /home/glw907/Projects/beautiful-aerc
git mv cmd/beautiful-aerc cmd/mailrender
```

- [ ] **Step 2: Update root.go — remove pick-link and save, rename binary**

In `cmd/mailrender/root.go`, replace the entire file:

```go
package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mailrender",
		Short:        "Themeable message rendering filters for the aerc email client",
		SilenceUsage: true,
	}
	cmd.AddCommand(newHeadersCmd())
	cmd.AddCommand(newHTMLCmd())
	cmd.AddCommand(newPlainCmd())
	return cmd
}
```

- [ ] **Step 3: Delete picklink.go and save.go**

```bash
rm cmd/mailrender/picklink.go cmd/mailrender/save.go
```

- [ ] **Step 4: Verify it builds**

```bash
go build ./cmd/mailrender
```

Expected: clean build, binary named `mailrender`.

- [ ] **Step 5: Commit**

```bash
git add cmd/mailrender/ cmd/beautiful-aerc/
git rm cmd/mailrender/picklink.go cmd/mailrender/save.go
git commit -m "Rename beautiful-aerc binary to mailrender

Remove pick-link and save subcommands (extracted separately).
Keeps headers, html, and plain filter subcommands.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Create standalone pick-link binary

**Files:**
- Create: `cmd/pick-link/main.go`
- Create: `cmd/pick-link/root.go`

The root command runs the picker directly (no subcommands). Duplicates `loadPalette()` and `termCols()` from `cmd/mailrender/headers.go`.

- [ ] **Step 1: Create cmd/pick-link/main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Create cmd/pick-link/root.go**

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
	"github.com/glw907/beautiful-aerc/internal/picker"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pick-link",
		Short:        "Interactive URL picker for aerc email messages",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}

			cols := termCols()
			links, err := filter.HTMLLinks(os.Stdin, cols)
			if err != nil {
				return err
			}

			colors := picker.ColorsFromPalette(p)
			url, err := picker.Run(links, cols, colors)
			if err != nil {
				return err
			}
			if url != "" {
				name := "xdg-open"
				if strings.HasPrefix(url, "mailto:") {
					name = "aerc"
				}
				open := exec.Command(name, url)
				open.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
				return open.Start()
			}
			return nil
		},
	}
	return cmd
}

// loadPalette finds and loads the palette file relative to the binary location.
func loadPalette() (*palette.Palette, error) {
	binPath, _ := os.Executable()
	genDir := ""
	if binPath != "" {
		resolved, err := filepath.EvalSymlinks(binPath)
		if err == nil {
			binPath = resolved
		}
		binDir := filepath.Dir(binPath)
		genDir = filepath.Join(binDir, "..", "..", ".config", "aerc", "generated")
	}
	path, err := palette.FindPath(genDir)
	if err != nil {
		return nil, err
	}
	return palette.Load(path)
}

// termCols returns the terminal column count from AERC_COLUMNS or a default of 80.
func termCols() int {
	if s := os.Getenv("AERC_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
```

- [ ] **Step 3: Verify it builds**

```bash
go build ./cmd/pick-link
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add cmd/pick-link/
git commit -m "Add standalone pick-link binary

Interactive URL picker for aerc, extracted from the former
beautiful-aerc binary. Imports internal/filter and internal/picker.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Delete internal/corpus

**Files:**
- Delete: `internal/corpus/corpus.go`
- Delete: `internal/corpus/corpus_test.go`

- [ ] **Step 1: Delete the package**

```bash
rm -r internal/corpus
```

- [ ] **Step 2: Verify tests still pass**

```bash
go vet ./...
go test ./...
```

Expected: all pass. No other package imports `internal/corpus`.

- [ ] **Step 3: Commit**

```bash
git rm -r internal/corpus
git commit -m "Remove internal/corpus package

No longer needed — save functionality moves to a shell script.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Create aerc-save-email shell script

**Files:**
- Create: `.local/bin/aerc-save-email`

Replaces the Go `save` subcommand. Reads stdin, detects HTML vs plain text, writes a timestamped file to the corpus directory, counts pending files, prints a summary.

- [ ] **Step 1: Create the script**

```bash
#!/usr/bin/env bash
# aerc-save-email: Save the current email part to the corpus directory.
# Pipe from aerc via :pipe -m aerc-save-email
#
# Reads stdin, writes a timestamped file (HTML or plain text) to
# the corpus/ directory at the project root. Used for collecting
# test fixtures and debugging rendering issues.

set -euo pipefail

# Locate corpus directory: AERC_CONFIG/../../../corpus, or ~/corpus
if [[ -n "${AERC_CONFIG:-}" ]]; then
    corpus_dir="$(cd "$AERC_CONFIG/../.." 2>/dev/null && pwd)/corpus"
else
    corpus_dir="$HOME/corpus"
fi
mkdir -p "$corpus_dir"

# Read stdin into a temp file
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp"

if [[ ! -s "$tmp" ]]; then
    echo "aerc-save-email: no input" >&2
    exit 1
fi

# Detect HTML by checking the first 1024 bytes for common markers
ext="txt"
if head -c 1024 "$tmp" | grep -qiE '<html|<head|<body|<!doctype|<table'; then
    ext="html"
fi

# Timestamped filename with collision avoidance
stamp="$(date +%Y%m%d-%H%M%S)"
name="${stamp}.${ext}"
path="${corpus_dir}/${name}"
if [[ -e "$path" ]]; then
    i=2
    while [[ -e "${corpus_dir}/${stamp}-${i}.${ext}" ]]; do
        ((i++))
    done
    name="${stamp}-${i}.${ext}"
    path="${corpus_dir}/${name}"
fi

cp "$tmp" "$path"

# Count pending files
pending="$(find "$corpus_dir" -maxdepth 1 -type f | wc -l)"

# Summary (matches the vertical centering style of the old Go version)
rows="${AERC_ROWS:-24}"
pad=$(( (rows - 4) / 3 ))
printf '\033[?25l'
printf '\n%.0s' $(seq 1 "$pad")
printf ' # SAVED TO CORPUS\n'
printf '\n'
printf ' %s\n' "$name"
printf ' %d pending\n' "$pending"
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x .local/bin/aerc-save-email
```

- [ ] **Step 3: Test manually**

```bash
echo "<html><body>test</body></html>" | AERC_CONFIG=/tmp/test-aerc .local/bin/aerc-save-email
```

Expected: creates a `.html` file in the corpus dir, prints summary.

- [ ] **Step 4: Commit**

```bash
git add .local/bin/aerc-save-email
git commit -m "Add aerc-save-email shell script

Replaces the Go save subcommand. Reads stdin, detects HTML vs
plain text, writes timestamped files to the corpus directory.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Update Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Replace Makefile contents**

```makefile
build:
	go build -o mailrender ./cmd/mailrender
	go build -o pick-link ./cmd/pick-link
	go build -o fastmail-cli ./cmd/fastmail-cli
	go build -o tidytext ./cmd/tidytext

test:
	go test ./...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

install:
	GOBIN=$(HOME)/.local/bin go install ./cmd/mailrender
	GOBIN=$(HOME)/.local/bin go install ./cmd/pick-link
	GOBIN=$(HOME)/.local/bin go install ./cmd/fastmail-cli
	GOBIN=$(HOME)/.local/bin go install ./cmd/tidytext

check: vet test

clean:
	rm -f mailrender pick-link fastmail-cli tidytext

.PHONY: build test vet lint install check clean
```

- [ ] **Step 2: Verify build**

```bash
make clean && make build
```

Expected: four binaries built.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "Update Makefile for mailrender and pick-link binaries

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: Update E2E tests

**Files:**
- Modify: `e2e/e2e_test.go`

Update binary name from `beautiful-aerc` to `mailrender`. Remove the `TestSaveHTMLFixture`, `TestSavePlainText`, and `setupSaveTest` functions (save is no longer a Go subcommand).

- [ ] **Step 1: Update TestMain to build mailrender**

In `e2e/e2e_test.go`, change the build section in `TestMain`:

Replace:
```go
	tmp, err := os.MkdirTemp("", "beautiful-aerc-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tmp, "beautiful-aerc")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/beautiful-aerc")
```

With:
```go
	tmp, err := os.MkdirTemp("", "mailrender-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tmp, "mailrender")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/mailrender")
```

Replace:
```go
	paletteDir, err = os.MkdirTemp("", "beautiful-aerc-palette")
```

With:
```go
	paletteDir, err = os.MkdirTemp("", "mailrender-palette")
```

- [ ] **Step 2: Remove save tests**

Delete the `setupSaveTest`, `TestSaveHTMLFixture`, and `TestSavePlainText` functions entirely (lines 147-223 of the current file).

- [ ] **Step 3: Run tests**

```bash
make check
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add e2e/e2e_test.go
git commit -m "Update E2E tests for mailrender binary

Rename build target from beautiful-aerc to mailrender.
Remove save subcommand tests (moved to shell script).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 7: Update .gitignore

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Update binary names and add signature**

Replace:
```
/beautiful-aerc
/fastmail-cli
```

With:
```
/mailrender
/pick-link
/fastmail-cli
/tidytext
.config/aerc/signature.md
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "Update .gitignore for new binary names and signature file

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 8: Extract signature to file and update init.lua

**Files:**
- Create: `.config/aerc/signature.md.example`
- Modify: `.config/nvim-mail/init.lua`

- [ ] **Step 1: Create signature.md.example**

```markdown
**Your Name**  
phone: 555-123-4567
```

- [ ] **Step 2: Update the signature keymap in init.lua**

Replace the current `<leader>sig` keymap (the block starting with `vim.keymap.set("n", "<leader>sig"` through the closing `end, { desc = "Insert email signature" })`):

```lua
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
```

- [ ] **Step 3: Verify nvim-mail still loads**

```bash
echo "From: test@example.com\nSubject: test\n\nBody" | nvim-mail /dev/stdin
```

Expected: opens without errors. `<leader>sig` warns about missing signature file (since signature.md doesn't exist in the repo).

- [ ] **Step 4: Create personal signature.md in dotfiles**

Create `~/.dotfiles/beautiful-aerc/.config/aerc/signature.md` with the personal signature (this file is gitignored in the project repo but tracked in the workstation repo):

```markdown
**Geoffrey L. Wright**  
h 907-277-9397 | m 907-317-8472 (sporadic)
```

- [ ] **Step 5: Commit project files**

```bash
git add .config/aerc/signature.md.example .config/nvim-mail/init.lua
git commit -m "Extract signature to external file

Read signature from ~/.config/aerc/signature.md instead of
hardcoding in init.lua. Ship signature.md.example with
placeholder text.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 9: Update aerc.conf and binds.conf

**Files:**
- Modify: `.config/aerc/aerc.conf`
- Modify: `.config/aerc/binds.conf`

This task updates the repo's default configs. The personal overrides (in `~/.dotfiles/`) are handled separately in Task 14.

- [ ] **Step 1: Rewrite aerc.conf with comprehensive comments**

This is a full rewrite with Opus-quality inline comments. Every section explains what it does and why. Filter commands use `mailrender`. The file should be understandable by someone who has never used aerc.

Replace the entire file with:

```ini
#
# aerc main configuration — beautiful-aerc
#
# This is a complete, ready-to-use config for aerc with the beautiful-aerc
# filter suite. Only non-default values are set here. For all options and
# defaults, see aerc-config(5).
#
# To use this config: clone the repo, run `make install`, generate a theme
# (see .config/aerc/themes/), then stow or symlink into ~/.config/aerc/.

[general]

[ui]
# --- Message list columns ---
# Order: padding | flags | sender | subject | date | padding
# Empty start/end columns provide 1-char edge padding.
# Two-space column separator keeps it readable without wasting space.
index-columns=start>0,flags<2,name<22,subject<*,date>12,end>1
column-separator="  "
column-date={{.DateAutoFormat .Date.Local}}
column-start=
column-end=
column-flags={{.Flags | join ""}}
column-name={{.Peer | names | join ", "}}

# --- Sidebar ---
# Flat folder list at 30 chars wide. Tree mode is intentionally off because
# aerc sorts tree children alphabetically, ignoring folders-sort order.
sidebar-width=30
dirlist-tree=false

# Folder icons (Nerd Font Material Design). Each folder gets a leading icon
# via a template conditional. The fallback icon is a multi-email symbol.
dirlist-left={{if eq .Folder "Inbox"}} 󰇰{{else if eq .Folder "Notifications"}} 󰂚{{else if eq .Folder "Drafts"}} 󰏫{{else if eq .Folder "Sent"}} 󰑚{{else if eq .Folder "Archive"}} 󰀼{{else if eq .Folder "Spam"}} 󰍷{{else if eq .Folder "Trash"}} 󰩺{{else if eq .Folder "Remind"}} 󰑴{{else}} 󰡡{{end}}  {{.Folder}}
dirlist-right= {{if .Unread}}{{humanReadable .Unread}} {{end}}

# --- Sort order ---
# Newest first by default. Inbox and Notifications override to oldest first
# (see [ui:folder=...] sections below) so conversations read chronologically.
sort=-r date
threading-enabled=true

# --- Appearance ---
# Set styleset-name to match your generated theme (e.g., nord, solarized-dark).
# Run themes/generate to create the styleset from a theme source file.
styleset-name=nord
border-char-vertical=│
fuzzy-complete=true
mouse-enabled=true

# --- Tab bar ---
# Account tab shows "Mail". Compose tab shows just the subject line
# (the default includes "to:recipient" which wastes tab bar space).
tab-title-account=Mail
tab-title-composer={{.Subject}}

# --- Status icons (Nerd Font Material Design) ---
icon-new=󰇮
icon-old=󰇮
icon-replied=󰑚
icon-forwarded=󰒊
icon-flagged=󰈻
icon-marked=󰄬
icon-draft=󰏫
icon-deleted=󰆴
icon-attachment=󰏢

# --- Thread display ---
# Box-drawing characters for clean thread visualization.
thread-prefix-tip = "›"
thread-prefix-indent = " "
thread-prefix-stem = "│"
thread-prefix-limb = "─"
thread-prefix-has-siblings = "├─"
thread-prefix-last-sibling = "└─"

# --- Per-folder sort overrides ---
# Primary folders sort oldest first so conversations read top-to-bottom.
[ui:folder=Inbox]
sort=date

[ui:folder=Notifications]
sort=date

[statusline]
column-left={{.StatusInfo}}

[viewer]
# Prefer HTML over plain text. Marketing emails have better structure
# (paragraphs, headings) in HTML; their plain text is often a wall of text.
alternatives=text/html,text/plain

# Custom header rendering via the mailrender headers filter. The built-in
# aerc header area has limited styling (header.fg and header.bold only).
# Setting header-layout to a nonexistent header (X-Collapse) hides the
# built-in header rows — only the filter output is visible. The built-in
# border between header area and body still renders as a top separator.
show-headers=true
header-layout=X-Collapse

[compose]
# nvim-mail is the compose editor — a dedicated Neovim profile with
# markdown support, spell check, and prose tidying. See .config/nvim-mail/.
editor=nvim-mail

# Headers are editable in the editor buffer. nvim-mail reformats them
# (unfolds continuations, strips bare brackets, wraps at recipient boundaries).
edit-headers=true

# Address book completion via khard (Ctrl-o in compose header fields).
# Remove or change this if you use a different contact manager.
address-book-cmd=khard email --parsable --remove-first-line %s

empty-subject-warning=true

# Warn if the message mentions "attach" but has no attachments.
# Only checks the author's text (lines not starting with ">").
no-attachment-warning=^[^>]*attach

# RFC 3676 format=flowed. The editor hard-wraps at 72 chars, and aerc
# adds reflow markers on send so recipients' clients can reflow the text.
format-flowed=true

[multipart-converters]
# The "y" key in compose review converts markdown to HTML multipart via pandoc.
text/html=pandoc -f markdown -t html --standalone

[filters]
# Message rendering filters. mailrender is the Go binary from this project.
# It replaces a tangle of shell scripts, awk, sed, and perl with a single
# binary that handles all filter types.
text/plain=mailrender plain
text/html=mailrender html

# Binary attachment placeholders — prevents the "No filter configured"
# prompt when viewing messages with these MIME types.
application/zip=echo "ZIP archive - use :open or :save to download"
application/pdf=echo "PDF document - use :open or :save to download"
application/*=echo "Binary attachment - use :open or :save to download"

# Calendar and delivery status use aerc's built-in colorize filter.
text/calendar=calendar
message/delivery-status=colorize
message/rfc822=colorize

# Header rendering filter — see [viewer] section above for why this exists.
.headers=mailrender headers

[openers]

[templates]
```

- [ ] **Step 2: Rewrite binds.conf with optional bindings commented out**

Replace the entire file. fastmail-cli bindings, aerc-save-email, and tidytext-related bindings are commented out with explanatory notes:

```ini
# aerc keybindings — beautiful-aerc
#
# Binds are of the form <key sequence> = <command to run>
# To use '=' in a key sequence, substitute it with "Eq": "<Ctrl+Eq>"
# If you wish to bind #, you can wrap the key sequence in quotes: "#" = quit

# --- Global ---
<C-p> = :prev-tab<Enter>
<C-PgUp> = :prev-tab<Enter>
<C-n> = :next-tab<Enter>
<C-PgDn> = :next-tab<Enter>
\[t = :prev-tab<Enter>
\]t = :next-tab<Enter>
<C-t> = :term<Enter>
? = :help keys<Enter>
<C-c> = :prompt 'Quit?' quit<Enter>
<C-q> = :prompt 'Quit?' quit<Enter>
<C-z> = :suspend<Enter>

# --- Message list ---
[messages]
q = :prompt 'Quit?' quit<Enter>

j = :next<Enter>
<Down> = :next<Enter>
<C-d> = :next 50%<Enter>
<C-f> = :next 100%<Enter>
<PgDn> = :next 100%<Enter>

k = :prev<Enter>
<Up> = :prev<Enter>
<C-u> = :prev 50%<Enter>
<C-b> = :prev 100%<Enter>
<PgUp> = :prev 100%<Enter>
g = :select 0<Enter>
G = :select -1<Enter>

J = :next-folder<Enter>
<C-Down> = :next-folder<Enter>
K = :prev-folder<Enter>
<C-Up> = :prev-folder<Enter>
H = :collapse-folder<Enter>
<C-Left> = :collapse-folder<Enter>
L = :expand-folder<Enter>
<C-Right> = :expand-folder<Enter>

v = :mark -t<Enter>
<Space> = :mark -t<Enter>:next<Enter>
V = :mark -v<Enter>

T = :toggle-threads<Enter>
zc = :fold<Enter>
zo = :unfold<Enter>
za = :fold -t<Enter>
zM = :fold -a<Enter>
zR = :unfold -a<Enter>
<tab> = :fold -t<Enter>

<Enter> = :view<Enter>
d = :prompt 'Really delete this message?' 'delete-message'<Enter>
D = :delete<Enter>
a = :archive flat<Enter>
A = :unmark -a<Enter>:mark -T<Enter>:archive flat<Enter>

C = :compose<Enter>
m = :compose<Enter>

rr = :reply -a<Enter>
rq = :reply -aq<Enter>
Rr = :reply<Enter>
Rq = :reply -q<Enter>

c = :cf<space>
$ = :term<space>
! = :term<space>
| = :pipe<space>

/ = :search<space>
\ = :filter<space>
n = :next-result<Enter>
N = :prev-result<Enter>
<Esc> = :clear<Enter>

s = :split<Enter>
S = :vsplit<Enter>

# --- Fastmail integration (optional) ---
# Requires: fastmail-cli binary and FASTMAIL_API_TOKEN env var.
# Uncomment to enable interactive mail filter rule creation and
# masked email address management from the message list.
#
# ff = :pipe -m fastmail-cli rules interactive from<Enter>
# fs = :pipe -m fastmail-cli rules interactive subject<Enter>
# ft = :pipe -m fastmail-cli rules interactive to<Enter>
# md = :pipe -m fastmail-cli masked delete<Enter>:delete<Enter>

# --- Patches (git-email) ---
pl = :patch list<Enter>
pa = :patch apply <Tab>
pd = :patch drop <Tab>
pb = :patch rebase<Enter>
pt = :patch term<Enter>
ps = :patch switch <Tab>

[messages:folder=Drafts]
<Enter> = :recall<Enter>

# --- Message viewer ---
[view]
/ = :toggle-key-passthrough<Enter>/
q = :close<Enter>
O = :open<Enter>
o = :open<Enter>
S = :save<space>
| = :pipe<space>

# Save current email to corpus for testing/debugging (optional).
# Requires: aerc-save-email script on PATH.
# b = :pipe -m aerc-save-email<Enter>

d = :delete<Enter>
D = :close<Enter>:delete<Enter>
a = :archive flat<Enter>
A = :close<Enter>:archive flat<Enter>

# Link picker: Tab opens an interactive URL picker for the message.
# Ctrl-l prompts for a URL to open manually.
<C-l> = :open-link <space>
<Tab> = :pipe pick-link<Enter>

f = :forward<Enter>
rr = :reply -a<Enter>
rq = :reply -aq<Enter>
Rr = :reply<Enter>
Rq = :reply -q<Enter>

# --- Fastmail integration (optional) ---
# Same as [messages] section above. Uncomment to enable in the viewer.
#
# Ff = :pipe -m fastmail-cli rules interactive from<Enter>
# Fs = :pipe -m fastmail-cli rules interactive subject<Enter>
# Ft = :pipe -m fastmail-cli rules interactive to<Enter>
# md = :pipe -m fastmail-cli masked delete<Enter>:delete<Enter>

H = :toggle-headers<Enter>
<C-k> = :prev-part<Enter>
<C-Up> = :prev-part<Enter>
<C-j> = :next-part<Enter>
<C-Down> = :next-part<Enter>
J = :next<Enter>
<C-Right> = :next<Enter>
K = :prev<Enter>
<C-Left> = :prev<Enter>

[view::passthrough]
$noinherit = true
$ex = <C-x>
<Esc> = :toggle-key-passthrough<Enter>

# --- Compose ---
[compose]
$noinherit = true
$ex = <C-x>
$complete = <C-o>
<C-k> = :prev-field<Enter>
<C-Up> = :prev-field<Enter>
<C-j> = :next-field<Enter>
<C-Down> = :next-field<Enter>
<A-p> = :switch-account -p<Enter>
<C-Left> = :switch-account -p<Enter>
<A-n> = :switch-account -n<Enter>
<C-Right> = :switch-account -n<Enter>
<tab> = :next-field<Enter>
<backtab> = :prev-field<Enter>
<C-p> = :prev-tab<Enter>
<C-PgUp> = :prev-tab<Enter>
<C-n> = :next-tab<Enter>
<C-PgDn> = :next-tab<Enter>

[compose::editor]
$noinherit = true
$ex = <C-x>
<C-k> = :prev-field<Enter>
<C-Up> = :prev-field<Enter>
<C-j> = :next-field<Enter>
<C-Down> = :next-field<Enter>
<C-p> = :prev-tab<Enter>
<C-PgUp> = :prev-tab<Enter>
<C-n> = :next-tab<Enter>
<C-PgDn> = :next-tab<Enter>

# --- Compose review screen ---
# After exiting the editor (exit code 0), aerc shows this review screen.
[compose::review]
y = :multipart text/html<Enter>:send<Enter>
n = :abort<Enter>
v = :preview<Enter>
p = :postpone<Enter>
q = :choose -o d discard abort -o p postpone postpone<Enter>
e = :edit<Enter>
a = :attach<space>
d = :detach<space>

[terminal]
$noinherit = true
$ex = <C-x>

<C-p> = :prev-tab<Enter>
<C-n> = :next-tab<Enter>
<C-PgUp> = :prev-tab<Enter>
<C-PgDn> = :next-tab<Enter>
```

- [ ] **Step 3: Remove mailrules.json from repo**

```bash
git rm .config/aerc/mailrules.json
```

- [ ] **Step 4: Verify aerc loads the config (if aerc is available)**

```bash
aerc --help 2>/dev/null && echo "aerc available" || echo "skip live test"
```

- [ ] **Step 5: Commit**

```bash
git add .config/aerc/aerc.conf .config/aerc/binds.conf
git commit -m "Update configs for mailrender and public release

Rewrite aerc.conf with comprehensive inline comments for newcomers.
Update filter commands to use mailrender. Comment out optional
fastmail-cli and aerc-save-email bindings. Remove personal
mailrules.json from repo.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 10: Improve nvim-mail comments

**Files:**
- Modify: `.config/nvim-mail/init.lua`
- Modify: `.config/nvim-mail/syntax/aercmail.vim`

Add comprehensive comments for newcomers. No behavior changes.

- [ ] **Step 1: Improve init.lua comments**

Add or improve section headers and inline comments throughout the file. Key sections to annotate:

- **File header**: explain this is a dedicated Neovim profile for aerc, how to launch it, and that it uses NVIM_APPNAME for isolation.
- **Plugins**: explain why only nord.nvim is needed (treesitter is built-in for markdown).
- **Editor settings**: explain hard wrap at 72, breakat, breakindent, format-flowed relationship.
- **Quote reflow**: explain what it does (joins jagged quoted lines into paragraphs, re-wraps at 72 chars) and when it runs (on buffer open).
- **Filetype and buffer prep**: explain why we use a custom `aercmail` filetype, what VimEnter does (unfold headers, strip brackets, add separators, position cursor).
- **BufWritePre**: explain why blank lines before headers must be stripped (RFC 2822 compliance).
- **Tidytext**: explain what it does, that it's optional, and how to remove it.
- **Keybindings**: brief comment on each explaining purpose.
- **Khard**: explain what khard is and that the contact picker is optional.

This is a comment-only task. Do not change any code logic.

- [ ] **Step 2: Improve aercmail.vim comments**

Add a file header and annotate each highlight group:

```vim
" aercmail.vim — Syntax highlighting for aerc compose buffers.
"
" This is a custom filetype used instead of the built-in 'mail' filetype,
" which defines many highlight groups that conflict with our color scheme.
" Set via VimEnter autocmd in init.lua.
"
" Highlight colors use the Nord palette. To customize for a different theme,
" change the guifg hex values below. The ctermfg values are 256-color
" approximations for terminals without truecolor support.
"
" Highlight groups:
"   aercmailHeaderKey    — Header field names (From:, To:, Subject:, etc.)
"   aercmailAngleBracket — Email addresses in <angle brackets>
"   aercmailQuote1       — Single-level quoted text (> )
"   aercmailQuote2       — Nested quoted text (> > )
"
" Spell check is excluded from all groups above so the spell checker
" only flags words in the author's own text.

if exists("b:current_syntax")
  finish
endif

" Header keys (To:, From:, Subject:, etc.) — bold blue
syntax match aercmailHeaderKey /^[A-Za-z-]\+:/

" Email addresses in angle brackets — dimmed grey
syntax match aercmailAngleBracket /<[^>]\+>/

" Quoted text — more specific match first so nested quotes override single
syntax match aercmailQuote2 /^> > .*$/
syntax match aercmailQuote2 /^> >$/
syntax match aercmailQuote1 /^> .*$/
syntax match aercmailQuote1 /^>$/

highlight aercmailHeaderKey guifg=#81A1C1 gui=bold ctermfg=110 cterm=bold
highlight aercmailAngleBracket guifg=#616E88 ctermfg=60
highlight aercmailQuote1 guifg=#8FBCBB ctermfg=108
highlight aercmailQuote2 guifg=#616E88 ctermfg=60

" Exclude all custom groups from spell checking
syntax cluster Spell remove=aercmailHeaderKey,aercmailAngleBracket,aercmailQuote1,aercmailQuote2

let b:current_syntax = "aercmail"
```

- [ ] **Step 3: Commit**

```bash
git add .config/nvim-mail/init.lua .config/nvim-mail/syntax/aercmail.vim
git commit -m "Improve nvim-mail comments for newcomers

Add comprehensive section headers and inline comments to init.lua
and aercmail.vim so anyone can understand and customize the setup.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 11: Update quick reference

**Files:**
- Modify: `~/.dotfiles/docs/aerc-quickref.html`

Add `<space>t` (tidytext) to the nvim-mail compose section.

- [ ] **Step 1: Add tidytext to the khard & Tools card**

In the nvim-mail section, find the "khard & Tools" card and add a new row after the existing entries:

```html
          <div class="row"><dt><kbd>Space</kbd><kbd>t</kbd></dt><dd>Tidy prose (tidytext)</dd></div>
```

Insert it after the `<Space><kbd>sig</kbd>` row and before the `g Ctrl-g` row.

- [ ] **Step 2: Verify in browser**

```bash
xdg-open ~/.dotfiles/docs/aerc-quickref.html
```

Expected: new row visible in the khard & Tools card.

- [ ] **Step 3: Commit (in workstation dotfiles repo)**

```bash
cd ~/.dotfiles
git add docs/aerc-quickref.html
git commit -m "Add tidytext keybinding to aerc quick reference

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 12: Rewrite README.md

**Files:**
- Modify: `README.md`

Full rewrite for public release. Opus-quality prose. Position beautiful-aerc as a cohesive email environment project. This task should be assigned to Opus for best writing quality.

- [ ] **Step 1: Write the new README**

The README should cover:

1. **Opening** — one-line description, then 2-3 sentence motivation (why this exists, what problem it solves).
2. **Components** — table or list of all components with brief descriptions:
   - `mailrender` — message rendering filters (headers, html, plain)
   - `pick-link` — interactive URL picker for the message viewer
   - `fastmail-cli` — Fastmail JMAP CLI (optional, for Fastmail users)
   - `tidytext` — Claude-powered prose tidier (optional, requires API key)
   - `nvim-mail` — Neovim compose editor profile
   - aerc config — theme system, keybindings, icons
3. **Prerequisites** — aerc, pandoc, Go 1.23+, GNU Stow. Optional: kitty, Neovim, khard, Fastmail account, Anthropic API key.
4. **Install** — clone, build, generate theme, stow, configure account.
5. **How email renders** — brief filter pipeline description.
6. **Footnote-style links** — show the example.
7. **Link picker** — brief description with keybindings.
8. **Theme system** — how to switch themes, how to create your own.
9. **Optional components** — fastmail-cli, tidytext, nvim-mail, kitty.
10. **Further reading** — links to docs/.

Do not include the full content here — the implementing agent should write this fresh using the spec, existing README, and codebase for reference. The key constraint is: update all binary names to `mailrender` and `pick-link`, frame optional components clearly, and write prose that invites contribution.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Rewrite README for public release

Position beautiful-aerc as a cohesive email environment. Document
all components, installation, theme system, and optional integrations.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 13: Update CLAUDE.md and contributing.md

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/contributing.md`

- [ ] **Step 1: Update CLAUDE.md**

Changes needed:
- Update project structure: replace `cmd/beautiful-aerc/` with `cmd/mailrender/`, add `cmd/pick-link/`, remove `internal/corpus/`.
- Update filter protocol to reference `mailrender`.
- Update link picker section: `pick-link` is now a standalone binary.
- Remove references to `save` subcommand and corpus package.
- Update build section if it references binary names.
- Add a "Personal Config" section noting that the author's personal configs (with all optional bindings enabled) live in `~/.dotfiles/` (workstation repo, `beautiful-aerc` stow package). This project may need to update those files when configs change.

- [ ] **Step 2: Update contributing.md**

Changes needed:
- Update project layout tree: `cmd/mailrender/` replaces `cmd/beautiful-aerc/`, add `cmd/pick-link/`, remove `save.go`, `picklink.go`, `internal/corpus/`.
- Update build commands to use `mailrender`.
- Update "two binaries" references to "four binaries" (mailrender, pick-link, fastmail-cli, tidytext).
- Remove save-related content from E2E test section.
- Update binary name in architecture section.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md docs/contributing.md
git commit -m "Update CLAUDE.md and contributing.md for binary split

Reflect new binary names (mailrender, pick-link), removed corpus
package, and personal config location.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 14: Create personal config overrides in dotfiles

**Files:**
- Create/update: `~/.dotfiles/beautiful-aerc/.config/aerc/binds.conf` (personal version with all bindings enabled)
- Create/update: `~/.dotfiles/beautiful-aerc/.config/aerc/aerc.conf` (personal version, same as repo but with any personal overrides)
- Move: `~/.dotfiles/beautiful-aerc/.config/aerc/mailrules.json` (already gitignored in project, tracked in dotfiles)

This task sets up the author's personal configs in the workstation dotfiles repo so that stowing `beautiful-aerc` from dotfiles overlays the project defaults with personal overrides.

- [ ] **Step 1: Create personal binds.conf**

Copy the repo's `binds.conf` and uncomment all optional bindings (fastmail-cli, aerc-save-email). This file lives in `~/.dotfiles/beautiful-aerc/.config/aerc/binds.conf`.

- [ ] **Step 2: Ensure mailrules.json is tracked in dotfiles**

Verify `~/.dotfiles/beautiful-aerc/.config/aerc/mailrules.json` exists (it should already be there via stow). If not, copy it from the project before removing it from the project repo.

- [ ] **Step 3: Ensure signature.md is in dotfiles**

Should already exist from Task 8, Step 4. Verify:

```bash
cat ~/.dotfiles/beautiful-aerc/.config/aerc/signature.md
```

- [ ] **Step 4: Re-stow to apply**

```bash
cd ~/.dotfiles && stow -R beautiful-aerc
```

- [ ] **Step 5: Commit dotfiles changes**

```bash
cd ~/.dotfiles
git add beautiful-aerc/.config/aerc/binds.conf beautiful-aerc/.config/aerc/signature.md
git commit -m "Add personal aerc config overrides

Personal binds.conf with all optional bindings enabled.
Signature file for nvim-mail compose.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 15: Final verification

- [ ] **Step 1: Clean build and test**

```bash
cd /home/glw907/Projects/beautiful-aerc
make clean && make build && make check
```

Expected: four binaries build, all tests pass.

- [ ] **Step 2: Install and verify binaries**

```bash
make install
which mailrender pick-link fastmail-cli tidytext
```

Expected: all four found in `~/.local/bin/`.

- [ ] **Step 3: Verify stow works**

```bash
cd ~/.dotfiles && stow -R beautiful-aerc
ls -la ~/.config/aerc/aerc.conf ~/.config/aerc/binds.conf ~/.local/bin/nvim-mail
```

Expected: symlinks pointing to dotfiles.

- [ ] **Step 4: Smoke test filters (if aerc available)**

Open aerc, view a message. Headers, HTML, and plain text filters should render correctly using `mailrender`. Tab should open the link picker via `pick-link`.

- [ ] **Step 5: Verify no stale references**

```bash
cd /home/glw907/Projects/beautiful-aerc
grep -r "beautiful-aerc" --include="*.go" --include="*.conf" --include="Makefile" .
```

Expected: no hits in Go source, config files, or Makefile. Only hits should be in docs (README, CLAUDE.md, contributing.md) where "beautiful-aerc" refers to the project name, not the binary.
