# Poplar Rebuild: Pass 0 Charter

**Status:** Draft, 2026-05-29.
**Companions:** `docs/poplar/research/2026-05-29-mail-client-gap-analysis.md` (the field survey this charter rests on) and `docs/poplar/research/2026-05-29-infra-refresh-research.md` (Pass I groundwork: model selection and Go-idiom tuning).

## Why this document exists

Poplar is being rebuilt clean. The current codebase is healthy (45 passes, near 1:1 test ratio, 244 ADRs), so the rebuild is not a rescue. It is a chance to lay down one coherent architecture, written natively for Opus 4.8, with the feature set checked against the modern field rather than against poplar's own history.

This charter sets the frame for that work: the product, the method, the locked stack, the load-bearing seams, the scope decisions the gap analysis forces, and the pass roadmap. It does not specify any subsystem in detail. Each domain pass does that and appends its section to the canonical spec.

## 1. The premise

- **Greenfield and spec-first.** Build to a strong spec. Reference the existing code minimally. The current UI is a sketch to riff on, not code to port.
- **Gap-checked.** The feature set is grounded in the field survey, so the rebuild does not narrow the product by accident.
- **UI/UX improvement is in scope.** Wireframe-first for any screen that changes.
- **The spec is built over passes.** This charter is Pass 0. The canonical functional spec is assembled across the domain passes and consolidated at the end.
- **Build only after the spec locks.** Numbered TDD plans and the clean build come last.

## 2. Product definition

Poplar is a single-binary bubbletea terminal email client. The philosophy is "better Pine": opinionated, approachable, showcase-quality, not "better mutt." The audience is coders, so developer-polish features (syntax highlighting, patch rendering, markdown-native compose) are first-class, not afterthoughts. The keyboard model is Pine-true: modifier-free single keys, no multi-key sequences. The name stays "poplar".

## 3. Method (the cairn-style machine)

The rebuild uses the methodology proven on the cairn-cms rebuild:

- One canonical functional spec locks behavior, stack, the load-bearing seams, and numbered acceptance scenarios. It leaves internal structure to the plans.
- Numbered plans turn spec sections into task lists. Each task is a red-green-commit TDD cycle with verbatim steps.
- An implementer subagent executes one task at a time and clears a full project gate (`make check`: fmt, vet, voice, modern-go, skipcheck, test) before reporting done, with pasted evidence.
- Reviewer subagents check spec-compliance and code quality per task; domain reviewers fan out at pass end.
- An execution record is appended to each plan after it runs.

The infrastructure-refresh pass (below) revisits this machine against current Opus 4.8 best practice before the domain passes lean on it.

## 4. Locked stack (provisional)

Inherited defaults, carried unless a pass argues otherwise. The infra-refresh pass may revisit tooling; the domain passes may revisit a library when they own its surface.

- Go (1.26 line), single module.
- UI: `charm.land/{bubbletea,lipgloss,bubbles}/v2`. Idiomatic bubbletea, Elm architecture.
- Mail: `rockorager/go-jmap` (JMAP) and `emersion/go-imap` v2 (IMAP), coequal. `go-message`, `go-smtp`, `go-sasl`, `go-webdav`, `go-vcard`.
- Storage: `modernc.org/sqlite` (pure Go), per-account cache.
- Content: `goldmark`, `html-to-markdown/v2`, `chroma`, `arran4/golang-ical`.
- Auth: `golang.org/x/oauth2`, `filippo.io/age`, `go-keyring`.
- CLI: `cobra`. Config: `BurntSushi/toml`.

## 5. Load-bearing seams

The spec locks these boundaries; the plans fill them in. Named here so passes share one vocabulary.

1. **Backend** (JMAP and IMAP behind one synchronous interface).
2. **Per-account cache** (offline store, sync, outbox, search index).
3. **UI tree** (Elm root plus bubbles-shaped subpackages; state in models, mutations in Update, I/O in Cmd).
4. **Catkin** (the markdown editor, kept poplar-agnostic for standalone spinoff; AI tidy lives inside it).
5. **Content renderer** (mail body to terminal lines).
6. **Rendering eval harness** (offline corpus plus AI judge that locks the rendering contract).

## 6. Scope decisions forced by the gap analysis

Each is owned by a pass. The first two are foundational and shape the data model, so they are settled early.

| # | Decision | Owner pass | Provisional stance |
|---|---|---|---|
| 1 | Multiple accounts + unified inbox | 1 (foundational) | In scope. Reshapes cache, backend, sidebar. |
| 2 | Folders-only vs folders + labels/tags | 1/2 (foundational) | Lean labels: JMAP multi-membership maps cleanly. Decide in pass. |
| 3 | Saved searches / virtual folders | 2 and 6 | In scope. |
| 4 | Server-side filters (Sieve) | 2 | In scope for JMAP; investigate. |
| 5 | Snooze and thread mute | 2/3 | In scope. |
| 6 | Rendering direction | 4 | Keep markdown, raise fidelity, golden corpus + AI loop, dev-first features. |
| 7 | RSVP + sender-verification display | 7 | In scope. |
| 8 | PGP / S-MIME | 7 | Scope decision, large; default-defer unless the pass argues otherwise. |
| 9 | Templates/snippets, attachment reminder | 5 | In scope, small. |

## 7. The rendering program

The gap analysis found poplar's markdown pipeline is ahead of the terminal field, not behind it. The rendering pass therefore keeps markdown and invests in fidelity and developer features:

- A **golden corpus**: a curated set of the user's own Fastmail mail (via the JMAP token) plus public sets (Apache SpamAssassin, TREC, public-inbox/lore for patches and threading, Enron for scale).
- An **offline AI eval/improve loop**: each corpus message renders, Claude judges the output against the source on fidelity, structure, readability, and developer cases, and emits failure clusters that drive fixes. The judge rationales become the rendering contract; the loop generates the golden files with confidence.
- **Dev-first features**: reader syntax highlighting, patch/diff rendering, and richer link handling.
- A runtime "LLM cleans this HTML" feature is a deferred opt-in, not the default path (cost, latency, privacy, offline).

## 8. Catkin and tidy

Catkin stays a self-contained, poplar-agnostic package so it can ship as a standalone terminal markdown editor later. The AI prose-tidy feature folds into catkin (no separate `tidytext` package). Tidy stays user-invoked, never on the send path.

## 9. The spec build plan

| Pass | Title | Goal |
|---|---|---|
| 0 | Charter | This document. |
| I | Claude infrastructure refresh | Re-derive the agents/skills/gate/orchestration from current Opus 4.8 best practice. Runs after 0, before the domain passes lean on the machine. |
| 1 | Accounts, protocols, sync | Backend model, multi-account, identities, OAuth, cache/sync contract. |
| 2 | Organization, threading, automation | Label/tag model, saved searches, filters, snooze, mute, triage. |
| 3 | Reading, triage, navigation | Keyboard model, pane model, list/reader UX. Wireframes. |
| 4 | Message rendering | Rendering contract, corpus, AI eval loop, dev-first features. |
| 5 | Compose and sending | Catkin surface, MIME assembly, signatures, drafts, send-later, undo-send. |
| 6 | Search | Index, operators, saved searches, scope, search-as-you-type. |
| 7 | Contacts, calendar, security | CardDAV, ICS + RSVP, sender verification, encryption scope. |
| 8 | Consolidation | Fold the sections into one canonical functional spec; self-review; user review gate. |

**Pass I inherits real assets.** The current poplar already tuned Claude to write idiomatic Go rather than Python- or JavaScript-shaped Go: the `go-conventions` skill, the go-comment-voice guide and its AI-tell catalogue, the modern-stdlib idiom table, and the grep-tier voice and modern-go checks in `make check`. Pass I carries these forward and improves them rather than starting from scratch. This is Claude infrastructure, not poplar code, so the greenfield premise does not discard it. For Go idiom specifically, the existing code is a positive reference even though its structure is not ported.

Pass I workstreams: (1) model selection per subagent role for the Opus 4.8 era (orchestrator, implementer, reviewers, search), revisiting the Sonnet-implementer / Opus-reviewer split and `CLAUDE_CODE_SUBAGENT_MODEL`, and where Haiku 4.5 fits; (2) carry forward and improve the idiomatic-Go tuning, research-backed; (3) the orchestration choice (`subagent-driven-development` vs the Workflow harness); (4) the implementer and reviewer agent definitions plus the project gate.

After Pass 8 locks the spec: numbered build plans, then the clean build.

## 10. Conventions for the spec work

- Wireframe-first for any UI that changes.
- The writing-voice standard applies to all docs (this charter included).
- Decisions are captured as ADRs or inline in the spec, not lost in chat.
- Pass size budget: 8 to 12 tasks; split before coding if it grows.
- Each domain pass ends with numbered acceptance scenarios; the consolidated spec carries the full set as the done-contract.

## 11. Open decisions log

Resolved decisions move out of this list into the spec. Open as of Pass 0:

- The nine scope decisions in section 6.
- Greenfield code-tree location (deferred to the spec-to-build boundary).
- Whether the build is driven by `subagent-driven-development` or the Workflow harness (the infra-refresh pass decides).
- Multi-account credential and cache layout (Pass 1).
- Whether labels are a poplar-side overlay or strictly server-backed (Pass 1/2).
