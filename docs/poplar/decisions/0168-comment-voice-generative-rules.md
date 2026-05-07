---
title: Generative rules for comment voice (§0 rubric, T38–T40)
status: accepted
date: 2026-05-07
---

## Context

ADR-0141 named the comment-voice policy and pointed
`~/.claude/docs/go-comment-voice.md` at it as the binding artifact.
ADR-0142 and ADR-0148 added the grep tier of the detective control:
`scripts/voice-check.sh` now catches T4, T10, T14, T16, T27, T28,
T33–T35 mechanically. The `/simplify` voice lens covers the
remaining semantic tells.

A comment audit against three reference TUI apps (glow, gh-dash,
k9s) and the current poplar tree found three structural divergences
the existing controls do not catch:

1. **Frequency.** Comment-line ratio across `internal/` is 13.7%,
   roughly double idiomatic Go application code (~7–9%, with
   stdlib library code closer to ~15%). Every helper carries a
   godoc; every loop has a preamble. The project's own policy
   ("comments default to none; skip godoc on unexported symbols
   unless the doc adds information beyond the name") is not
   followed at write-time.
2. **Location.** 1,227 in-function comments cluster at structural
   seams — top of a `for`, before a transformation step — and
   paraphrase the next 3–5 lines. Humans comment where
   understanding *fails*; the codebase comments where structure
   *changes*.
3. **Shape.** Godocs carry label-colon paragraphs (`Picker list:`,
   `Footnote section:`), `NOTE:` / `IMPORTANT:` / `TODO:` prefixes,
   closing aphoristic summary sentences, and reference-stuffing
   (multiple ADR cites per comment). This is markdown/blog voice,
   not Go-stdlib voice.

The grep tier and the `/simplify` lens are **detective** controls —
they catch tells after writing. The structural issues above don't
grep cleanly, so reactive enforcement misses them. The fix is
**generative** controls: write-time rules upstream of the keyboard.

## Decision

Add generative controls to the voice doc and downstream consumers.

**§0 Comment-or-not decision rubric.** A three-question gate that
runs before the existing §1 placement rubric:

- (a) Does the function/type name already say this?
- (b) Is the why obvious from the next ≤5 lines?
- (c) Would a reader otherwise miss a hidden constraint, invariant,
  or surprising consequence?

Skip rule: if (a) or (b), don't write the comment. Write rule: only
when (c). Mechanical test: *if the comment paraphrases the next
≤5 lines, delete it.* The paraphrase test is the primary check
because it operationalizes the location finding above.

**Three new structural tells.** Numbered T38–T40 in the catalogue:

- **T38 — Comment frequency.** Comment-line ratio over a single
  file or function should track idiomatic Go application code
  (~7–9%), not library code (~15%). Symptom: every helper carries
  a godoc; every `for` block has a preamble.
- **T39 — Section-boundary commenting.** Comments placed at
  structural seams (top of loop, before a transformation)
  restating what the next lines do. Replace with a comment on the
  surprising bit, or delete.
- **T40 — Markdown shape leaking into godoc.** Label-colon
  paragraphs (`Picker list:`), `NOTE:` / `IMPORTANT:` / `TODO:`
  prefixes, closing aphoristic summary sentences,
  reference-stuffing (>1 ADR or RFC cite per godoc). Idiomatic Go
  godoc is prose paragraphs.

**Calibrated examples** drawn from the audit (4–6 good/bad pairs
in §7) so the rules have concrete poplar-shaped illustrations.

**Detective extensions.** `scripts/voice-check.sh` picks up the
greppable subset:

- T39 catches the label-colon godoc shape (`^// [A-Z][a-zA-Z]+: `).
- T40a catches the `NOTE:` / `IMPORTANT:` / `TODO:` prefix.

T38 (frequency) and the broader T39 (paraphrase comments at loop
boundaries) cannot grep cleanly without false positives — they
stay in the `/simplify` Agent-4 voice lens, which gains the
paraphrase test as a primary check.

The `go-conventions` skill points at §0 inline so the rubric
applies before any Go file is written, not just during review.

## Consequences

- Pass 9j installs the rules and gates only. The file-by-file
  sweep across `internal/` (~53k LOC) is Pass 9k, which runs with
  the new gates already live so each commit lands clean.
- The new grep scans (T39, T40a) are calibrated zero-FP on the
  current tree and activate immediately. Future drift trips the
  gate at commit time.
- The `/simplify` voice lens gains the paraphrase test as a
  primary check. Calibrated to fire on comments that paraphrase
  the next ≤5 lines, not on legitimate package summaries.
- The generative-vs-detective split is now explicit: §0 (write-
  time rubric) and the doc-shape rules are upstream; the grep
  scans and the voice-lens agent are downstream. Both layers
  needed because neither alone catches all three findings.
- T38–T40 extend the catalogue introduced by ADR-0141 and
  expanded by ADR-0148. ADR-0148 is the canonical detective-tier
  reference; this ADR is the canonical generative-tier reference.
