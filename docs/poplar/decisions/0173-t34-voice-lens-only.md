---
title: T34 (semicolon clause-joiner) demoted to voice-lens
status: accepted
date: 2026-05-07
---

## Context

T34 (semicolon clause-joiner in Go comments) was added to the
mechanical commit-gate `scripts/voice-check.sh` during Pass 9j as a
calibrated zero-FP signature of AI-generated prose. Through Pass
9k.2 it caught the regression pattern explicitly called out in the
9k.3 starter prompt.

Pass 9k.3 found a calibration miss. The mechanical gate was
rejecting legitimate uses where two tightly-related clauses read
better as one sentence with a semicolon than as two staccato
sentences. Re-writes to satisfy the gate produced choppier prose,
which is itself a (different) AI tell. The gate was creating the
problem it was meant to prevent.

T33 (em dash) and T39/T40 (godoc shape) stay mechanical — they
don't have the same legitimate-use slope.

## Decision

Demote T34 from mechanical commit-gate to `/simplify` voice-lens
semantic check. `scripts/voice-check.sh` no longer scans for it;
its position in the script carries a comment explaining the
demotion. The catalogue entry at `~/.claude/docs/go-comment-voice.md`
gains a `Status:` field marking it voice-lens-only and rewrites the
avoidance rule to "default to a period, but a considered semicolon
is fine."

A second principle is codified at the same time: the voice rules
apply to *all* Claude-authored docs, not only Go source. The
catalogue, ADRs, plan docs, skills, and any other prose follow the
same standard. Reinforces the habit; prevents the AI-prose voice
from leaking back through docs into code.

## Consequences

- Reviews still flag overuse — three semicolons in a paragraph is
  still the chorus that needs rewriting.
- Pass 9k.3 lands without the regression-fight cycle that 9k.2 had.
  Density numbers tracked across the slice (uicore 28.1% → 17.6%,
  messagelist 19.6% → 12.6%, account 15.6% → 11.1%, reader 14.2%
  → 10.8%, App layer 11.4% → 9.6%) show the rule wasn't carrying
  the density work; trimming paraphrase godocs and label-colon
  shapes was.
- Future passes that author large skill or doc bodies inherit the
  voice rules without needing a separate framing. The "this also
  applies to docs" line lives at the bottom of T34 in the
  catalogue.
