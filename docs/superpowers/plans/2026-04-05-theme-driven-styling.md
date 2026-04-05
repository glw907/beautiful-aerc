# Theme-Driven Styling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate all hardcoded ANSI style modifiers from Go source so every text styling attribute flows through composite palette tokens defined in theme files.

**Architecture:** Theme files define composite tokens (color + modifiers like bold/italic/underline). The generator resolves them to ANSI parameter strings in `palette.sh`. Go code reads pre-resolved strings via `palette.ANSI(key)` — zero modifier logic in Go. Base color tokens are pure hex variables; composite tokens are "CSS classes" that reference them.

**Tech Stack:** Shell (generator), Go (palette consumer), aerc theme system

---

### Task 1: Add composite tokens to theme files

**Files:**
- Modify: `.config/aerc/themes/nord.sh`
- Modify: `.config/aerc/themes/gruvbox-dark.sh`
- Modify: `.config/aerc/themes/solarized-dark.sh`

- [ ] **Step 1: Add header, picker, and message UI tokens to `nord.sh`**

Replace the existing `# -- Message UI tokens` section and add header/picker sections. The file currently ends with `C_MSG_TITLE_ACCENT` at line 39. Replace everything from `# -- Message UI tokens` through end of file:

```sh
# -- Header tokens --
C_HDR_KEY="$ACCENT_PRIMARY bold"
C_HDR_VALUE="$FG_BASE"
C_HDR_DIM="$FG_DIM"

# -- Picker tokens --
C_PICKER_NUM="$ACCENT_PRIMARY"
C_PICKER_LABEL="$FG_BASE"
C_PICKER_URL="$FG_DIM"
C_PICKER_SEL_BG="$BG_SELECTION"
C_PICKER_SEL_FG="$FG_BRIGHT"

# -- Message UI tokens (overlays and confirmations) --
C_MSG_MARKER="$FG_DIM bold"
C_MSG_TITLE_SUCCESS="$COLOR_SUCCESS bold"
C_MSG_TITLE_ACCENT="$ACCENT_PRIMARY bold"
C_MSG_DETAIL="$FG_BASE"
C_MSG_DIM="$FG_DIM"
```

- [ ] **Step 2: Add the same token sections to `gruvbox-dark.sh`**

Append after the existing `C_RULE` line (line 27):

```sh

# -- Header tokens --
C_HDR_KEY="$ACCENT_PRIMARY bold"
C_HDR_VALUE="$FG_BASE"
C_HDR_DIM="$FG_DIM"

# -- Picker tokens --
C_PICKER_NUM="$ACCENT_PRIMARY"
C_PICKER_LABEL="$FG_BASE"
C_PICKER_URL="$FG_DIM"
C_PICKER_SEL_BG="$BG_SELECTION"
C_PICKER_SEL_FG="$FG_BRIGHT"

# -- Message UI tokens (overlays and confirmations) --
C_MSG_MARKER="$FG_DIM bold"
C_MSG_TITLE_SUCCESS="$COLOR_SUCCESS bold"
C_MSG_TITLE_ACCENT="$ACCENT_PRIMARY bold"
C_MSG_DETAIL="$FG_BASE"
C_MSG_DIM="$FG_DIM"
```

- [ ] **Step 3: Add the same token sections to `solarized-dark.sh`**

Same content as step 2, appended after `C_RULE` (line 27).

- [ ] **Step 4: Commit**

```bash
git add .config/aerc/themes/nord.sh .config/aerc/themes/gruvbox-dark.sh .config/aerc/themes/solarized-dark.sh
git commit -m "Add composite tokens for headers, picker, and message UI to all themes"
```

---

### Task 2: Update generator to emit new tokens

**Files:**
- Modify: `.config/aerc/themes/generate`

- [ ] **Step 1: Add new token sections to the palette.sh emit block**

In the `generate` script, find the heredoc that writes palette.sh (starts around line 101 with `cat >> "$PALETTE" << EOF`). After the `C_RESET="0"` line (line 127), add the new token sections before the closing `EOF`:

```sh

# -- Header tokens (ANSI) --
C_HDR_KEY="$(resolve_token "$C_HDR_KEY")"
C_HDR_VALUE="$(resolve_token "$C_HDR_VALUE")"
C_HDR_DIM="$(resolve_token "$C_HDR_DIM")"

# -- Picker tokens (ANSI) --
C_PICKER_NUM="$(resolve_token "$C_PICKER_NUM")"
C_PICKER_LABEL="$(resolve_token "$C_PICKER_LABEL")"
C_PICKER_URL="$(resolve_token "$C_PICKER_URL")"
C_PICKER_SEL_BG="$(resolve_token "$C_PICKER_SEL_BG")"
C_PICKER_SEL_FG="$(resolve_token "$C_PICKER_SEL_FG")"

# -- Message UI tokens (ANSI) --
C_MSG_MARKER="$(resolve_token "$C_MSG_MARKER")"
C_MSG_TITLE_SUCCESS="$(resolve_token "$C_MSG_TITLE_SUCCESS")"
C_MSG_TITLE_ACCENT="$(resolve_token "$C_MSG_TITLE_ACCENT")"
C_MSG_DETAIL="$(resolve_token "$C_MSG_DETAIL")"
C_MSG_DIM="$(resolve_token "$C_MSG_DIM")"
```

- [ ] **Step 2: Regenerate palette from nord theme and verify new tokens**

```bash
cd .config/aerc && themes/generate themes/nord.sh
```

Expected: output shows `Theme: themes/nord.sh`, `Palette: generated/palette.sh`, `Styleset: stylesets/nord`.

- [ ] **Step 3: Verify new tokens appear in generated palette**

```bash
grep -E '^C_(HDR|PICKER|MSG)' .config/aerc/generated/palette.sh
```

Expected output (ANSI param strings, not hex):

```
C_HDR_KEY="1;38;2;129;161;193"
C_HDR_VALUE="38;2;216;222;233"
C_HDR_DIM="38;2;97;110;136"
C_PICKER_NUM="38;2;129;161;193"
C_PICKER_LABEL="38;2;216;222;233"
C_PICKER_URL="38;2;97;110;136"
C_PICKER_SEL_BG="38;2;57;67;83"
C_PICKER_SEL_FG="38;2;229;233;240"
C_MSG_MARKER="1;38;2;97;110;136"
C_MSG_TITLE_SUCCESS="1;38;2;163;190;140"
C_MSG_TITLE_ACCENT="1;38;2;129;161;193"
C_MSG_DETAIL="38;2;216;222;233"
C_MSG_DIM="38;2;97;110;136"
```

- [ ] **Step 4: Commit**

```bash
git add .config/aerc/themes/generate .config/aerc/generated/palette.sh
git commit -m "Emit header, picker, and message UI tokens in generator"
```

---

### Task 3: Update e2e test palette with new tokens

**Files:**
- Modify: `e2e/e2e_test.go`

The e2e tests create a test palette in `TestMain`. It needs the new composite tokens so the binary can load them.

- [ ] **Step 1: Add new tokens to test palette string**

In `e2e/e2e_test.go`, find the `palette` string literal in `TestMain` (around line 42). After the `C_RESET="0"` line, add:

```go
C_HDR_KEY="1;38;2;129;161;193"
C_HDR_VALUE="38;2;216;222;233"
C_HDR_DIM="38;2;97;110;136"
C_PICKER_NUM="38;2;129;161;193"
C_PICKER_LABEL="38;2;216;222;233"
C_PICKER_URL="38;2;97;110;136"
C_PICKER_SEL_BG="38;2;57;67;83"
C_PICKER_SEL_FG="38;2;229;233;240"
C_MSG_MARKER="1;38;2;97;110;136"
C_MSG_TITLE_SUCCESS="1;38;2;163;190;140"
C_MSG_TITLE_ACCENT="1;38;2;129;161;193"
C_MSG_DETAIL="38;2;216;222;233"
C_MSG_DIM="38;2;97;110;136"
```

- [ ] **Step 2: Run tests to verify nothing breaks**

```bash
make check
```

Expected: all tests pass (the new tokens are additive — existing code ignores them).

- [ ] **Step 3: Commit**

```bash
git add e2e/e2e_test.go
git commit -m "Add composite tokens to e2e test palette"
```

---

### Task 4: Switch headers.go to composite tokens

**Files:**
- Modify: `cmd/beautiful-aerc/headers.go`

- [ ] **Step 1: Replace `colorsFromPalette` to use composite tokens**

Replace the entire `colorsFromPalette` function (lines 49-70):

```go
// colorsFromPalette builds a ColorSet from palette entries.
func colorsFromPalette(p *palette.Palette) *filter.ColorSet {
	return &filter.ColorSet{
		HdrKey: p.ANSI("C_HDR_KEY"),
		HdrFG:  p.ANSI("C_HDR_VALUE"),
		HdrDim: p.ANSI("C_HDR_DIM"),
		Reset:  p.Reset(),
	}
}
```

This removes:
- Three `p.Get()` + `palette.HexToANSI()` calls
- Hardcoded bold in `"\033[1;" + ansiKey + "m"`
- Manual `"\033[" + ansi + "m"` assembly

- [ ] **Step 2: Remove unused `palette` import if needed**

Check if `palette.HexToANSI` is still referenced. Since `p` is `*palette.Palette`, the `palette` import is still needed for the type. But the direct `palette.HexToANSI()` call is gone. The import stays because `loadPalette` returns `*palette.Palette`.

- [ ] **Step 3: Run tests**

```bash
make check
```

Expected: all tests pass. The e2e `TestHeadersFixture` validates the headers filter still works — ANSI sequences will differ (composite token values vs manually assembled) but the test only checks for `From:` and `Subject:` presence, so it passes.

- [ ] **Step 4: Commit**

```bash
git add cmd/beautiful-aerc/headers.go
git commit -m "Switch header styling to composite palette tokens"
```

---

### Task 5: Switch picker.go to composite tokens

**Files:**
- Modify: `internal/picker/picker.go`

- [ ] **Step 1: Replace `ColorsFromPalette` to use composite tokens**

Replace the entire `ColorsFromPalette` function (lines 61-83):

```go
// ColorsFromPalette builds picker colors from a loaded palette.
func ColorsFromPalette(p *palette.Palette) *Colors {
	c := &Colors{Reset: "\033[0m"}
	if v := p.ANSI("C_PICKER_NUM"); v != "" {
		c.Number = v
	}
	if v := p.ANSI("C_PICKER_LABEL"); v != "" {
		c.Label = v
	}
	if v := p.ANSI("C_PICKER_URL"); v != "" {
		c.URL = v
	}
	selBG := p.Get("C_PICKER_SEL_BG")
	selFG := p.Get("C_PICKER_SEL_FG")
	if selBG != "" && selFG != "" {
		bgParam := strings.Replace(selBG, "38;2;", "48;2;", 1)
		c.Selected = "\033[" + bgParam + "m\033[" + selFG + "m"
	}
	return c
}
```

This removes all `palette.HexToANSI()` calls and the `FG_PRIMARY` reference (which was a bug — the token is `FG_BASE` in the palette, `FG_PRIMARY` doesn't exist).

- [ ] **Step 2: Update the `render` function heading line**

Find the heading line in `render` (line 204):

```go
fmt.Fprintf(w, "\033[2K \033[1m%s#%s %s%s%s\n", colors.URL, colors.Reset, colors.Number, pickerHeading, colors.Reset)
```

The `\033[1m` hardcodes bold on the `#`. Replace with two new fields on the `Colors` struct. First, add two fields to the `Colors` struct (after line 23):

```go
type Colors struct {
	Number   string // shortcut number (1-9, 0)
	Label    string // link label text
	URL      string // URL text (dim)
	Selected string // highlighted line (bg + fg)
	Marker   string // heading # marker
	Title    string // heading text
	Reset    string
}
```

Then update `ColorsFromPalette` to populate the new fields. Add before the `return c` line:

```go
	if v := p.ANSI("C_MSG_MARKER"); v != "" {
		c.Marker = v
	}
	if v := p.ANSI("C_MSG_TITLE_ACCENT"); v != "" {
		c.Title = v
	}
```

Then replace the heading line in `render`:

```go
fmt.Fprintf(w, "\033[2K %s#%s %s%s%s\n", colors.Marker, colors.Reset, colors.Title, pickerHeading, colors.Reset)
```

- [ ] **Step 3: Remove unused imports**

After removing `palette.HexToANSI()` calls, the `palette` import may need updating. Check: `ColorsFromPalette` still takes `*palette.Palette` and calls `p.ANSI()` and `p.Get()`, so the import stays. But verify the `strings` import is still used (yes, for `strings.Replace` on the bg param).

- [ ] **Step 4: Run tests**

```bash
make check
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/picker/picker.go
git commit -m "Switch picker styling to composite palette tokens"
```

---

### Task 6: Switch save.go to composite tokens

**Files:**
- Modify: `cmd/beautiful-aerc/save.go`

- [ ] **Step 1: Replace `printSaveNotification` to use composite tokens**

Replace the entire `printSaveNotification` function (lines 62-90):

```go
func printSaveNotification(filename string, pending int) {
	p, _ := loadPalette()
	marker := ""
	heading := ""
	detail := ""
	dim := ""
	reset := ""
	if p != nil {
		marker = p.ANSI("C_MSG_MARKER")
		heading = p.ANSI("C_MSG_TITLE_SUCCESS")
		detail = p.ANSI("C_MSG_DETAIL")
		dim = p.ANSI("C_MSG_DIM")
		reset = p.Reset()
	}

	rows := termRows()
	pad := (rows - 4) / 3
	fmt.Print("\033[?25l")
	fmt.Print(strings.Repeat("\n", pad))

	fmt.Printf(" %s#%s %sSAVED TO CORPUS%s\n", marker, reset, heading, reset)
	fmt.Println()
	fmt.Printf(" %s%s%s\n", detail, filename, reset)
	fmt.Printf(" %s%d pending%s\n", dim, pending, reset)
}
```

This removes:
- Three `palette.HexToANSI()` calls
- Hardcoded bold in `"\033[" + ansi + ";1m"` and `"\033[1m"`
- Manual `"\033[" + ansi + "m"` assembly

- [ ] **Step 2: Remove unused `palette` import**

The `palette` package is no longer directly referenced in `save.go` (it was used for `palette.HexToANSI`). The `loadPalette()` function returns `*palette.Palette` but that's defined in `headers.go` in the same package — so `save.go` doesn't need the import. Remove the `"github.com/glw907/beautiful-aerc/internal/palette"` import from `save.go`.

- [ ] **Step 3: Run tests**

```bash
make check
```

Expected: all tests pass. The e2e save tests (`TestSaveHTMLFixture`, `TestSavePlainText`) validate file creation, not ANSI output.

- [ ] **Step 4: Commit**

```bash
git add cmd/beautiful-aerc/save.go
git commit -m "Switch save notification styling to composite palette tokens"
```

---

### Task 7: Update documentation

**Files:**
- Modify: `docs/themes.md`
- Create: `docs/styling.md`
- Remove: `docs/message-ui.md`
- Modify: `docs/filters.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update `docs/themes.md`**

After the existing "Markdown tokens" section (ends around line 49), add a new section for UI tokens. Insert before the "Built-in themes" heading:

```markdown
## UI tokens

Beyond markdown, composite tokens control styling for headers,
the link picker, and message overlays. Like markdown tokens, they
reference base color slots and can include style modifiers.

| Token | Controls |
|-------|----------|
| `C_HDR_KEY` | Header field names (From, Subject, etc.) |
| `C_HDR_VALUE` | Header field values |
| `C_HDR_DIM` | Header secondary text (angle brackets, etc.) |
| `C_PICKER_NUM` | Picker shortcut digits (1-9, 0) |
| `C_PICKER_LABEL` | Picker link label text |
| `C_PICKER_URL` | Picker URL text |
| `C_PICKER_SEL_BG` | Picker selected row background |
| `C_PICKER_SEL_FG` | Picker selected row foreground |
| `C_MSG_MARKER` | Message heading `#` marker |
| `C_MSG_TITLE_SUCCESS` | Success heading (confirmations) |
| `C_MSG_TITLE_ACCENT` | Interactive heading (picker, prompts) |
| `C_MSG_DETAIL` | Message detail text (filenames, labels) |
| `C_MSG_DIM` | Message secondary text (counts, hints) |

Available modifiers: `bold`, `italic`, `underline`. Combine freely:

```sh
C_HDR_KEY="$ACCENT_PRIMARY bold italic"
C_MSG_TITLE_SUCCESS="$COLOR_SUCCESS bold underline"
C_PICKER_LABEL="$FG_BASE underline"
```

All text styling in the Go binary uses composite tokens. ANSI
modifiers are never hardcoded — if you need a different style for
any element, change its token in the theme file.
```

- [ ] **Step 2: Create `docs/styling.md`**

This absorbs `docs/message-ui.md` content and broadens it. Write to `docs/styling.md`:

```markdown
# Styling

Guidelines for visual styling across all beautiful-aerc UI
elements. Every color, weight, italic, and underline attribute
comes from the theme — nothing is hardcoded in Go source.

## Principle

Use composite palette tokens for all text styling. Base color
tokens (`ACCENT_PRIMARY`, `FG_DIM`, etc.) are pure hex values —
think CSS variables. Composite tokens (`C_HDR_KEY`, `C_MSG_MARKER`,
etc.) reference base tokens and add modifiers — think CSS classes.

Go code reads pre-resolved ANSI parameter strings via
`palette.ANSI(key)`. It never assembles ANSI codes manually or
hardcodes modifiers like bold, italic, or underline.

See [themes.md](themes.md) for the full token reference and
modifier syntax.

## Visual Hierarchy

Every message screen uses a three-tier hierarchy:

1. **Title** — markdown header style: `# ALL CAPS`. The `#` in
   `C_MSG_MARKER`, the title in bold + semantic color
   (`C_MSG_TITLE_SUCCESS` or `C_MSG_TITLE_ACCENT`). Short and
   scannable.
2. **Detail** — normal case, `C_MSG_DETAIL`. The key information
   (filename, list items). Left-aligned.
3. **Secondary** — normal case, `C_MSG_DIM`. Counts, hints,
   metadata. Left-aligned.

A blank line separates the title from the content below it.

### Examples

```
 # SAVED TO CORPUS          (# C_MSG_MARKER, title: C_MSG_TITLE_SUCCESS)

 20260404-220235.html       (C_MSG_DETAIL)
 10 pending                 (C_MSG_DIM)
```

```
 # OPEN LINK                (# C_MSG_MARKER, title: C_MSG_TITLE_ACCENT)

 1  Download invoice  …     (number: C_PICKER_NUM,
 2  Download receipt  …      label: C_PICKER_LABEL, max 72 chars
 3  support site      …      url: C_PICKER_URL, fills to edge)
```

## Layout

- **Vertically at the 1/3 mark.** Compute the block height (title +
  blank line + content lines) and pad from the top by
  `(rows - blockHeight) / 3`. Query terminal size from the tty fd
  with `TIOCGWINSZ`, or fall back to `AERC_ROWS` / 24.
- **Left-aligned** with a single space indent.
- **Full terminal width.** The picker reads actual terminal width
  from the tty (not `AERC_COLUMNS`, which reflects the viewer pane).
  URLs fill to the terminal edge.

## Color Token Reference

| Role | Token | Usage |
|------|-------|-------|
| Success title | `C_MSG_TITLE_SUCCESS` | confirmations |
| Interactive title | `C_MSG_TITLE_ACCENT` | picker, prompts |
| Title marker | `C_MSG_MARKER` | `#` prefix |
| Detail text | `C_MSG_DETAIL` | filenames, labels |
| Secondary text | `C_MSG_DIM` | counts, hints, URLs |
| Header keys | `C_HDR_KEY` | From, Subject, etc. |
| Header values | `C_HDR_VALUE` | field values |
| Header dim | `C_HDR_DIM` | angle brackets |
| Selection bg | `C_PICKER_SEL_BG` | picker row |
| Selection fg | `C_PICKER_SEL_FG` | picker row |
| Shortcut numbers | `C_PICKER_NUM` | picker digits |
| Link labels | `C_PICKER_LABEL` | picker link text |
| Link URLs | `C_PICKER_URL` | picker URL text |

Always pair color sequences with a `\033[0m` reset.

## Interactive Overlays (picker)

- Use the **alternate screen buffer** (`\033[?1049h` / `\033[?1049l`)
  so aerc's view restores cleanly on exit.
- **Hide the cursor** (`\033[?25l`) during interaction; restore on
  exit.
- Read keyboard from `/dev/tty` opened `O_RDWR` — write UI output
  to the same fd for full terminal independence from aerc.
- Flicker-free updates: cursor-home (`\033[H`) + per-line clear
  (`\033[2K`) instead of full screen clear.

## Confirmation Screens (save)

- Output goes to **stdout** so aerc's `:pipe` terminal widget
  displays it. aerc appends "Process complete, press any key to
  close" automatically.
- Follow the three-tier hierarchy: title, detail, secondary.

## Launching External Processes

- Detach with `SysProcAttr{Setsid: true}` so the child survives
  aerc's process group cleanup.
- Route `mailto:` URLs to `aerc` (IPC compose); everything else
  to `xdg-open`.

## aerc Constraints

aerc has no overlay modal API. The two feedback mechanisms are:

- **`:pipe`** — runs a command, shows stdout in a terminal widget,
  then "Process complete, press any key to close." Best for
  confirmations and interactive UIs.
- **`:pipe -b`** — runs in background, shows "completed with
  status 0" briefly in the status bar. Too brief for user-facing
  messages; avoid.

Interactive overlays work around `:pipe` limitations by opening
`/dev/tty` directly for both input and output, bypassing aerc's
I/O capture entirely.
```

- [ ] **Step 3: Remove `docs/message-ui.md`**

```bash
git rm docs/message-ui.md
```

- [ ] **Step 4: Update `docs/filters.md` color references**

In `docs/filters.md`, update these sections:

Line 153 (Header colorization): Replace "Header field names (From, To, Date, Subject) are printed in `ACCENT_PRIMARY` bold." with "Header field names (From, To, Date, Subject) are styled with `C_HDR_KEY`. Field values use `C_HDR_VALUE`. Angle brackets around email addresses use `C_HDR_DIM`."

Line 131 (Picker colors section, around lines 131-133): Replace:
```
- Number: `ACCENT_PRIMARY`
- URL text: `FG_DIM`
- Selected line: `BG_SELECTION` + `FG_BRIGHT`
```
with:
```
- Number: `C_PICKER_NUM`
- Label: `C_PICKER_LABEL`
- URL text: `C_PICKER_URL`
- Selected line: `C_PICKER_SEL_BG` + `C_PICKER_SEL_FG`
```

- [ ] **Step 5: Update `CLAUDE.md`**

Two changes:

1. In the "Message UI" section (around line 102), replace the pointer to `docs/message-ui.md` with `docs/styling.md`:

```markdown
## Styling

**Read `docs/styling.md` before building any UI element.** It
defines the visual hierarchy, layout patterns, color token usage,
and aerc integration patterns. See `docs/themes.md` for the token
reference and theme file format.
```

2. In the "Theme System" section (around line 55), add after the last line of that section:

```markdown
**Never hardcode ANSI color codes or style modifiers (bold, italic,
underline) in Go source.** All text styling must use composite
palette tokens defined in the theme file. If a UI element needs
styling, add a token to the theme and reference it through the
palette.
```

- [ ] **Step 6: Commit**

```bash
git add docs/themes.md docs/styling.md docs/filters.md CLAUDE.md
git rm docs/message-ui.md
git commit -m "Consolidate styling docs, add no-hardcoded-styles directive"
```

---

### Task 8: Final validation

- [ ] **Step 1: Run make check**

```bash
make check
```

Expected: all tests pass (vet + unit + e2e).

- [ ] **Step 2: Regenerate palette and verify**

```bash
cd .config/aerc && themes/generate themes/nord.sh && cd ../..
```

Verify the generated palette has all tokens:

```bash
grep -c '^C_' .config/aerc/generated/palette.sh
```

Expected: 20 tokens (6 markdown + 1 reset + 3 header + 8 picker-ish + 5 message — actually count: C_HEADING, C_BOLD, C_ITALIC, C_LINK_TEXT, C_LINK_URL, C_RULE, C_RESET, C_HDR_KEY, C_HDR_VALUE, C_HDR_DIM, C_PICKER_NUM, C_PICKER_LABEL, C_PICKER_URL, C_PICKER_SEL_BG, C_PICKER_SEL_FG, C_MSG_MARKER, C_MSG_TITLE_SUCCESS, C_MSG_TITLE_ACCENT, C_MSG_DETAIL, C_MSG_DIM = 20).

- [ ] **Step 3: Verify no hardcoded modifiers remain in Go source**

```bash
grep -rn '\\033\[1m\|\\033\[3m\|\\033\[4m\|;1m\|;3m\|;4m' cmd/ internal/ --include='*.go' | grep -v _test.go | grep -v '// '
```

Expected: no matches in non-test Go files.

- [ ] **Step 4: Install and verify visually**

```bash
make install
```

Then use tmux-based testing (see `.claude/docs/tmux-testing.md`) to verify headers, save notification, and picker render correctly in aerc.
