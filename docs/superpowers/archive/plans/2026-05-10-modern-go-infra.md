# Modern-Go Defaults: Claude Infra Pass (16a)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal.** Make Claude reach for Go 1.21–1.26 stdlib idioms by default when writing or reviewing poplar code. By the end of this pass, the `go-conventions` skill names the preferred modern form for each common pre-1.21 pattern, `/simplify`'s efficiency agent surfaces pre-1.21 patterns as findings, and a grep-tier `modern-go-check.sh` catches the mechanical ones in `make check`. No production Go is touched in this pass — that lands in 16b–d.

**Why now.** Poplar's toolchain is `go 1.26.1` (go.mod `1.26.0`), but a recent audit found ~30 sites across the codebase that still use pre-1.21 idioms: `sort.SliceStable` everywhere (zero `slices.SortFunc`), `sync.Once` + package-var pairs that `OnceValue` would collapse, push-callback iterators that `iter.Seq` would simplify, raw `fmt.Fprintf(os.Stderr,...)` for structured logging, `for i := 0; i < N; i++` with `i` unused. Three concrete passes (16b mechanical, 16c `iter.Seq`, 16d `slog`) will follow this one to apply the new defaults. This pass runs **before** the queued 17a sidebar tree work (which has a plan + spec on disk but no committed code), so 17a's new `internal/ui/sidebar/tree.go` lands already-using modern idioms instead of being a fresh source of pre-1.21 patterns that 16b would then have to rewrite. Same logic for 17b/17c/18.

**Scope.** Three files outside poplar's tree (`~/.claude/skills/go-conventions/SKILL.md`, `~/.claude/skills/simplify/SKILL.md`, optional `~/.claude/docs/go-comment-voice.md` cross-ref), one new script + Makefile wire-up in poplar, one ADR, one invariants pointer. No source-code changes in `internal/` or `cmd/`.

**Tech Stack.** Markdown (skill files, ADR), bash (the check script). No Go.

**Spec.** None — this plan is the spec; the body is small enough to inline.

---

## File Structure

| File | Role |
|---|---|
| `~/.claude/skills/go-conventions/SKILL.md` | **Edit.** Add `## Modern Stdlib Idioms` section between Anti-Patterns and Project Structure. Names the preferred form for each gap with one-line rationale. Cross-references the §7 tell catalogue where overlap exists. |
| `~/.claude/skills/simplify/SKILL.md` | **Edit.** Extend Agent 3 (Efficiency) checklist with the pre-1.21 patterns as findings (surface, not auto-fix). Note grep-tier sites covered by `modern-go-check.sh` to skip-scan, mirroring the existing voice-check exemption block. |
| `scripts/modern-go-check.sh` | **New.** Grep-tier scan for the mechanical anti-patterns (`sort.SliceStable(`, `sort.Slice(`, `for i := 0; i <`, `sync.Once\b` paired with package var in same file). Same shape and exit conventions as `scripts/voice-check.sh`. |
| `Makefile` | **Edit.** Add `modern-go-check` target; thread into the `check` target alongside `voice`. |
| `docs/poplar/decisions/0196-modern-go-defaults.md` | **New ADR.** Records the convention bump: poplar runs on go 1.26.1, so we use the language we have. Lists the preferred forms with one-line rationale each. Points at 16b/16c/16d as the apply passes. |
| `docs/poplar/invariants.md` | **Edit.** In the "Build & verification" section, add a one-line pointer: "`make check` runs `scripts/modern-go-check.sh`; pre-1.21 idioms (manual sort, `for i := 0; i < N`, `sync.Once`+var) are findings, not errors." Bump skills line to mention `go-conventions` covers modern idioms. |
| `CLAUDE.md` | **Edit.** Under "Conventions," append one sentence to the `go-conventions` bullet: "Includes the modern-stdlib-idiom table (slices/maps/iter/slog/OnceValue defaults)." |
| `docs/poplar/STATUS.md` | **Edit.** Mark 16a done; renumber 15b→17a, 15c→17b, 15d→17c, 16→18, 17→19; stub 16b/16c/16d with one-line goals; draft starter prompt for 16b. |

> The ADR number `0196` assumes the latest committed ADR is `0195` (Pass 15a.5 filepicker-lessons). Confirm at task 6 by listing `docs/poplar/decisions/`. 17a's sidebar-tree ADR becomes `0197` once this pass ships.

---

## Pre-flight

- [ ] **Step 0: Confirm clean working tree and queue state**

```bash
cd /home/glw907/Projects/poplar
git status
grep -A1 "Current pass" docs/poplar/STATUS.md | head -3
ls docs/poplar/decisions/ | tail -3
```

Expected: working tree clean (or only the modernization + sidebar-tree plan/spec docs untracked); STATUS shows 16a as the active pass; latest committed ADR is `0195` (Pass 15a.5), so the new ADR slot is `0196`.

---

## Task list

### Task 1 — Draft the "Modern Stdlib Idioms" section for go-conventions

- [ ] **1.1** Read `~/.claude/skills/go-conventions/SKILL.md` end-to-end. Note the voice (stdlib-formal, terse rationale per rule, anti-pattern → preferred pattern shape).
- [ ] **1.2** Draft the new section in a scratch buffer. Target ~60–80 lines. Subsections:
  - **Sorting:** `slices.SortFunc` / `slices.SortStableFunc` + `cmp.Compare` / `cmp.Or` over `sort.SliceStable`. `slices.Sort(s)` over `sort.Strings(s)` / `sort.Ints(s)`.
  - **One-shot init:** `sync.OnceValue[T]` / `sync.OnceFunc` over `sync.Once` paired with a package var and a wrapper function.
  - **Iterators:** `iter.Seq[T]` / `iter.Seq2[K,V]` for push iterators with ≥ 2 consumers. Hand-rolled `Next()`/`Stop()` and `ForEach(func)` callbacks are pre-1.23.
  - **Logging:** `log/slog` for internal-package logs. `fmt.Fprintln(os.Stderr,...)` is acceptable only in `cmd/` for user-facing startup errors.
  - **Loops:** `for range N` over `for i := 0; i < N; i++` when `i` is never read inside the body.
  - **Maps:** `maps.Keys` / `maps.Values` + `slices.Sorted` over manual collect-then-sort. `maps.Clone` over hand-rolled copy loops.
  - **Builtins:** `min` / `max` / `clear` over conditional helpers.
  - **Comparators:** `cmp.Or(a, b, c)` over nil-coalescing `if a != "" { return a }; if b != "" { ... }` chains.
  - **Errors:** `errors.Join(errs...)` for multi-error accumulation. First-error-wins still wins when call order matters.
  - **Loop scoping (1.22):** every loop variable is per-iteration. Delete leftover `x := x` shadow lines.
- [ ] **1.3** For each subsection, include exactly one short before/after snippet (3–5 lines each side). No commentary beyond one rationale line.
- [ ] **1.4** Cross-reference where these overlap with §7 tells (e.g., the T28 "over-explained Go idioms" rule already covers `min`/`max` helpers — note it once, do not duplicate).

### Task 2 — Land the section in go-conventions

- [ ] **2.1** Insert the section after `## Anti-Patterns: Go That Looks Like Python or JavaScript` (line 83 region) and before `## Project Structure`. Numbering: `## Modern Stdlib Idioms`.
- [ ] **2.2** Verify the SKILL.md frontmatter `description` still accurately summarizes the skill. If it claims to cover "anti-patterns, project structure, …" without mentioning modern idioms, extend it.
- [ ] **2.3** Smoke read: open the file, confirm no formatting drift, headings nest cleanly.

### Task 3 — Extend `/simplify` Agent 3 (Efficiency)

- [ ] **3.1** Read `~/.claude/skills/simplify/SKILL.md` lines around the Agent 3 definition (currently ~line 41–53 region). Note that Agent 4 (voice) already has a "grep-tier tells covered by tooling — do not re-scan" block. Mirror that pattern.
- [ ] **3.2** Add to Agent 3's checklist (semantic findings — surface, do not auto-fix):
  - Pre-1.21 sort calls when the project's go.mod is `>= 1.21`.
  - `sync.Once` + package-var pairs that `OnceValue` would collapse.
  - Push-callback iterators with ≥ 2 call sites (single-caller push iterators are fine).
  - Raw `fmt.Fprintf(os.Stderr, ...)` for log-shaped messages inside `internal/` packages.
  - Hand-rolled multi-error accumulation when `errors.Join` would fit.
  - Nil-coalescing chains where `cmp.Or` would fit.
- [ ] **3.3** Add a parallel "grep-tier exemption" block: "If the project ships `scripts/modern-go-check.sh` (poplar: yes, run by `make check`), skip the mechanical patterns it covers (`sort.SliceStable(`, `sort.Slice(`, `for i := 0; i < N; i++` with unused `i`, `sync.Once` paired with package var). Spend attention on the semantic findings above."
- [ ] **3.4** Verify Agent 3's bias-statement (precision vs recall) still reads correctly. Modern-idiom findings should be **high precision** — false positives waste apply-phase time more than missed findings hurt, mirroring the voice agent.

### Task 4 — Write `scripts/modern-go-check.sh`

- [ ] **4.1** Read `scripts/voice-check.sh` for the shape (shebang, set flags, exit conventions, grep invocations, output format). Mirror it exactly.
- [ ] **4.2** Implement four grep tiers:
  - **M1:** `\bsort\.SliceStable\b` and `\bsort\.Slice\b` outside `_test.go` — preferred: `slices.SortStableFunc`/`slices.SortFunc`.
  - **M2:** `for i := 0; i <` followed by a body that does not reference `i` (best-effort: flag the loop header; the agent disambiguates false positives).
  - **M3:** `\bsync\.Once\b` declared at package scope (not inside a function) — manual review for `OnceValue`/`OnceFunc` candidacy.
  - **M4:** `\bsort\.Strings\b` and `\bsort\.Ints\b` — preferred: `slices.Sort`.
- [ ] **4.3** Default exit code matches voice-check.sh: non-zero on findings, zero on clean. If voice-check is currently soft (warn-only), match that.
- [ ] **4.4** Sanity test:
  ```bash
  bash scripts/modern-go-check.sh
  ```
  Expected: surfaces the ~30 sites the audit found. If counts are wildly off, tune the regexes — but err on **precision**, not recall (false positives kill trust in grep-tier checks).
- [ ] **4.5** `chmod +x scripts/modern-go-check.sh`.

### Task 5 — Wire `modern-go-check` into Makefile

- [ ] **5.1** Read current Makefile, find the `voice` target and the `check` aggregate.
- [ ] **5.2** Add a `modern-go-check` target that runs `scripts/modern-go-check.sh`.
- [ ] **5.3** Thread it into `check` (order: `fmt-check vet voice modern-go-check test` or whatever the existing convention dictates).
- [ ] **5.4** Run `make check`. Expected: passes if the script is soft-warn, fails listing the ~30 sites if hard. Either way, the surface is now visible.

### Task 6 — Write ADR 0196

- [ ] **6.1** Confirm ADR number by listing `docs/poplar/decisions/` and picking the next free slot. Expected: `0196`. Update the file structure table in this plan if the number shifts.
- [ ] **6.2** Draft `0196-modern-go-defaults.md`:
  - **Context:** Toolchain is `go 1.26.1`, but a 2026-05-10 audit found ~30 pre-1.21 sites. Without a convention bump, queued passes (17a sidebar tree, 17b messagelist on `bubbles/v2/list`, 17c help audit, 18 Polish II) would keep growing the gap.
  - **Decision:** The `go-conventions` skill now names the preferred modern-stdlib form for each common pattern (sorting, init, iteration, logging, loops, maps, builtins, comparators, errors). `/simplify` Agent 3 surfaces gaps as findings. `scripts/modern-go-check.sh` catches the mechanical ones in `make check`.
  - **Consequences:** Three follow-up passes (16b mechanical, 16c `iter.Seq`, 16d `slog` + logging ADR) apply the new defaults to existing code. New code in 17a/17b/17c/18 lands already-modern.
  - **Alternatives considered:** (a) Apply via one giant pass — rejected, mixes mechanical + judgment work, blows the 8–12 task budget. (b) Skip the script, rely on the agent — rejected, grep-tier checks have zero false-negatives and run in `make check` for free.
- [ ] **6.3** Add the ADR to `docs/poplar/decisions/INDEX.md` under the appropriate theme (probably "Tooling & conventions" or whichever section holds the existing voice-check ADR).

### Task 7 — Update invariants and CLAUDE.md pointers

- [ ] **7.1** In `docs/poplar/invariants.md`, locate the "Build & verification" section. Add a sentence mentioning `modern-go-check.sh` alongside the existing `voice-check.sh` description. Keep the prose tight — this is an index, not the contract.
- [ ] **7.2** In the same section, the line listing the binding skills currently mentions `go-conventions`, `elm-conventions`, `poplar-pass`. Extend the `go-conventions` clause: "Anti-patterns, **modern-stdlib idiom defaults**, project structure, cobra shape, error wrapping, tests, Makefile, naming."
- [ ] **7.3** In root `CLAUDE.md`, under "Conventions," extend the `go-conventions` bullet with one sentence pointing at the modern-idiom table. Do not duplicate the table — the skill is the single source.

### Task 8 — Smoke test the whole loop

- [ ] **8.1** Write a throwaway file `/tmp/poplar-modern-smoke.go` containing each anti-pattern (sort.SliceStable, sync.Once + var, for-i-unused, fmt.Fprintf-as-log).
- [ ] **8.2** Run `bash scripts/modern-go-check.sh /tmp/poplar-modern-smoke.go`. Expected: all mechanical tiers fire.
- [ ] **8.3** Run `/simplify` against a tiny throwaway diff that introduces the same anti-patterns into a real poplar file (revertable). Expected: Agent 3 surfaces the semantic findings; the grep-tier ones are deferred to `modern-go-check.sh` per the exemption block.
- [ ] **8.4** Revert the throwaway diff.

### Task 9 — Renumber the queue and stub 16b/16c/16d in STATUS

- [ ] **9.1** Edit `docs/poplar/STATUS.md`. Renumber the queue:
  - Mark **16a** done.
  - Add **16b — Mechanical Go modernization sweep**: `slices.SortFunc` + `slices.Sort` + `for range N` + `sync.OnceValue` + `maps.Keys`. Mechanical, ~12 tasks across ~15 files. No new ADR.
  - Add **16c — `iter.Seq` adoption**: `catkin/style.go` `walkSpans` + 3 callers. Judgment-light. BACKLOG #46 (messagelist `iter.Seq2`) is **not** in this pass — 17b absorbs it as part of the `bubbles/v2/list` rewrite.
  - Add **16d — `log/slog` adoption + logging ADR**: mailjmap push-loop logs, error transcript shape. ~6 tasks + 1 ADR (logging convention: when slog, when stderr, when neither).
  - Rename **15b → 17a** (sidebar tree — has plan/spec on disk, code not yet started), **15c → 17b** (messagelist on `bubbles/v2/list`), and **15d → 17c** (`bubbles/v2/help` audit). The 17 series is "bubbles-adoption remainder."
  - Renumber **16 → 18** (Polish II) and **17 → 19** (v0.9.0 prep).
  - Update the existing prose framing "Pass 15b — sidebar folder hierarchy ... Third of four bubbles-adoption passes (15a, 15a.5, 15b, 15c, 15d)" to reflect the new naming. The 15a / 15a.5 historical entries stay as-is.
- [ ] **9.2** Draft the "Next starter prompt (Pass 16b)" block in STATUS. Mirror the format of prior pass starter prompts: goal, scope, settled-do-not-rebrainstorm, still-open, approach.

### Task 10 — Pass-end ritual

- [ ] **10.1** Invoke the `poplar-pass` skill (end-of-pass branch). It covers:
  - Verify ADR is in place and indexed.
  - Verify invariants.md updated.
  - Move this plan to `docs/superpowers/archive/plans/2026-05-10-modern-go-infra.md`.
  - `make check` — must pass (or warn cleanly if the new script is soft).
  - Commit with the standard footer; the commit touches the three skill files (which live outside the poplar repo — committed separately to `~/.dotfiles` via the `claude` stow package), the new script, Makefile, ADR, invariants, CLAUDE.md, STATUS, and the archived plan.
  - `git push` poplar; `cd ~/.dotfiles && git add claude/.claude/skills/go-conventions/SKILL.md claude/.claude/skills/simplify/SKILL.md && git commit && git push` for the skill changes.
  - `make install` — no-op for this pass since no Go changed, but run it to keep the ritual habit.
- [ ] **10.2** Confirm STATUS reflects 16b as the next pass.

---

## Verification gates

1. **Skill content** — `grep -c "## Modern Stdlib Idioms" ~/.claude/skills/go-conventions/SKILL.md` → 1.
2. **Simplify extension** — `grep -c "modern-go-check.sh" ~/.claude/skills/simplify/SKILL.md` → 1.
3. **Script executable** — `test -x scripts/modern-go-check.sh`.
4. **Makefile wiring** — `make modern-go-check` runs cleanly; `make check` invokes it.
5. **ADR indexed** — `grep "0196\|0197" docs/poplar/decisions/INDEX.md` resolves to the new ADR.
6. **STATUS stubs** — `grep -cE "\*\*16[bcd]\*\*" docs/poplar/STATUS.md` → 3; the 15b/15c/15d/16/17 rows are gone (renamed to 17a/17b/17c/18/19).

---

## Out of scope (deferred to 16b–d / 17b)

- Any change to `*.go` files under `cmd/` or `internal/`.
- Any change to `go.mod` (the toolchain bump already happened).
- The `iter.Seq2` messagelist conversion (BACKLOG #46 — absorbed into 17b's `bubbles/v2/list` rewrite, not a standalone pass).
- The mailjmap structured-logging ADR text (16d writes it; this pass just stubs the slot).

---

## Appendix — 2026-05-10 audit (evidence for 16b / 16c / 16d)

Captured 2026-05-10 by the `Explore` agent. Verify file:line before each apply pass — line numbers may have drifted by the time 16b–d execute.

### 16b — Mechanical sweep targets

**Sorting** (10 sites — `slices.SortFunc` / `slices.SortStableFunc` + `cmp.Compare` / `cmp.Or`):

- `internal/catkin/annotate.go:42` — `sort.SliceStable` by `.Range.Start`
- `internal/ui/contacts/list.go:218` — `sort.SliceStable` by sort key string
- `internal/ui/sidebar/model.go:383` — `sort.SliceStable` by rank + name (multi-key — use `cmp.Or`)
- `internal/ui/messagelist/model.go:190` — `sort.SliceStable`
- `internal/ui/messagelist/model.go:399` — `sort.SliceStable`
- `internal/contacts/vcard.go:93` — `sort.SliceStable`
- `internal/catkin/spellcheck.go:209` — `sort.SliceStable`
- `internal/ui/compose/attachpicker.go:160` — `sort.SliceStable`
- `internal/mailimap/messages.go:37` — `sort.SliceStable`
- `internal/theme/themes.go:307` — `sort.SliceStable` (also a `maps.Keys` candidate — see below)
- `internal/config/accounts.go:634` — `sort.SliceStable`

**Map iteration patterns** (`maps.Keys` + `slices.Sorted`):

- `internal/theme/themes.go:307` — iterates a map, collects keys, then `sort.Strings`. Folds into the sort fix above.

**`sync.Once` + package-var pairs** (`sync.OnceValue` / `sync.OnceFunc`):

- `internal/term/font.go:16-37` — `hasNerdFontOnce sync.Once` + `hasNerdFontResult bool` as two package-level vars; `HasNerdFont()` wraps `Do(func() { hasNerdFontResult = ... }); return hasNerdFontResult`. Collapses to `var HasNerdFont = sync.OnceValue(func() bool { ... })`. ~15 lines deleted.
- `internal/catkin/spellcheck.go:26` — `once sync.Once` struct field paired with `delIdx` built in `buildIndex`. `OnceFunc` wraps `buildIndex`; `once` field drops.

**`for i := 0; i < N; i++` with `i` unused** (`for range N`):

- `internal/ui/reader/linkpicker.go:139` — `for i := 0; i < 2; i++`
- `internal/content/render_footnote.go:147` — `for i := 0; i < n; i++`
- `internal/ui/status_bar.go:115` — `for i := 0; i < width; i++`
- `internal/ui/top_line.go:28` — `for i := 0; i < fillWidth; i++`
- `internal/ui/compose/attachpicker.go:351` — `for i := 0; i < rows; i++`

**Other** (verify during 16b — were not exhaustively scanned):

- Any leftover `x := x` loop-var shadows (1.22 made these redundant) — grep `^\s*\w+ := \w+\s*$` inside loop bodies.
- Any `min`/`max` helper functions that could collapse to builtins (1.21).

### 16c — `iter.Seq` conversion target

- `internal/catkin/style.go:101` — `walkSpans(s string, fn func(kind spanKind, text string, submatch []string))`. Pure push iterator. Called at:
  - `internal/catkin/style.go:82` (self-call)
  - `internal/catkin/spellcheck.go:368` (closure captures `after` pointer)
  - `internal/catkin/match.go:39` (closure captures `found` bool)
- Convert signature to `func spans(s string) iter.Seq3[spanKind, string, []string]` (or struct yield). Call sites become `for kind, text, sub := range spans(s) { ... }` — early-return becomes real `break`, captured-bool sentinels become loop-local vars.
- **Not in this pass:** BACKLOG #46 messagelist `iter.Seq2` — 17b absorbs it.

### 16d — `log/slog` adoption targets

Internal-package log calls (convert to `slog`):

- `internal/mailjmap/push.go:127` — `fmt.Fprintf(os.Stderr, "mailjmap: ...")` in handler
- `internal/mailjmap/push.go:189` — same shape, push-loop hot path
- `internal/mailjmap/push.go:265` — same shape
- `internal/mailjmap/jmap.go:862` — `fmt.Fprintln(os.Stderr, "push-draft: destroy prior:", err)`

Cmd-layer stderr (stay as-is — user-facing startup errors, not log-shaped):

- `cmd/poplar/root.go:92,109,115,117,248`
- `cmd/poplar/reauth.go:25,29,39,43`

The logging-convention ADR (drafted in 16d) codifies this split: `slog` inside `internal/`, `fmt.Fprintln(os.Stderr, ...)` only for user-facing messages in `cmd/`.

