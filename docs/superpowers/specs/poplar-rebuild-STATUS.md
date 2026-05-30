# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. The sole active track. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. The old dogfood client is archived at tag `poplar-legacy` and branch `legacy`; its tracker `docs/poplar/STATUS.md` is retired reference.

**Canonical functional spec:** `docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md`. Domain passes append sections; Pass 8 consolidates.

## Current state (2026-05-29)

Pass 0 (Charter), Pass I (Claude infrastructure refresh), Pass 1 (Accounts, protocols, sync), and Pass 2 (Organization, threading, automation) complete.

Pass 0 artifacts:
- Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`.
- Gap analysis: `docs/poplar/research/2026-05-29-mail-client-gap-analysis.md`.
- Infra-refresh research: `docs/poplar/research/2026-05-29-infra-refresh-research.md`.

Pass I outcomes:
- `CLAUDE_CODE_SUBAGENT_MODEL` set to `inherit`; rebuild agents in `.claude/agents/` (`poplar-implementer`, `poplar-reviewer`, `poplar-go-reviewer`); gate artifacts in `docs/superpowers/specs/rebuild-gate/`; Go-idiom catalogue extended (T42, T43). Record: `docs/superpowers/plans/2026-05-29-pass-I-infra-refresh.md`.
- Orchestration default: `subagent-driven-development` for domain passes; Workflow for wide fan-out.

Pass 1 outcomes (functional spec §1, 15 acceptance scenarios):
- Decision 1: account-partitioned with a unified view (Thunderbird/K-9 model); one SQLite DB per account; unified inbox is a read-side k-way merge by `SentAt`, cursor composite `(account, UID, SentAt)`; writes always account-scoped.
- Decision 2: folders single-membership, labels server-backed and capability-gated (`SupportsLabels`), gated off on plain IMAP without persistable custom keywords; labels additive so the folder tree stays primary nav.
- Best-practice-first framing adopted at Geoff's direction (see memory). OAuth per RFC 8252 loopback+PKCE plus RFC 8628 device-code; bring-your-own client for v1 (Gmail restricted-scope CASA reality, verified 2026-05-29); credential source is config so a shipped verified client is a post-1.0 preset change; alias-pattern identities in; performance-by-locality cache principle.
- Audience is Gmail-first; Geoff dogfoods Fastmail (JMAP bearer token, no OAuth), so Gmail/Outlook OAuth onboarding needs explicit test coverage and a live-OAuth gate in the build phase.

Pass 2 outcomes (functional spec §2, 16 acceptance scenarios):
- Threading: conversation-grouped is the default list mode; both modes ship (flat is a Pass 3 presentation toggle); the thread is a first-class unit that triage, mute, and snooze target. Threads stay account-scoped, so one external conversation across two accounts shows as two unified-inbox rows.
- Labels: multi-select picker with create-on-apply; a label-scoped view is a saved search, so labels and saved searches share one stored-query mechanism.
- Saved searches and virtual folders: one stored-query type (name, query, scope), config-persisted and runtime-creatable through the config round-trip, run offline against the FTS index, per-account or cross-account. Cross-account stored queries are the opt-in path to unified surfaces beyond the inbox.
- Filters: an abstract rule model (a condition plus actions) compiles to a sentinel-fenced managed Sieve block, preserving hand-written Sieve, with a raw read-only view. Capability-gated through `SupportsServerRules`: JMAP Sieve (RFC 9661) and ManageSieve (RFC 5804, its own connection). Gmail server filters need the Gmail REST API and are deferred post-1.0.
- Snooze and mute: capability-tiered with an always-available UX. Server snooze where advertised (JMAP `snoozed` / Sieve `snooze`, both unratified drafts, verified at build), client-managed-on-sync fallback otherwise. Mute via Gmail's native label, a generated Sieve rule, or a cache mute list.
- Triage: per-owning-account dispatch from the unified inbox; deterministic next-unread across folders then accounts; bulk-by-criteria on the full result set, not the visible page.
- Backend additions: `SupportsServerRules`, `SupportsServerSnooze`, `SupportsNativeMute`; ManageSieve is a third IMAP connection alongside command and idle.
- Protocol research backing these decisions: RFC 9661 (JMAP Sieve), RFC 5804 (ManageSieve), RFC 5228/5232/5490 (Sieve), the expired EXTRA snooze drafts, the Gmail-filters-need-REST-API boundary, and saved-search-is-client-side everywhere.

## Next: Pass 3, reading, triage, navigation

Starter prompt (paste after /clear, or say "start Pass 3"):

```
Start Pass 3 of the poplar rebuild (reading, triage, navigation). A domain
spec pass: it appends a spec section, it does not build. Read first:
docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md sections
1 and 2 (the settled account, folder, label, threading, and automation
model), the charter section 2 (product definition) and section 6 decision 6,
and the gap analysis sections 3 and 4.

Pass 2 settled the organizational semantics: a threaded-by-default
conversation model, label and saved-search views, and the triage model
(per-account dispatch, next-unread across folders, bulk-by-criteria). Pass 3
designs the surfaces that drive them. Wireframe-first for every screen.
Brainstorm and settle, then spec:
- Keyboard model: modifier-free single keys, the full triage and navigation
  key map, the flat-vs-threaded toggle.
- Pane model: sidebar, list, reader layout; one-pane Pine-style reading;
  responsive tiers.
- List UX: conversation rows, label and flag display, the results-mode list
  for saved searches and search.
- Reader UX: header presentation, body region, link and attachment
  affordances (rendering itself is Pass 4).
- Triage and bulk UX: the tag-pattern-then-apply surface, the undo window,
  snooze/mute/label entry points.
End with numbered acceptance scenarios appended to the functional spec
section 3.

Pass-end: update the rebuild STATUS and append the execution record.
```

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [done] -> 2 Organization [done] -> 3 Reading/triage [next] -> 4 Rendering -> 5 Compose -> 6 Search -> 7 Contacts/calendar/security -> 8 Consolidation -> build plans + clean build.
