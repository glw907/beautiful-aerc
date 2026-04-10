# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## Medium

- [ ] **#1** Clean up pick-link references from live docs `#improvement` `#docs` *(2026-04-09)*
  Binary was archived but `~/.claude/docs/aerc-setup.md` and `CLAUDE.md` still reference it extensively.
- [x] **#2** Clean up stale pandoc references from docs `#improvement` `#docs` *(2026-04-09)*
  pandoc is no longer part of the project but `~/.claude/docs/aerc-setup.md` still references it in the filter pipeline and compose settings.
- [ ] **#4** Investigate JMAP blob preloading for faster message open `#improvement` `#upstream` *(2026-04-09)*
  New messages are slow to open (~6s) because aerc fetches body blobs lazily from Fastmail on first open. `cache-blobs=true` only helps on second open. Investigate whether aerc's JMAP backend supports blob prefetching (e.g., preload next 2-3 messages) or if this needs an upstream aerc patch.
- [ ] **#3** Glamour: hanging indent for wrapped list items `#upstream` `#rendering` *(2026-04-09)*
  Glamour has no hanging indent — wrapped continuation lines align with the bullet, not the text. Simple defaults for now (`Item.BlockPrefix: "- "`, `LevelIndent: 2`). Track charmbracelet/glamour#56, charmbracelet/glamour#314, and unmerged PR charmbracelet/glamour#481. Update `internal/theme/glamour.go` when upstream merges a fix.
