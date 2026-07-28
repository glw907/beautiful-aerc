# Poplar vision

Date 2026-07-27. Status: Phase 2 output of the re-founding. Charter:
`2026-07-19-poplar-refounding-charter.md`. Evidence base: the
rendering-bet verdict
(`docs/poplar/research/2026-07-19-rendering-bet-verdict.md`). This
document locks at the Phase 2 gate and binds Phases 3 through 5:
requirements make it testable, the technical design is judged against
its horizon register, and the build delivers its switch bar.

## What poplar is

Poplar is an opinionated, vim-first terminal email client for coders.
Its central bet is that messy modern HTML mail can be turned into
prose a person reads and answers in a terminal. That bet is now
measured. A deterministic rule pipeline renders 87% of coder-relevant
mail (GitHub and CI notifications, personal mail, transactional
receipts, list and patch mail) at usable or better, with no LLM at
render time, and the rules improve offline from real graded failures.

The product carries the Pine lineage rather than the mutt lineage.
Poplar is beautiful and functional out of the box, and it stays
opinionated where aerc and mutt are configurable. A coder who lives
in vim and a terminal should be able to run poplar for the first time
and do all of their mail work that day, with no configuration file.

Poplar is as self-contained as possible: one binary that syncs,
stores, searches, renders, composes, and sends on its own. It never
shells out to an external editor, indexer, or delivery agent, and it
needs no companion daemon. The mutt-family stack of mbsync, notmuch,
and sendmail is exactly what poplar exists to replace.

## The speed goal

Every operation feels almost instant. Mail and contacts live in a
local store, and JMAP is the sync layer that keeps that store current
in the background. Reading, triage, and search run against local data
and never wait on the network. Rendering is deterministic Go on the
same machine. Phase 3 turns this goal into numbers for startup time,
keypress-to-paint, search latency, and sync convergence; Phase 4's
architecture is judged against those numbers.

## The switch bar: what v1 must do

v1 is the smallest client Geoff switches to full-time, and full-time
means routine mail work never falls back to the Fastmail web UI. That
sets the bar at full mail replacement for one account:

- Read, triage (archive, delete, flag, move), threading, search, and
  folder management.
- Compose and reply in Catkin, the live-markdown editor with visible
  delimiters (the iA Writer shape).
- Contacts autocomplete backed by the local contact store, and
  multiple sending identities and aliases on the one account.
- Calendar invites rendered from the `text/calendar` part, never
  scraped from invite HTML, with accept, decline, and tentative
  answered from the reader. The verdict expects this class to lead
  once rendered from structured data.
- Offline capability: reading and triage work without a connection,
  and changes sync when it returns.

## Differentiators

Rendering leads. The rest of the list earns the daily-driver bar and
the audience.

1. **Readable mail.** The deterministic rendering pipeline, with the
   rule engine as a first-class subsystem: every rule carries a name,
   an observable trigger and transform, motivating corpus references,
   and tests, and the renderer reports which rules fired on a message.
   A fact-inventory self-check verifies that actionable facts (links,
   amounts, dates, codes) survive into the render. Degraded renders
   fall back honestly to filtered plain text, the raw HTML part, or
   open-in-browser.
2. **Near-instant speed.** The local-first store and the speed goal
   above, as a product trait the user can feel.
3. **Catkin compose.** Writing mail as markdown with live styling,
   instead of a bare textarea bolted to a TUI.
4. **Coder polish.** Syntax-highlighted code fences, diff and patch
   rendering, and git URL handling as first-class features, because
   the audience reads code in mail daily.
5. **Out-of-the-box finish.** Curated defaults, clean visual design,
   universal folder handling, and footer hints that teach the mail
   workflow instead of restating vim. The UI is attractive and
   research-grounded at the same time: layout, navigation, and
   interaction patterns derive from the Phase 3 field survey and
   established email-UX practice, and text wireframes precede any
   screen build.
6. **Showcase code.** `internal/ui` readable as a reference bubbletea
   project. Contributor-attracting code quality is a product goal,
   because the post-v1 horizon needs contributors.

## Audience

Coders who live in the terminal: vim-first, keyboard-only,
modifier-free single-key bindings. v1 serves the founder's mailbox,
Fastmail over JMAP, because that is the path dogfooded every day. The
audience majority lives on Gmail, and that fact is deferred
deliberately rather than ignored. The backend seam in the Phase 4
design must make a Gmail backend a bounded addition, and the OAuth
onboarding ships only when it can be exercised against real accounts.

## Non-goals for v1

- User configurability. No keybinding remaps, no theme files, no
  layout options. Themes compile in as Go values.
- PGP and S/MIME.
- A plugin or scripting system, and no external-editor integration.
  Catkin is the compose surface, in keeping with self-containment.
- Multi-account. One account in v1, with multi-account as the first
  item on the horizon register.
- A Gmail or generic IMAP backend.
- Any non-terminal UI.

## Knowable-horizon register

The Phase 4 design must foreclose none of these, or must state why a
foreclosure is accepted. Each item is named so the design review can
check it explicitly.

1. **Multi-account.** The first post-v1 priority. The data model,
   sync engine, and UI state must not assume a single account
   identity even though v1 ships with one.
2. **Gmail backend with OAuth onboarding.** The majority audience's
   first experience, behind the backend seam.
3. **Flag-a-bad-render loop** (backlog #63). A one-key flag in the
   reader that captures raw MIME into a local problem corpus, with a
   strictly double-opt-in hosted collection path. Needs a raw-source
   capture seam and a local corpus store.
4. **Capture-mailbox corpus.** A dedicated collection mailbox that
   de-biases the single-mailbox limitation the verdict recorded, and
   feeds the same offline rule-improvement loop.
5. **Catkin standalone spinoff.** Already poplar-agnostic by rule; no
   inbound coupling may creep in.
6. **Encryption.** PGP and S/MIME stay out of v1, and the compose and
   reader seams must not weld them out.
7. **Contacts sidebar micro-highlight.** Minor polish, listed so the
   contacts UI keeps per-letter cursor state reachable.

## From vision to requirements

Phase 3 opens with a grounding survey before requirements are
drafted: Protonmail as the polished-incumbent baseline, the actively
used TUI clients (aerc, neomutt, alpine, meli, himalaya) for the
terminal state of the art, plus the existing mail-client field survey
and the 132-scenario rebuild spec as mining inputs. The survey covers
UX patterns as well as features, so the interaction design rests on
observed practice rather than invention. It exists so the
requirements are complete before the work starts, and the requirement
priorities then set the implementation order for the Phase 5 build
passes.
