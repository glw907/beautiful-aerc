# Pass 9d.2a — Invariants compaction via subsystem extraction

## Goal

Stop `docs/poplar/invariants.md` from bumping the 400-line ceiling
every pass. Promote stable subsystems to path-scoped on-demand
docs, leave one-line pointers in `invariants.md`. Target post-pass
size: ~257 lines.

## Background

Pass 9d added the Catkin subsection (~35 lines) and Pass 8.6/8.7
added attachments lines across Mail model, Cache, and Config.
Combined with the existing Cache section (~98 lines), `invariants.md`
sits at exactly 400 lines — the size hook's hard ceiling. The
proven path-scoped pattern (`.claude/rules/ui-invariants.md`,
ADR-0095) demonstrates that subsystem invariants can live in
auto-loaded path-scoped rule files without losing discoverability.

## Decision

### Location: `.claude/rules/<name>-invariants.md`

Mirror the `ui-invariants.md` precedent. Single source of truth;
content lives directly in the rule file with `paths:` frontmatter.
No separate copy under `docs/poplar/`. The starter prompt's
"`docs/poplar/`" wording was a first-pass guess; the proven
pattern keeps content in `.claude/rules/`.

### Extractions (3 subsystems)

| Subsystem | Source | New rule file | Auto-load paths |
|---|---|---|---|
| Cache I/II/III | `invariants.md` Cache section | `.claude/rules/cache-invariants.md` | `internal/cache/**/*.go`, `cmd/poplar/cache*.go`, plans/specs |
| Catkin | `invariants.md` Architecture > Catkin subsection | `.claude/rules/catkin-invariants.md` | `internal/catkin/**/*.go`, plans/specs |
| Attachments | `invariants.md` Mail model `mail.Attachment` block + Config `download_dir` block | `.claude/rules/attachments-invariants.md` | `internal/mail/attach*.go`, `internal/ui/attach_picker.go`, `internal/ui/viewer*.go`, plans/specs |

The cache-side attachments paragraph stays inside the Cache rule
(it's fundamentally cache schema/eviction). The
`attachments-invariants.md` doc cross-links to the cache rule for
storage details.

### Deferred

**Mail backends (JMAP, IMAP).** OAuth refresh is queued for Pass
9.6 and will rewrite parts of the IMAP backend invariants block.
Extract after 9.6 lands.

### Decision index

Stays in `invariants.md`. One source of truth for ADR mapping,
unchanged by this pass.

### Subsystem invariants pointer section

`invariants.md` gains a short section listing the three new
subsystem rules + their auto-load triggers, so a human reader
knows where the extracted content went.

## Extraction-readiness criteria

Codified in ADR-0153. A subsystem is ready for extraction when:

- It has settled — no upcoming pass is queued to rewrite its
  binding facts.
- Its current footprint in `invariants.md` is ≥ ~25 lines (smaller
  blocks aren't worth the indirection).
- It has a natural path scope — a directory or set of files where
  edits are clearly "this subsystem."

## Non-goals

- Compacting the decision index (stays as-is).
- Compacting `Repo & libraries`, `Elm architecture`,
  `Config & theming`, `Icon mode`, `Mail model` (non-attachment
  parts), `Build & verification` — these are universal and
  always-loaded.
- Touching `internal/` source code.

## Math

| | Lines |
|---|---|
| Current `invariants.md` | 400 |
| − Cache section | −98 |
| − Catkin subsection | −35 |
| − `mail.Attachment` block | −14 |
| − `download_dir` block | −7 |
| + new "Subsystem invariants" pointer section | +12 |
| **Post-pass** | **~258** |

Well under the 300-line target the size hook enforces.

## Verification

- `wc -l docs/poplar/invariants.md` ≤ 260.
- `make check` green.
- The three new rule files validate against the existing
  frontmatter format (compare against `.claude/rules/ui-invariants.md`).
- ADR-0153 codifies the split policy.

## ADR

**ADR-0153** — Path-scoped subsystem invariants. Codifies the
split policy, extraction-readiness criteria, and the rule that
subsystem invariants live in `.claude/rules/<name>-invariants.md`
not `docs/poplar/`. Updates `invariants.md`'s decision index.
