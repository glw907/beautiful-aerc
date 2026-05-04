# Pass 8.7 — Attachments II Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render attachment chips in the viewer, give users a
keyboard-driven `@` picker that opens or saves a chosen attachment,
and route saves into a configurable download dir.

**Architecture:** A new chip-row region in `Viewer` between the
header panel and body. A new `AttachPicker` overlay (mirrors
`LinkPicker`) owned by `App`. Three new tea.Cmds — metadata fetch
(batched with body fetch), `xdg-open` on a temp file, and save to
the configured download dir with collision suffixing. Config gains
`[ui] download_dir`.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, bubbles/viewport,
modernc.org/sqlite via `internal/cache`.

**Spec:** `docs/superpowers/specs/2026-05-04-attachments-ii-design.md`

---

## File Structure

**Create:**

- `internal/ui/attachpicker.go` — `AttachPicker` modal (mirrors
  `LinkPicker`).
- `internal/ui/attachpicker_test.go` — picker behavior +
  rendering tests.
- `internal/ui/humanize.go` — `humanizeBytes(int64) string` (one
  function, used by chip row + picker).
- `internal/ui/humanize_test.go`.

**Modify:**

- `internal/ui/icons.go` — add `Attachment` field to `IconSet`,
  populate `SimpleIcons` + `FancyIcons`.
- `internal/ui/icons_test.go` — extend class-invariant tests to
  cover the new field.
- `internal/ui/viewer.go` — chip row state + layout + `@` key.
- `internal/ui/viewer_test.go` — chip row render tests.
- `internal/ui/keys.go` — `ViewerKeys.OpenAttachPicker`.
- `internal/ui/cmds.go` — new Msg types + Cmds (fetch / open /
  save).
- `internal/ui/cmds_test.go` — open/save Cmd tests.
- `internal/ui/account_tab.go` — batch metadata fetch in
  `openMessage`; route `attachmentsLoadedMsg`.
- `internal/ui/account_tab_test.go` — extend openMessage tests.
- `internal/ui/app.go` — `NewApp` signature + downloadDir field +
  overlay cascade slot + new Msg routing.
- `internal/ui/app_test.go` — overlay routing tests.
- `internal/config/ui.go` — `DownloadDir string` + TOML key +
  resolver.
- `internal/config/ui_test.go` — resolution table.
- `cmd/poplar/root.go` — resolve and thread `DownloadDir`.
- `docs/poplar/keybindings.md` — add `@` row + picker table.
- `docs/poplar/wireframes.md` — viewer chip row + picker frame.
- `.claude/rules/ui-invariants.md` — Viewer / Overlays additions.

ADRs + invariants index update happen at pass end via the
`poplar-pass` consolidation skill, not as task steps.

---

## Task 1: humanizeBytes helper

**Files:**
- Create: `internal/ui/humanize.go`
- Create: `internal/ui/humanize_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/ui/humanize_test.go
package ui

import "testing"

func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{999, "999 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(2.4 * 1024 * 1024), "2.4 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		if got := humanizeBytes(c.n); got != c.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/ui/humanize.go
// SPDX-License-Identifier: MIT

package ui

import "fmt"

// humanizeBytes formats a byte count for the attachment chip row
// and picker. Decimal one-place precision above 1 KB.
func humanizeBytes(n int64) string {
	const k = 1024.0
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	}
}
```

- [ ] **Step 3: Run tests**

```
go test ./internal/ui/ -run TestHumanizeBytes -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```
git add internal/ui/humanize.go internal/ui/humanize_test.go
git commit -m "Pass 8.7: humanizeBytes helper for attachment sizes"
```

---

## Task 2: IconSet.Attachment field

**Files:**
- Modify: `internal/ui/icons.go`
- Modify: `internal/ui/icons_test.go`

- [ ] **Step 1: Add field + populate both tables**

In `internal/ui/icons.go`, add `Attachment string` to `IconSet`
(after `FlagUnread`). In `SimpleIcons` add:

```go
Attachment: "📎", // U+1F4CE — wait, this is Wide; pick narrow
```

Actually U+1F4CE is East Asian Wide. Use a Narrow alternative:
`"⊕"` (U+2295) is Na/N, but no clear paperclip mnemonic. Pick
ASCII `"@"` won't work (ambiguous with the picker key). Use
`"§"` (U+00A7, Na). Verify with the existing icon class test —
if the runtime check fails, fall back to ASCII `"+"`.

Concrete choice for Simple: `"§"`. If the existing class test
flags it as non-Narrow, switch to `"+"` and re-run. Document the
decision in the ADR written at pass end.

In `FancyIcons` add (must be in U+F0000–U+FFFFD per existing
invariant — Nerd Font paperclip is U+F0C6, which is *not* in
SPUA-A and would break the existing fancy-class test). Use
`"\U000F0184"` (Nerd Font `nf-md-paperclip`, U+F0184) — confirmed
SPUA-A.

```go
// SimpleIcons addition
Attachment: "§", // narrow fallback; § ≈ paperclip-ish

// FancyIcons addition
Attachment: "\U000F0184", // nf-md-paperclip
```

- [ ] **Step 2: Update icons_test.go**

The existing `TestSimpleIcons_AllNarrow` and the fancy SPUA-A
test iterate via reflection or list each field. Inspect, then add
`Attachment` to whichever inventory list it uses. If it iterates
via reflection over all `IconSet` string fields, no change needed.

```
go test ./internal/ui/ -run TestSimpleIcons -v
go test ./internal/ui/ -run TestFancyIcons -v
```

Expected: PASS. If either fails on the Simple choice, swap
SimpleIcons.Attachment to `"+"` and re-run.

- [ ] **Step 3: Commit**

```
git add internal/ui/icons.go internal/ui/icons_test.go
git commit -m "Pass 8.7: IconSet.Attachment field"
```

---

## Task 3: UIConfig.DownloadDir + resolver

**Files:**
- Modify: `internal/config/ui.go`
- Modify: `internal/config/ui_test.go`

- [ ] **Step 1: Add field, raw key, default resolver**

In `internal/config/ui.go`:

```go
// in UIConfig:
// DownloadDir is where SaveAttachment writes files. Resolved at
// LoadUI time: explicit [ui] download_dir > $XDG_DOWNLOAD_DIR >
// $HOME/Downloads.
DownloadDir string

// in rawUI:
DownloadDir string `toml:"download_dir"`

// in DefaultUIConfig():
DownloadDir: defaultDownloadDir(),

// new helper at file bottom:
func defaultDownloadDir() string {
	if v := os.Getenv("XDG_DOWNLOAD_DIR"); v != "" {
		return v
	}
	if home := os.Getenv("HOME"); home != "" {
		return home + "/Downloads"
	}
	return ""
}
```

In `LoadUI`, after the icons block, before the folders loop:

```go
if raw.UI.DownloadDir != "" {
	out.DownloadDir = expandHome(raw.UI.DownloadDir)
}
```

Add `expandHome` helper:

```go
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			return home + p[1:]
		}
	}
	return p
}
```

Add the `"strings"` import.

- [ ] **Step 2: Test**

Append to `internal/config/ui_test.go`:

```go
func TestLoadUI_DownloadDir(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	t.Setenv("XDG_DOWNLOAD_DIR", "")
	cases := []struct {
		name string
		toml string
		want string
	}{
		{"default", `[ui]`, "/home/test/Downloads"},
		{"explicit absolute", `[ui]
download_dir = "/tmp/dl"`, "/tmp/dl"},
		{"tilde expansion", `[ui]
download_dir = "~/dl"`, "/home/test/dl"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeTempUIFile(t, c.toml)
			cfg, err := LoadUI(path)
			if err != nil {
				t.Fatalf("LoadUI: %v", err)
			}
			if cfg.DownloadDir != c.want {
				t.Errorf("DownloadDir = %q, want %q", cfg.DownloadDir, c.want)
			}
		})
	}
}

func TestLoadUI_DownloadDir_XDG(t *testing.T) {
	t.Setenv("XDG_DOWNLOAD_DIR", "/var/dl")
	t.Setenv("HOME", "/home/test")
	path := writeTempUIFile(t, `[ui]`)
	cfg, err := LoadUI(path)
	if err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
	if cfg.DownloadDir != "/var/dl" {
		t.Errorf("DownloadDir = %q, want /var/dl", cfg.DownloadDir)
	}
}
```

If `writeTempUIFile` doesn't exist, inline a `t.TempDir()` +
`os.WriteFile` block.

- [ ] **Step 3: Run**

```
go test ./internal/config/ -run TestLoadUI -v
```

Expected: PASS. Existing tests must keep passing — they don't
specify `DownloadDir`, so they hit the default; they may need
`t.Setenv("HOME", "...")` to keep their goldens stable.

If a default-resolution change breaks an existing case-table
entry, update its `want.DownloadDir` to whatever `defaultDownloadDir()`
returns under the test env.

- [ ] **Step 4: Commit**

```
git add internal/config/ui.go internal/config/ui_test.go
git commit -m "Pass 8.7: [ui] download_dir resolution"
```

---

## Task 4: Msg + Cmd plumbing in cmds.go

**Files:**
- Modify: `internal/ui/cmds.go`

- [ ] **Step 1: Add Msg types**

Append to `internal/ui/cmds.go` near the existing
`OpenLinkPickerMsg` block:

```go
// attachmentsLoadedMsg carries metadata fetched via cache for the
// viewer's current UID. Stale UIDs are dropped at the AccountTab
// boundary like bodyLoadedMsg.
type attachmentsLoadedMsg struct {
	uid   mail.UID
	items []mail.Attachment
}

// OpenAttachPickerMsg requests App open the attachment picker.
// Emitted by Viewer when the user presses @ on a message that has
// at least one attachment.
type OpenAttachPickerMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

// AttachPickerClosedMsg signals the picker has closed.
type AttachPickerClosedMsg struct{}

// OpenAttachmentMsg requests App fire openAttachmentCmd for att on uid.
type OpenAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

// SaveAttachmentMsg requests App fire saveAttachmentCmd for att on uid.
type SaveAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

// attachmentSavedMsg reports a successful save. Carries the resolved
// path so App can populate the toast.
type attachmentSavedMsg struct {
	path string
}
```

- [ ] **Step 2: Add metadata fetch Cmd**

```go
// loadAttachmentsCmd resolves attachment metadata via the cache.
// Errors route through the standard ErrorMsg banner. Stale-UID
// drops happen at the AccountTab boundary.
func loadAttachmentsCmd(c *cache.Account, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		items, err := c.Attachments(context.Background(), uid)
		if err != nil {
			return ErrorMsg{Op: "fetch attachments", Err: err}
		}
		return attachmentsLoadedMsg{uid: uid, items: items}
	}
}
```

- [ ] **Step 3: Run vet**

```
go vet ./internal/ui/...
```

Expected: clean. (No tests added yet — Cmd is exercised in Task 5.)

- [ ] **Step 4: Commit**

```
git add internal/ui/cmds.go
git commit -m "Pass 8.7: attachment Msg + metadata fetch Cmd"
```

---

## Task 5: Save / open Cmds + filename helpers

**Files:**
- Modify: `internal/ui/cmds.go`
- Modify: `internal/ui/cmds_test.go`

- [ ] **Step 1: Add filename helpers**

Append to `internal/ui/cmds.go`:

```go
// sanitizeAttachFilename strips path separators and falls back to a
// stable name keyed on partID when the attachment has no filename.
func sanitizeAttachFilename(name, partID string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment-" + partID
	}
	return name
}

// resolveSaveTarget returns the first non-existing path in dir
// derived from base, suffixing -1, -2, ... before the extension.
// Caps at 999 to avoid pathological loops.
func resolveSaveTarget(dir, base string) (string, error) {
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
		return candidate, nil
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i <= 999; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("collision suffix exhausted for %q", base)
}
```

Add imports if missing: `"errors"`, `"fmt"`, `"io/fs"`,
`"os"`, `"path/filepath"`, `"strings"`.

- [ ] **Step 2: Add the Cmds**

```go
// openAttachmentCmd writes att's bytes to a tempfile and shells out
// to xdg-open. Fire-and-forget. Errors surface via ErrorMsg.
func openAttachmentCmd(c *cache.Account, opener URLOpener, uid mail.UID, att mail.Attachment) tea.Cmd {
	return func() tea.Msg {
		body, err := c.FetchAttachment(context.Background(), uid, att.PartID)
		if err != nil {
			return ErrorMsg{Op: "open attachment", Err: err}
		}
		name := sanitizeAttachFilename(att.Filename, att.PartID)
		path := filepath.Join(os.TempDir(), fmt.Sprintf("poplar-%s-%s", uid, name))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return ErrorMsg{Op: "open attachment", Err: err}
		}
		_ = opener(path)
		return nil
	}
}

// saveAttachmentCmd writes att's bytes to dir with collision-suffix
// resolution and emits attachmentSavedMsg with the final path.
func saveAttachmentCmd(c *cache.Account, dir string, uid mail.UID, att mail.Attachment) tea.Cmd {
	return func() tea.Msg {
		if dir == "" {
			return ErrorMsg{Op: "save attachment", Err: fmt.Errorf("no download dir configured")}
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		body, err := c.FetchAttachment(context.Background(), uid, att.PartID)
		if err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		name := sanitizeAttachFilename(att.Filename, att.PartID)
		target, err := resolveSaveTarget(dir, name)
		if err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		return attachmentSavedMsg{path: target}
	}
}
```

The `URLOpener` type is reused as the open shell — it just takes a
string and returns error. App passes the same opener used for URLs.

- [ ] **Step 3: Tests**

Append to `internal/ui/cmds_test.go`:

```go
func TestSanitizeAttachFilename(t *testing.T) {
	cases := []struct {
		name, partID, want string
	}{
		{"report.pdf", "2", "report.pdf"},
		{"", "2.1", "attachment-2.1"},
		{"a/b/c.txt", "1", "a_b_c.txt"},
		{"  spaced.bin  ", "3", "spaced.bin"},
	}
	for _, c := range cases {
		if got := sanitizeAttachFilename(c.name, c.partID); got != c.want {
			t.Errorf("sanitize(%q, %q) = %q, want %q", c.name, c.partID, got, c.want)
		}
	}
}

func TestResolveSaveTarget_Collision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-1.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := resolveSaveTarget(dir, "a.pdf")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := filepath.Join(dir, "a-2.pdf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSaveTarget_Fresh(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveSaveTarget(dir, "fresh.bin")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != filepath.Join(dir, "fresh.bin") {
		t.Errorf("got %q", got)
	}
}
```

For an end-to-end test of `saveAttachmentCmd` + `openAttachmentCmd`,
use the existing `blockingBackend` / cache fixtures in
`cmds_test.go` and `cache_helpers_test.go`. If those don't already
populate attachments, extend the helper to seed a single attachment
with bytes:

```go
func TestSaveAttachmentCmd(t *testing.T) {
	dir := t.TempDir()
	acct := newTestAccount(t) // existing helper; if not, build inline with cache.OpenInMemory
	// Seed a message + attachment via direct cache writes or a
	// custom backend — match the pattern from attachments_test.go.
	att := mail.Attachment{PartID: "2", Filename: "report.pdf", Size: 5}
	cmd := saveAttachmentCmd(acct, dir, mail.UID("u1"), att)
	msg := cmd()
	saved, ok := msg.(attachmentSavedMsg)
	if !ok {
		t.Fatalf("got %T, want attachmentSavedMsg (%v)", msg, msg)
	}
	if filepath.Dir(saved.path) != dir {
		t.Errorf("saved outside dir: %q", saved.path)
	}
	if _, err := os.Stat(saved.path); err != nil {
		t.Fatalf("file missing: %v", err)
	}
}
```

Skip this end-to-end test if the existing test helpers don't make
seeding ergonomic; the unit tests above plus the live tmux
verification in Task 11 are sufficient floor coverage.

- [ ] **Step 4: Run**

```
go test ./internal/ui/ -run "Sanitize|ResolveSave|SaveAttachment" -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/ui/cmds.go internal/ui/cmds_test.go
git commit -m "Pass 8.7: openAttachmentCmd + saveAttachmentCmd"
```

---

## Task 6: Viewer chip row + @ key

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/viewer.go`
- Modify: `internal/ui/viewer_test.go`

- [ ] **Step 1: Add the key**

In `internal/ui/keys.go`, extend `ViewerKeys`:

```go
type ViewerKeys struct {
	Close             key.Binding
	OpenPicker        key.Binding
	OpenAttachPicker  key.Binding
	BodyTop           key.Binding
	BodyBottom        key.Binding
	Links             [9]key.Binding
}
```

In `NewViewerKeys()`:

```go
OpenAttachPicker: key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "attachments")),
```

- [ ] **Step 2: Extend Viewer state**

In `internal/ui/viewer.go`, add to the `Viewer` struct (group with
`blocks`/`links`):

```go
attachments []mail.Attachment
attachReady bool
chipRow     string
chipHeight  int
icons       IconSet
```

Add `icons` to `NewViewer` parameters and propagate from
`AccountTab.NewAccountTab` and `App.NewApp`:

```go
func NewViewer(styles Styles, t *theme.CompiledTheme, accountEmail string, icons IconSet) Viewer {
	return Viewer{
		styles:       styles,
		theme:        t,
		accountEmail: accountEmail,
		icons:        icons,
		spinner:      NewSpinner(t),
		keys:         NewViewerKeys(),
	}
}
```

Update the call site in `account_tab.go`'s constructor.

- [ ] **Step 3: Reset chip state on Open**

```go
func (v Viewer) Open(msg mail.MessageInfo) Viewer {
	v.open = true
	v.phase = viewerLoading
	v.msg = msg
	v.blocks = nil
	v.links = nil
	v.panel = ""
	v.attachments = nil
	v.attachReady = false
	v.chipRow = ""
	v.chipHeight = 0
	return v
}
```

- [ ] **Step 4: SetAttachments accessor**

```go
// SetAttachments installs the attachment metadata list. Idempotent
// for stale UIDs — caller drops stale messages before invoking.
func (v Viewer) SetAttachments(items []mail.Attachment) Viewer {
	v.attachments = items
	v.attachReady = true
	if v.phase == viewerReady && v.open {
		v.layout()
	}
	return v
}
```

Also add an accessor for App to read the current attachment list
when `@` is pressed:

```go
// Attachments returns the harvested attachment metadata. Exposed
// for the App-owned attachment picker dispatch.
func (v Viewer) Attachments() []mail.Attachment {
	return v.attachments
}
```

- [ ] **Step 5: Chip rendering**

Add to `viewer.go`:

```go
// renderChipRow returns the wrapped chip block plus its row count,
// rendered at width. Returns ("", 0) when there are no attachments.
func (v Viewer) renderChipRow(width int) (string, int) {
	if len(v.attachments) == 0 || width < 1 {
		return "", 0
	}
	icon := v.icons.Attachment
	bg := v.styles.ViewerBg
	chips := make([]string, len(v.attachments))
	for i, a := range v.attachments {
		name := a.Filename
		if name == "" {
			name = "attachment"
		}
		chips[i] = fmt.Sprintf("%s %d. %s (%s)",
			icon, i+1, name, humanizeBytes(a.Size))
	}
	// Greedy line-fill. Each chip is rendered as-is; if a single
	// chip exceeds width, truncate it mid-name with `…`.
	var lines []string
	var cur string
	for _, c := range chips {
		if displayCells(c) > width {
			c = displayTruncate(c, width)
		}
		if cur == "" {
			cur = c
			continue
		}
		if displayCells(cur)+2+displayCells(c) > width {
			lines = append(lines, cur)
			cur = c
			continue
		}
		cur = cur + "  " + c
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for i, l := range lines {
		lines[i] = fillRowToWidth(l, width, bg)
	}
	return strings.Join(lines, "\n"), len(lines)
}
```

- [ ] **Step 6: Integrate into layout**

Update `Viewer.layout`:

```go
func (v *Viewer) layout() {
	hdrs := content.ParsedHeaders{
		// ... unchanged ...
	}
	contentWidth := max(1, v.width-1)
	headerStr := content.RenderHeaders(hdrs, v.theme, contentWidth)
	v.panel = v.styles.ViewerHeader.Width(v.width).Render(headerStr)

	if v.attachReady {
		row, h := v.renderChipRow(v.width)
		v.chipRow = row
		v.chipHeight = h
	} else {
		v.chipRow = ""
		v.chipHeight = 0
	}

	body, urls := content.RenderBodyWithFootnotes(v.blocks, v.theme, contentWidth)
	v.links = urls
	bodyHeight := max(1, v.height-lipgloss.Height(v.panel)-v.chipHeight)
	vp := viewport.New(contentWidth, bodyHeight)
	// keymap unchanged ...
	vp.SetContent(body)
	v.viewport = vp
}
```

Update `View()` to insert the chip row between panel and body:

```go
parts := []string{v.panel}
if v.chipRow != "" {
	parts = append(parts, v.chipRow)
}
parts = append(parts, strings.Join(bodyLines, "\n"))
return lipgloss.JoinVertical(lipgloss.Left, parts...)
```

Adjust the `bodyHeight` math in `View()` to subtract `v.chipHeight`
as well:

```go
bodyHeight := max(0, v.height-lipgloss.Height(v.panel)-v.chipHeight)
```

- [ ] **Step 7: Hook the `@` key**

In `Viewer.handleKey`, before the `OpenPicker` case:

```go
case key.Matches(msg, v.keys.OpenAttachPicker):
	if len(v.attachments) == 0 {
		return v, nil
	}
	uid := v.msg.UID
	items := append([]mail.Attachment(nil), v.attachments...)
	return v, func() tea.Msg { return OpenAttachPickerMsg{UID: uid, Items: items} }
```

- [ ] **Step 8: Tests**

Append to `internal/ui/viewer_test.go`:

```go
func TestViewer_ChipRow_Hidden_WhenEmpty(t *testing.T) {
	v := newTestViewer(t) // existing helper or inline
	v = v.SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "u1", Subject: "x"})
	v = v.SetBody(nil)
	v = v.SetAttachments(nil)
	out := v.View()
	// Chip row absent → height should be the body height with no
	// extra row consumed.
	if strings.Contains(out, "📎") || strings.Contains(out, "§") {
		t.Errorf("chip glyph present despite no attachments")
	}
}

func TestViewer_ChipRow_Visible(t *testing.T) {
	v := newTestViewer(t)
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1", Subject: "x"})
	v = v.SetBody(nil)
	v = v.SetAttachments([]mail.Attachment{
		{PartID: "2", Filename: "report.pdf", Size: 2400},
	})
	out := v.View()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("chip row missing filename: %s", out)
	}
	if !strings.Contains(out, "2.3 KB") {
		t.Errorf("chip row missing size: %s", out)
	}
}

func TestViewer_AtKey_Inert_WhenEmpty(t *testing.T) {
	v := newTestViewer(t)
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1"})
	v = v.SetBody(nil)
	v = v.SetAttachments(nil)
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when no attachments; got one")
	}
}

func TestViewer_AtKey_OpensPicker(t *testing.T) {
	v := newTestViewer(t)
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1"})
	v = v.SetBody(nil)
	v = v.SetAttachments([]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 1}})
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	if cmd == nil {
		t.Fatal("expected OpenAttachPickerMsg Cmd")
	}
	msg := cmd()
	open, ok := msg.(OpenAttachPickerMsg)
	if !ok {
		t.Fatalf("got %T, want OpenAttachPickerMsg", msg)
	}
	if len(open.Items) != 1 || open.UID != "u1" {
		t.Errorf("unexpected payload: %+v", open)
	}
}
```

If `newTestViewer` doesn't exist, inline:

```go
func newTestViewer(t *testing.T) Viewer {
	t.Helper()
	th := theme.MustCompile(theme.OneDark())
	return NewViewer(NewStyles(th), th, "me@example.com", SimpleIcons)
}
```

- [ ] **Step 9: Run**

```
go test ./internal/ui/ -run TestViewer -v
```

Expected: PASS. Existing viewer goldens may shift one or zero rows;
update goldens with `-update` if your test harness supports it, or
adjust the golden files inline.

- [ ] **Step 10: Commit**

```
git add internal/ui/keys.go internal/ui/viewer.go internal/ui/viewer_test.go
git commit -m "Pass 8.7: viewer chip row + @ key"
```

---

## Task 7: AttachPicker overlay

**Files:**
- Create: `internal/ui/attachpicker.go`
- Create: `internal/ui/attachpicker_test.go`

- [ ] **Step 1: Implementation**

Mirror `LinkPicker` structure. The picker emits one of three Msg
types per dispatched action: `AttachPickerClosedMsg` on Esc/q/`@`,
`OpenAttachmentMsg` on Enter/`o`/digit, `SaveAttachmentMsg` on `s`.
Action keys also emit `AttachPickerClosedMsg` so the picker shuts
after dispatch.

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
)

// AttachPicker is the modal overlay launched by `@` in the viewer.
// Single-column list of attachment metadata; cursor + Enter (open),
// `o` (open), `s` (save), 1-9 (open Nth), Esc/q/@ close.
type AttachPicker struct {
	shell  ModalShell
	uid    mail.UID
	items  []mail.Attachment
	cursor int
	offset int
	styles Styles
	icons  IconSet
	keys   attachPickerKeys
}

type attachPickerKeys struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Open   key.Binding
	Save   key.Binding
	Close  key.Binding
	Digits [9]key.Binding
}

func NewAttachPicker(styles Styles, icons IconSet) AttachPicker {
	keys := attachPickerKeys{
		Up:    key.NewBinding(key.WithKeys("k", "up")),
		Down:  key.NewBinding(key.WithKeys("j", "down")),
		Enter: key.NewBinding(key.WithKeys("enter")),
		Open:  key.NewBinding(key.WithKeys("o")),
		Save:  key.NewBinding(key.WithKeys("s")),
		Close: key.NewBinding(key.WithKeys("esc", "q", "@")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}
	return AttachPicker{styles: styles, icons: icons, keys: keys}
}

func (p AttachPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p AttachPicker) Cursor() int  { return p.cursor }

func (p AttachPicker) Open(uid mail.UID, items []mail.Attachment) AttachPicker {
	p.shell = p.shell.WithOpen(true)
	p.uid = uid
	p.items = items
	p.cursor = 0
	p.offset = 0
	return p
}

func (p AttachPicker) Close() AttachPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p AttachPicker) SetSize(width, height int) AttachPicker {
	p.shell = p.shell.SetSize(width, height)
	return p
}

func (p AttachPicker) Update(msg tea.Msg) (AttachPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Enter), key.Matches(keyMsg, p.keys.Open):
		return p, p.openCursor()
	case key.Matches(keyMsg, p.keys.Save):
		return p, p.saveCursor()
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return AttachPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.items) {
				return p, p.openIndex(i)
			}
			return p, nil
		}
	}
	return p, nil
}

func (p AttachPicker) openCursor() tea.Cmd {
	if p.cursor < 0 || p.cursor >= len(p.items) {
		return nil
	}
	return p.openIndex(p.cursor)
}

func (p AttachPicker) openIndex(i int) tea.Cmd {
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return OpenAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

func (p AttachPicker) saveCursor() tea.Cmd {
	if p.cursor < 0 || p.cursor >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[p.cursor]
	return tea.Batch(
		func() tea.Msg { return SaveAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

const attachPickerMaxWidth = 70

func (p AttachPicker) clampOffset() AttachPicker {
	p.offset = clampScrollOffset(p.cursor, visibleLinkRows(len(p.items), p.shell.Height()), p.offset)
	return p
}

func (p AttachPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p AttachPicker) Box(w, h int) string {
	boxW := attachPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 24 {
		boxW = 24
	}
	contentW := boxW - 2
	maxIndexDigits := len(strconv.Itoa(max(1, len(p.items))))
	visibleRows := visibleLinkRows(len(p.items), h)

	bodyRows := make([]string, visibleRows)
	for i := 0; i < visibleRows; i++ {
		row := p.offset + i
		if row >= len(p.items) {
			bodyRows[i] = padOrTruncate("", contentW)
			continue
		}
		bodyRows[i] = p.formatRow(row, maxIndexDigits, contentW)
	}

	footer := padOrTruncate("Enter/o open  s save  Esc close", contentW)
	footerRows := []string{footer}

	return p.shell.Box("Attachments", bodyRows, footerRows, contentW)
}

func (p AttachPicker) formatRow(row, maxIndexDigits, contentW int) string {
	att := p.items[row]
	idxStr := strconv.Itoa(row + 1)
	idxPad := strings.Repeat(" ", maxIndexDigits-len(idxStr))
	name := att.Filename
	if name == "" {
		name = "attachment"
	}
	size := humanizeBytes(att.Size)
	body := padOrTruncate(fmt.Sprintf("%s%s[%d] %s (%s)",
		idxPad, p.icons.Attachment, row+1, name, size), contentW)
	if row == p.cursor {
		return p.styles.MsgListCursor.Render(body)
	}
	return body
}

func (p AttachPicker) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
```

- [ ] **Step 2: Tests**

```go
// internal/ui/attachpicker_test.go
package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func newTestAttachPicker(t *testing.T) AttachPicker {
	t.Helper()
	th := theme.MustCompile(theme.OneDark())
	return NewAttachPicker(NewStyles(th), SimpleIcons).SetSize(120, 40)
}

func TestAttachPicker_OpenClose(t *testing.T) {
	p := newTestAttachPicker(t)
	if p.IsOpen() {
		t.Fatal("new picker should be closed")
	}
	p = p.Open("u1", []mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	if !p.IsOpen() {
		t.Fatal("Open should set open")
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should emit close cmd")
	}
	if _, ok := cmd().(AttachPickerClosedMsg); !ok {
		t.Errorf("got %T, want AttachPickerClosedMsg", cmd())
	}
}

func TestAttachPicker_OpenAction(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a Cmd")
	}
	// Batch: OpenAttachmentMsg + AttachPickerClosedMsg
	batch := cmd()
	if batch == nil {
		t.Fatal("batch nil")
	}
}

func TestAttachPicker_SaveAction(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("s should emit a Cmd")
	}
}

func TestAttachPicker_DigitOpensIndex(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1", []mail.Attachment{
		{PartID: "1", Filename: "a", Size: 1},
		{PartID: "2", Filename: "b", Size: 2},
	})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if cmd == nil {
		t.Fatal("digit should emit a Cmd")
	}
}

func TestAttachPicker_RenderContainsFilename(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "report.pdf", Size: 2400}})
	out := p.View()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("View missing filename: %s", out)
	}
	if !strings.Contains(out, "2.3 KB") {
		t.Errorf("View missing size: %s", out)
	}
}
```

- [ ] **Step 3: Run**

```
go test ./internal/ui/ -run TestAttachPicker -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```
git add internal/ui/attachpicker.go internal/ui/attachpicker_test.go
git commit -m "Pass 8.7: AttachPicker overlay"
```

---

## Task 8: AccountTab — batch metadata fetch + route load Msg

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Batch the Cmd in openMessage**

In `account_tab.go`, update `openMessage`:

```go
func (m AccountTab) openMessage(msg mail.MessageInfo) (AccountTab, tea.Cmd) {
	m = m.cancelInflightBodyFetch()
	ctx, cancel := context.WithCancel(context.Background())
	m.bodyFetchCancel = cancel
	m.viewer = m.viewer.Open(msg)
	cmds := []tea.Cmd{
		loadBodyCmd(ctx, m.acct, msg.UID),
		loadAttachmentsCmd(m.acct, msg.UID),
		m.viewer.SpinnerTick(),
	}
	if msg.Flags&mail.FlagSeen == 0 {
		cmds = append(cmds, markReadCmd(m.acct, m.currentFolderName(), msg.UID))
	}
	return m, tea.Batch(cmds...)
}
```

- [ ] **Step 2: Route the load Msg**

Add a case alongside `bodyLoadedMsg` in `AccountTab.Update`:

```go
case attachmentsLoadedMsg:
	if m.viewer.CurrentUID() == msg.uid {
		m.viewer = m.viewer.SetAttachments(msg.items)
	}
	return m, nil
```

Update `NewAccountTab` to pass `icons` into `NewViewer`. Inspect
the existing constructor and thread the field through (it already
has `icons IconSet` from Pass 8.4 cutover; just propagate to the
viewer).

- [ ] **Step 3: Test**

In `account_tab_test.go`, locate the existing `openMessage` /
`Enter` test path (search for `openMessage` or `KeyEnter`). Either
extend it to assert that an `attachmentsLoadedMsg` for the wrong
UID is dropped, or add:

```go
func TestAccountTab_AttachmentsLoadedMsg_StaleUIDDropped(t *testing.T) {
	tab := newTestAccountTab(t) // existing helper
	// Open viewer on uid "u1"
	tab, _ = tab.openMessage(mail.MessageInfo{UID: "u1"})
	// Send a message for "u2"
	tab2, _ := tab.Update(attachmentsLoadedMsg{
		uid:   "u2",
		items: []mail.Attachment{{PartID: "2", Filename: "stale.pdf"}},
	})
	if got := tab2.(AccountTab).viewer.Attachments(); len(got) != 0 {
		t.Errorf("stale attachment list applied: %v", got)
	}
	// Now send the right one
	tab3, _ := tab.Update(attachmentsLoadedMsg{
		uid:   "u1",
		items: []mail.Attachment{{PartID: "2", Filename: "real.pdf", Size: 1}},
	})
	if got := tab3.(AccountTab).viewer.Attachments(); len(got) != 1 {
		t.Errorf("correct UID's attachments not applied")
	}
}
```

If `tab.Update` returns `(tea.Model, tea.Cmd)` already, drop the
type assertion. If it returns `(AccountTab, tea.Cmd)` directly,
match that.

- [ ] **Step 4: Run**

```
go test ./internal/ui/ -run TestAccountTab -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "Pass 8.7: AccountTab batches attachment metadata fetch"
```

---

## Task 9: App wiring — overlay + Msg routing

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: NewApp signature + field**

Update `App` struct:

```go
type App struct {
	// ... existing ...
	attachPicker AttachPicker
	downloadDir  string
}
```

Update `NewApp`:

```go
func NewApp(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons IconSet) App {
	// ... existing ...
	return App{
		// ... existing ...
		attachPicker: NewAttachPicker(styles, icons),
		downloadDir:  uiCfg.DownloadDir,
	}
}
```

`uiCfg.DownloadDir` is already populated by `LoadUI` (Task 3).

- [ ] **Step 2: WindowSizeMsg propagation**

In `App.Update`'s `tea.WindowSizeMsg` case, add a `SetSize` +
`Update` block for `attachPicker` mirroring `linkPicker`:

```go
m.attachPicker = m.attachPicker.SetSize(m.width, m.height)
m.attachPicker, cmd = m.attachPicker.Update(msg)
cmds = append(cmds, cmd)
```

- [ ] **Step 3: Open/Close handlers**

Add cases in `App.Update`:

```go
case OpenAttachPickerMsg:
	m.attachPicker = m.attachPicker.Open(msg.UID, msg.Items)
	return m, nil

case AttachPickerClosedMsg:
	m.attachPicker = m.attachPicker.Close()
	return m, nil

case OpenAttachmentMsg:
	return m, openAttachmentCmd(m.acct.Cache(), m.opener, msg.UID, msg.Att)

case SaveAttachmentMsg:
	return m, saveAttachmentCmd(m.acct.Cache(), m.downloadDir, msg.UID, msg.Att)

case attachmentSavedMsg:
	// Surface the resolved path through the toast row. Reuse
	// pendingAction with op = opSavedAttachment, n = 0, dest = path.
	// (Pick whichever existing toast affordance fits — if no
	// matching op exists, add `opSavedAttachment` to the triageOp
	// enum and gate the toast formatter on it.)
	m.toast = pendingAction{
		op:       opSavedAttachment,
		dest:     msg.path,
		deadline: m.now().Add(time.Duration(m.undoSeconds) * time.Second),
	}
	return m, tea.Tick(time.Duration(m.undoSeconds)*time.Second, func(t time.Time) tea.Msg {
		return toastExpireMsg{deadline: m.toast.deadline}
	})
```

Find the `triageOp` enum (likely in `account_tab.go` or a small
adjacent file). Add `opSavedAttachment`. Update the toast
renderer to format it as `Saved to <dest>` and to suppress the
`[u undo]` hint (mirrors `opEmpty`'s suppression).

- [ ] **Step 4: Overlay cascade**

Find the keypress short-circuit block (the area around
`m.linkPicker.IsOpen()` in `App.Update` — search for that
exact identifier). Add an adjacent branch:

```go
if m.attachPicker.IsOpen() {
	var cmd tea.Cmd
	m.attachPicker, cmd = m.attachPicker.Update(msg)
	return m, cmd
}
```

Place it directly after the `linkPicker` branch (priority slot
per spec: confirm > conflict > outbox > help > link > attach >
move). Mirror the rendering composition near `m.linkPicker.Box`
(typically around `App.View`'s overlay rendering):

```go
if m.attachPicker.IsOpen() {
	box := m.attachPicker.Box(m.width, m.height)
	x, y := m.attachPicker.Position(box, m.width, m.height)
	frame = PlaceOverlay(x, y, box, frame)
}
```

- [ ] **Step 5: Tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_OpenAttachPickerMsg_OpensOverlay(t *testing.T) {
	app := newTestApp(t)
	app2, _ := app.Update(OpenAttachPickerMsg{
		UID:   "u1",
		Items: []mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 1}},
	})
	if !app2.(App).attachPicker.IsOpen() {
		t.Error("OpenAttachPickerMsg did not open the picker")
	}
}

func TestApp_SaveAttachmentMsg_DispatchesCmd(t *testing.T) {
	app := newTestApp(t)
	_, cmd := app.Update(SaveAttachmentMsg{
		UID: "u1",
		Att: mail.Attachment{PartID: "2", Filename: "x.pdf", Size: 1},
	})
	if cmd == nil {
		t.Error("SaveAttachmentMsg should dispatch a Cmd")
	}
}
```

If `App.Update`'s signature returns `(App, tea.Cmd)` rather than
`(tea.Model, tea.Cmd)`, drop the type assertion.

- [ ] **Step 6: Run**

```
go test ./internal/ui/ -run TestApp -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 8.7: App wires attachment picker + open/save"
```

---

## Task 10: cmd/poplar wiring + smoke

**Files:**
- Modify: `cmd/poplar/root.go`

- [ ] **Step 1: Verify NewApp call site**

`config.LoadUI` already produces a `UIConfig` with `DownloadDir`
populated; `ui.NewApp(... uiCfg ...)` already takes the config
struct, so no signature change is needed here. Confirm by
reading `cmd/poplar/root.go`'s `NewApp` invocation — if any
test or alternate constructor bypasses `LoadUI`, route them
through it or set `DownloadDir` explicitly.

If `cmd/poplar/cache.go` or any sibling spawns an `App` for a
subcommand without going through `LoadUI`, leave them unchanged
— they don't surface the viewer.

- [ ] **Step 2: Run the build**

```
make build
```

Expected: clean.

- [ ] **Step 3: make check**

```
make check
```

Expected: PASS. Address any compile errors or test failures
discovered now (most commonly: a goldenfile shifted by one row
because of a chip row in a fixture that has attachments — refresh
the golden and confirm visually).

- [ ] **Step 4: Commit (if anything changed)**

```
git add cmd/poplar/root.go
git commit -m "Pass 8.7: smoke build" --allow-empty
```

(Skip the commit if root.go didn't change. The empty-commit form
is shown only as an anchor; prefer not committing nothing.)

---

## Task 11: Live tmux verification + docs

**Files:**
- Modify: `docs/poplar/keybindings.md`
- Modify: `docs/poplar/wireframes.md`
- Modify: `.claude/rules/ui-invariants.md`

- [ ] **Step 1: Install + launch**

```
make install
poplar
```

Open a message that has attachments. Expected:

- Spinner phase covers body + metadata; chips appear with body.
- Chip row sits between header panel and body, no gap above.
- `@` opens the picker; `Esc` / `q` / `@` closes.
- `Enter` / `o` invokes `xdg-open` on a temp file (the OS
  decides what handler runs).
- `s` writes the file to `~/Downloads/` (or the configured
  path); the toast row reads `Saved to <path>` and disappears
  after the undo timer.
- A second save of the same file appears as `name-1.ext`.

Tmux capture at 120×40 and 80×24 (both with 0 / 1 / 5 chips).
Save captures under `internal/ui/testdata/captures/2026-05-04-attachments-ii/`
following the existing capture naming.

- [ ] **Step 2: keybindings.md**

In the **Viewer** section, add:

```markdown
| `@` | Attachment picker (when ≥1 attachment; inert otherwise) | V |
```

Add a new **Attachment picker** subsection:

```markdown
### Attachment picker

| Key | Action |
|-----|--------|
| `j` / `k` | Cursor down / up |
| `Enter`, `o` | Open via `xdg-open` (temp file) |
| `s` | Save to `[ui] download_dir` |
| `1`–`9` | Open Nth attachment |
| `Esc`, `q`, `@` | Close picker |
```

- [ ] **Step 3: wireframes.md**

Add an attachment-row mock for the viewer (chip row between
headers and body) and a picker frame mirroring the link picker
section. Use the captures from Step 1 as the source of truth.

- [ ] **Step 4: ui-invariants.md**

Under the **Viewer** bullet block, append:

```markdown
- Chip row sits between header panel and body. Hidden when
  `len(attachments) == 0`. Layout owned by `Viewer.layout`;
  body height = `v.height - panel - chipHeight`. Chip row
  populates from `attachmentsLoadedMsg` batched in the same
  Cmd as `bodyLoadedMsg` on viewer open.
```

Under the **Overlays** bullet block, edit the cascade order:

```markdown
Cascade order: confirm > conflict > outbox > help > link
picker > attach picker > move picker.
```

Add to the picker enumeration: "attachment picker (`@` from
viewer; `o`/`s`/`Enter`/digit/`Esc`)".

- [ ] **Step 5: make check (final gate)**

```
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```
git add docs/poplar/keybindings.md docs/poplar/wireframes.md \
        .claude/rules/ui-invariants.md \
        internal/ui/testdata/captures/2026-05-04-attachments-ii/
git commit -m "Pass 8.7: docs + tmux captures"
```

---

## Pass-end consolidation

Do NOT do this as part of the implementation — invoke the
`poplar-pass` skill at the end of execution. It runs `/simplify`,
the bubbletea conventions checklist, writes the three ADRs (chip
row placement, picker key + shape, download dir resolution),
updates `docs/poplar/invariants.md` with the new bindings (Viewer
state, Overlays cascade), updates `STATUS.md` (Pass 8.7 → done,
draft Pass 9 starter prompt), archives the plan + spec under
`docs/superpowers/archive/`, runs `make check`, commits, pushes,
installs.
