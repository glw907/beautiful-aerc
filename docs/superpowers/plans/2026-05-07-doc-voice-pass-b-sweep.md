# Doc voice — Pass B: drain the backlog

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** With the Vale gate live from Pass A, widen `VALE_PATHS` to the rest of the in-scope tree and clean each tree of error-severity findings. Trim the global tell catalogue. Update the `/simplify` voice agent brief. Sweep the poplar-related global skills.

**Architecture:** No tooling changes. Each task widens `VALE_PATHS` by one entry, sweeps the newly-covered tree, and commits. The last two tasks reach outside the repo to the global `~/.claude/` files that are voice-load-bearing for poplar but not part of `make check`.

**Tech Stack:** Vale, Make, bash, markdown.

**Spec:** `docs/superpowers/specs/2026-05-07-doc-voice-design.md`

**Predecessor:** `2026-05-07-doc-voice-pass-a-gate.md`. Pass A must be merged before this one starts (`docs/poplar/STYLE.md`, the Vale ruleset, ADR-0174, and the narrow `make check` gate are prerequisites).

---

## Files

**Modify:**
- `Makefile` (widen `VALE_PATHS` once per sweep task)
- `CLAUDE.md`, `.claude/rules/*.md`, `.claude/skills/poplar-pass/*.md`, `.claude/docs/*.md`
- `docs/poplar/decisions/*.md`
- `docs/superpowers/plans/*.md`, `docs/superpowers/specs/*.md`
- `~/.claude/docs/go-comment-voice.md` (trim catalogue, cross-reference STYLE.md)
- Global poplar-related skill files (locate via `find`)
- The `/simplify` skill's voice review agent brief

**No deletes, no creates.** The catalogue trim is an in-place edit.

---

## Task 1: Widen to .claude/ + CLAUDE.md; sweep

**Files:**
- Modify: `Makefile`
- Modify: `CLAUDE.md`, `.claude/rules/*.md`, `.claude/skills/poplar-pass/*.md`, `.claude/docs/*.md` (per Vale findings)

- [ ] **Step 1: Widen VALE_PATHS in Makefile**

Old:
```make
VALE_PATHS := docs/poplar internal cmd
```

New:
```make
VALE_PATHS := docs/poplar internal cmd \
              CLAUDE.md .claude/rules .claude/skills .claude/docs
```

- [ ] **Step 2: Run Vale, capture findings**

```bash
vale --minAlertLevel=error --output=line CLAUDE.md .claude/rules .claude/skills .claude/docs > /tmp/vale-claude.txt 2>&1 || true
cat /tmp/vale-claude.txt
```

- [ ] **Step 3: Add a CLAUDE.md pointer to STYLE.md**

In the existing "Human voice" section of `CLAUDE.md`, append one paragraph:

```markdown
The `docs/poplar/STYLE.md` guide governs prose voice across the
repo: invariants, ADRs, plans, specs, this file. Vale enforces
the mechanical floor (`make check` runs `vale`). STYLE.md is the
human-readable companion. New tells log to STYLE.md §4 and add
a Vale rule in the same commit. ADR-0174.
```

Verify the new paragraph passes Vale (re-read with the catalogue in mind; expected clean).

- [ ] **Step 4: Fix findings file by file**

Open each flagged file. Apply edits per the standard triage:

- Em dash → period, comma, parens, or colon.
- Padding adjective → swap per the rule's `swap:` table or drop.
- Ensure → rephrase as the action.
- Signposting → drop the signpost.
- Label-colon godoc opener in prose → rephrase as a sentence.

After each file, verify clean:

```bash
vale --minAlertLevel=error <file>
```

- [ ] **Step 5: Verify the new VALE_PATHS scope is clean**

```bash
make vale
```

Expected: exit 0, no findings.

- [ ] **Step 6: Run full make check**

```bash
make check
```

Expected: green.

- [ ] **Step 7: Commit**

```bash
git add Makefile CLAUDE.md .claude/
git commit -m "$(cat <<'EOF'
Widen VALE_PATHS to .claude/ and CLAUDE.md; sweep

CLAUDE.md gains a pointer to docs/poplar/STYLE.md under the
Human voice section. Errors fixed across CLAUDE.md,
.claude/rules/, .claude/skills/, .claude/docs/.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Widen to docs/poplar/decisions/; sweep ADRs

ADRs are immutable in spirit. Edits remove tells only; we do not rewrite voice or hedge decisions.

**Files:**
- Modify: `Makefile`
- Modify: `docs/poplar/decisions/*.md` (per Vale findings, errors only)

- [ ] **Step 1: Widen VALE_PATHS**

Old:
```make
VALE_PATHS := docs/poplar internal cmd \
              CLAUDE.md .claude/rules .claude/skills .claude/docs
```

New:
```make
VALE_PATHS := docs/poplar internal cmd \
              CLAUDE.md .claude/rules .claude/skills .claude/docs \
              docs/poplar/decisions
```

(Note: `docs/poplar` already covers `docs/poplar/decisions/` via path inclusion. The explicit entry exists for documentation; verify Vale's recursion is enabled. If `vale docs/poplar` already scans `decisions/`, drop the explicit entry. The `make vale` step in Pass A may have already scanned ADRs; if so, this task's findings should be small.)

```bash
vale --minAlertLevel=error docs/poplar/decisions/ | head -5
```

If output is empty, decisions/ was already swept by Pass A's `vale docs/poplar`. Skip to Step 5 and commit only the Makefile cleanup.

- [ ] **Step 2: Run Vale, capture findings**

```bash
vale --minAlertLevel=error --output=line docs/poplar/decisions/ > /tmp/vale-adrs.txt 2>&1 || true
cat /tmp/vale-adrs.txt
```

- [ ] **Step 3: Fix em dashes, padding adjectives, signposting only**

Conservative edits. Do *not* edit ADR decisions, rationale, or structure. If a finding requires more than a punctuation/word swap, leave it (warning level deferred).

- Em dashes → period, comma, parens, or colon.
- Padding adjectives → swap or drop, preserving the claim.
- Signposting → drop.
- Ensure → rephrase.

- [ ] **Step 4: Verify clean**

```bash
vale --minAlertLevel=error docs/poplar/decisions/
```

Expected: no findings.

- [ ] **Step 5: Run full make check**

```bash
make check
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add Makefile docs/poplar/decisions/
git commit -m "$(cat <<'EOF'
Sweep ADRs for voice errors

Em dashes, padding adjectives, signposting removed across
docs/poplar/decisions/. Decisions and rationale unchanged.
Warnings deferred per the spec's cleanup scope.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Widen to docs/superpowers/; sweep active plans and specs

Skip `docs/superpowers/archive/` (archived plans are immutable).

**Files:**
- Modify: `Makefile`
- Modify: `docs/superpowers/plans/*.md`, `docs/superpowers/specs/*.md` (per Vale findings)

- [ ] **Step 1: Widen VALE_PATHS**

```make
VALE_PATHS := docs/poplar internal cmd \
              CLAUDE.md .claude/rules .claude/skills .claude/docs \
              docs/superpowers/plans docs/superpowers/specs
```

(Drop the explicit `docs/poplar/decisions` if Step 1 of Task 2 confirmed redundancy. Keep otherwise.)

- [ ] **Step 2: Run Vale, capture findings**

```bash
vale --minAlertLevel=error --output=line docs/superpowers/plans docs/superpowers/specs > /tmp/vale-superpowers.txt 2>&1 || true
cat /tmp/vale-superpowers.txt
```

The doc-voice spec, Pass A plan, and Pass B plan should already be clean from authoring. The other active plans (Pass 9-series, etc.) are likely the bulk of findings.

- [ ] **Step 3: Fix findings file by file**

Same triage as Task 1.

- [ ] **Step 4: Verify clean**

```bash
vale --minAlertLevel=error docs/superpowers/plans docs/superpowers/specs
```

Expected: no findings.

- [ ] **Step 5: Run full make check**

```bash
make check
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add Makefile docs/superpowers/
git commit -m "$(cat <<'EOF'
Widen VALE_PATHS to docs/superpowers/; sweep active plans/specs

Em dashes and signposting removed across the active superpowers
tree. Archived plans untouched.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Sweep global poplar-related skills and trim go-comment-voice.md

The voice-load-bearing surfaces outside the poplar repo: `~/.claude/docs/go-comment-voice.md` (catalogue trim) and the global skills that Claude invokes during poplar work (`go-conventions`, `elm-conventions`, `poplar-pass`, `simplify`).

These files don't sit in `make check` (they're outside the repo) but they prime every poplar conversation. Voice tells in their prose drift into Claude's drafts before any review can catch it.

**Files:**
- Modify: `~/.claude/docs/go-comment-voice.md` (trim §7 catalogue)
- Modify: global skill SKILL.md files (locate via `find`; sweep errors)

- [ ] **Step 1: Locate the global poplar-related skill files**

```bash
find ~/.claude -type f -name '*.md' \( \
    -path '*go-conventions*' -o \
    -path '*elm-conventions*' -o \
    -path '*poplar-pass*' -o \
    -path '*simplify*' \
  \) 2>/dev/null
```

Note the paths. There may be both global (`~/.claude/skills/...`) and plugin-cached (`~/.claude/plugins/...`) copies; the active versions are the ones the harness loads (typically the canonical user-level skill, not the cache). When in doubt, edit both and let the next sync settle them.

- [ ] **Step 2: Run Vale on each skill file**

```bash
for f in $(find ~/.claude -type f -name 'SKILL.md' \( \
    -path '*go-conventions*' -o \
    -path '*elm-conventions*' -o \
    -path '*poplar-pass*' -o \
    -path '*simplify*' \
  \) 2>/dev/null); do
  echo "=== $f ==="
  vale --minAlertLevel=error "$f" || true
done
```

Capture findings. Triage as in earlier tasks.

- [ ] **Step 3: Fix findings in each skill file**

Apply the same triage rules. Skill files are imperative-genre — terse commands, no narrative. Most findings will be em dashes (T16/T35 in prose form) or signposting that crept into rule explanations.

- [ ] **Step 4: Verify each skill file is clean**

```bash
for f in $(find ~/.claude -type f -name 'SKILL.md' \( \
    -path '*go-conventions*' -o \
    -path '*elm-conventions*' -o \
    -path '*poplar-pass*' -o \
    -path '*simplify*' \
  \) 2>/dev/null); do
  vale --minAlertLevel=error "$f" || echo "FAIL: $f"
done
```

Expected: no `FAIL:` output.

- [ ] **Step 5: Trim the catalogue from `~/.claude/docs/go-comment-voice.md`**

Locate §7:

```bash
grep -n '^## §7' ~/.claude/docs/go-comment-voice.md
grep -n '^## ' ~/.claude/docs/go-comment-voice.md
```

Identify the §7 boundary (start to next `^## ` heading or EOF). Replace the entire §7 catalogue with a cross-reference paragraph:

```markdown
## §7 The tell catalogue

The 32-tell catalogue lives in `docs/poplar/STYLE.md §4` (in the
poplar repo) and as Vale rules in `.vale/styles/Poplar/AITells/`
and `.vale/styles/Poplar/ProseTells/`. STYLE.md is the
human-readable face. Vale is the gate. New tells log to STYLE.md
and add a Vale rule in the same commit.

This file (go-comment-voice.md) keeps the positive palette for
Go comments. The negative space (what to avoid) is one source of
truth, no longer duplicated here.

See poplar ADR-0174 for the policy extension.
```

- [ ] **Step 6: Verify go-comment-voice.md passes Vale**

```bash
vale --minAlertLevel=error ~/.claude/docs/go-comment-voice.md
```

Expected: no findings (or only findings inside fenced quoted-tell examples, which are legitimate per the allowlist mechanism).

- [ ] **Step 7: Commit the global file edits to ~/.dotfiles**

The user's global `~/.claude/` is tracked via Stow in `~/.dotfiles`. Commit there:

```bash
cd ~/.dotfiles
git status
```

Inspect which files changed. Stage and commit:

```bash
git add claude/
git commit -m "$(cat <<'EOF'
Sweep poplar voice-load-bearing global files

Trim the §7 catalogue from claude/.claude/docs/go-comment-voice.md
in favor of the single source of truth at poplar's
docs/poplar/STYLE.md §4 plus the Vale ruleset.

Sweep poplar-related skills (go-conventions, elm-conventions,
poplar-pass, simplify) for em dashes, padding adjectives,
signposting. Skills prime every poplar conversation; voice tells
in their prose drift into drafts before review can catch them.

See poplar ADR-0174.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
cd -
```

If `~/.dotfiles` does not track these specific files (some plugin-cached copies are not Stow-managed), edit in place and skip the commit; document the manual edits in STATUS.md so the next sync surfaces them.

---

## Task 5: Update /simplify voice agent brief; pass-end consolidation

**Files:**
- Modify: the `/simplify` skill's voice review agent file (located in Task 4)
- Modify: `docs/poplar/STATUS.md` (pass-end)

- [ ] **Step 1: Locate the /simplify voice agent definition**

`/simplify` runs four review agents in parallel; the fourth is the voice lens. The agent's brief lives inside the skill's body or in a referenced agent definition file.

```bash
find ~/.claude -path '*simplify*' -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.json' \) 2>/dev/null
```

Identify the file that defines the voice agent's prompt or brief.

- [ ] **Step 2: Update the voice agent brief**

Add this paragraph (or equivalent phrasing if the brief is structured differently) to the voice agent's instructions:

```markdown
For Markdown changes in the diff, run `vale --output=JSON` on
each changed file and treat findings as the mechanical floor.
The squishy layer (sentence rhythm, false confidence,
genre-mismatched voice) is yours to assess against
`docs/poplar/STYLE.md` §1 and §2. Vale findings are the floor,
not the ceiling.

The shared catalogue lives at `docs/poplar/STYLE.md §4` and as
Vale rules in `.vale/styles/Poplar/`. When you flag a
mechanical tell that Vale missed, log it to STYLE.md §4 and
add a Vale rule in the same commit (this is part of the diff,
not a follow-up).
```

- [ ] **Step 3: Commit the /simplify update**

```bash
cd ~/.dotfiles
git add claude/
git commit -m "$(cat <<'EOF'
/simplify voice agent: consume Vale findings

The voice review agent now treats Vale's diff scan as the
mechanical floor and STYLE.md §1–§2 as the squishy layer it
assesses on its own. New mechanical tells the agent catches
ship as STYLE.md row + Vale rule in the same commit.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
cd -
```

If the /simplify skill is plugin-cached and not in `~/.dotfiles`, edit in place and document in STATUS.md.

- [ ] **Step 4: Run the full check gate**

```bash
make check
```

Expected: `fmt-check`, `vet`, `vale`, `test` all pass. Exit 0.

- [ ] **Step 5: Run vale-test**

```bash
make vale-test
```

Expected: `checked 17 fixtures, 0 failures`.

- [ ] **Step 6: Update STATUS.md**

Mark Pass B complete. The doc-voice initiative spans Pass A + Pass B; STATUS reflects both passes landing.

- [ ] **Step 7: Push**

```bash
git push
cd ~/.dotfiles && git push && cd -
```

- [ ] **Step 8: Install**

```bash
make install
which poplar && poplar --version
```

- [ ] **Step 9: Pass-end commit**

```bash
git add docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass B complete: doc voice backlog drained

VALE_PATHS widened to the full in-scope tree (.claude/,
decisions/, superpowers/). Catalogue trimmed from
go-comment-voice.md. Poplar-related global skills swept.
/simplify voice agent brief updated to consume Vale findings.

Doc voice initiative complete. ADR-0174 records the policy.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
git push
```

---

## Self-review checks

- [ ] `make check` exits 0 with the full `VALE_PATHS`.
- [ ] `vale --minAlertLevel=error` finds nothing across the in-scope tree.
- [ ] `~/.claude/docs/go-comment-voice.md` §7 cross-references STYLE.md (no inline catalogue).
- [ ] Global poplar-related skill files (`go-conventions`, `elm-conventions`, `poplar-pass`, `simplify`) pass Vale at error level.
- [ ] `/simplify` voice agent brief mentions Vale.
- [ ] `STATUS.md` reflects Pass B complete and the doc-voice initiative closed.
- [ ] `~/.dotfiles` commits pushed (where applicable).

If any check fails, return to the relevant task.
