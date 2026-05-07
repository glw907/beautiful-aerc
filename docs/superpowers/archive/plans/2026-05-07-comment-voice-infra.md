# Pass 9j — Comment voice infrastructure

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tighten the upstream rules and downstream gates that
govern Go comment voice, so Claude writes better first-pass
comments and the remaining slips are caught mechanically. Land
the SPDX-header policy decision in the same pass. No code-logic
changes. **The actual file-by-file sweep of existing comments
across `internal/` is Pass 9k**, which runs against the gates
this pass installs.

**Background.** A comment audit against three reference TUI apps
(glow, gh-dash, k9s) found three structural divergences in
poplar's commenting:

1. **Frequency** — 13.7% comment-line ratio across `internal/`,
   roughly double idiomatic Go application code. The project's
   own policy ("comments default to none; skip godoc on
   unexported symbols unless the doc adds information beyond the
   name") is not being followed at write-time.
2. **Location** — 1,227 in-function comments cluster at
   structural seams (top of a `for`, before a transformation
   step) and paraphrase the next 3–5 lines. Humans comment
   where understanding *fails*; the codebase comments where
   structure *changes*.
3. **Shape** — godoc carries label-colon paragraphs
   (`Picker list:` / `Footnote section:`), `NOTE:` / `IMPORTANT:`
   prefixes, closing aphoristic summary sentences, and inline
   reference-stuffing (multiple ADR cites per comment). This is
   markdown/blog voice, not Go-stdlib voice.

The existing `make check` voice-grep + `/simplify` voice lens are
**detective** controls — they catch tells after writing. The
structural issues above don't grep cleanly, so reactive
enforcement misses them. The pass adds **generative** controls
(write-time rules in `~/.claude/docs/go-comment-voice.md`) and
extends the detective controls for what does grep.

**Working directory:** master branch (per project convention; no
worktree).

**Spec:** none — the brainstorm + design lives in the conversation
that produced this plan; the audit findings (frequency 13.7%,
1,227 in-fn comments, 235 SPDX headers across 154 non-test files)
and the option-2-vs-3 license analysis are codified in the ADRs
this pass writes.

---

## File map

**New files:**
- `docs/poplar/decisions/0168-comment-voice-generative-rules.md` —
  ADR for the structural-tell additions to the voice doc + the
  decision rubric.
- `docs/poplar/decisions/0169-spdx-header-removal.md` — ADR for
  Option 3 (drop SPDX from all `.go` files; rely on `LICENSE`).

**Modified files:**
- `~/.claude/docs/go-comment-voice.md` — new §0 decision rubric,
  new §N structural tells (T38 frequency, T39 location, T40
  shape), 4–6 calibrated good/bad examples drawn from the audit.
- `~/.claude/skills/go-conventions/SKILL.md` (or equivalent) —
  one-line pointer to §0 of the voice doc, plus the
  paraphrase-test rule.
- `scripts/voice-check.sh` — `scan` calls for T39 (label-colon
  godoc), T40 (`NOTE:` / `IMPORTANT:` / `TODO:` prefix), T41
  (SPDX header — installs *after* the sweep so it stays clean).
- `~/.claude/skills/simplify/SKILL.md` — Agent-4 (voice lens)
  prompt picks up the structural tells, with the
  paraphrase-test as a primary check.
- All `.go` files under `cmd/` and `internal/` — strip SPDX
  header (`// SPDX-License-Identifier: MIT` + the blank line
  beneath it). Vendored-provenance blocks (e.g.
  `internal/ui/uicore/overlay.go`, `internal/mailauth/xoauth2.go`,
  `internal/mailauth/keepalive/`) keep their existing
  source-attribution comment; only the SPDX line goes.
- `docs/poplar/STATUS.md` — Pass 9j row + 9k starter prompt.
- `docs/poplar/invariants.md` — decision-index entries for ADR
  0168 + 0169.

---

## Tasks

### Generative (write-time rules)

- [ ] **T1.** Draft `~/.claude/docs/go-comment-voice.md` §0
      *Comment-or-not decision rubric*. Three questions, one
      mechanical test:
      - (a) Does the function/type name already say this?
      - (b) Is the why obvious from the next ≤5 lines?
      - (c) Would a reader otherwise miss a hidden constraint,
        invariant, or surprising consequence?
      - **Skip rule:** if (a) or (b) → don't write the comment.
      - **Write rule:** only when (c).
      - **Mechanical test:** *if the comment paraphrases the
        next ≤5 lines, delete it.*

- [ ] **T2.** Add structural-tell entries to the voice
      catalogue. Numbering continues from T37:
      - **T38 — Comment frequency.** Comment-line ratio over a
        single file or function should track idiomatic Go
        application code (~7–9%), not stdlib library code
        (~15%). Symptom: every helper carries a godoc; every
        `for` block has a preamble.
      - **T39 — Section-boundary commenting.** Comments placed
        at structural seams (top of loop, before a
        transformation) restating what the next lines do.
        Replace with a comment on the surprising bit, or delete.
      - **T40 — Markdown shape leaking into godoc.** Label-colon
        paragraphs (`Picker list:`), `NOTE:` / `IMPORTANT:` /
        `TODO:` prefixes, closing aphoristic summary sentences,
        reference-stuffing (>1 ADR or RFC cite per godoc).
        Idiomatic Go godoc is prose paragraphs.

- [ ] **T3.** Curate 4–6 calibrated good/bad pairs in the voice
      doc, drawn from the real audit:
      - `messagelist/model.go:272` — `matchMessage` 5-line godoc
        on a 14-line obvious function (frequency).
      - `render_footnote.go:42` — section-boundary inline
        comment paraphrasing the loop below it (location).
      - `render_footnote.go:20` — label-colon godoc structure
        with closing summary sentence + 3 ADR cites (shape).
      - `mail/types.go:14,27` — `Flag represents…` /
        `Folder represents…` restating the type name (T4 +
        frequency).
      - One *good* example: a comment that flags a non-obvious
        invariant the reader would otherwise miss
        (`drainer.go:33` "5s is a comfortable floor" rationale
        is a candidate).

- [ ] **T4.** Update the `go-conventions` skill: one-line
      pointer to the new §0 decision rubric and the
      paraphrase-test. Also re-state the rule that godoc on
      unexported symbols is opt-in (Google "unobvious" bar),
      not opt-out.

### Detective (commit-gate)

- [ ] **T5.** Extend `scripts/voice-check.sh` with three new
      scans, calibrated to zero false-positives on the current
      tree (calibrate by running each pattern *before* enabling
      its `scan` call):
      - **T39 (label-colon godoc).** Pattern target:
        `^// [A-Z][a-zA-Z]+: ` — godoc lines opening with a
        capitalized word followed by `:`. Note: this is
        narrower than T35 (which already covered some doc
        labels) — calibrate against the post-T35-cleanup tree.
      - **T40a (NOTE:/IMPORTANT:/TODO: prefix).** Pattern
        target: `^\s*// (NOTE|IMPORTANT|TODO):`. Idiomatic Go
        uses `// TODO(user):` for tracked TODOs and prose for
        notes.
      - **T41 (SPDX header).** Pattern target:
        `^// SPDX-License-Identifier:`. Installs *after* the
        sweep in T8 so the gate stays green.

- [ ] **T6.** Update the `/simplify` skill's voice-lens agent
      (Agent 4) prompt: add the three structural tells (T38
      frequency, T39 location, T40 shape) and the
      paraphrase-test as a primary check. Keep the existing
      word-level catalogue checks intact. Calibrate the prompt
      so it doesn't fire on legitimate godoc — the bar is
      *paraphrasing the next 5 lines*, not *summarizing a
      package*.

### License

- [ ] **T7.** Write `decisions/0169-spdx-header-removal.md`.
      Codify Option 3: SPDX line removed from all `.go` files;
      `LICENSE` at repo root is canonical. Document the carve-
      out: vendored files keep their existing source-attribution
      comment block (which names repo + commit + license — more
      informative than SPDX). Reference the three peer-app
      precedents (glow, gh-dash, k9s — none carry SPDX).

- [ ] **T8.** Sweep `cmd/` and `internal/` to remove SPDX
      headers. Mechanical: for each `.go` file, delete the
      first line if it matches `^// SPDX-License-Identifier:`,
      and delete the immediately-following blank line. Confirm
      vendored files (`uicore/overlay.go`, `mailauth/xoauth2.go`,
      `mailauth/keepalive/*.go`) retain their provenance
      comment block. Run `make check` after the sweep — must be
      green before T5's T41 scan goes live.

### Pass-end

- [ ] **T9.** Write `decisions/0168-comment-voice-generative-
      rules.md`. Codify the §0 rubric, the three structural
      tells (T38–T40), and the generative-vs-detective control
      split. Cite the audit findings (13.7% ratio, 1,227 in-fn
      comments, three concrete shape examples) as the
      motivation. Reference ADR-0141 (the parent voice policy)
      and ADR-0148 (the prior voice-grep gate) as the chain
      this extends.

- [ ] **T10.** Pass-end ritual via `poplar-pass`: update
      `STATUS.md` (mark 9j done; set 9k as next with a starter
      prompt for the file-by-file comment sweep), update
      `invariants.md` decision index for ADR 0168 + 0169,
      archive this plan to `docs/superpowers/archive/plans/`,
      run `/simplify` on the diff (the new agent prompt eats
      its own dogfood here), commit, push, `make install`.

---

## What's deliberately out of scope

- **The cleanup sweep itself.** Pass 9k handles file-by-file
  rewriting of existing godocs and in-function comments against
  the new rubric. Doing it in this pass would balloon past the
  8–12 task budget (53k LOC of comment auditing is unbounded
  once you start finding patterns). Pass 9k runs with the new
  gates already live, so each commit lands clean.
- **Docstring policy on test files.** Test files have their own
  voice (often more tutorial). Defer until 9k surfaces a real
  signal that tests are off-key.
- **CLAUDE.md or `invariants.md` rewrites of the comment
  policy.** The policy line in CLAUDE.md ("comments default to
  none; WHY-comments only when the why is non-obvious") is
  already correct. The expansion lives in the binding
  artifact (the voice doc) per ADR-0141.

## Acceptance

- `make check` is green with the three new `scan` calls live.
- `~/.claude/docs/go-comment-voice.md` carries §0, T38–T40, and
  4–6 good/bad pairs, all drawn from real poplar files.
- `go-conventions` and `/simplify` pick up the new rules.
- Zero `// SPDX-License-Identifier:` lines remain under `cmd/`
  or `internal/`.
- Two ADRs land (0168 + 0169), indexed in `invariants.md`.
- STATUS.md shows 9j done and 9k pending with a starter prompt.
