# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. The sole active track. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. The old dogfood client is archived at tag `poplar-legacy` and branch `legacy`; its tracker `docs/poplar/STATUS.md` is retired reference.

**Canonical functional spec:** `docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md`. Domain passes append sections; Pass 8 consolidates.

## Pass-end ritual

This runs by default at every pass close ("ship pass", "finish pass", or a completed pass), whether or not the pass's starter prompt restates it. Updating this STATUS is step one and is never optional.

1. Append a `Pass N outcomes (functional spec §N, K acceptance scenarios)` block to Current state below, summarizing the settled decisions.
2. Add Pass N to the Current-state completion line.
3. Replace the `## Next` section with the next pass's title and its starter prompt.
4. Mark `[done]` and `[next]` on the Pass roadmap line.
5. Run `prose-guard` on every changed doc; a draft that fails is rewritten before commit.
6. Commit the changed spec section and this STATUS as `Rebuild Pass N: <title> spec section`, then push to `origin/master`.
7. Refresh the `project_poplar_rebuild_initiative` memory's settled-decisions digest only when a load-bearing decision changed. The done/next-pass cursor is not tracked in memory; this STATUS owns it.

## Current state (2026-05-30)

Pass 0 (Charter), Pass I (Claude infrastructure refresh), Pass 1 (Accounts, protocols, sync), Pass 2 (Organization, threading, automation), Pass 3 (Reading, triage, navigation), Pass 4 (Message rendering), Pass 5 (Compose and sending), and Pass 6 (Search) complete.

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

Pass 5 outcomes (functional spec §5, 18 acceptance scenarios):
- Compose is a modeless full-pane catkin surface, non-overlay so it does not
  dim, opened by `c`/`r`/`R`/`f`. Vim modal editing is named and deferred; the
  rebuild ships the Pico-tradition modeless editor. Poplar owns the
  message-level chords (`Ctrl-X` send, `Ctrl-O` draft, `Ctrl-A` attach,
  `Ctrl-J` snippet, `Esc` leave) and catkin owns the body and tidy (`Ctrl-T`);
  the surface claims its chords before delegating, so the boundary holds for
  the standalone editor.
- Address entry: To/Cc/Bcc with recency-decayed cache suggestions and
  `Tab`/`Enter` accept, on the same suggest seam Pass 7 contacts feed.
- Reply and forward seed depth-preserving `>` quotes with a top-post default
  and a trimmable quote, RFC 5322 `In-Reply-To` and `References`, and
  alias-aware identity auto-select (§1.5); forward carries the attachments.
- MIME: default multipart/alternative with a text/plain markdown source and a
  text/html goldmark render on the Pass 4 reader's config, so a message
  round-trips faithfully; multipart/mixed wraps it when attachments are
  present. A per-identity `text-only` flag plus a per-message toggle drop the
  HTML part for mailing-list and patch mail, the coder-audience call, and the
  markdown source becomes the wire text so code and diffs pass unmangled.
- Drafts autosave to cache, sync to server Drafts, and restore fully on
  reopen; the outbox row links the draft and the drainer deletes it in the
  send transaction (§1.7).
- Outbox: send never dispatches inline. `Ctrl-X` queues with `scheduled_for =
  now + [ui] send-delay` (a few seconds by default, zero disables) and the
  undo chrome row cancels within the window. Send-later sets a future
  `scheduled_for` through the snooze picker's time parse, and the outbox
  overlay (`Q`) cancels-to-draft or reschedules. Undo-send and send-later are
  one mechanism with no special-cased state.
- Templates and snippets share one mechanism: named `[[snippet]]` config
  bodies inserted on `Ctrl-J` with a single `{{cursor}}` placeholder and no
  further substitution, kept small per the charter. The attachment reminder
  scans the body at send for intent keywords and pauses on a confirm modal
  when none is attached.
- AI prose tidy is a catkin command on `Ctrl-T`, user-invoked, an in-place
  accept-or-reject diff, never on send or save, and inert with no provider
  configured (charter §8).
- Best-practice-first throughout at Geoff's direction; legacy poplar's compose
  is evidence, not the template.

Pass 6 outcomes (functional spec §6, 19 acceptance scenarios):
- FTS5 substrate: one own-content `messages_fts` table per account, rowid-keyed
  to the message row, written in the message's own transaction (DELETE+INSERT,
  no UPSERT). Full text and structured metadata split: FTS5 columns hold
  subject, addresses, body, and attachment text, while folder, account, date,
  size, flags, label, and attachment presence stay SQL constraints, so a query
  compiles to one MATCH plus WHERE clauses. Tokenizer is `unicode61` with
  diacritic folding and a prefix index, no stemming, so code tokens match
  exactly and recall comes from the explicit prefix wildcard.
- Index coverage: envelope eager and complete; body indexed when its MIME is
  cached; attachments index filename, content-type, and `text/*` extracted
  text, with binary extraction (PDF, DOCX) deferred post-1.0. A throttled
  per-account body backfiller fetches un-cached bodies to deepen body search,
  backing off under backend pressure with a status-bar `warn` substate. The FTS
  table is a rebuildable projection per §1.7.
- Query language: Gmail-compatible, parsed by a pure `Parse`. Text operators
  (`from:` `to:` `cc:` `bcc:` `subject:` `body:` `filename:`), structured
  (`in:`/`folder:` `account:` `label:` `has:attachment` `is:`), date (`before:`
  `after:` `newer_than:` `older_than:`), size (`larger:` `smaller:`), boolean
  AND and OR, parentheses, `-` negation, and quoted phrases. `account:` is the
  multi-account addition; an unknown `key:value` falls through as a bare term.
  Sort is `sent_at` descending with no relevance toggle.
- Scope: folder, account, or cross-account, defaulting to the sidebar cursor's
  context; the shelf scope key (`\`) cycles the three stops; `in:`/`account:`
  override from inside the query. Results render in the §3.4 results mode.
- Search-as-you-type: incremental against the local index only, debounced, with
  a trailing-term prefix wildcard, an in-flight query superseded rather than
  queued, and a row limit; operator suggestion completes values for `in:`,
  `is:`, and `label:`.
- Saved searches: §2.3's stored query gains the §6.2 grammar and a run surface.
  Saving the shelf query writes a `[[saved-search]]` config block through the
  round-trip; saved searches run from the sidebar group, re-run on open and on
  scope-touching change events, and never store results. A label view is the
  saved search `label:<name>`, and cross-account saved searches are the path to
  unified surfaces past the inbox.
- Best-practice-first at Geoff's direction; legacy poplar's FTS5 search layer is
  evidence, not the template. Three tradeoffs settled with Geoff: background
  body backfill for index completeness, `text/*` attachment extraction, and the
  full Gmail-compatible grammar.

## Next: Pass 7, contacts, calendar, security

Starter prompt (paste after /clear, or say "start Pass 7"):

```
Start Pass 7 of the poplar rebuild (contacts, calendar, security). A domain
spec pass: it appends a spec section, it does not build. Read first: the gap
analysis section 7 (contacts, calendar, security), and functional spec section
1.6 (OAuth and credentials, which CardDAV contacts credentials fall back to),
section 3.5 (the reader's sender line reserves `i` as this pass's contact hook,
and the Labels row carrying snooze and mute state), and section 5.2 (the
compose address autocomplete seam this pass's contacts feed).

Pass 7 owns contacts, calendar, and security. Derive from the field and current
best practice, not from legacy poplar. Brainstorm and settle, then spec:
- Contacts: CardDAV sync with multiple address books and auto-collect from
  sent, the contact popover on the reader's `i` hook, and the contacts mode,
  and how the section 5.2 suggestion seam and the cache feed both.
- Calendar: inline ICS invite display and one-action RSVP, which the gap
  analysis flags as the missing piece, and how the response sends.
- Security: a sender-verification surface from DKIM and DMARC results, and the
  scope decision on PGP and S/MIME, which the gap analysis flags as a scope
  call rather than an automatic include.
- List-Unsubscribe one-click carries forward from the legacy reader; place it
  against this pass's security surface.
End with numbered acceptance scenarios appended to the functional spec
section 7.

Pass-end: run the Pass-end ritual at the top of this STATUS. It updates this
STATUS by default.
```

## Carry-forward considerations

Items that span passes and resolve at consolidation, not in the pass that
raised them.

- Keyboard command order and grouping, reviewed holistically across every
  key-bearing surface: the account view and reader (§3.1), the compose editor
  chords (§5.1), the search keys (Pass 6), and the contacts keys (Pass 7).
  Their full set exists only after Pass 7, so Pass 8 orders and groups it in
  one place, in the editor and outside it, and reflects the result back into
  the owning sections and the help popover. The build also wants a single
  source of truth for the binding tables so the help popover and the docs do
  not drift from the code.
- Command visibility across the responsive tiers (§3.2). That grouping review
  decides which commands and on-screen affordances surface, fold into the help
  popover, or recede at narrower widths, the same data-driven cliff the chrome
  (date and flag columns, label chips, sidebar) already rides down to Spartan.
  A command stays bound when its surface is reachable; what changes by tier is
  what the chrome advertises and how the command rows and the help popover
  prioritize, so a smaller screen hides the hint, not the key.

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [done] -> 2 Organization [done] -> 3 Reading/triage [done] -> 4 Rendering [done] -> 5 Compose [done] -> 6 Search [done] -> 7 Contacts/calendar/security [next] -> 8 Consolidation -> build plans + clean build.
