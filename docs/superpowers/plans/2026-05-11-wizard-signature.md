# Wizard Signature Step Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Insert a one-signature step into the first-run setup wizard, backed by catkin, between the identity-name and account-label stages.

**Architecture:** Domain change adds a `Signature` string to `wizard.Model` and widens `Apply` to attach a `[]Signature` to the synthesized identity. UI change adds a new `stageSignature` inside `accountSection`, parallel to `stageProbe`, that hosts a `catkin.Model` with an immutable `-- ` chrome row above it and a description block explaining markdown + HTML rendering. The writer learns to emit multi-line TOML literals for signature text so the resulting `config.toml` reads naturally.

**Tech Stack:** Go 1.26, charm.land bubbletea/v2, charm.land huh/v2, internal/catkin (markdown editor), internal/theme.

**Spec:** `docs/superpowers/specs/2026-05-11-wizard-signature-design.md`.

---

## File Structure

- `internal/wizard/model.go` — add `Signature string` field.
- `internal/wizard/apply.go` — widen `Apply` synthesis branch, extend `FromAccount` to strip sentinel.
- `internal/wizard/apply_test.go` — table cases for signature flow.
- `internal/config/writer.go` — `multilineQuoted` helper, use it for signature `text`.
- `internal/config/writer_test.go` — multi-line round-trip case.
- `internal/ui/wizard/section_signature.go` — new file, catkin-hosting section.
- `internal/ui/wizard/section_signature_test.go` — Update / View tests.
- `internal/ui/wizard/section_account.go` — add `stageSignature` between identity and label; wire `signature *signatureSection`.

---

## Task 1: Add `Signature` field to `wizard.Model`

**Files:**
- Modify: `internal/wizard/model.go`

- [ ] **Step 1: Add the field**

Insert above the `Probe` field, in the order the wizard collects it:

```go
// Signature is the raw markdown body the user typed in the wizard's
// catkin editor. config's decoder injects the RFC 3676 "-- \n"
// sentinel at load time, so this field never carries it.
Signature string
```

- [ ] **Step 2: Verify compile**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/wizard/model.go
git commit -m "Wizard: collect signature body on Model"
```

---

## Task 2: Apply test for signature → signature → Identity.Signatures

**Files:**
- Test: `internal/wizard/apply_test.go`

- [ ] **Step 1: Read the existing test file**

Read `internal/wizard/apply_test.go` and locate the table that drives `TestApply` (or whatever the top-level test is). Note the case shape — these new cases must match it exactly.

- [ ] **Step 2: Add three table cases**

Append to the existing table:

```go
{
    name: "signature only — no identity name",
    in: Model{
        Preset:    "fastmail",
        Email:     "geoff@907.life",
        Token:     "tok",
        Signature: "Geoff\nlinks at [907.life](https://907.life)",
    },
    want: config.AccountConfig{
        Name:    "geoff@907.life",
        Preset:  "fastmail",
        Backend: "jmap",
        Email:   "geoff@907.life",
        Password: "tok",
        Identities: []config.Identity{{
            Email: "geoff@907.life",
            Signatures: []config.Signature{{
                Name: "default",
                Text: "Geoff\nlinks at [907.life](https://907.life)",
            }},
        }},
    },
},
{
    name: "identity name plus signature",
    in: Model{
        Preset:       "fastmail",
        Email:        "geoff@907.life",
        Token:        "tok",
        IdentityName: "Geoff Wright",
        Signature:    "Geoff Wright\ngeoff@907.life",
    },
    want: config.AccountConfig{
        Name:    "geoff@907.life",
        Preset:  "fastmail",
        Backend: "jmap",
        Email:   "geoff@907.life",
        Password: "tok",
        Identities: []config.Identity{{
            Name:  "Geoff Wright",
            Email: "geoff@907.life",
            Signatures: []config.Signature{{
                Name: "default",
                Text: "Geoff Wright\ngeoff@907.life",
            }},
        }},
    },
},
{
    name: "empty signature with identity name produces no signatures",
    in: Model{
        Preset:       "fastmail",
        Email:        "geoff@907.life",
        Token:        "tok",
        IdentityName: "Geoff Wright",
    },
    want: config.AccountConfig{
        Name:    "geoff@907.life",
        Preset:  "fastmail",
        Backend: "jmap",
        Email:   "geoff@907.life",
        Password: "tok",
        Identities: []config.Identity{{
            Name:  "Geoff Wright",
            Email: "geoff@907.life",
        }},
    },
},
```

If the existing test's case struct shape is different (e.g., separate `inModel` and `wantCfg` fields), adapt these blocks to match it — do not reshape the existing cases.

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/wizard/ -run TestApply -v`
Expected: FAIL — the three new cases mismatch because `Apply` does not yet read `m.Signature` nor synthesize an identity in the signature-only case.

---

## Task 3: Wire `Signature` into `Apply`

**Files:**
- Modify: `internal/wizard/apply.go`

- [ ] **Step 1: Replace the identity-synthesis block at the end of `Apply`**

Locate the trailing block (current source, lines 78–82):

```go
if m.IdentityName != "" {
    cfg.Identities = []config.Identity{
        {Name: m.IdentityName, Email: m.Email},
    }
}
```

Replace with:

```go
if m.IdentityName != "" || m.Signature != "" {
    id := config.Identity{Name: m.IdentityName, Email: m.Email}
    if m.Signature != "" {
        id.Signatures = []config.Signature{
            {Name: "default", Text: m.Signature},
        }
    }
    cfg.Identities = []config.Identity{id}
}
```

- [ ] **Step 2: Run the apply tests**

Run: `go test ./internal/wizard/ -run TestApply -v`
Expected: PASS (all original cases plus the three new ones).

- [ ] **Step 3: Commit**

```bash
git add internal/wizard/apply.go internal/wizard/apply_test.go
git commit -m "Wizard: write Signature onto the synthesized identity"
```

---

## Task 4: `FromAccount` strips the sentinel on repair

**Files:**
- Modify: `internal/wizard/apply.go`
- Test: `internal/wizard/apply_test.go`

- [ ] **Step 1: Add a `TestFromAccount` case for signature round-trip**

Either extend the existing `TestFromAccount` table or, if the test does not exist, add it. If extending, append a case shaped like the existing ones; if creating, the minimal form is:

```go
func TestFromAccountSignature(t *testing.T) {
    cfg := config.AccountConfig{
        Email:  "geoff@907.life",
        Preset: "fastmail",
        Identities: []config.Identity{{
            Name:  "Geoff Wright",
            Email: "geoff@907.life",
            Signatures: []config.Signature{{
                Name: "default",
                Text: "-- \nGeoff Wright\ngeoff@907.life",
            }},
        }},
    }
    got := FromAccount(cfg)
    if got.Signature != "Geoff Wright\ngeoff@907.life" {
        t.Errorf("Signature = %q, want sentinel-stripped body", got.Signature)
    }
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test ./internal/wizard/ -run TestFromAccount -v`
Expected: FAIL — `got.Signature` is empty because `FromAccount` does not read signatures yet.

- [ ] **Step 3: Extend `FromAccount`**

In `internal/wizard/apply.go`, inside the existing `if len(cfg.Identities) > 0` block, after the `m.IdentityName` assignment, add:

```go
if sigs := cfg.Identities[0].Signatures; len(sigs) > 0 {
    m.Signature = strings.TrimPrefix(sigs[0].Text, "-- \n")
}
```

Add `"strings"` to the import block at the top of the file.

- [ ] **Step 4: Run the test**

Run: `go test ./internal/wizard/ -run TestFromAccount -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wizard/apply.go internal/wizard/apply_test.go
git commit -m "Wizard: round-trip Signature through FromAccount"
```

---

## Task 5: Multi-line TOML literal for signature `text`

**Files:**
- Test: `internal/config/writer_test.go`
- Modify: `internal/config/writer.go`

- [ ] **Step 1: Read `internal/config/writer_test.go`**

Find the existing render test that covers signatures (search for `signature` or `Signatures`). Note the assertion shape — usually a golden string comparison.

- [ ] **Step 2: Add a multi-line signature render test**

Append, adapting case shape to the file's convention:

```go
func TestRenderMultilineSignature(t *testing.T) {
    accts := []AccountConfig{{
        Name:    "geoff",
        Preset:  "fastmail",
        Backend: "jmap",
        Email:   "geoff@907.life",
        Password: "tok",
        Identities: []Identity{{
            Name:  "Geoff",
            Email: "geoff@907.life",
            Signatures: []Signature{{
                Name: "default",
                Text: "Geoff Wright\ngeoff@907.life",
            }},
        }},
    }}
    got := string(Render(accts, UIConfig{}, nil))
    want := "text = \"\"\"\nGeoff Wright\ngeoff@907.life\n\"\"\"\n"
    if !strings.Contains(got, want) {
        t.Errorf("Render output missing multi-line signature block.\n--- want substring ---\n%s\n--- got ---\n%s", want, got)
    }
}
```

If the file does not already import `strings`, add it.

- [ ] **Step 3: Add a round-trip test**

```go
func TestRenderParseSignatureRoundTrip(t *testing.T) {
    body := "Geoff Wright\ngeoff@907.life"
    accts := []AccountConfig{{
        Name:    "geoff",
        Preset:  "fastmail",
        Backend: "jmap",
        Email:   "geoff@907.life",
        Password: "tok",
        Identities: []Identity{{
            Name:  "Geoff",
            Email: "geoff@907.life",
            Signatures: []Signature{{Name: "default", Text: body}},
        }},
    }}
    toml := Render(accts, UIConfig{}, nil)
    parsed, err := ParseAccounts(toml)
    if err != nil {
        t.Fatalf("ParseAccounts: %v", err)
    }
    if len(parsed) != 1 || len(parsed[0].Identities) != 1 || len(parsed[0].Identities[0].Signatures) != 1 {
        t.Fatalf("unexpected shape: %+v", parsed)
    }
    want := "-- \n" + body
    if got := parsed[0].Identities[0].Signatures[0].Text; got != want {
        t.Errorf("round-trip Text = %q, want %q", got, want)
    }
}
```

If the parse entrypoint is named differently (e.g., `parseAccounts` or a method on a config loader), adjust the call. Grep for `func.*Accounts.*\[\]byte` in `internal/config/` to find it.

- [ ] **Step 4: Run the tests to confirm they fail**

Run: `go test ./internal/config/ -run "TestRender(Multiline|ParseSignature)" -v`
Expected: FAIL — current writer escapes `\n` to `\\n` inside the basic-string form.

- [ ] **Step 5: Add `multilineQuoted` and switch the signature writer**

In `internal/config/writer.go`, add below `quoted`:

```go
// multilineQuoted renders s as a TOML basic multi-line literal when it
// contains a newline, otherwise as a single-line basic string. Used
// for signature bodies so rendered config reads naturally.
func multilineQuoted(s string) string {
    if !strings.Contains(s, "\n") {
        return quoted(s)
    }
    body := strings.ReplaceAll(s, `"""`, `\"\"\"`)
    return "\"\"\"\n" + body + "\n\"\"\""
}
```

Change the signature `text` emit at `writer.go:247` from:

```go
writeKV(b, "text", sig.Text)
```

to:

```go
if sig.Text != "" {
    fmt.Fprintf(b, "text = %s\n", multilineQuoted(sig.Text))
}
```

The empty-string skip mirrors `writeKV`'s behavior; the decoder rejects an empty `text` so the guard is also a safety belt.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/config/ -run "TestRender" -v`
Expected: PASS, including the existing single-line signature tests.

- [ ] **Step 7: Commit**

```bash
git add internal/config/writer.go internal/config/writer_test.go
git commit -m "Config: render multi-line signatures as TOML literals"
```

---

## Task 6: Wizard message — `BackMsg` from signature step needs a target stage

**Files:**
- Read: `internal/ui/wizard/section_account.go`

This task is investigation only — confirm the existing `BackMsg` handler at the top of `accountSection.Update` resets to a fixed stage. It currently resets to `stageCredentials`. The signature step must reset to `stageIdentity` instead. Note the change but do not implement it yet; it lands in Task 8.

- [ ] **Step 1: Read the relevant block**

Read `internal/ui/wizard/section_account.go` lines 124–137 (the `BackMsg` branch).

- [ ] **Step 2: Note the change shape**

The branch will become a switch on `s.stage`:

```go
case stageIdentity, stageProbe:
    s.stage = stageCredentials
case stageLabel:
    s.stage = stageSignature
case stageSignature:
    s.stage = stageIdentity
default:
    s.stage = stageCredentials
```

No commit. The actual edit happens in Task 8.

---

## Task 7: `signatureSection` UI — failing tests first

**Files:**
- Test: `internal/ui/wizard/section_signature_test.go` (new)

- [ ] **Step 1: Create the test file**

```go
package wizard

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

func newSignatureSectionForTest(t *testing.T) (*signatureSection, *Model) {
	t.Helper()
	parent := &Model{
		State:  wizdomain.Model{},
		Theme:  theme.Default(),
		Styles: NewStyles(theme.Default()),
	}
	return newSignatureSection(parent), parent
}

func TestSignatureSection_EscSkipsWithEmptyBody(t *testing.T) {
	s, parent := newSignatureSectionForTest(t)
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit a command")
	}
	msg := cmd()
	if _, ok := msg.(AdvanceMsg); !ok {
		t.Fatalf("Esc msg = %T, want AdvanceMsg", msg)
	}
	if parent.State.Signature != "" {
		t.Errorf("Signature = %q, want empty", parent.State.Signature)
	}
}

func TestSignatureSection_CtrlXSavesAndAdvances(t *testing.T) {
	s, parent := newSignatureSectionForTest(t)
	s.editor.SetValue("Geoff\ngeoff@907.life")
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+X must emit a command")
	}
	msg := cmd()
	if _, ok := msg.(AdvanceMsg); !ok {
		t.Fatalf("Ctrl+X msg = %T, want AdvanceMsg", msg)
	}
	if parent.State.Signature != "Geoff\ngeoff@907.life" {
		t.Errorf("Signature = %q, want catkin body", parent.State.Signature)
	}
}

func TestSignatureSection_CtrlPGoesBack(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+P must emit a command")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Fatalf("Ctrl+P msg = %T, want BackMsg", cmd())
	}
}

func TestSignatureSection_OtherKeysReachCatkin(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	before := s.editor.Value()
	s.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if s.editor.Value() == before {
		t.Errorf("editor did not receive 'a' keypress")
	}
}

func TestSignatureSection_ViewShowsChromeAndDescription(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	v := s.View()
	for _, want := range []string{
		"Email signature",
		"Markdown is supported and will be rendered as HTML on send.",
		"-- ",
		"^X save",
		"Esc skip",
	} {
		if !contains(v, want) {
			t.Errorf("View missing %q in:\n%s", want, v)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

If `theme.Default()` is named differently (e.g., `theme.OneDark()` or `theme.Compile("one-dark")`), grep `internal/theme/` for the default constructor and substitute. `probe_screen_test.go`, if it exists, is the canonical reference for wizard-section test scaffolding.

If `contains` / `indexOf` already exist as test helpers in the package, drop the bottom two functions and use the existing ones (or import `"strings"` and call `strings.Contains`).

- [ ] **Step 2: Run to confirm the tests fail**

Run: `go test ./internal/ui/wizard/ -run TestSignatureSection -v`
Expected: FAIL — the package will not compile because `signatureSection` and `newSignatureSection` do not exist.

---

## Task 8: Implement `signatureSection`

**Files:**
- Create: `internal/ui/wizard/section_signature.go`

- [ ] **Step 1: Write the file**

```go
package wizard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/catkin"
)

// signatureSection is the wizard's signature-entry step. It hosts a
// catkin.Model below an immutable "-- " chrome row and renders a
// description telling the user that markdown will be rendered as HTML
// on send. The sentinel is not in catkin's buffer — config's decoder
// adds it on the next load (ADR-0177).
type signatureSection struct {
	parent *Model
	editor catkin.Model
}

func newSignatureSection(parent *Model) *signatureSection {
	ed := catkin.New()
	ed.SetSize(64, 8)
	if parent.State.Signature != "" {
		ed.SetValue(parent.State.Signature)
	}
	return &signatureSection{parent: parent, editor: ed}
}

func (s *signatureSection) Init() tea.Cmd { return s.editor.Init() }

func (s *signatureSection) Update(msg tea.Msg) (*signatureSection, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case k.Code == 'x' && k.Mod == tea.ModCtrl:
			s.parent.State.Signature = s.editor.Value()
			return s, func() tea.Msg { return AdvanceMsg{} }
		case k.Code == tea.KeyEsc:
			s.parent.State.Signature = ""
			return s, func() tea.Msg { return AdvanceMsg{} }
		case k.Code == 'p' && k.Mod == tea.ModCtrl:
			return s, func() tea.Msg { return BackMsg{} }
		}
	}
	var cmd tea.Cmd
	s.editor, cmd = s.editor.Update(msg)
	return s, cmd
}

func (s *signatureSection) View() string {
	st := s.parent.Styles
	var b strings.Builder

	b.WriteString(st.Body.Render("Email signature — optional"))
	b.WriteString("\n\n")
	b.WriteString(st.Help.Render("Markdown is supported and will be rendered as HTML on send."))
	b.WriteString("\n")
	b.WriteString(st.Help.Render("Leave blank to skip."))
	b.WriteString("\n\n")

	// "-- " (two dashes, trailing space) is the RFC 3676 signature
	// boundary; ADR-0177 requires it on every saved signature. The
	// trailing space is load-bearing.
	b.WriteString(st.Help.Render("-- "))
	b.WriteString("\n")
	b.WriteString(s.editor.View())
	b.WriteString("\n\n")

	b.WriteString(st.Help.Render("Markdown  ^B bold · ^I italic · ^K link · ^L list · ^Q quote · ^Space task"))
	b.WriteString("\n")
	b.WriteString(st.Help.Render("Wizard    ^X save · Esc skip · ^P back"))

	return lipgloss.NewStyle().PaddingLeft(2).Render(b.String())
}
```

If `tea.ModCtrl` is named differently in this bubbletea version (e.g., `tea.ModifierCtrl`), grep the codebase for `Mod ==` to find the canonical form and substitute. `internal/ui/compose/bind.go` is a good reference — it dispatches `Ctrl+X` for send.

- [ ] **Step 2: Run the section tests**

Run: `go test ./internal/ui/wizard/ -run TestSignatureSection -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/wizard/section_signature.go internal/ui/wizard/section_signature_test.go
git commit -m "Wizard: add signature step (catkin + sentinel chrome)"
```

---

## Task 9: Wire `stageSignature` into `accountSection`

**Files:**
- Modify: `internal/ui/wizard/section_account.go`

- [ ] **Step 1: Add the new stage**

In the `accountStage` const block, insert `stageSignature` between `stageIdentity` and `stageLabel`:

```go
const (
    stageProvider accountStage = iota
    stageEmail
    stageCredentials
    stageProbe
    stageIdentity
    stageSignature
    stageLabel
    stageDone
)
```

- [ ] **Step 2: Add the `signature` field to `accountSection`**

```go
type accountSection struct {
    parent    *Model
    stage     accountStage
    form      *huh.Form
    probe     *probeScreen
    oauthSub  *oauthSection
    signature *signatureSection
}
```

- [ ] **Step 3: Construct the signature screen in `buildForm`**

Add a case between `stageIdentity` and `stageLabel`:

```go
case stageSignature:
    s.signature = newSignatureSection(s.parent)
    s.form = nil
```

- [ ] **Step 4: Route updates to the signature sub-screen**

In `Update`, after the `s.probe != nil` branch, before the `s.form == nil` guard, add:

```go
if s.signature != nil {
    updated, cmd := s.signature.Update(msg)
    s.signature = updated
    if _, ok := msg.(AdvanceMsg); ok {
        s.signature = nil
        s.advance()
        if s.stage == stageDone {
            return s, tea.Batch(cmd, func() tea.Msg { return AdvanceMsg{} })
        }
        return s, tea.Batch(cmd, s.Init())
    }
    return s, cmd
}
```

- [ ] **Step 5: Route view to the signature sub-screen**

In `View`, after the `s.probe != nil` branch:

```go
if s.signature != nil {
    return s.signature.View()
}
```

- [ ] **Step 6: Init the signature sub-screen**

In `Init`, after the `s.probe != nil` branch:

```go
if s.signature != nil {
    return s.signature.Init()
}
```

- [ ] **Step 7: Fix the `BackMsg` handler**

Replace the current `BackMsg` branch (lines 125–137) with stage-aware routing:

```go
if _, ok := msg.(BackMsg); ok {
    switch s.stage {
    case stageSignature:
        s.stage = stageIdentity
    case stageLabel:
        s.stage = stageSignature
    default:
        s.stage = stageCredentials
    }
    s.probe = nil
    s.oauthSub = nil
    s.signature = nil
    s.buildForm()
    switch {
    case s.form != nil:
        return s, s.form.Init()
    case s.signature != nil:
        return s, s.signature.Init()
    case s.oauthSub != nil:
        return s, s.oauthSub.Init()
    }
    return s, nil
}
```

- [ ] **Step 8: Build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 9: Run the full wizard suite**

Run: `go test ./internal/ui/wizard/ ./internal/wizard/ -v`
Expected: all PASS. If any existing `accountSection` test broke because it assumed the stage chain length, adjust the test fixture (not the stage order).

- [ ] **Step 10: Commit**

```bash
git add internal/ui/wizard/section_account.go
git commit -m "Wizard: thread stageSignature through accountSection"
```

---

## Task 10: Live verify in tmux

**Files:** none — observation only.

- [ ] **Step 1: Build + install**

Run: `make install`
Expected: poplar binary in `~/.local/bin/`.

- [ ] **Step 2: Spin up an isolated config**

```bash
mkdir -p /tmp/poplar-wizard-test
POPLAR_CONFIG=/tmp/poplar-wizard-test/config.toml poplar
```

Expected: wizard launches.

- [ ] **Step 3: Drive the wizard through to the signature step**

Pick Fastmail, enter `geoff@907.life`, paste `$FASTMAIL_API_TOKEN`, let probe finish, accept identity name. The signature screen appears.

- [ ] **Step 4: Verify visuals**

Confirm:
- Title reads `Email signature — optional`.
- Description reads `Markdown is supported and will be rendered as HTML on send.` / `Leave blank to skip.`
- `-- ` row sits above the editor, dim.
- Catkin cursor is at line 1, col 0, **below** the `-- ` row.
- Hint rows at bottom show markdown shortcuts and wizard controls.

- [ ] **Step 5: Verify behavior**

- Type `Geoff Wright` + Enter + `geoff@907.life`. Apply `Ctrl+B` to one word, confirm catkin renders bold styling.
- Press `Ctrl+X`. Wizard advances to the label step.
- Inspect `/tmp/poplar-wizard-test/config.toml`. The `[[account.identity.signature]]` block must show `text = """\nGeoff Wright\ngeoff@907.life\n"""` (with the catkin's actual output preserved).

- [ ] **Step 6: Re-run with `--repair`**

```bash
POPLAR_CONFIG=/tmp/poplar-wizard-test/config.toml poplar --repair=geoff
```

Confirm the signature step seeds catkin with the previously-saved body (sentinel-stripped).

- [ ] **Step 7: Clean up**

```bash
rm -rf /tmp/poplar-wizard-test
```

---

## Task 11: Ship gate

- [ ] **Step 1: Run the commit gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 2: Push**

```bash
git push
```

---

## Spec coverage check

- Stage insertion between identity and label → Tasks 1, 9.
- Catkin editor with markdown shortcuts → Task 8.
- Immutable `-- ` chrome row above catkin → Task 8 `View`.
- Description block surfacing markdown + HTML rendering → Task 8 `View`.
- Hint rows for markdown + wizard controls → Task 8 `View`.
- `Ctrl+X` / `Esc` / `Ctrl+P` semantics → Tasks 7, 8.
- `Signature` field on domain `Model` → Task 1.
- `Apply` widening and sentinel-free storage → Tasks 2, 3.
- `FromAccount` sentinel strip → Task 4.
- Writer multi-line TOML literal → Task 5.
- Round-trip preservation through decoder's `injectSentinel` → Task 5.
- Tests (apply, FromAccount, writer, section) → Tasks 2, 4, 5, 7.
- Live tmux verification → Task 10.

All spec sections accounted for. No new ADR required (ADR-0177 covers the signature schema; ADR-0191 covers the wizard's section structure).
