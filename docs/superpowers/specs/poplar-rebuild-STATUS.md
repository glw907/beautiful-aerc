# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. The sole active track. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. The old dogfood client is archived at tag `poplar-legacy` and branch `legacy`; its tracker `docs/poplar/STATUS.md` is retired reference.

**Canonical functional spec:** `docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md`. Domain passes append sections; Pass 8 consolidates.

## Current state (2026-05-30)

Pass 0 (Charter), Pass I (Claude infrastructure refresh), Pass 1 (Accounts, protocols, sync), Pass 2 (Organization, threading, automation), Pass 3 (Reading, triage, navigation), and Pass 4 (Message rendering) complete.

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

Pass 3 outcomes (functional spec §3, 19 acceptance scenarios):
- Pane model: strict one-pane by default and on the narrow and standard
  tiers; a widescreen tier (around 130 cols) adds an opt-in follower
  preview pane (`P`). The pane re-renders the cursored message top-aligned,
  never scrolls on its own, and does not mark read; `Enter` opens the
  full-width reader and marks read. This reverses the legacy no-preview
  stance at Geoff's direction, keeping the Pine one-pane keyboard model
  while adding the widescreen scan-and-read flow.
- Sidebar: the Thunderbird and K-9 shape settled in §1. A pinned Unified
  Inbox, per-account collapsible classified trees, and a Saved Searches
  group where label views are saved searches. `J`/`K` walk the whole
  column; the active account follows the cursor; `h`/`l` (arrows alias)
  collapse and expand nodes; `[`/`]` step accounts and cycle through the
  unified scope; `Tab`/`Shift-Tab` are next and previous unread. `[ui]
  unified-inbox` opts the merge out, and a single-account config collapses
  to a plain classified tree.
- Keyboard model: modifier-free single keys, no `:` mode, two contexts
  sharing one pane with no focus cycling. New verbs are `L` label, `z`
  snooze, `M` mute, `t` thread-toggle, `P` preview, `V` select-by-criteria,
  and `E` empty-folder; `Space`/`F` fold threads; `n`/`N` step search
  matches. Vim-aligned choices win over pine and mutt precedent at Geoff's
  direction, since the audience carries vim muscle memory.
- Overlays: a single-active cascade, and modal overlays composite over a
  dimmed underlay. The dim is a principled focus scrim that keeps the
  underlay legible as context, reversing the legacy undimmed-underlay rule
  (ADR-0202). Neither the preview pane nor inline compose is an overlay, so
  they do not dim.
- List and reader: a per-account color marker in cross-account views, label
  chips on rows, and the flat-vs-threaded toggle (`t`). The reader gains an
  account chip and a Labels row that carries snooze and mute state. Its
  sender line reserves `i` as the Pass 7 contact hook.
- An independent vim-priority keymap review folded in before lock: it fixed
  a `Tab` double-binding, restored `Space`/`F` and `n`/`N`, added `E`,
  moved label off the `l` motion key to `L`, put tree collapse/expand on
  `h`/`l`, and demoted the preview toggle to `P`.

Pass 4 outcomes (functional spec §4, 18 acceptance scenarios):
- Default render is poplar's own cleaned markdown, derived from the
  structure-richest message part, with `[ui] body-render` overriding to `html`
  or `plain` and a per-message reader toggle. The well-crafted-markdown render
  is the product; the source MIME part is an implementation detail the pipeline
  picks.
- The rendering contract is a versioned normative doc holding principles,
  structural-inference rules, RFC 2119 syntactic rules, and density signals. A
  gold standard of hand-blessed exemplar renders defines excellent, where
  excellent is high-quality markdown relative to the source, so a valid render
  that loses the message's meaning fails. Tunable data lives in one hand-edited
  `rules.go`; nothing is code-generated.
- Fidelity gains include pane-relative reflow with no fixed column cap, quote
  folding collapsed by default, a GFM-vs-layout table split, and entity-decode,
  tracking-strip, and footnote extraction as MUSTs.
- Remote resources never load, the privacy floor; inline images are opt-in and
  capability-detected (kitty/iTerm2/sixel). Dev-first body features are chroma
  syntax highlighting, patch and diff rendering, and per-link copy.
- The golden corpus commits public sets directly and adds the user's Fastmail
  mail through a minimize-and-human-scrub gate; locked goldens and public sets
  gate `make check` as a permanent regression check.
- A poplar-internal package holds the renderer, and the eval tool is a
  dev-tagged CLI decoupled from the app, built so Claude's
  render-judge-fix-gate loop is fast and isolated. The loop runs interactively
  and as an unattended batch mode that applies and commits fixes on a schedule,
  and the tool emits a human-readable evolution report written with Claude.
  Runtime LLM HTML cleaning is named and deferred.
- Best-practice-first throughout at Geoff's direction; the legacy
  `mailrender-training` system is evidence, not the template.

## Next: Pass 5, compose and sending

Starter prompt (paste after /clear, or say "start Pass 5"):

```
Start Pass 5 of the poplar rebuild (compose and sending). A domain spec pass:
it appends a spec section, it does not build. Read first: the charter section 8
(catkin and tidy) and section 6 decision 9 (templates/snippets, attachment
reminder), the gap analysis section 5 (composing and sending), and functional
spec section 1.5 (identities and alias auto-selection) and section 3.5 (the
reader the reply and forward flows launch from).

Pass 5 owns the outbound surface. Derive from the field and current best
practice, not from legacy poplar. Brainstorm and settle, then spec:
- The catkin compose surface: the markdown editor kept poplar-agnostic for a
  standalone spinoff, address entry with autocomplete, and how reply,
  reply-all, and forward seed a draft.
- MIME assembly: markdown to multipart/alternative, attachments to
  multipart/mixed, and the text-plus-HTML fidelity contract against Pass 4's
  renderer.
- Signatures and identities: per-identity multi-signature selection and
  alias-pattern auto-selection on reply.
- Drafts, send-later, and undo-send: the outbox state machine, the scheduled
  send window, and the cancel-and-reschedule mechanics.
- Templates and snippets and the attachment reminder.
- AI prose tidy folded into catkin, user-invoked and never on the send path.
End with numbered acceptance scenarios appended to the functional spec
section 5.

Pass-end: update the rebuild STATUS and append the execution record.
```

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [done] -> 2 Organization [done] -> 3 Reading/triage [done] -> 4 Rendering [done] -> 5 Compose [next] -> 6 Search -> 7 Contacts/calendar/security -> 8 Consolidation -> build plans + clean build.
