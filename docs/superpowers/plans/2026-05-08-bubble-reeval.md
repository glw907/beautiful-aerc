# Bubble Re-eval (post-ansix) + Eval B Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce one triage spec that re-evaluates Eval A's five candidates against the post-ansix world, runs Eval B on its four candidates, and settles the fork-vs-accept call for bubbles still gated on internal `lipgloss.Width` calls.

**Architecture:** Research-only pass. Each candidate task reads the library's render-path source (`go/pkg/mod` for cached modules, GitHub for uncached), reads the corresponding poplar consumer, then writes a candidate section into the spec. Output is a single Markdown spec; no Go code; no ADR (eval is research, ADRs land at swap time).

**Tech Stack:** Markdown, `go mod download`, ripgrep over `~/go/pkg/mod`, GitHub API for uncached repos. No build, no test, no `make check` — this pass produces no Go diff.

**Spec source:** `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`

**Output spec:** `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md` evolves in place — candidate sections are appended to the design as the eval runs. The current document is the framing; tasks 2–11 add the verdicts.

---

## Conventions for every candidate task

Each candidate task follows the same five-step shape:

1. **Read library source.** The render path that previously gated adoption — typically the function calling `lipgloss.JoinHorizontal` or `lipgloss.Width`. Use the cached path when present (`~/go/pkg/mod/...`); for uncached candidates run `go mod download <module>` first or fetch via GitHub raw URL.
2. **Read poplar consumer.** The current hand-roll. File paths listed per task.
3. **Decide:** does ansix change the verdict? (Eval A revisits.) Or, fresh from scratch (Eval B.)
4. **Append a candidate section** to `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md` under a new top-level heading `## <candidate>`. Section structure:
   - `**Does this make poplar better?**` — lead with the answer, one paragraph.
   - `**Feature parity:**` — gap analysis vs poplar's current behavior.
   - `**Customization seams:**` — theme + keymap + domain state injection points.
   - `**Theming integration:**` — `lipgloss.Style` injection or owns colors.
   - `**Maintenance signal:**` — last commit, version cadence, releases past 12 months.
   - `**Code delta estimate:**` — LOC removed from poplar vs LOC added.
   - `**License:**` — MIT/BSD/Apache only (hard veto).
   - `**Verdict:**` — one of `Adopt` / `Adopt-with-fork` / `Keep + harvest`.
   - `**Rationale (one line):**`
   - `**Interacts with:**` — bullets on dependencies/blockers among other candidates.
5. **Commit** with message `Pass 9w.2: <candidate> verdict`.

Per-candidate prose budget: 300–400 words for Eval A revisits and single-library Eval B candidates; up to 600 for the list-vs-bubblelister compare in Task 8.

---

### Task 1: ansix capability re-read

**Files:**
- Read: `internal/ansix/ansix.go`, `internal/ansix/ansix_test.go`
- Read: `docs/poplar/decisions/0181-ansix-width-shim.md`
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md` — add a `## ansix capability summary` section between `## Per-candidate process` and `## Fork-vs-accept call`.

- [ ] **Step 1: Read ansix source.** `internal/ansix/ansix.go` and `internal/ansix/ansix_test.go`. Note every exported function (`Width`, `Truncate`, `TruncateEllipsis`, `SetSPUACellWidth`, etc.), what it does, and how it differs from `ansi.StringWidth` / `lipgloss.Width`.

- [ ] **Step 2: Read ADR-0181** for the design rationale.

- [ ] **Step 3: Find ansix call sites.** Run:
  ```bash
  grep -rn "ansix\." internal/ --include="*.go" | head -40
  ```
  Note the surfaces that already use ansix (icon-bearing strings, JoinHorizontal-substitute join paths) and the surfaces that still use `lipgloss.Width` correctly (non-icon strings).

- [ ] **Step 4: Append the section.** Body covers:
  - Exported surface (one paragraph).
  - The seam: ansix wraps poplar's call sites; it does not patch upstream libraries. Bubbles whose render path calls `lipgloss.Width` internally cannot be fixed by ansix in poplar.
  - The implication for the re-eval: ansix flips a verdict only when the pre-ansix blocker was poplar's own width math, not the library's.

- [ ] **Step 5: Commit.**
  ```bash
  git add docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md
  git commit -m "Pass 9w.2: ansix capability summary"
  ```

---

### Task 2: `rmhubbert/bubbletea-overlay`

**Files:**
- Read: `~/go/pkg/mod/cache/download/github.com/rmhubbert/bubbletea-overlay/...` (run `go mod download github.com/rmhubbert/bubbletea-overlay` if absent), specifically `overlay.go` and `model.go`.
- Read: `internal/ui/uicore/overlay.go`, `internal/ui/uicore/modal_shell.go`, `internal/ui/app.go` (the cascade in `App.View`).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Fetch the library.** If not cached:
  ```bash
  cd /tmp && go mod download github.com/rmhubbert/bubbletea-overlay 2>/dev/null
  find ~/go/pkg/mod/github.com/rmhubbert -name '*.go' 2>/dev/null
  ```
  If `go mod download` fails standalone, fetch the raw source via:
  ```bash
  curl -s https://raw.githubusercontent.com/rmhubbert/bubbletea-overlay/main/overlay.go
  ```

- [ ] **Step 2: Read `Composite`, `Model`, `Position` types.** Note whether `Model` handles cascade/z-ordering, or just two-layer composition.

- [ ] **Step 3: Read poplar's overlay surface.** `internal/ui/uicore/overlay.go` (PlaceOverlay/DimANSI), `internal/ui/uicore/modal_shell.go`, and the cascade `if IsOpen()` chain in `internal/ui/app.go`.

- [ ] **Step 4: Decide.** Eval A's verdict was Keep + harvest because the library doesn't model the cascade. ansix doesn't change that. Confirm or revise in light of what ansix changes (probably nothing — the library's `Composite` already uses `charmbracelet/x/ansi` for cell widths and works on rendered strings).

- [ ] **Step 5: Write the section.** See the conventions header for structure.

- [ ] **Step 6: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubbletea-overlay verdict"
  ```

---

### Task 3: `bubbles/help`

**Files:**
- Read: `~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/help/help.go`
- Read: `internal/ui/helppopover/model.go`, `internal/ui/helppopover/styles.go`
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Read `help.go`.** Find every call to `lipgloss.JoinHorizontal` and `lipgloss.Width`. Note `Styles` field shape (`FullKey`, `FullDesc`, etc.). Note `KeyMap` interface (`ShortHelp() []key.Binding`, `FullHelp() [][]key.Binding`).

- [ ] **Step 2: Read poplar's helppopover.** Note the three-named-groups-per-row layout, `joinColumnsRow`, `renderGotoGrid`, `wired bool` semantics, and the bottom hint line outside any group.

- [ ] **Step 3: Decide.** Eval A's verdict cited two blockers: (i) `JoinHorizontal` ban under SPUA-A, (ii) `wired` dim affordance has no analogue. ansix does not patch the library's internal `JoinHorizontal` call — blocker (i) survives. Blocker (ii) is poplar-domain, not width-related. Verdict almost certainly stays Keep + harvest. **This candidate goes on the (b) list for fork-vs-accept synthesis (Task 11).**

- [ ] **Step 4: Write the section.** Lead with the ansix-doesn't-help finding; do not regurgitate Eval A's prose verbatim.

- [ ] **Step 5: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubbles/help verdict"
  ```

---

### Task 4: `daltonsw/bubbleup`

**Files:**
- Fetch source via `go mod download` or raw GitHub.
- Read: `internal/ui/uicore/...` (toast — likely `internal/ui/account/toast.go` or similar; locate with `grep -rn "renderToast" internal/`).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Locate poplar's toast.** Run:
  ```bash
  grep -rn "renderToast\|toastExpire\|triageOp\|pendingAction" internal/ --include="*.go" | head -10
  ```
  Read the file(s) returned.

- [ ] **Step 2: Fetch bubbleup.**
  ```bash
  cd /tmp && go mod download go.dalton.dog/bubbleup 2>/dev/null
  find ~/go/pkg/mod/go.dalton.dog -name '*.go' 2>/dev/null
  ```
  Read the main `alert.go` or equivalent (animation lerp, `AlertDefinition`, `Render`).

- [ ] **Step 3: Decide.** Eval A's verdict cited domain mismatch (poplar's toast carries triage payload + undo Cmd + countdown; bubbleup is category-styled animated alerts). ansix doesn't change the domain shape. Verdict almost certainly stays Keep + harvest. **Not a (b) candidate** — bubbleup uses its own color lerp, not `lipgloss.Width` gating.

- [ ] **Step 4: Write the section.** Confirm Eval A's verdict with the post-ansix angle (no change).

- [ ] **Step 5: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubbleup verdict"
  ```

---

### Task 5: `charmbracelet/huh`

**Files:**
- Fetch: `go mod download github.com/charmbracelet/huh` then read `form.go`, `group.go`, `field.go` and the public `Field` interface.
- Read: `internal/ui/contacts/form.go` (the contact edit form).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Fetch huh.**
  ```bash
  cd /tmp && go mod download github.com/charmbracelet/huh 2>/dev/null
  find ~/go/pkg/mod/github.com/charmbracelet -maxdepth 2 -name 'huh*' -type d
  ```

- [ ] **Step 2: Read `form.go`, `group.go`.** Confirm Eval A's findings: (Q1) chrome always rendered, no body-only mode; (Q2) `NewGroup(fields ...Field)` — fields fixed at construction.

- [ ] **Step 3: Read poplar's contact form.** `internal/ui/contacts/form.go`. Note the `(input, cycler, ★, −)` quartet, dynamic `focusList()` rebuild on every mutation.

- [ ] **Step 4: Decide.** Both Eval A blockers are structural to huh's design; ansix doesn't help. Verdict almost certainly stays Keep + harvest. Note the residual question for Pass 14 (first-run wizard) — huh may still fit a static wizard. **Not a (b) candidate** — the blocker is dynamic-row architecture, not width math.

- [ ] **Step 5: Write the section.**

- [ ] **Step 6: Commit.**
  ```bash
  git commit -am "Pass 9w.2: huh verdict"
  ```

---

### Task 6: `evertras/bubble-table`

**Files:**
- Fetch: `go mod download github.com/evertras/bubble-table`. Read `table/render.go` or whichever file holds `renderRowData` / `renderHeaders`.
- Read: `internal/ui/messagelist/model.go` (renderRow + threading), `internal/ui/contacts/list.go`.
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Fetch bubble-table.**
  ```bash
  cd /tmp && go mod download github.com/evertras/bubble-table 2>/dev/null
  find ~/go/pkg/mod/github.com/evertras -name 'render*.go' -o -name 'row*.go' 2>/dev/null
  ```

- [ ] **Step 2: Read render path.** Note every call to `lipgloss.JoinHorizontal` and `lipgloss.Width`. Note `RowData map[string]any` shape, `WithRowStyleFunc`, `StyledCell`.

- [ ] **Step 3: Read poplar consumers.** `internal/ui/messagelist/model.go` (threading prefix walk, SPUA-A flag-cell compensation), `internal/ui/contacts/list.go` (137 LOC, `metaCol`, `SetSelectionLetter`).

- [ ] **Step 4: Decide.** Eval A's blocker was `JoinHorizontal` in core render path. ansix does not patch the library. **This candidate goes on the (b) list for Task 11.** Verdict stays Keep + harvest unless fork is chosen in Task 11.

- [ ] **Step 5: Write the section.** Cover both consumers (messagelist and contacts/List).

- [ ] **Step 6: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubble-table verdict"
  ```

---

### Task 7: `bubbles/list` × picker sites (movepicker / linkpicker / attachpicker)

**Files:**
- Read: `~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/list/list.go`, `list/defaultitem.go`, `list/keys.go`.
- Read: `internal/ui/movepicker/model.go`, `internal/ui/reader/linkpicker.go`, `internal/ui/reader/attachpicker.go`.
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Read `bubbles/list`.** Specifically: `list.go` (Model, View, paginator integration), `defaultitem.go` (delegate pattern), `keys.go` (KeyMap). Note every `lipgloss.JoinHorizontal` and `lipgloss.Width` call. Note styling injection points.

- [ ] **Step 2: Read poplar's three picker consumers.** Locate via:
  ```bash
  grep -rn "type.*Model struct" internal/ui/movepicker internal/ui/reader/linkpicker.go internal/ui/reader/attachpicker.go 2>/dev/null
  ```
  Note the shared shape: ModalShell-mounted, j/k navigation, Enter/digit launch, Esc close. Note the differences: movepicker has destination resolution, linkpicker has URL launch, attachpicker has open vs save split.

- [ ] **Step 3: Decide.** Eval B is fresh — no Eval A baseline. Key questions:
  - Does the delegate pattern map onto poplar's ModalShell row shape?
  - Does the library's filter/paginator/title chrome interfere with the modal-frame embedding?
  - Does `lipgloss.JoinHorizontal` appear in the row render? (If yes — (b) candidate.)
  - Per-site fit: which of the three is cleanest, which fights hardest?

- [ ] **Step 4: Write the section.** One section, three sub-paragraphs (one per site). One verdict (Adopt / Keep + harvest); per-site notes inline.

- [ ] **Step 5: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubbles/list × pickers verdict"
  ```

---

### Task 8: `bubbles/list` vs `treilik/bubblelister` for `compose.Dropdown`

**Files:**
- Read: `bubbles/list` (already from Task 7 — cross-reference).
- Fetch: `go mod download github.com/treilik/bubblelister`. Read its main file.
- Read: `internal/ui/compose/dropdown.go` (locate via `grep -rn "Dropdown" internal/ui/compose`).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Fetch bubblelister.**
  ```bash
  cd /tmp && go mod download github.com/treilik/bubblelister 2>/dev/null
  find ~/go/pkg/mod/github.com/treilik -name '*.go' 2>/dev/null
  ```

- [ ] **Step 2: Read both libraries' relevant render paths.** For `bubbles/list`: cross-ref Task 7 notes. For `bubblelister`: read its main render function and key handling.

- [ ] **Step 3: Read `compose.Dropdown`.** Note the suggest-fn seam, the trailing-fragment matching, the splice-below-header rendering, the small list size (≤7 rows per ADR-0174). Note dropdown is positionally inline (not modal).

- [ ] **Step 4: Decide.** Compare both libraries against the dropdown's actual needs (small N, dynamic, inline-positioned, no filter chrome wanted). Which (if either) reduces poplar code without hurting the surface? Up to 600 words for this compare.

- [ ] **Step 5: Write the section.** Two-library compare; one verdict naming which (if either) wins; the loser gets a one-line dismissal.

- [ ] **Step 6: Commit.**
  ```bash
  git commit -am "Pass 9w.2: compose dropdown library compare"
  ```

---

### Task 9: `bubbles/list` for sidebar folder column

**Files:**
- Read: cross-ref Task 7 for `bubbles/list`.
- Read: `internal/ui/sidebar/model.go`, `internal/ui/sidebar/styles.go`, `internal/ui/sidebar/sidebar_column.go` (locate via `ls internal/ui/sidebar/`).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Read poplar's sidebar.** Note the three-folder-group layout (Primary/Disposal/Custom — fixed groups, blank-line separators), nested-name flat rendering, `J/K` navigation tied to the cache's classified-folder list, `SidebarColumn` composite (account header rows + sidebar + spacer + search shelf).

- [ ] **Step 2: Decide.** `bubbles/list` doesn't model named-group separators or composite columns. Likely Keep + harvest, but eval the actual fit — does the library's delegate pattern at least clean up row rendering? Worth not over-investing per the spec's risk note.

- [ ] **Step 3: Write the section.** Short — likely 200–300 words.

- [ ] **Step 4: Commit.**
  ```bash
  git commit -am "Pass 9w.2: bubbles/list × sidebar verdict"
  ```

---

### Task 10: `knipferrc/teacup` statusbar

**Files:**
- Fetch: `go mod download github.com/knipferrc/teacup`. Read `statusbar/statusbar.go` or equivalent.
- Read: `internal/ui/uicore/...` or `internal/ui/account/...` for the status bar (locate via `grep -rn "status_bar\|StatusBar\|renderStatusBar" internal/ui/`).
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`.

- [ ] **Step 1: Fetch teacup.**
  ```bash
  cd /tmp && go mod download github.com/knipferrc/teacup 2>/dev/null
  find ~/go/pkg/mod/github.com/knipferrc -name 'statusbar*' 2>/dev/null
  ```

- [ ] **Step 2: Read teacup statusbar.** Note its segment model, color injection, and width math (`lipgloss.JoinHorizontal`?).

- [ ] **Step 3: Read poplar's status bar.** Note: the chrome row (`──┴──╯` bottom), connection-state dot (`●`/`◐`/`○` color + shape), outbox depth segment (`⇅N` / `⚠N`), drop-rank command footer.

- [ ] **Step 4: Decide.** poplar's statusbar carries multiple domain-specific segments (connection state with a3-shape encoding for colorblind accessibility, outbox depth, command footer with drop ranks). Likely Keep + harvest unless teacup's segment model maps cleanly.

- [ ] **Step 5: Write the section.**

- [ ] **Step 6: Commit.**
  ```bash
  git commit -am "Pass 9w.2: teacup statusbar verdict"
  ```

---

### Task 11: Fork-vs-accept synthesis

**Files:**
- Read: ADR-0002 (`docs/poplar/decisions/0002-*.md` — locate exact filename), ADR-0075 (same), ADR-0181.
- Modify: `docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md` — replace the existing `## Fork-vs-accept call` section's framing prose with the actual decision and rationale.

- [ ] **Step 1: Locate the relevant ADRs.**
  ```bash
  ls docs/poplar/decisions/ | grep -E "0002|0075|0181"
  ```
  Read each in full.

- [ ] **Step 2: Collect the (b) list.** From Tasks 3, 6, and any picker/dropdown/sidebar/statusbar verdict that hit `lipgloss.Width` internally. Write the list explicitly with one-line per candidate naming the gating call (e.g., "`bubbles/help` — `lipgloss.JoinHorizontal` in `FullHelpView`").

- [ ] **Step 3: Investigate upstream PR viability.** Check whether `charmbracelet/x/ansi` or `charmbracelet/lipgloss` exposes (or has open issues for) a configurable width hook. Quick check:
  ```bash
  curl -s "https://api.github.com/repos/charmbracelet/x/issues?state=open&labels=&per_page=30" | grep -iE "ansi|width|spua|east-asian" | head -10
  curl -s "https://api.github.com/repos/charmbracelet/lipgloss/issues?state=open&per_page=30" | grep -iE "width|east-asian" | head -10
  ```
  Note any existing issue or PR.

- [ ] **Step 4: Write the synthesis.** Replace the spec's `## Fork-vs-accept call` section with:
  - The (b) list (collected in Step 2).
  - The fork option, costs (rebase, ADR-0002/0075 supersession), and what it unlocks.
  - The accept option and what it forecloses.
  - The upstream PR finding (Step 3).
  - **The decision.** One of `Fork` or `Accept`, with a paragraph of rationale. Include any conditional follow-up (e.g., "Accept now; revisit if upstream issue #NNN lands by Pass 9z").

- [ ] **Step 5: Update the spec's intro.** The opening `## Context` paragraph mentions "evolves into the eval result." Trim that placeholder note now that the eval is complete.

- [ ] **Step 6: Pass-end ritual.** Per `poplar-pass` skill:
  - `make check` is **not** required (no Go diff this pass).
  - **Skip ADR.** Eval is research; ADRs land at swap time.
  - Update `docs/poplar/invariants.md` only if the fork-vs-accept call **chose Fork** — that supersedes ADR-0002/0075 and needs an invariant update + new ADR. If accept, no invariant change.
  - Update `docs/poplar/STATUS.md`: mark 9w.2 done. Eval B has folded into 9w.2, so the original 9x slot is empty — promote subsequent passes one letter: 9y → 9x (Eval C lesson harvests), 9z → 9y (consolidation roadmap), 9z.1+ → 9y.1+ (swap/harvest passes). Rewrite the "current pass" line and the next starter prompt for the new 9x.
  - Archive plan + spec via `git mv` to `docs/superpowers/archive/plans/` and `docs/superpowers/archive/specs/`.

- [ ] **Step 7: Commit.**
  ```bash
  git add -A
  git commit -m "Pass 9w.2: fork-vs-accept call + pass-end ritual"
  git push
  ```

  No `make install` (no binary change).

---

## Self-review checklist (run after Task 11)

- [ ] Every candidate from the spec's candidate set has a verdict section.
- [ ] Every (b) candidate identified in Tasks 2–10 appears in Task 11's collected list.
- [ ] The fork-vs-accept call has a single clear decision (not "TBD" or "depends").
- [ ] STATUS table renumbering (9y/9z → 9x/9y) is applied consistently — pass row, queued row, "current pass" line.
- [ ] If Fork was chosen: a new ADR exists superseding 0002 and 0075, and `invariants.md` is updated.
- [ ] If Accept was chosen: no ADR, no invariant change, but the decision is recorded inside the spec.
