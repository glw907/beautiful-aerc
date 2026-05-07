# Address Book — Pass 9.1 (UI Mockups) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land all four address-book visual surfaces (autocomplete dropdown, `i`-popover, Contacts mode three-column shell, contact edit form) wired to in-memory stub fixtures, with no data layer, no sync, no persistence. Pass ends with 80×24 / 120×40 screenshots and an ADR locking visual decisions plus the 9.2 wiring contract.

**Architecture:** Single new `internal/ui/contacts/` package holding the `Contact` value type, fixture pool, the four bubbles-shaped sub-models (`Popover`, `Sidebar`, `List`, `Form`), and a shared `RenderDetailCard` pure function reused by `Popover` and `List`. The autocomplete dropdown lives in `internal/ui/compose/` (it's a compose surface). App owns mode switching (`C` / `M`), the `i`-popover overlay, and the form modal — same pattern as existing overlays in `app.go`. Per-package `Styles` follows the standard `NewStyles(*theme.CompiledTheme)` shape (ADR-0163). Fixtures are a Go literal slice (no embed, no JSON) — matches existing test fixtures.

**Tech Stack:** Go 1.26, bubbletea, bubbles (`textinput`, `textarea`, `viewport`), lipgloss, `internal/ui/uicore` (ModalShell, PlaceOverlay, DimANSI, DisplayCells), `internal/theme`. No third-party libraries land this pass — `emersion/go-vcard` and `nyaruka/phonenumbers` arrive in 9.2.

**Settled at plan time** (per spec §"Open questions"):
- One package `internal/ui/contacts/` holds Popover, Sidebar, List, Form, and the shared detail-card render. Following helppopover/movepicker's bubbles-shaped split would scatter five files across multiple packages with no real seam — they share the `Contact` type and the detail-card renderer, so they live together.
- Fixtures are a Go literal slice in `internal/ui/contacts/fixtures.go`. No `//go:embed`, no JSON. Toolchain stays uniform; tests get type-safety for free.

---

## File Structure

**Created:**
- `internal/ui/contacts/types.go` — `Contact`, `Email`, `Phone`, `Kind` enum, `Suggestion`.
- `internal/ui/contacts/fixtures.go` — `Fixtures()` returning ~30-row mixed Person/Business slice.
- `internal/ui/contacts/styles.go` — `Styles` + `NewStyles(*theme.CompiledTheme)`.
- `internal/ui/contacts/detail.go` — `RenderDetailCard(c Contact, s Styles, width int) string`.
- `internal/ui/contacts/detail_test.go` — golden-style render tests.
- `internal/ui/contacts/popover.go` — `Popover` model (wraps `RenderDetailCard` in ModalShell + no-match variant).
- `internal/ui/contacts/popover_test.go`.
- `internal/ui/contacts/sidebar.go` — T9 sidebar component.
- `internal/ui/contacts/sidebar_test.go`.
- `internal/ui/contacts/list.go` — middle-column scrollable contact list.
- `internal/ui/contacts/list_test.go`.
- `internal/ui/contacts/form.go` — Person/Business edit form modal.
- `internal/ui/contacts/form_test.go`.
- `internal/ui/contacts/msgs.go` — exported Msg types (`OpenPopoverMsg`, `ClosePopoverMsg`, `EnterContactsModeMsg`, `ExitContactsModeMsg`, `ContactSaveMsg`, `ContactCancelMsg`, `OpenFormMsg`).
- `internal/ui/compose/suggest.go` — `Dropdown` model + `SuggestFn` seam + key dispatch.
- `internal/ui/compose/suggest_test.go`.
- `docs/poplar/wireframes/contacts/` — directory; six 80×24 + six 120×40 PNG/text captures.
- `docs/poplar/decisions/0166-address-book-mockups.md` — ADR.

**Modified:**
- `internal/ui/app.go` — add `contactsMode bool`, `popover *contacts.Popover`, `form *contacts.Form` fields; wire `i` (mail mode), `C` / `M` (mode toggle), form open/close; add overlay branches in `Update` and `View`.
- `internal/ui/compose/model.go` — embed `Dropdown` per focused header field; route Up/Down/Tab/Enter/Esc into Dropdown when non-empty; on accept rewrite the textinput.
- `docs/poplar/wireframes.md` — add Contacts mode + popover + form + autocomplete sections, embed the new captures.
- `docs/poplar/keybindings.md` — add the rows from spec §Keybindings (mark `i`/`C`/`M`/`n`/`e`/Tab/Enter/Esc as wired this pass; `D` deferred to 9.3).
- `docs/poplar/STATUS.md` — flip Pass 9.1 to done; populate next starter prompt for Pass 9.2.
- `docs/poplar/invariants.md` — add a row to the decision index for ADR-0166; add a short Address Book section under Architecture summarizing the package shape and the stub-data status.

---

## Sub-Skill Invocations

Every Go-touching task: invoke `go-conventions` first.
Every `internal/ui/` task: also invoke `elm-conventions` first.
Before any color or style decision (Tasks 3, 4, 5, 6, 7, 8): read `docs/poplar/styling.md`.
Before any UI planning detail not pinned here: read `docs/poplar/bubbletea-conventions.md` and `docs/poplar/responsive-design.md`.
Pass-end (Task 10): invoke `poplar-pass`.

---

## Task 1: Stub types + fixture pool

**Files:**
- Create: `internal/ui/contacts/types.go`
- Create: `internal/ui/contacts/fixtures.go`

- [ ] **Step 1: Write `types.go`**

```go
// Package contacts provides poplar's address-book UI surfaces:
// the i-popover, Contacts mode, and the contact edit form. Pass 9.1
// renders these against in-memory fixtures; data wiring lands in 9.2.
package contacts

// Kind distinguishes a person card from an organization card. Person
// cards render first/last, org/title; org cards collapse to a single
// name line plus emails/phones/note.
type Kind int

const (
	KindPerson Kind = iota
	KindOrg
)

// Contact is the value rendered by every surface in this package.
// It mirrors the schema in docs/superpowers/specs/2026-05-06-addressbook-design.md
// minus the storage columns (id, source, account, external_id, rev,
// updated_at) — those land in 9.2 alongside the SQLite layer.
type Contact struct {
	Kind   Kind
	Name   string // FN; for KindOrg this is the entire visible identity
	Family string // empty for KindOrg
	Given  string // empty for KindOrg
	Org    string // empty for KindOrg
	Title  string // empty for KindOrg
	Note   string
	Emails []Email
	Phones []Phone
}

// Email pairs an address with an optional label. Position-as-primary:
// index 0 is the primary email; the form rewrites the slice to reorder.
type Email struct {
	Address string
	Label   string // "work", "home", or "" for unlabeled
}

// Phone pairs an E.164 number with an optional label.
type Phone struct {
	E164  string
	Label string // "mobile", "work", "home", "fax", or ""
}

// Suggestion is one row in the compose autocomplete dropdown. A
// contact with N emails appears N times — each suggestion is one
// (contact, email) pair with the org annotation flattened in.
type Suggestion struct {
	Name    string // person FN or org name
	Email   string
	Org     string // dim suffix; empty when Kind == KindOrg or Org unset
	IsOrg   bool
}
```

- [ ] **Step 2: Write `fixtures.go`**

Provide ~30 contacts covering: Person with one/two/three emails; Person with note; Person with no phones; Person with no org/title; Org rows; mixed-case names spanning every T9 group; one with a Unicode name (Méndez); two long-name rows that test 80-cell truncation. Names must include at least one entry per T9 group (`ABC`, `DEF`, `GHI`, `JKL`, `MNO`, `PQRS`, `TUV`, `WXYZ`) so the sidebar walks deterministically in tests.

```go
package contacts

// Fixtures returns the canonical Pass 9.1 mockup pool. Stable order;
// tests assert against indices.
func Fixtures() []Contact {
	return []Contact{
		{Kind: KindPerson, Name: "Alice Chen", Given: "Alice", Family: "Chen",
			Org: "ACME", Title: "Senior Engineer",
			Emails: []Email{{Address: "alice@example.com", Label: "work"}, {Address: "a.chen@personal.io", Label: "home"}},
			Phones: []Phone{{E164: "+15555550100", Label: "mobile"}, {E164: "+15555550199", Label: "work"}},
			Note:   "Met at GopherCon 2024.\nCares about error messages."},
		{Kind: KindPerson, Name: "Bob Iyer", Given: "Bob", Family: "Iyer",
			Emails: []Email{{Address: "bob@iyer.dev"}}},
		// ... ~28 more entries spanning every T9 group, both kinds,
		// and the edge cases listed in step 1.
		{Kind: KindOrg, Name: "ACME Support",
			Emails: []Email{{Address: "support@acme.com"}},
			Phones: []Phone{{E164: "+15555550199"}},
			Note:   "Vendor for the\nbuild-pipeline contract."},
	}
}

// FixtureSuggestions returns a deterministic 5–7-row slice for the
// autocomplete-dropdown stub SuggestFn. Order matches lexicographic
// rank by Name then Email.
func FixtureSuggestions(prefix string) []Suggestion {
	// Walk Fixtures(), expand each contact into one Suggestion per
	// email, lowercase-prefix-match against Name and Email, return
	// up to 7 rows in lexicographic order.
}
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: PASS — types and fixtures compile.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/contacts/types.go internal/ui/contacts/fixtures.go
git commit -m "Pass 9.1: contacts types + fixture pool"
```

---

## Task 2: Per-package Styles

**Files:**
- Create: `internal/ui/contacts/styles.go`

- [ ] **Step 1: Read styling map**

Read `docs/poplar/styling.md`. Pick palette slots for: card body text (`FgPrimary`), dim metadata in parentheses (`FgDim`), separator rule under name/title block (`FgDim`), label suffix in autocomplete row (`FgDim`), cursor row in list / sidebar (`AccentPrimary`), kind toggle selected (`AccentPrimary`), form field focus border (`AccentPrimary`), form input border (`FgDim`), validation error (`ColorWarning`).

- [ ] **Step 2: Write `styles.go`**

```go
package contacts

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is the narrow projection of theme.CompiledTheme that
// every contacts surface needs. Per ADR-0163, lipgloss.NewStyle is
// permitted only in this file within the contacts package.
type Styles struct {
	Name        lipgloss.Style
	TitleOrg    lipgloss.Style
	Body        lipgloss.Style
	Dim         lipgloss.Style
	Rule        lipgloss.Style
	CursorRow   lipgloss.Style
	GroupLabel  lipgloss.Style
	GroupCount  lipgloss.Style
	LetterTick  lipgloss.Style // the inline ┃ in the T9 micro-highlight
	Border      lipgloss.Style
	FieldFocus  lipgloss.Style
	FieldBlur   lipgloss.Style
	KindOn      lipgloss.Style
	KindOff     lipgloss.Style
	Warn        lipgloss.Style
}

// NewStyles compiles Styles against a CompiledTheme. Construct once
// per theme; pass into every sub-model's New(...).
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Name:       lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		TitleOrg:   lipgloss.NewStyle().Foreground(t.FgPrimary),
		Body:       lipgloss.NewStyle().Foreground(t.FgPrimary),
		Dim:        lipgloss.NewStyle().Foreground(t.FgDim),
		Rule:       lipgloss.NewStyle().Foreground(t.FgDim),
		CursorRow:  lipgloss.NewStyle().Foreground(t.AccentPrimary).Bold(true),
		GroupLabel: lipgloss.NewStyle().Foreground(t.FgPrimary),
		GroupCount: lipgloss.NewStyle().Foreground(t.FgDim),
		LetterTick: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Border:     lipgloss.NewStyle().Foreground(t.FgDim),
		FieldFocus: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		FieldBlur:  lipgloss.NewStyle().Foreground(t.FgDim),
		KindOn:     lipgloss.NewStyle().Foreground(t.AccentPrimary),
		KindOff:    lipgloss.NewStyle().Foreground(t.FgDim),
		Warn:       lipgloss.NewStyle().Foreground(t.ColorWarning),
	}
}
```

- [ ] **Step 2: Build + commit**

```bash
make build
git add internal/ui/contacts/styles.go
git commit -m "Pass 9.1: contacts Styles"
```

---

## Task 3: Detail card renderer (pure function)

**Files:**
- Create: `internal/ui/contacts/detail.go`
- Create: `internal/ui/contacts/detail_test.go`

The detail card is shared by `Popover` and `List`'s right column. It's a pure string-building function — no model, no Update, no Cmd. Render rules from spec §"Detail card".

- [ ] **Step 1: Write the failing test (golden-style)**

```go
package contacts

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestRenderDetailCard_Person(t *testing.T) {
	c := Contact{
		Kind: KindPerson, Name: "Alice Chen",
		Given: "Alice", Family: "Chen",
		Org: "ACME", Title: "Senior Engineer",
		Emails: []Email{
			{Address: "alice@example.com", Label: "work"},
			{Address: "a.chen@personal.io", Label: "home"},
		},
		Phones: []Phone{
			{E164: "+15555550100", Label: "mobile"},
			{E164: "+15555550199", Label: "work"},
		},
		Note: "Met at GopherCon 2024.\nCares about error messages.",
	}
	got := RenderDetailCard(c, NewStyles(theme.OneDark()), 40)
	for _, want := range []string{
		"Alice Chen",
		"Senior Engineer · ACME",
		"alice@example.com",
		"(work, primary)",
		"a.chen@personal.io",
		"(home)",
		"+1 555-0100",
		"(mobile, primary)",
		"Met at GopherCon 2024.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderDetailCard_Org(t *testing.T) {
	c := Contact{
		Kind:   KindOrg,
		Name:   "ACME Support",
		Emails: []Email{{Address: "support@acme.com"}},
		Phones: []Phone{{E164: "+15555550199"}},
		Note:   "Vendor for the\nbuild-pipeline contract.",
	}
	got := RenderDetailCard(c, NewStyles(theme.OneDark()), 40)
	// Org: no title/org line ever; emails render with primary marker on row 0.
	for _, want := range []string{"ACME Support", "support@acme.com", "(primary)", "Vendor for the"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, banned := range []string{"Senior", "·"} { // no title-org line for org kind
		if strings.Contains(got, banned) {
			t.Errorf("unexpected %q in:\n%s", banned, got)
		}
	}
}
```

- [ ] **Step 2: Run test — expect FAIL (function not defined)**

Run: `go test ./internal/ui/contacts/ -run TestRenderDetailCard -v`
Expected: FAIL — undefined `RenderDetailCard`.

- [ ] **Step 3: Implement `detail.go`**

```go
package contacts

import (
	"strings"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// RenderDetailCard renders a contact card. Used by the i-popover and
// by Contacts mode's right column. Pure: the same Contact, Styles,
// and width always render the same string.
//
// Layout (per spec §Detail card):
//   line 1: name (bold)
//   line 2: title · org (skipped on KindOrg, or when both blank)
//   blank
//   email rows: address  (label, primary) on row 0; (label) elsewhere
//   blank (only when phones present)
//   phone rows: e164  (label, primary) | (label)
//   ─── (only when note present and non-empty after trim)
//   note rows
//
// width is the cell width the caller has reserved for the card; long
// rows truncate via uicore.DisplayTruncateEllipsis.
func RenderDetailCard(c Contact, s Styles, width int) string {
	var b strings.Builder
	// implementation follows the layout above; pad each line to width
	// with uicore.PadOrTruncate; format phone via formatPhone helper
	// (E.164 → "+1 555-0100" pretty form for US, raw otherwise — keep
	// minimal in 9.1; 9.2 swaps in libphonenumber).
	return b.String()
}

func formatPhone(e164 string) string {
	// Minimal pretty-print for US +1XXXXXXXXXX. Anything else returns
	// e164 unchanged. 9.2 replaces this with phonenumbers.Format.
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./internal/ui/contacts/ -run TestRenderDetailCard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/contacts/detail.go internal/ui/contacts/detail_test.go
git commit -m "Pass 9.1: contacts detail-card renderer"
```

---

## Task 4: `i`-popover model + App wiring

**Files:**
- Create: `internal/ui/contacts/popover.go`
- Create: `internal/ui/contacts/popover_test.go`
- Create: `internal/ui/contacts/msgs.go`
- Modify: `internal/ui/app.go`

Popover wraps `RenderDetailCard` in `uicore.ModalShell`. Two render paths: a matched contact (full card) and the "no match" variant (display name + email + `n add contact   Esc dismiss` hint).

The popover's lookup is supplied by the App via a `LookupFn` seam — Pass 9.1 wires it to `func(string) (Contact, bool) { return Contact{}, false }` for some addresses and to `Fixtures()` for others (so demos cover both branches). The 9.2 wiring contract: same signature, real cache-backed lookup.

- [ ] **Step 1: Define cross-package Msgs in `msgs.go`**

```go
package contacts

import "github.com/glw907/poplar/internal/ui/uicore"

// OpenPopoverMsg asks App to open the i-popover for the given
// display name + email pair. App resolves Lookup at this point and
// stores the result on the popover model before opening.
type OpenPopoverMsg struct {
	DisplayName string
	Email       string
}

// ClosePopoverMsg dismisses the popover. Re-press of i, Esc, and
// successful save from the no-match form all emit it.
type ClosePopoverMsg struct{}

// EnterContactsModeMsg / ExitContactsModeMsg toggle Contacts mode.
type EnterContactsModeMsg struct{}
type ExitContactsModeMsg struct{}

// OpenFormMsg asks App to render the contact edit form. Initial is
// the contact being edited (zero-valued for `n` new). FromPopover
// flags whether to layer the form as a centered modal over dimmed
// mail chrome (true) or replace the right pane in Contacts mode
// (false).
type OpenFormMsg struct {
	Initial      Contact
	FromPopover  bool
}

// ContactSaveMsg fires from the form on Ctrl+S. Pass 9.1 routes it
// to App which logs and discards (no data layer); 9.2 wires it to
// addressbook.Upsert.
type ContactSaveMsg struct {
	Contact Contact
	SaveTo  string // "Local file" or account name
}

// ContactCancelMsg fires from the form on Esc. Carries Dirty so App
// can decide whether to gate cancel through ConfirmModal.
type ContactCancelMsg struct{ Dirty bool }

// Sentinel: the popover surfaces errors via uicore.ErrorMsg, never
// via package-private types.
var _ = uicore.ErrorMsg{}
```

- [ ] **Step 2: Write `popover.go`**

```go
package contacts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Popover is the i-popover sub-model. Owned by App while open.
type Popover struct {
	shell       uicore.ModalShell
	styles      Styles
	displayName string // From-header fallback when Match is zero
	email       string
	match       Contact
	hasMatch    bool
	width       int
	height      int
}

func NewPopover(s Styles) Popover { /* ... */ }

func (p *Popover) SetMatch(displayName, email string, match Contact, hasMatch bool) {
	p.displayName, p.email, p.match, p.hasMatch = displayName, email, match, hasMatch
}

func (p Popover) SetSize(w, h int) Popover { p.width, p.height = w, h; return p }

func (p Popover) Update(msg tea.Msg) (Popover, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyEsc), key.Matches(msg, keyI):
			return p, func() tea.Msg { return ClosePopoverMsg{} }
		case !p.hasMatch && key.Matches(msg, keyN):
			return p, func() tea.Msg {
				return OpenFormMsg{
					Initial:     Contact{Kind: KindPerson, Name: p.displayName, Emails: []Email{{Address: p.email}}},
					FromPopover: true,
				}
			}
		}
	}
	return p, nil
}

func (p Popover) View() string {
	body, footer := p.composeBody()
	return p.shell.Box("Sender", body, footer, p.contentWidth())
}

// composeBody returns the body rows + footer rows. Match path calls
// RenderDetailCard; no-match path renders the spec §Detail card "no
// contact" block.
```

- [ ] **Step 3: Write `popover_test.go`**

Test: matched render contains card content; no-match render contains `No contact in address book.` + `n add contact`; Esc returns `ClosePopoverMsg`; `i` returns `ClosePopoverMsg`; `n` on no-match returns `OpenFormMsg{FromPopover: true}` pre-filled.

- [ ] **Step 4: Wire into App**

Modify `internal/ui/app.go`:
- Add `popover *contacts.Popover` field.
- In `Update`, before AccountTab delegation:
  - Intercept `tea.KeyMsg` `i` on the account view (no overlays open) — emit `contacts.OpenPopoverMsg{}` constructed from the focused message's From header (cursor row from `m.acct.MessageList().CursorRow()` — add accessor if missing; pre-beta posture says fix it inline).
  - Handle `contacts.OpenPopoverMsg`: instantiate `Popover`, call its `LookupFn` (Pass 9.1: scan `Fixtures()` for case-insensitive email match, return first hit), call `SetMatch`, store on App.
  - Handle `contacts.ClosePopoverMsg`: clear `popover`.
  - Route keys into `popover.Update` when non-nil.
- In `View`, when `popover != nil`, dim the underlay via `uicore.DimANSI` and composite via `uicore.PlaceOverlay`.

- [ ] **Step 5: Build, run tests, live-verify**

```bash
make check
```

Then live-verify per `.claude/docs/tmux-testing.md`: launch poplar in 80×24, focus a message whose From email matches a fixture, press `i`, capture; press `i` again to dismiss. Then a non-matching From, press `i`, see no-match variant.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/contacts/popover.go internal/ui/contacts/popover_test.go internal/ui/contacts/msgs.go internal/ui/app.go
git commit -m "Pass 9.1: i-popover wired to fixture lookup"
```

---

## Task 5: T9 sidebar component

**Files:**
- Create: `internal/ui/contacts/sidebar.go`
- Create: `internal/ui/contacts/sidebar_test.go`

Eight T9 groups in fixed order: `ABC`, `DEF`, `GHI`, `JKL`, `MNO`, `PQRS`, `TUV`, `WXYZ`. One row per group with right-aligned count. Blank row between groups. Active group's matching letter renders with the inline `┃` indicator (per-letter micro-highlight from the WIP memory). `J/K` walks groups; `a`–`z` jumps to per-letter precision. Width sourced from the standard sidebar floor (14) — three-tier responsive doesn't apply here in 9.1; the sidebar takes its mail-mode width verbatim.

- [ ] **Step 1: Write the failing test**

```go
package contacts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func TestSidebar_GroupOrderAndCounts(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark()), Fixtures())
	s = s.SetSize(14, 24)
	v := s.View()
	want := []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"}
	for _, w := range want {
		if !strings.Contains(v, w) {
			t.Errorf("missing %q\n%s", w, v)
		}
	}
}

func TestSidebar_LetterMicroHighlight(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark()), Fixtures())
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if s.activeGroup != 0 {
		t.Errorf("expected ABC group active, got %d", s.activeGroup)
	}
	if s.activeLetter != 'B' {
		t.Errorf("expected letter B, got %c", s.activeLetter)
	}
}

func TestSidebar_WalkGroups(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark()), Fixtures())
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if s.activeGroup != 1 {
		t.Errorf("J should advance to DEF group")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./internal/ui/contacts/ -run TestSidebar -v`
Expected: FAIL.

- [ ] **Step 3: Implement `sidebar.go`**

```go
package contacts

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/glw907/poplar/internal/ui/uicore"
)

var t9Groups = []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"}

type Sidebar struct {
	styles       Styles
	contacts     []Contact
	groupCounts  [8]int
	activeGroup  int
	activeLetter rune // uppercase A–Z; 0 means "no letter selected"
	width, height int
}

func NewSidebar(s Styles, all []Contact) Sidebar {
	sb := Sidebar{styles: s, contacts: all}
	sb.recount()
	return sb
}

// SelectionLetter returns the active per-letter cursor or 0 when only
// the group is active. Consumed by List.SetSelectionLetter to keep
// the list scroll position synced with `a`–`z` jumps.
func (s Sidebar) SelectionLetter() rune { return s.activeLetter }
func (s Sidebar) SelectionGroup() int   { return s.activeGroup }

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(k, keyJUpper):
			if s.activeGroup < 7 { s.activeGroup++; s.activeLetter = 0 }
		case key.Matches(k, keyKUpper):
			if s.activeGroup > 0 { s.activeGroup--; s.activeLetter = 0 }
		default:
			if len(k.Runes) == 1 && k.Runes[0] >= 'a' && k.Runes[0] <= 'z' {
				ltr := k.Runes[0] - 'a' + 'A'
				s.activeLetter = ltr
				s.activeGroup = groupOfLetter(ltr)
			}
		}
	}
	return s, nil
}

func (s Sidebar) SetSize(w, h int) Sidebar { s.width, s.height = w, h; return s }

func (s Sidebar) View() string {
	// Render each of the 8 groups as one row, group's letters rendered
	// with `┃` glued before the active letter only on activeGroup
	// when activeLetter != 0. Right-align count via uicore.DisplayCells.
	// Blank row between groups.
}
```

Plus `groupOfLetter`, `recount`, and `keyJUpper` / `keyKUpper` declared in a shared `keys.go` for the package.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/ui/contacts/ -run TestSidebar -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/contacts/sidebar.go internal/ui/contacts/sidebar_test.go
git commit -m "Pass 9.1: contacts T9 sidebar"
```

---

## Task 6: Contacts list (middle column)

**Files:**
- Create: `internal/ui/contacts/list.go`
- Create: `internal/ui/contacts/list_test.go`

Scrollable contact list using `bubbles/viewport`. Row shape per spec:
```
Alice Chen          alice@example.com  +1 555-0100   Senior Engineer · ACME
```
Last-name sort variant: `Chen, Alice ...`. Sort mode comes from a config key (`[ui] contacts_sort`) — Pass 9.1 reads it from a constructor parameter; the config wiring lands in 9.2.

`j/k` cursor; `n` emits `OpenFormMsg{}` (new); `e` emits `OpenFormMsg{Initial: cursor}`; `D` is wired but inert in 9.1 (deferred to 9.3 per spec keybinding table).

`SetSelectionLetter(rune)` from the sidebar scrolls the cursor to the first row whose sort key starts with that letter.

- [ ] **Step 1: Write list_test.go**

Cover: cursor advances on `j`, retreats on `k`; `n` emits `OpenFormMsg` with zero Contact; `e` emits `OpenFormMsg` with cursor's contact; `SetSelectionLetter('M')` jumps cursor to first row whose sort key starts with M; first-name-sort vs last-name-sort row formatting differs as specified.

- [ ] **Step 2: Run tests — expect FAIL**

- [ ] **Step 3: Implement `list.go`**

```go
package contacts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
)

type SortMode int

const (
	SortFirstName SortMode = iota
	SortLastName
)

type List struct {
	styles   Styles
	all      []Contact
	sort     SortMode
	cursor   int
	vp       viewport.Model
	width, height int
}

func NewList(s Styles, all []Contact, sort SortMode) List { /* sort + viewport setup */ }
func (l List) Cursor() Contact { return l.all[l.cursor] }
func (l List) SetSelectionLetter(letter rune) List { /* find first row whose sort key starts with letter; clamp; sync vp */ }
func (l List) SetSize(w, h int) List { /* delegate to vp; reflow rows */ }
func (l List) Update(msg tea.Msg) (List, tea.Cmd) { /* j/k, n, e */ }
func (l List) View() string { /* row builder, cursor highlighted via styles.CursorRow */ }
```

- [ ] **Step 4: Run tests — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/ui/contacts/list.go internal/ui/contacts/list_test.go
git commit -m "Pass 9.1: contacts list (middle column)"
```

---

## Task 7: Contacts mode shell + App `C`/`M` toggle

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/contacts/msgs.go` (only if a new Msg is needed)

Contacts mode is a three-column composition: `Sidebar` | `List` | `RenderDetailCard(list.Cursor())`. App owns the mode flag and the three sub-models. Mode toggle: `C` from account view enters; `M` from Contacts mode exits. `q` in Contacts mode quits poplar (matches account-view behavior).

Sidebar header row reads `CONTACTS · All sources` for Pass 9.1 (no per-source filtering yet).

- [ ] **Step 1: Add fields to App**

```go
// in internal/ui/app.go
type App struct {
	// ... existing fields ...
	contactsMode    bool
	contactsSidebar contacts.Sidebar
	contactsList    contacts.List
}
```

Initialize both in `NewApp` against `contacts.Fixtures()` and a default `contacts.SortFirstName`.

- [ ] **Step 2: Wire `C` / `M` and key routing**

In `Update`:
- On `tea.KeyMsg` `C` from account view (no overlays): set `contactsMode = true`.
- On `tea.KeyMsg` `M` while `contactsMode`: set `contactsMode = false`.
- While `contactsMode`, route keys to `contactsSidebar.Update` and `contactsList.Update` in that order; after each, call `list.SetSelectionLetter(sidebar.SelectionLetter())` if it changed.
- Handle `OpenFormMsg`, `ContactSaveMsg`, `ContactCancelMsg` (Task 8 lands the form; for now, log and discard the save — pre-beta is fine with this).

- [ ] **Step 3: Wire `View()`**

When `contactsMode`, render three columns row-by-row (per ADR-0084 — no `JoinHorizontal` when SPUA cell width != 1). Reuse `uicore.PadOrTruncate` and the row-by-row `strings.Join` pattern from `account/model.go`'s `RenderWithRightPane`.

Top chrome row: `CONTACTS · All sources`; bottom: standard footer with the contacts-mode key hints (`j/k cursor · J/K group · a–z jump · n new · e edit · M mail · q quit`).

- [ ] **Step 4: Live-verify**

```bash
make install
poplar
```

Press `C` from account view; verify three columns render at 80×24 and 120×40; `j/k` moves cursor; right column updates; `a`–`z` jumps; `J/K` walks groups; `M` returns to mail mode.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/contacts/msgs.go
git commit -m "Pass 9.1: Contacts mode shell + C/M toggle"
```

---

## Task 8: Contact edit form

**Files:**
- Create: `internal/ui/contacts/form.go`
- Create: `internal/ui/contacts/form_test.go`
- Modify: `internal/ui/app.go` (form lifecycle + ConfirmModal gate)

The form is the most complex sub-model in this pass: kind toggle, repeating email/phone rows with label cyclers and position-as-primary widgets, multiline note, save-destination radio, full validation. Per spec §"Contact edit form".

Two render contexts:
- From Contacts mode: replaces the right pane (sidebar + list stay drawn but inert).
- From popover's "no contact" affordance: centered `ModalShell` over dimmed mail chrome.

App owns lifecycle: `form *contacts.Form` field. While non-nil, App routes keys to it (with right-pane underlay still drawn when `contactsMode` and `!FromPopover`, dimmed mail chrome with overlay when `FromPopover`).

Validation runs on `Ctrl+S`; failures highlight the offending field via `Styles.Warn` and block save. Successful save emits `ContactSaveMsg`. `Esc` emits `ContactCancelMsg{Dirty: m.dirty}`; App opens ConfirmModal when Dirty.

- [ ] **Step 1: Write form_test.go**

Cover: kind toggle flips form layout (Person hides single Name field; Business hides First/Last/Org/Title); add-email button appends row; ★ on row N>0 promotes to row 0; − removes row (disabled when only 1 email); validation blocks save with empty Name (Person: empty First and Last; Business: empty Name); validation blocks save with 0 emails; validation blocks save with malformed email (use `mail.ParseAddress`); save emits `ContactSaveMsg`; cancel-while-dirty emits `ContactCancelMsg{Dirty: true}`.

- [ ] **Step 2: Run tests — expect FAIL**

- [ ] **Step 3: Implement `form.go`**

The implementation is mechanical given the spec; key shape:

```go
package contacts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/textarea"
)

type Form struct {
	styles   Styles
	initial  Contact
	dirty    bool
	kind     Kind
	first    textinput.Model // KindPerson only
	last     textinput.Model // KindPerson only
	bizName  textinput.Model // KindOrg only
	org      textinput.Model
	title    textinput.Model
	emails   []emailRow
	phones   []phoneRow
	note     textarea.Model
	saveTo   []string // ["Local file", "Fastmail", ...]
	saveIdx  int
	focusIdx int
	err      string
	width, height int
	fromPopover bool
}

type emailRow struct {
	input   textinput.Model
	label   int // index into [Work Home Blank]
	primary bool
}

type phoneRow struct {
	input   textinput.Model
	label   int // index into [Mobile Work Home Fax Blank]
}

func NewForm(s Styles, initial Contact, fromPopover bool, saveDestinations []string) Form { /* ... */ }

func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	// Tab/Shift+Tab cycles focus across visible elements (kind toggle,
	// name fields per kind, emails (input + cycler + ★ + −), + add email,
	// phones, + add phone, note, save-to, save-button-implicit).
	// Space/←/→ on kind toggle flips kind.
	// On save-attempt (Ctrl+S): validate; if pass, return ContactSaveMsg
	// Cmd; if fail, set f.err and stay open.
	// Esc returns ContactCancelMsg{Dirty: f.dirty}.
}

func (f Form) View() string {
	// Build rows pre-padded to contentW; render via uicore.ModalShell
	// when fromPopover, plain panel otherwise.
}

func (f Form) Validate() error { /* per spec §Validation summary */ }
```

- [ ] **Step 4: Wire form lifecycle into App**

```go
// in internal/ui/app.go
type App struct {
	// ... existing ...
	form *contacts.Form
}
```

Handle `OpenFormMsg`: build `saveTo` slice (`["Local file"]` plus account names — Pass 9.1 hard-codes the single account name; multi-account lands in 9.2), instantiate `NewForm`, store on App.

While `form != nil`, route keys to it. Handle `ContactSaveMsg` (log + discard for 9.1; `f = nil`). Handle `ContactCancelMsg`: if `Dirty`, open `ConfirmModal` ("Discard changes?"); else clear form. ConfirmModal Yes path clears form; No keeps it open.

`View()`: when `form != nil && form.fromPopover`, dim underlay and overlay form via `uicore.PlaceOverlay`; when `form != nil && !fromPopover`, render Contacts mode with form replacing the right pane.

- [ ] **Step 5: Run all tests + live-verify**

```bash
make check
make install
```

In `poplar`: enter Contacts mode, press `n`, fill out form, press `Ctrl+S` — App logs save and form closes. Press `e` on a row, modify, `Esc`, ConfirmModal opens. Then from mail mode, `i`-popover with no match, press `n` — form opens centered with email pre-filled.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/contacts/form.go internal/ui/contacts/form_test.go internal/ui/app.go
git commit -m "Pass 9.1: contact edit form (Person + Business)"
```

---

## Task 9: Compose autocomplete dropdown

**Files:**
- Create: `internal/ui/compose/suggest.go`
- Create: `internal/ui/compose/suggest_test.go`
- Modify: `internal/ui/compose/model.go`

Inline dropdown anchored under the focused To/Cc/Bcc textinput. `SuggestFn(prefix string) []Suggestion` is the seam — Pass 9.1 wires it to `contacts.FixtureSuggestions`. 9.2 swaps in the cache-backed query.

Renders only when the focused field is To/Cc/Bcc, the prefix has ≥ 2 characters, and `SuggestFn` returns a non-empty slice. `Up`/`Down` cursor; `Tab`/`Enter` accepts (rewrites textinput to `Name <email>, ` and clears the dropdown); `Esc` dismisses.

The dropdown is positional — Compose's `View()` already row-pads each section; the dropdown's rows splice into the View's row stream below the focused-field row. No overlay positioning math needed; Compose just inserts the dropdown rows at the right index.

- [ ] **Step 1: Write suggest_test.go**

Cover: dropdown empty when prefix < 2 chars; dropdown populated when ≥ 2 chars and SuggestFn returns rows; Up/Down moves cursor with wrap; Enter emits `AcceptMsg{Suggestion}`; Esc emits `DismissMsg{}`; row format includes `Name <email> · org` for individuals and `Org Name <email>` (no suffix) for orgs.

- [ ] **Step 2: Run tests — expect FAIL**

- [ ] **Step 3: Implement `suggest.go`**

```go
package compose

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/glw907/poplar/internal/ui/contacts"
)

type SuggestFn func(prefix string) []contacts.Suggestion

type Dropdown struct {
	fn      SuggestFn
	rows    []contacts.Suggestion
	cursor  int
	width   int
	prefix  string
}

func NewDropdown(fn SuggestFn) Dropdown { return Dropdown{fn: fn} }

func (d Dropdown) Empty() bool { return len(d.rows) == 0 }

// SetPrefix re-runs SuggestFn unless the prefix is shorter than 2.
func (d Dropdown) SetPrefix(p string) Dropdown { /* clamp + refresh */ }

func (d Dropdown) Update(msg tea.Msg) (Dropdown, tea.Cmd) {
	// Up/Down cursor (wrap); Tab/Enter accepts; Esc dismisses.
	// Accept emits a SuggestAcceptMsg{Suggestion} that compose.Model
	// catches and rewrites the focused textinput.
}

func (d Dropdown) View() string { /* one row per Suggestion, cursor highlighted */ }
```

- [ ] **Step 4: Splice into compose/model.go**

In `compose.Model`:
- Add field: `suggest Dropdown`.
- On every focused-textinput value change in To/Cc/Bcc, call `m.suggest = m.suggest.SetPrefix(currentVal)`.
- Route Up/Down/Tab/Enter/Esc to `m.suggest.Update` *before* the textinput when `!m.suggest.Empty()`.
- Handle `SuggestAcceptMsg`: rewrite focused textinput to `<Name> <<Email>>, ` (RFC 5322 angle-addr) and clear the dropdown via `SetPrefix("")`.
- In `View()`, when `!m.suggest.Empty()` and the focused field is To/Cc/Bcc, splice `m.suggest.View()` rows immediately after the focused-field row.

`NewModel` accepts a `SuggestFn` — Pass 9.1 default in App is `func(p string) []contacts.Suggestion { return contacts.FixtureSuggestions(p) }`.

- [ ] **Step 5: Live-verify + commit**

```bash
make check && make install
```

Open compose, type `ali` in To: dropdown shows fixture suggestions; arrows navigate; Tab accepts and rewrites the field.

```bash
git add internal/ui/compose/suggest.go internal/ui/compose/suggest_test.go internal/ui/compose/model.go internal/ui/app.go
git commit -m "Pass 9.1: compose autocomplete dropdown wired to fixtures"
```

---

## Task 10: Wireframes, screenshots, and ADR

**Files:**
- Create: `docs/poplar/wireframes/contacts/` (twelve captures: 6 surfaces × 2 sizes)
- Modify: `docs/poplar/wireframes.md`
- Modify: `docs/poplar/keybindings.md`
- Create: `docs/poplar/decisions/0166-address-book-mockups.md`

Surfaces to capture:
1. `i`-popover with match
2. `i`-popover no-match
3. Contacts mode three-column shell (cursor on a row)
4. Contact edit form — Person variant
5. Contact edit form — Business variant
6. Compose with autocomplete dropdown open

Each captured at 80×24 and 120×40 per `.claude/docs/tmux-testing.md`.

- [ ] **Step 1: Capture screenshots**

Follow `.claude/docs/tmux-testing.md`. Save under `docs/poplar/wireframes/contacts/<surface>-80x24.txt` and `<surface>-120x40.txt`.

- [ ] **Step 2: Update wireframes.md**

Add a top-level `## Contacts` section with subsections per surface. Embed each capture as a fenced ASCII block. Cross-link the spec.

- [ ] **Step 3: Update keybindings.md**

Add a `## Contacts mode` section. List every key from spec §Keybindings. Mark wired-this-pass keys with the standard wired notation; mark `D` (delete) as deferred to 9.3.

- [ ] **Step 4: Write ADR-0166**

Path: `docs/poplar/decisions/0166-address-book-mockups.md`. Sections: Status (accepted), Context (the spec, the three-pass split, why mockups land first), Decision (single `internal/ui/contacts/` package, fixture pool as Go literal, the four sub-models, the App-owned popover and form lifecycle, the SuggestFn signature, the ContactSaveMsg / OpenFormMsg / OpenPopoverMsg contracts that 9.2 will consume), Consequences (locked visual decisions; data-layer-shaped seams already in place; CardDAV plumbing deferred to 9.2/9.3 without code changes to these surfaces).

Lock the **9.2 wiring contract** explicitly:
- `SuggestFn(prefix string) []contacts.Suggestion`
- `ContactSaveMsg{Contact, SaveTo string}`
- `OpenPopoverMsg{DisplayName, Email string}` + a `LookupFn(email string) (Contact, bool)` seam owned by App
- The `Sidebar.SelectionLetter / SelectionGroup` accessors used by `List.SetSelectionLetter`

- [ ] **Step 5: Run pass-end ritual**

Invoke the `poplar-pass` skill. It handles invariants update (add the Address Book section + ADR-0166 row to the decision index), STATUS flip (Pass 9.1 → done; populate Pass 9.2 starter prompt from spec), `/simplify` review of the diff, plan archival (`docs/superpowers/plans/2026-05-07-address-book-mockups.md` → `docs/superpowers/archive/plans/`), commit + push + `make install`.

- [ ] **Step 6: Verify install**

```bash
poplar
```

Confirm: `i` works in mail mode (match + no-match), `C` enters Contacts mode, `n` and `e` open form, autocomplete renders in compose. Quit cleanly with `q`.

---

## Self-review checklist (run before handing off to executor)

- [ ] **Spec coverage.** Walk spec §"Pass 9.1 — UI mockups, stub data" deliverables 1–7 and confirm each maps to a task. (1 popover → Task 4; 2 Contacts mode → Tasks 5/6/7; 3 form → Task 8; 4 autocomplete → Task 9; 5 live-tmux + screenshots → Task 10; 6 ADR → Task 10; 7 pass-end ritual → Task 10.)
- [ ] **Type consistency.** `Contact`, `Email`, `Phone`, `Suggestion`, `Kind`, `SortMode` defined in Task 1 and used unchanged in Tasks 3–9. `OpenFormMsg.Initial` is `Contact` (not `*Contact`). `ContactSaveMsg.SaveTo` is `string` (not enum) — matches spec §"Save to".
- [ ] **No placeholders.** Every step has either complete code, a precise file modification target, or an explicit reference to a spec section. The form's lifecycle in Task 8 step 4 says "Pass 9.1 hard-codes the single account name" — that's a 9.1-bounded settled decision, not a placeholder.
- [ ] **Pass budget.** 10 tasks, one ADR. Within the 8–12 budget.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-07-address-book-mockups.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Recommended given the task count and the breadth of touched packages.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
