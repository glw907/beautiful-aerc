# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. The sole active track. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. The old dogfood client is archived at tag `poplar-legacy` and branch `legacy`; its tracker `docs/poplar/STATUS.md` is retired reference.

**Canonical functional spec:** `docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md`. Domain passes append sections; Pass 8 consolidates.

## Current state (2026-05-29)

Pass 0 (Charter), Pass I (Claude infrastructure refresh), and Pass 1 (Accounts, protocols, sync) complete.

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

## Next: Pass 2, organization, threading, automation

Starter prompt (paste after /clear, or say "start Pass 2"):

```
Start Pass 2 of the poplar rebuild (organization, threading, automation). A
domain spec pass: it appends a spec section, it does not build. Read first:
docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md section 1
(the settled account, folder, and label model), the charter section 6
decisions 3 to 5, and the gap analysis section 2.

Pass 1 settled the data model: folders single-membership, labels server-backed
and capability-gated. Pass 2 builds organization on top of it. Brainstorm and
settle, then spec:
- Saved searches and virtual folders (ties to Pass 6 search).
- Server-side filters: Sieve for JMAP, with a client-visible rule config.
- Snooze and thread mute (mute routes future replies out of the inbox).
- Label UX and operations: apply, remove, label-scoped views, multi-label.
- Triage model across folders and the unified inbox; next-unread across folders.
End with numbered acceptance scenarios appended to the functional spec
section 2.

Pass-end: update the rebuild STATUS and append the execution record.
```

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [done] -> 2 Organization [next] -> 3 Reading/triage -> 4 Rendering -> 5 Compose -> 6 Search -> 7 Contacts/calendar/security -> 8 Consolidation -> build plans + clean build.
