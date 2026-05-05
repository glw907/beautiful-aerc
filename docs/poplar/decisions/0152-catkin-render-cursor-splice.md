---
title: Catkin annotation splice respects the cursor block
status: accepted
date: 2026-05-05
---

## Context

Pass 9d.2 audited Catkin's annotation render path with adversarial
inputs. Two bugs surfaced.

First, `applyAnnotationsToLine` shifted the splice column right by
one cell for any annotation starting at or after the cursor byte.
The shift was added to compensate for a cursor block believed to
have been *inserted* into the styled line. `insertCursorBlock`
actually *replaces* the rune at the cursor column with `█`, so
styled and plain widths match for every in-line cursor. The shift
was off by one in every case it fired, duplicating the rune at the
splice's left edge.

Second, an annotation whose range enclosed the cursor byte spliced
its styled content over the cursor cell, erasing the block.
Spellcheck commonly flags a word the cursor sits inside, so this
hit on every keystroke into a misspelling.

Per-task review during Pass 9d had a guard test for the shift, but
its assertion only checked that the underline SGR appeared
somewhere after the cursor block — not that the underlined runes
were the right ones. The bug rendered visually wrong but the test
passed.

## Decision

The cursor block replaces (not inserts) a rune. Annotation column
math operates against `plain` directly with no cursor-aware shift.
When an annotation range encloses the cursor byte, the splice
splits around it: one splice for `[start, cursorByte)`, a second
for `[cursorByte+runeBytes, end)`. The cursor cell stays untouched.

The pass added eight adversarial tests (`render_adversarial_test.go`)
covering: cursor-row visible runes, annotation spanning the cursor,
left- and right-abutting annotations, two annotations bracketing
the cursor, annotation past a multi-byte rune, annotation across a
soft-wrap boundary, empty annotation list, and annotation at the
last column. Each asserts the rendered plain text by character, not
by SGR-presence alone. The earlier loose guard test was removed.

## Consequences

Cursor-row spellcheck and any future range annotator (TODO
highlights, link squiggles, code-action markers) render correctly
without a special case in the annotator. Annotators continue to
emit byte-offset ranges over raw source — no cursor awareness
required.

Test discipline: annotation tests assert the visible plain string,
not just SGR fragments. Loose-presence checks miss splice
miscounts. New annotation tests follow the same pattern.

The cursor-spanning case now does up to two splices per spanning
annotation. Cost is bounded by the number of visible annotations
on the cursor row, which is one or two in typical use.
