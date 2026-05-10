# Release stance — phase rules

ADR-0105. Poplar moves through three phases with different rules.
`STATUS.md` shows the active phase. The pre-beta operational rules
sit in `CLAUDE.md` because they bind every pass right now; this
file holds the dormant phases (beta-soak, post-1.0) so they don't
weigh on the auto-loaded context until they activate.

## Beta soak (Pass 16 ships `v0.9.0` → `v1.0.0`)

**Stability first.**

- Master accepts bug fixes only. No new features on master.
- On-disk data formats frozen. Schema versions + automatic
  lossless migrations across beta releases.
- Refactors that don't touch user-visible behavior are OK if
  small, reviewable, and tested.
- New features queue on the `1.1` branch.

## Post-1.0 (`v1.0.0` ships)

Standard SemVer. `v1.x.y` backwards-compatible; breaking changes
wait for `v2.0.0`.
