# Pass 9d.2a — Invariants compaction plan

Spec: `docs/superpowers/specs/2026-05-05-invariants-compaction-design.md`.

## Steps

1. **Create `.claude/rules/cache-invariants.md`.** Frontmatter
   paths: `internal/cache/**/*.go`, `cmd/poplar/cache*.go`,
   `docs/superpowers/plans/**/*.md`, `docs/superpowers/specs/**/*.md`.
   Body = the entire `## Cache` section from `invariants.md`,
   verbatim, with a one-line preamble.

2. **Create `.claude/rules/catkin-invariants.md`.** Frontmatter
   paths: `internal/catkin/**/*.go`, plans/specs. Body = the
   `### Catkin` subsection from `invariants.md`, verbatim.

3. **Create `.claude/rules/attachments-invariants.md`.**
   Frontmatter paths: `internal/mail/attach*.go`,
   `internal/ui/attach_picker.go`, `internal/ui/viewer*.go`,
   plans/specs. Body = `mail.Attachment` paragraph + `download_dir`
   paragraph, with a cross-link line to the cache rule for
   storage details.

4. **Trim `invariants.md`:**
   - Delete the Cache section.
   - Delete the Catkin subsection from Architecture.
   - Delete the `mail.Attachment` paragraph from Mail model.
   - Delete the `[ui] download_dir` paragraph from Config & theming.
   - Insert a `## Subsystem invariants` section after `## Build &
     verification` (before `## Decision index`) listing the three
     rule files + their auto-load triggers.
   - Verify `wc -l` ≤ 260.

5. **Write `docs/poplar/decisions/0153-path-scoped-subsystem-invariants.md`.**
   Standard ADR format. Codify location (`.claude/rules/`),
   extraction criteria (settled, ≥ 25 lines, natural path scope),
   what was extracted in this pass, what was deferred (mail
   backends until after 9.6).

6. **Update decision index in `invariants.md`** — add ADR-0153 row.

7. **Update STATUS.md.** Mark 9d.2a done. Replace starter prompt
   with 9d.3 (golangci-lint on `internal/catkin/`). Update Next
   steps if needed.

8. **Archive plan + spec.** `git mv` both files into
   `docs/superpowers/archive/{plans,specs}/`.

9. **`make check`.** Must be green.

10. **Commit + push + install.** Single commit titled
    `Pass 9d.2a: invariants compaction via subsystem extraction`.

## Risks

- **Frontmatter path globs miss intended files.** Mitigation:
  match the pattern from `ui-invariants.md` exactly; both
  `internal/<pkg>/**/*.go` and plan/spec doc globs.
- **`invariants.md` line count creeps back up next pass.** ADR-0153
  documents the 25-line threshold so future passes know when to
  extract.
- **Mid-extraction drift.** Copy verbatim — do not rephrase or
  reorder during the move. Stylistic edits land in a separate pass.
