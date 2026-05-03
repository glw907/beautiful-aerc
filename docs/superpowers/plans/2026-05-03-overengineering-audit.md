# Pass 8.5 — Overengineering Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sweep the entire poplar codebase (~34k LOC, 98 Go files)
for overengineering — speculative abstractions, dead scaffolding,
defensive code for impossible paths, single-impl preemptive
interfaces, one-call-site helpers, zero-value-only config knobs,
Middle Man wrappers — and delete what doesn't earn its place.

**Architecture:** Four phases. **A** runs three Go static-analysis
tools (`deadcode`, `unparam`, `staticcheck`/`golangci-lint`) and
applies the trivially-safe deletions mechanically. **B** dispatches
eight parallel review subagents, one per package, each grounded in
Phase A's residual findings and bounded by an evidence-anchored
forced-deletion budget (floor 3, ceiling 10 per package). **C** is a
single-reviewer manual cross-cutting re-read of seams between
packages. **D** aggregates findings, triages with the `/simplify`
skip-rationale guard, and applies per-package with `make check`
between commits.

**Tech Stack:** Go 1.26.1, `golang.org/x/tools/cmd/deadcode`,
`mvdan.cc/unparam`, `staticcheck`, `golangci-lint`, Claude Code
Agent tool for parallel review dispatch.

**Working directory:** master branch (per project convention; no
worktree).

**Spec:** `docs/superpowers/specs/2026-05-03-overengineering-audit-design.md`

---

## File map

**New files (created during the pass):**
- `docs/poplar/audits/2026-05-03-deadcode.txt` — raw deadcode output
- `docs/poplar/audits/2026-05-03-unparam.txt` — raw unparam output
- `docs/poplar/audits/2026-05-03-golangci.txt` — raw golangci-lint output
- `docs/poplar/audits/2026-05-03-pkg-cmd-poplar.md`
- `docs/poplar/audits/2026-05-03-pkg-cache.md`
- `docs/poplar/audits/2026-05-03-pkg-mail.md`
- `docs/poplar/audits/2026-05-03-pkg-mailjmap.md`
- `docs/poplar/audits/2026-05-03-pkg-mailimap.md`
- `docs/poplar/audits/2026-05-03-pkg-config.md`
- `docs/poplar/audits/2026-05-03-pkg-leaves.md` — theme + term + backoff
- `docs/poplar/audits/2026-05-03-pkg-ui.md`
- `docs/poplar/audits/2026-05-03-cross-cutting.md`
- `docs/poplar/audits/2026-05-03-overengineering-audit.md` — final aggregated triage
- ADR files as needed (`docs/poplar/decisions/0125-*.md` and onward)

**Modified files:**
- `.golangci.yml` — enable additional checks (Task A2)
- `Makefile` — add `audit` convenience target (Task A2)
- `STATUS.md` — Pass 8.5 row (Task PE3)
- `docs/poplar/invariants.md` — deletions reflected (Task PE2)
- `.claude/rules/ui-invariants.md` — only if `internal/ui/` deletions
  changed binding facts (Task PE2)
- All Go source under `cmd/poplar/` and `internal/` (except
  `internal/mailauth/`) as findings dictate during Phase D.

---

## Phase A — Static analysis baseline

### Task A1: Install Go static-analysis tools

**Files:**
- No source files modified; tools installed into `$GOPATH/bin`
  (which is in `$PATH` per `.bashrc`).

- [ ] **Step 1: Install deadcode**

```bash
go install golang.org/x/tools/cmd/deadcode@latest
```

Expected: command returns silently. `which deadcode` should resolve.

- [ ] **Step 2: Install unparam**

```bash
go install mvdan.cc/unparam@latest
```

Expected: command returns silently. `which unparam` should resolve.

- [ ] **Step 3: Verify both tools run**

```bash
deadcode -help 2>&1 | head -3
unparam -help 2>&1 | head -3
```

Expected: usage text printed for each.

- [ ] **Step 4: Verify staticcheck and golangci-lint are present**

```bash
which staticcheck golangci-lint
```

Expected: both paths resolve. If either is missing, install via
`go install honnef.co/go/tools/cmd/staticcheck@latest` and
`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`.

(No commit — tools are local-machine state.)

### Task A2: Configure `.golangci.yml` and Makefile

**Files:**
- Modify: `/home/glw907/Projects/poplar/.golangci.yml`
- Modify: `/home/glw907/Projects/poplar/Makefile`

- [ ] **Step 1: Update `.golangci.yml` to enable additional checks**

Replace the file contents with:

```yaml
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosimple
    - unparam

linters-settings:
  errcheck:
    check-type-assertions: true
  unused:
    exported-fields-as-used: false

issues:
  exclude-use-default: false
```

The two changes: `unparam` added to enabled linters; `unused` configured
with `exported-fields-as-used: false` so unused exported struct fields
in this single-binary application are surfaced.

- [ ] **Step 2: Add `audit` target to Makefile**

Insert after the `lint:` target:

```makefile
audit:
	@mkdir -p docs/poplar/audits
	@echo "Running deadcode..."
	@deadcode ./cmd/poplar > docs/poplar/audits/2026-05-03-deadcode.txt 2>&1 || true
	@echo "Running unparam..."
	@unparam ./... > docs/poplar/audits/2026-05-03-unparam.txt 2>&1 || true
	@echo "Running golangci-lint..."
	@golangci-lint run ./... > docs/poplar/audits/2026-05-03-golangci.txt 2>&1 || true
	@echo "Audit outputs written to docs/poplar/audits/"
```

Add `audit` to the `.PHONY` line:

```makefile
.PHONY: build test test-imap vet lint audit install check clean
```

- [ ] **Step 3: Verify Makefile parses**

```bash
make -n audit
```

Expected: prints the commands without executing. No "missing
separator" or "*** no rule" errors.

- [ ] **Step 4: Verify golangci-lint config parses**

```bash
golangci-lint config verify
```

Expected: "no issues found" or empty output. Exit code 0.

- [ ] **Step 5: Commit**

```bash
git add .golangci.yml Makefile
git commit -m "$(cat <<'EOF'
Pass 8.5 A: add static-analysis audit tooling

Enable unparam in golangci-lint; configure unused with
exported-fields-as-used: false for single-binary application
field detection. Add `make audit` target that runs deadcode,
unparam, and golangci-lint and writes raw output under
docs/poplar/audits/.

Spec: docs/superpowers/specs/2026-05-03-overengineering-audit-design.md
Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task A3: Run static analysis and commit raw output

**Files:**
- Create: `docs/poplar/audits/2026-05-03-deadcode.txt`
- Create: `docs/poplar/audits/2026-05-03-unparam.txt`
- Create: `docs/poplar/audits/2026-05-03-golangci.txt`

- [ ] **Step 1: Run the audit**

```bash
make audit
```

Expected: three output files written. Each may contain findings or
be empty.

- [ ] **Step 2: Check the outputs are non-binary and reasonable**

```bash
wc -l docs/poplar/audits/2026-05-03-{deadcode,unparam,golangci}.txt
head -20 docs/poplar/audits/2026-05-03-deadcode.txt
head -20 docs/poplar/audits/2026-05-03-unparam.txt
head -20 docs/poplar/audits/2026-05-03-golangci.txt
```

Expected: line counts in the tens to low thousands; output is plain
text with file:line citations.

- [ ] **Step 3: Commit the raw output**

```bash
git add docs/poplar/audits/
git commit -m "$(cat <<'EOF'
Pass 8.5 A: capture static-analysis baseline

Raw output from deadcode, unparam, and golangci-lint as the
grounding input for Phase B per-package review agents.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task A4: Apply trivially-safe deletions

**Files:**
- Modify: any Go source flagged by `deadcode` as unreachable from
  `main`, or by `unused` (U1000) with no plausible consumer.

The spec defers `unparam` findings to Phase D triage (some are test
fixtures); only `deadcode` and `unused` deletions are mechanical
here.

- [ ] **Step 1: Read the deadcode output and identify unreachable symbols**

```bash
cat docs/poplar/audits/2026-05-03-deadcode.txt
```

For each finding, the output names a function/method that is
unreachable from `main`. These are deletion candidates.

- [ ] **Step 2: Read the unused output (golangci-lint) and identify unused symbols**

```bash
grep -E "(unused|U1000)" docs/poplar/audits/2026-05-03-golangci.txt
```

For each finding, the output names an unused unexported (or
exported, given the new config) symbol.

- [ ] **Step 3: For each deadcode finding, delete the symbol**

For each function/method in the deadcode output:
1. Open the file at the cited location.
2. Verify the symbol is genuinely unreachable (no string-keyed
   reflection, no test-only consumer that the analyzer missed).
3. Delete the function or method body.
4. Delete any sibling symbols (types, constants) that become dead
   as a result. (Re-running `deadcode` after each batch will catch
   these.)

If a deadcode finding turns out to have a non-obvious consumer
(reflection, build-tag-gated path, generated-code reference), do
NOT delete; record it as a Phase B finding for the relevant
package's audit doc instead.

- [ ] **Step 4: For each unused finding, delete or downgrade visibility**

For each `unused` finding:
1. If unexported and unused: delete.
2. If exported and unused: check whether it's part of an API
   contract (interface implementation, package public API). If
   yes, leave for Phase B agent triage. If no, delete.

- [ ] **Step 5: Re-run audit to confirm cascade**

```bash
make audit
```

Expected: deadcode and unused outputs shrunk. New findings may
appear if cascading dead code emerged; repeat Step 3-4 until the
output stabilizes.

- [ ] **Step 6: Verify the build**

```bash
make check
```

Expected: PASS. If anything fails, the deletion was wrong; revert
the offending change with `git checkout -- <file>` and record as a
Phase B candidate instead.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 8.5 A: static-analysis deletions

Mechanical deletions of symbols flagged unreachable by deadcode or
unused by golangci-lint's unused checker. Symbols requiring human
judgment (unparam findings, edge cases) deferred to Phase B/D.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If no symbols were safely deletable, skip the commit and note in
the next phase's commit message that Phase A deletions were empty.

---

## Phase B — Parallel subagent audit

### Task B1: Dispatch all eight per-package review agents in parallel

**Files:**
- Create: `docs/poplar/audits/2026-05-03-pkg-cmd-poplar.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-cache.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-mail.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-mailjmap.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-mailimap.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-config.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-leaves.md`
- Create: `docs/poplar/audits/2026-05-03-pkg-ui.md`

All eight agents are dispatched in a single message with eight
parallel `Agent` tool calls (one per package).

- [ ] **Step 1: Pre-build the shared agent brief**

The shared brief is identical across all eight agents except for
the package path, the special-lens line, and the output file. The
template:

```
You are reviewing one Go package in the poplar codebase for
overengineering. Poplar is a single-binary bubbletea terminal
email client; you are NOT writing code, only producing a findings
document at the path given below. Read-only.

Package under review: <PKG_PATH>
Static-analysis residue (already-applied deletions removed):
- docs/poplar/audits/2026-05-03-deadcode.txt
- docs/poplar/audits/2026-05-03-unparam.txt
- docs/poplar/audits/2026-05-03-golangci.txt

Special lens for this agent: <SPECIAL_LENS>

Spec (read this first):
docs/superpowers/specs/2026-05-03-overengineering-audit-design.md

Project posture (binding):
- Pre-beta. Refactor freely. No "cross-package," "schema change,"
  "churn cost," or "out of scope" defers — those describe exactly
  the work the project posture endorses.
- Only valid skip rationales: speculative future consumer with a
  named scheduled pass; upstream-blocked; premature optimization
  without measurement.

Common checklist (run all of these against the package):
1.  Interfaces with exactly one implementation in the same package
    (Lindamood preemptive-interface anti-pattern).
2.  Unexported functions with exactly one call site (inline candidates).
3.  Struct fields never read outside their setter, or never set
    outside their constructor.
4.  Function parameters / config fields where every call site passes
    the zero value.
5.  Error returns on functions where every caller discards the error
    or routes it identically.
6.  errors.As / errors.Is paths where no caller branches on the result.
7.  Wrapper types whose only methods delegate verbatim to the wrapped
    type (Middle Man).
8.  Types declared but only used as a field of one other type and
    never constructed independently (Lazy Element).
9.  Defensive nil / length / range checks on values the call path
    guarantees.
10. Commented-out code, TODO/FIXME blocks older than 2 passes
    (use `git blame` to date them).

<UI_AGENT_EXTRA_CHECKLIST_OR_EMPTY>

Forced-deletion budget (evidence-anchored, see spec):
- Soft floor: ≥ 3 findings. Below floor, you MUST justify with
  "Nothing found because X" — no silent passes.
- Hard ceiling: 10 findings. Above ceiling, STOP after 10 and add
  a note: "Ceiling reached — escalating to Phase C for human
  triage. Additional candidates not enumerated."
- IMPORTANT: The floor must be drawn primarily from HIGH-LLM-RECALL
  categories — items 1, 2, 3, 7, 9, 10 (and items 11-17 for the UI
  agent). Items 1 (single-impl interfaces only — not Speculative
  Generality more broadly) and 8 (Lazy Element) have low LLM recall
  per EASE25; do NOT invent these to fill the floor.

Output format (one line per finding):
- file:line  — <category-number>  — <one-line description>
  Action: delete | inline | keep | refactor
  Rationale: <one sentence; if "keep," cite the consumer>

Write findings to: <OUTPUT_PATH>

Begin by reading the spec, then the static-analysis residue
filtered to your package (grep the .txt files for your package
path), then walk the package methodically. Use Read, Grep, and
Bash (read-only commands like git blame) only — do NOT use Edit
or Write on Go files. The only file you write is your output
findings doc.
```

- [ ] **Step 2: Dispatch all eight agents in one message (parallel)**

Use eight `Agent` tool calls with `subagent_type: Explore` in a
single message. The eight invocations:

| # | PKG_PATH | SPECIAL_LENS | OUTPUT_PATH |
|---|---|---|---|
| 1 | `cmd/poplar/` | "CLI seam — config touchpoints, command wiring, flag/env-var pairs that are wired but unused at the consumer." | `docs/poplar/audits/2026-05-03-pkg-cmd-poplar.md` |
| 2 | `internal/cache/` | "Cache II/III scaffolding. Pay particular attention to OpArgs sealed-sum reservations (SendArgs/AppendArgs) and outbox state-machine helpers — but verify any deletion against the 8.4c starter prompt in STATUS.md before recommending it." | `docs/poplar/audits/2026-05-03-pkg-cache.md` |
| 3 | `internal/mail/` | "Backend interface shape and classifier surface. Post-cutover (ADR-0121) the Backend interface shrunk; verify no vestigial methods remain. Check whether ChangeTracker is earning its place over its two concrete implementations." | `docs/poplar/audits/2026-05-03-pkg-mail.md` |
| 4 | `internal/mailjmap/` | "JMAP backend; provider preset code paths." | `docs/poplar/audits/2026-05-03-pkg-mailjmap.md` |
| 5 | `internal/mailimap/` | "IMAP backend; capability-negotiation paths. Vendored snippets in internal/mailauth/ are OUT OF SCOPE — do not include findings from there." | `docs/poplar/audits/2026-05-03-pkg-mailimap.md` |
| 6 | `internal/config/` | "Config decoder; provider registry; first-run flow." | `docs/poplar/audits/2026-05-03-pkg-config.md` |
| 7 | `internal/theme/`, `internal/term/`, `internal/backoff/` (combined) | "Three leaf packages combined. Verify icon-mode and palette invariants are codified once, not duplicated. Check backoff.Exponential is genuinely used by all three claimed consumers (cache drainer, JMAP push, IMAP idle)." | `docs/poplar/audits/2026-05-03-pkg-leaves.md` |
| 8 | `internal/ui/` | "Overengineering lens + bubbletea-specific items. Elm-architecture conformance is OUT OF SCOPE — that's Pass 8.5b. Use the UI extended checklist below." | `docs/poplar/audits/2026-05-03-pkg-ui.md` |

For agent 8 only, append the UI extended checklist to the brief
under `<UI_AGENT_EXTRA_CHECKLIST_OR_EMPTY>`:

```
UI extended checklist (additional items 11-17, run alongside 1-10):
11. tea.Msg types defined but never sent (grep -rn "Msg{" vs declarations).
12. tea.Cmd-returning helpers called from exactly one place
    (inline into the Update branch).
13. Defensive width/height clamps in View() that duplicate
    clipPane's self-enforcement.
14. lipgloss.NewStyle() outside the two permitted files
    (internal/ui/styles.go, internal/theme/palette.go).
15. Hex literals outside internal/theme/themes.go.
16. len() used on icon-bearing strings where displayCells is
    required (ADR-0083/0084).
17. lipgloss.JoinHorizontal / JoinVertical used in icon-bearing
    contexts (forbidden when spuaCellWidth != 1 per ADR-0084).
```

For all other agents, `<UI_AGENT_EXTRA_CHECKLIST_OR_EMPTY>` is the
empty string.

- [ ] **Step 3: Wait for all eight agents to complete**

Each agent reports back with the path to the findings file it
wrote. If any agent reports "Ceiling reached," note that package
for Phase C escalation.

- [ ] **Step 4: Verify all eight files exist and are non-empty**

```bash
ls -la docs/poplar/audits/2026-05-03-pkg-*.md
wc -l docs/poplar/audits/2026-05-03-pkg-*.md
```

Expected: eight files, each at least 10 lines (≥3 findings × ~3
lines per finding + header).

- [ ] **Step 5: Verify floor compliance**

```bash
for f in docs/poplar/audits/2026-05-03-pkg-*.md; do
  echo "=== $f ==="
  grep -c "^- " "$f" || true
done
```

Expected: each file has either ≥3 findings (lines starting with
`- `) or contains the explicit string "Nothing found because" with
a justification.

- [ ] **Step 6: Commit the eight findings docs**

```bash
git add docs/poplar/audits/2026-05-03-pkg-*.md
git commit -m "$(cat <<'EOF'
Pass 8.5 B: per-package overengineering findings

Eight per-package review agents dispatched in parallel; each
produced findings against the common checklist (and the UI
extended checklist for internal/ui/), bounded by the
evidence-anchored floor (≥3) and ceiling (10) per package.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase C — Cross-cutting re-read

### Task C1: Identify highest-yield packages

**Files:**
- Read: all eight `docs/poplar/audits/2026-05-03-pkg-*.md`

- [ ] **Step 1: Rank packages by finding count**

```bash
for f in docs/poplar/audits/2026-05-03-pkg-*.md; do
  count=$(grep -c "^- " "$f")
  echo "$count $f"
done | sort -rn
```

Expected: a sorted list. The top 2-3 are the targets for the
cross-cutting re-read.

- [ ] **Step 2: Note any ceiling-escalated packages**

```bash
grep -l "Ceiling reached" docs/poplar/audits/2026-05-03-pkg-*.md || true
```

Any matching files are mandatory targets for Phase C regardless of
their position in the count ranking.

### Task C2: Cross-cutting seam re-read

**Files:**
- Read: source files listed below
- Create: `docs/poplar/audits/2026-05-03-cross-cutting.md`

This task is performed by the user (or main-context Claude
working with the user), not a subagent. The output is a findings
file in the same format as Phase B.

- [ ] **Step 1: Read the highest-yield packages from Task C1**

For each of the top 2-3 packages, read the source files end-to-end
in dependency order (leaf-most files first). Look for cross-file
patterns the per-package agent may have missed:

- A type whose constructor and only mutator are in different files
  but every consumer is in a third file (consider relocating).
- A method on type A that only ever calls a method on type B with
  no transformation (Middle Man across files).
- Parallel structures (two files implementing similar shapes) that
  could be unified.

- [ ] **Step 2: Inspect named cross-package seams**

The spec named three specific suspects. For each, read both sides
of the seam:

**Seam 1: `mail.ChangeTracker` ↔ `mailjmap`/`mailimap` impls**

```bash
grep -n "ChangeTracker" internal/mail/*.go internal/mailjmap/*.go internal/mailimap/*.go
```

Question: does `ChangeTracker` add value over the two concrete
impls? Is the interface used as a polymorphic substrate (caller
doesn't know which backend), or is every call site already
backend-specific? If the latter, the interface is Middle Man.

**Seam 2: `mail.Backend` post-cutover (ADR-0121)**

```bash
grep -n "type Backend interface" internal/mail/*.go
```

Read the interface definition and verify every method has a real
consumer outside the implementing packages. Methods called only
within `internal/mailjmap/` or `internal/mailimap/` itself are
candidates for removal from the interface (move to concrete type).

**Seam 3: `cache.OpArgs` reserved sum members**

```bash
grep -n "SendArgs\|AppendArgs" internal/cache/*.go
```

Per ADR-0117 these are reserved but unused. Question: is there a
named pass on STATUS.md that requires them in the next 1-2 passes?
If not, they are speculative-generality and should be deleted now
(can be re-added with their consumer).

**Seam 4: `cmd/poplar/` ↔ `internal/config/` flag/env wiring**

```bash
grep -rn "cobra\|Flag\|GetString\|GetBool\|Getenv" cmd/poplar/
```

For each flag/env-var, grep for its consumer. Flags wired but never
consumed are deletion candidates.

- [ ] **Step 3: Write the cross-cutting findings doc**

Create `docs/poplar/audits/2026-05-03-cross-cutting.md` with the
header:

```markdown
# Pass 8.5 Cross-Cutting Findings

Phase C output — single-reviewer manual re-read of high-yield
packages and named seams (per
docs/superpowers/specs/2026-05-03-overengineering-audit-design.md).

## Findings
```

Then list findings in the Phase B format. Soft floor: ≥3 findings
across the entire seam set, OR an explicit "Nothing found because
X" justification.

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/audits/2026-05-03-cross-cutting.md
git commit -m "$(cat <<'EOF'
Pass 8.5 C: cross-cutting findings

Manual re-read of the highest-yield packages from Phase B and the
four named cross-package seams (ChangeTracker, Backend post-cutover,
OpArgs reservations, cmd/config wiring).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase D — Triage and apply

### Task D1: Aggregate findings into the triage doc

**Files:**
- Read: all `docs/poplar/audits/2026-05-03-pkg-*.md` and
  `docs/poplar/audits/2026-05-03-cross-cutting.md`
- Create: `docs/poplar/audits/2026-05-03-overengineering-audit.md`

- [ ] **Step 1: Build the aggregate doc**

Create `docs/poplar/audits/2026-05-03-overengineering-audit.md`
with the structure:

```markdown
# Pass 8.5 Overengineering Audit — Triage

Aggregate of Phase B (eight per-package agents) and Phase C
(cross-cutting re-read). Each finding has an apply/skip decision.

## Skip-rationale guard

Reused verbatim from /simplify (CLAUDE.md pre-beta posture). The
only valid skip rationales are:
1. Speculative future consumer with a named, scheduled pass on
   STATUS.md (not "Pass N might want this").
2. Upstream-blocked.
3. Premature optimization without measurement (efficiency only).

Forbidden skip rationales (any use of these invalidates the skip):
"cross-package," "schema change," "would require interface change,"
"churn cost," "out of scope," "non-trivial refactor."

## Findings

### internal/theme/ + internal/term/ + internal/backoff/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| ... | ... | apply | ... |
| ... | ... | skip | (valid rationale) |

### internal/mailjmap/

(same shape)

### internal/mailimap/

(same shape)

### internal/mail/

(same shape)

### internal/cache/

(same shape)

### internal/config/

(same shape)

### internal/ui/

(same shape)

### cmd/poplar/

(same shape)

### Cross-cutting

(same shape, file:line cites both packages involved)

## Summary

- Total findings: N
- Apply: A
- Skip (speculative consumer): X
- Skip (upstream-blocked): Y
- Skip (no-measurement): Z
```

For every finding from Phase B and C, copy it into the matching
package section. For each, apply the skip-rationale guard:

1. If the finding's "Action" is `delete`/`inline`/`refactor`, the
   default is **apply** unless one of the three valid skip
   rationales clearly applies.
2. If "Action" is `keep` and the rationale cites a real consumer,
   it is informational (not applied, not skipped — recorded).
3. If a skip rationale uses any forbidden phrase, force the finding
   back to **apply**.

- [ ] **Step 2: Verify the summary counts add up**

```bash
grep -c "| apply |" docs/poplar/audits/2026-05-03-overengineering-audit.md
grep -c "| skip |" docs/poplar/audits/2026-05-03-overengineering-audit.md
```

Expected: numbers match the Summary block.

- [ ] **Step 3: Commit the triage doc**

```bash
git add docs/poplar/audits/2026-05-03-overengineering-audit.md
git commit -m "$(cat <<'EOF'
Pass 8.5 D: triage doc with apply/skip decisions

Aggregate findings from Phases B and C with the /simplify
skip-rationale guard applied. Pre-beta posture forbids
"cross-package," "schema change," "churn cost," "out of scope," and
similar defers.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task D2 through D9: Apply per-package, leaf-first

Eight near-identical apply tasks. Each task: open the triage doc,
read the rows for one package, apply the changes, verify the build,
commit.

The order is leaf-first per the spec:

| # | Package(s) |
|---|---|
| D2 | `internal/theme/` + `internal/term/` + `internal/backoff/` |
| D3 | `internal/mailjmap/` |
| D4 | `internal/mailimap/` |
| D5 | `internal/mail/` |
| D6 | `internal/cache/` |
| D7 | `internal/config/` |
| D8 | `internal/ui/` |
| D9 | `cmd/poplar/` |

The template for each:

- [ ] **Step 1: Read the relevant section of the triage doc**

```bash
grep -A 100 "### <PACKAGE>" docs/poplar/audits/2026-05-03-overengineering-audit.md
```

- [ ] **Step 2: Apply each "apply" row**

For each finding marked **apply**:
1. Open the cited file.
2. Perform the action (`delete` / `inline` / `refactor`) at the
   cited line(s).
3. If a deletion cascades (other symbols become unreferenced),
   delete the cascading symbols too.

- [ ] **Step 3: Run `make check`**

```bash
make check
```

Expected: PASS. If it fails, the finding's recommended action was
wrong or had non-obvious dependencies. Either fix forward (delete
the cascading dependent) or revert this specific finding's edits
and mark the row as `skip` with rationale "non-obvious consumer
discovered at apply time" (this is a valid skip — the rationale is
a real-world signal, not one of the forbidden framings).

- [ ] **Step 4: For Task D8 (`internal/ui/`) only, capture tmux screenshots**

```bash
make install
# Launch poplar in tmux per .claude/docs/tmux-testing.md
# Capture at 80x24 and 120x40
```

Verify the UI still renders cleanly. Save captures alongside the
commit if the convention in `.claude/docs/tmux-testing.md` calls
for it.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 8.5 D-<n>: <package> overengineering deletions

Apply findings from docs/poplar/audits/2026-05-03-overengineering-audit.md
for <package>: <one-sentence summary of what changed, e.g.,
"removed 3 unused config knobs and inlined 2 single-call-site
helpers">.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Replace `<n>` with 1-8 (D2 → 1, D3 → 2, ..., D9 → 8) and
`<package>` with the package name.

If the package's apply set is empty (every row was a `skip` or
`keep`), still produce a commit message documenting that — but with
no code changes, skip the commit entirely and note in the next
package's commit message: "(D-<n> <package>: no apply rows.)"

---

## Pass-end ritual

The `poplar-pass` skill defines this; the steps below are the
concrete invocation for Pass 8.5.

### Task PE1: ADRs for architectural deletions

**Files:**
- Create: `docs/poplar/decisions/0125-*.md` and onward as needed.

- [ ] **Step 1: Identify architectural deletions**

Scan the triage doc for findings that crossed package boundaries
or removed an interface, sealed-sum member, or other invariant-
referenced symbol. Each such deletion needs an ADR.

- [ ] **Step 2: Determine starting ADR number**

```bash
ls docs/poplar/decisions/ | sort | tail -1
```

The next number is one greater than the highest existing.

- [ ] **Step 3: Write one ADR per architectural deletion**

For each, follow the existing ADR template (read any recent ADR for
the format). Title pattern: "ADR-XXXX: Remove `<thing>`". Body:
context (why it was added), decision (delete; cite the audit
finding), consequences (what changes). Reference the spec and the
audit doc.

- [ ] **Step 4: Commit ADRs**

```bash
git add docs/poplar/decisions/
git commit -m "$(cat <<'EOF'
Pass 8.5 PE1: ADRs for architectural deletions

ADR-XXXX through ADR-YYYY document the removal of <list of removed
items> per the Pass 8.5 audit findings.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task PE2: Update invariants

**Files:**
- Modify: `/home/glw907/Projects/poplar/docs/poplar/invariants.md`
- Modify (only if `internal/ui/` deletions changed binding facts):
  `/home/glw907/Projects/poplar/.claude/rules/ui-invariants.md`

- [ ] **Step 1: Read invariants.md and identify references to deleted symbols**

```bash
# For each deleted symbol/type/interface, grep:
grep -n "<deleted-symbol>" docs/poplar/invariants.md
```

- [ ] **Step 2: Edit invariants.md in place**

Per the file's own header rule: "edited in place — new facts replace
or narrow old facts, they do not append." Remove sentences/clauses
that reference deleted symbols. Update the Decision Index table at
the bottom to list the new ADR numbers.

- [ ] **Step 3: If `internal/ui/` deletions touched a UI invariant, edit `.claude/rules/ui-invariants.md`**

Same rule: edit in place. Likely candidates: changes to component
fields, removed Msg types, simplified renderers.

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/invariants.md .claude/rules/ui-invariants.md
git commit -m "$(cat <<'EOF'
Pass 8.5 PE2: update invariants for deletions

Reflect Pass 8.5 deletions in docs/poplar/invariants.md (and
.claude/rules/ui-invariants.md if UI invariants changed). Decision
Index updated to cite ADRs from PE1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task PE3: Update STATUS.md

**Files:**
- Modify: `/home/glw907/Projects/poplar/docs/poplar/STATUS.md`

- [ ] **Step 1: Mark Pass 8.5 done; confirm 8.5b and 8.4c rows**

Read `docs/poplar/STATUS.md`. The pass table currently lists 8.4b
done and 8.4c pending. Insert/update rows so the order is:

| 8.4b   | done    |
| 8.5    | **done** (this pass) |
| 8.5b   | pending |
| 8.4c   | pending |

- [ ] **Step 2: Update the "Next starter prompt" section**

The current Next starter prompt is for 8.4c. Replace it with the
8.5b starter prompt. Use this body:

```
> **Goal.** Pass 8.5b — Elm architecture conformance audit of
> internal/ui/. Verify state-in-models, mutations-only-in-Update,
> I/O-only-in-Cmd, child→parent communication via Msg only,
> shared state hoisted to root.
>
> **Scope.** internal/ui/ only. Sibling to Pass 8.5
> (overengineering audit, just shipped); operates on the slimmer
> UI surface 8.5 produced.
>
> **Settled (do not re-brainstorm):** elm-conventions skill rules.
> ADR-0023, 0035-0037, 0042, 0044, 0054, 0088 (Elm-architecture ADRs).
>
> **Approach.** Brainstorm → spec at
> docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md
> → plan at
> docs/superpowers/plans/2026-05-03-elm-conformance-audit.md → execute.
> Standard pass-end checklist applies.
```

- [ ] **Step 3: Verify STATUS.md is internally consistent**

Read the whole file. The "Current pass" line at the top should
match the "Next starter prompt" topic.

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass 8.5 PE3: STATUS — mark 8.5 done; queue 8.5b

Pass 8.5 (overengineering audit) shipped. Next starter prompt is
8.5b (Elm architecture conformance audit, internal/ui/ only).
Pass 8.4c remains queued behind 8.5b.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task PE4: Push and install

- [ ] **Step 1: Push**

```bash
git push origin master
```

Expected: clean push.

- [ ] **Step 2: Install**

```bash
make install
```

Expected: `~/.local/bin/poplar` updated. The CLI binary now reflects
all Pass 8.5 deletions.

- [ ] **Step 3: Smoke-test the binary**

```bash
poplar --help
poplar config check
```

Expected: both commands run without panic. Output matches what was
working before the audit.

If anything broke, revert the bad commit(s) and re-investigate.

---

## Self-review checklist

Before declaring Pass 8.5 done:

- [ ] All eight per-package findings docs exist and meet the floor.
- [ ] Cross-cutting findings doc exists.
- [ ] Triage doc exists with no forbidden skip rationales.
- [ ] `make check` passes on master.
- [ ] `make install` produced a working binary.
- [ ] STATUS.md, invariants.md, and ADRs reflect the current state.
- [ ] No findings doc references a symbol that no longer exists in
      the code (sanity grep: pick 5 random "apply" findings, grep
      for the symbol — should return zero matches in source).
- [ ] Pass-end commits all pushed.
