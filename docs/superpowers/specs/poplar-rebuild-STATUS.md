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

Pass 0 (Charter), Pass I (Claude infrastructure refresh), Pass 1 (Accounts, protocols, sync), Pass 2 (Organization, threading, automation), Pass 3 (Reading, triage, navigation), Pass 4 (Message rendering), Pass 5 (Compose and sending), Pass 6 (Search), Pass 7 (Contacts, calendar, security), and Pass 8 (Consolidation) complete.

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

Pass 7 outcomes (functional spec §7, 17 acceptance scenarios):
- Contacts model: vCard (RFC 6350) end to end. Three sources behind one
  query-and-suggest seam: synced CardDAV books (first-class, credentials per
  §1.6, cached for offline), a local `contacts.vcf`/`contacts.d/` store for
  non-syncing users, and a suggestion-only auto-collect cache that never writes
  back. The surfaced and editable field set is mail-client-minimal at Geoff's
  direction, a display name plus multiple emails and multiple phones, each with
  a type and a primary marker; richer vCard fields round-trip untouched.
  Multi-value primary cascades PREF=1, then TYPE=pref, then first-seen.
- Suggest-and-lookup seam: one path feeds compose autocomplete (§5.2) and the
  reader card, ranks curated sources above auto-collect, and dedupes on email.
- Contacts mode and the reader card: a slim contacts mode on a dedicated mode
  key (reconciled against the §3 keymap, since legacy `M` is now mute), an A-Z
  sidebar index, and a detail card on `Enter`. The §3.5 `i` hook opens a sender
  contact-card popover with add-to-contacts. Create and edit write
  name/emails/phones to one default destination; the multi-book picker is
  deferred post-1.0 (ADR-0176).
- Calendar: an inline ICS invite block; one-action RSVP (accept, tentative,
  decline) sends an iMIP `METHOD=REPLY` with updated `PARTSTAT` through the §5
  outbox, needing no calendar backend; CalDAV calendar write is deferred (no
  calendar account model in v1). ICS parses through the locked
  `arran4/golang-ical`.
- Security, verification: a reader-header badge from the `Authentication-Results`
  header (RFC 8601) for DKIM, SPF, and DMARC, trusting the delivery boundary; a
  DMARC failure or From-mismatch warns; local DNS re-verification is deferred.
- Security, encryption: read-side PGP only at Geoff's direction. v1 verifies
  PGP/MIME (RFC 3156) and inline-PGP signatures and decrypts incoming mail
  through the local GnuPG keyring and gpg-agent; a missing key shows an honest
  unverified state. Signing and encrypting on send and S/MIME are deferred to a
  post-1.0 encryption pass with the send-side scope stated.
- List-Unsubscribe one-click carries forward unchanged (§3.5, ADR-0185).
- Best-practice-first at Geoff's direction; legacy poplar's contacts, calendar,
  and unsubscribe code is evidence, not the template. Two tradeoffs settled with
  Geoff: read-side PGP scope, and a minimal local vCard store alongside
  first-class CardDAV sync.

Pass 8 outcomes (functional spec §8, 10 acceptance scenarios):
- Consolidated §8, Appendix A (glossary), and Appendix B (config-key index) are
  written, and the seven domain sections were edited in place to agree with §8.
- A unified keyboard map is the single source of truth (§8.1). The five keys
  earlier passes deferred are assigned: render-mode cycle `v` and invite RSVP
  `V` in the reader, contacts mode `C` in the account view, scope cycle `\` and
  save-search `Ctrl-S` in the search shelf. An overlay-shadow rule lets a picker
  bind triage letters without colliding with live reader triage. The build phase
  keeps one Go `key.Binding` registry; the help popover and any keymap doc
  generate from it, and a test asserts they match.
- Command visibility across the responsive tiers is settled (§8.2). Bindings
  stay constant at and above the Spartan floor of around 60 columns; narrowing
  recedes hints and chrome, never a key, and the help popover is the complete
  reference at every tier.
- Capability vocabulary is unified into three families (§8.3): backend
  `Supports*` flags each with a fallback, transport capabilities detected from
  the session, and terminal capabilities resolved at startup. The mute (§2.5)
  and snooze (§3.6) sites now name their flags.
- Cross-section seams and shared contracts are stated once (§8.4): the six
  charter seams plus the shared time parser, the per-account accent color, and
  the suggest-and-lookup seam.
- Drift is fixed in the owning sections (§8.5): saved search over virtual
  folder, star and starred over flagged, the Disposal folder anchored in §1.2,
  consistent FTS5 naming, the goldmark engine disentangled from the reader
  pipeline, the legacy-baseline references reworded for the greenfield, and the
  bottom-of-screen surfaces named.
- A deferral register (§8.6), the master acceptance index of 122 domain
  scenarios plus 10 Pass 8 scenarios for 132 total (§8.7), the glossary, and the
  config-key index are gathered. prose-guard passes on the whole spec.
- Spec phase is complete pending the user review gate. Next work is the numbered
  build plans.

## Next: build plans (post-spec)

Before the build plans, a three-pass spec-hardening review runs: a gap hunt, a critique, then an integrate-and-reformat pass. Its tracker and starter prompts are `docs/superpowers/specs/spec-hardening-STATUS.md`. The build-plan starter prompt below applies once that review completes.

Spec phase is complete. The canonical functional spec is locked pending Geoff's
review at the consolidation gate. Next comes the phase that turns the locked spec
into numbered TDD build plans, then the clean build (charter §9).

Starter prompt (paste after /clear, or say "start the build-plan phase"):

```
Start the build-plan phase of the poplar rebuild. The spec is locked. Read
docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md end to end
and the charter docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md
(§3 method, §5 seams, §9 plan sequence, §10 conventions).

Draft the numbered build-plan roadmap from the locked spec, then write the first
plan. Sequence the plans so the foundational seams land first: the per-account
cache and the Backend interface (charter §5 seams 1 and 2), since every later
surface reads from them. Each plan turns one spec area into a task list of
red-green-commit TDD cycles with verbatim steps. An implementer subagent runs
one task at a time and clears make check before reporting done, and reviewer
subagents check spec-compliance and code per task. Use subagent-driven-
development for the domain work and Workflow for wide fan-out.

Settle the greenfield code-tree location at the build boundary (charter §11 open
decision) before the first plan writes code. Pass-end: run the Pass-end ritual.
```

## Carry-forwards (resolved at Pass 8)

Functional spec §8.1 and §8.2 resolve the keyboard-command ordering and grouping
across every surface, the single source of truth for the binding tables, and
command visibility across the responsive tiers. No carry-forward remains open.

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [done] -> 2 Organization [done] -> 3 Reading/triage [done] -> 4 Rendering [done] -> 5 Compose [done] -> 6 Search [done] -> 7 Contacts/calendar/security [done] -> 8 Consolidation [done] -> build plans [next] + clean build.
