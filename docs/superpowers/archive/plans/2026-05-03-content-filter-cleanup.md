# Pass 8.5d — content/filter cleanup

**Date:** 2026-05-03
**Status:** active
**Spec:** `docs/superpowers/specs/2026-05-03-content-filter-cleanup.md`

## State of the spec

Re-reading the spec against the tree shows #23 (HTML word fusing) is
**already fixed** in commit `9174f85` (2026-05-02) — `prepareHTML`
calls `inlineBoundaryPad`, and tests `TestCleanHTML_InlineBoundaryFusion`
+ `TestCleanHTML_FusionFixtures` cover the spec's two fixtures
(`safari-update-fragment.html`, `dave-johnson-fragment.html`). Tests
green. BACKLOG/STATUS entries are stale.

So this pass collapses to **#13 only** plus closing the stale #23
entry.

## #13 — drop dead `blockKind` / `spanKind` enums

### Decisions (settled)

- **Naming:** `isBlock()` / `isSpan()` (no return). Matches the Go
  sealed-sum convention (e.g. `go/ast` `exprNode()`, `stmtNode()`).
  Keeping the original names with `()` collisions adds no value.
- **Test rewrite:** swap `[]blockKind` for `[]string` of type names
  produced via `fmt.Sprintf("%T", b)` (returns
  `content.Paragraph` etc.). Reads naturally; no extra helper.

### Steps

1. `internal/content/blocks.go`
   - Drop `blockKind`, `spanKind` types and their const blocks.
   - Change `Block` interface marker to `isBlock()`; `Span` to
     `isSpan()`.
   - Rewrite the nine `blockType()` methods and five `spanType()`
     methods as no-arg, no-return markers.
2. `internal/content/parse_test.go`
   - Replace `types []blockKind` → `types []string`.
   - Replace each `kindFoo` literal with the corresponding type name
     string (`"content.Paragraph"`, …).
   - Replace `b.blockType() != tt.types[i]` checks with
     `fmt.Sprintf("%T", b) != tt.types[i]`.
3. Run `make check`; fix any fallout.

### Pass-end ritual

- ADR for the marker simplification (one ADR, ties to invariants
  via the small footnote that `Block`/`Span` are sealed sums; no
  invariant change needed since the existing invariants don't name
  the kind enums).
- BACKLOG: close #13, close #23 (note the latter was actually fixed
  in 9174f85; just hadn't been ticked).
- Update `docs/poplar/STATUS.md` — mark 8.5d done; queue next pass
  (8.4c — Cache III).
- Archive plan + spec.
- `make check` → commit → push → install.
