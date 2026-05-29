# Poplar Rebuild: Mail Client Gap Analysis

**Date:** 2026-05-29
**Purpose:** Establish what a complete, modern mail client does, so the poplar rebuild spec is grounded in the field rather than in poplar's own current behavior. This is the raw material for the domain spec passes.

## Method

Eight parallel surveys ran against the comparators the project weights: Thunderbird, Apple Mail, Outlook (classic and new), mutt/neomutt, aerc, K-9 / Thunderbird Android, Geary, Evolution, alpine/pine, plus the Fastmail and Gmail web clients as convention-setters. The terminal three (mutt, aerc, alpine) are the closest analogues. A ninth survey inventoried poplar's current behavior from its code, tests, ADRs, and docs. Claims were web-verified with sources; the full per-client matrices and citations live in the survey transcripts. This document distills the decision-relevant findings.

## How to read the matrices

Columns: **TB** Thunderbird, **aerc** aerc, **NM** neomutt (with notmuch where noted), **FM** Fastmail web, **Gm** Gmail web, **Pop** poplar today. Cells: Y = built-in, ~ = partial or external-tool, N = absent.

---

## 1. Accounts, protocols, sync

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| Multiple accounts | Y | Y | Y | Y | Y | N |
| Unified inbox | Y | N | N | Y | N | N |
| JMAP | ~ | Y | N | Y | N | Y |
| IMAP | Y | Y | Y | Y | Y | Y |
| OAuth2 built-in | Y | Y | ~ | Y | Y | Y |
| Offline cache | Y | Y | ~ | ~ | ~ | Y |
| IDLE / push | Y | Y | Y | Y | Y | Y |
| Setup wizard | Y | Y | N | Y | Y | Y |
| Alias auto-select on reply | Y | Y | ~ | Y | Y | ~ |

**What a complete client needs:** IMAP and JMAP as coequal first-class protocols; OAuth2 built in (Google disabled basic auth in 2025, Microsoft in 2026); offline-first durable cache with compose queuing; IDLE on the selected folder plus polling for the rest; per-account identities with alias auto-selection on reply.

**Poplar today:** Single account in v1. JMAP and IMAP both native, OAuth2 built in (PKCE loopback plus device-code), per-account SQLite cache, two-connection IMAP with IDLE, TUI wizard, identities with reply matching.

**Gap (lacks):** Multiple accounts and a unified inbox. This is the single biggest gap and it is load-bearing: it shapes the cache schema, the backend layer, and the sidebar.

**Lead opportunity:** JMAP-native from the ground up (aerc is the only other TUI with native JMAP).

---

## 2. Organization, threading, automation

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| Folders | Y | Y | Y | Y | labels | Y |
| Labels / tags (multi-membership) | Y | ~ (notmuch) | ~ (notmuch) | Y | Y | N |
| Thread grouping | Y | Y | Y | Y | Y | Y |
| Thread mute / ignore | Y | N | ~ | ~ | Y | N |
| Virtual folders / saved searches | Y | Y (notmuch) | Y (notmuch) | ~ web | N | N |
| Server-side filters (Sieve) | N | N | N | Y | Y | N |
| Snooze | N | N | N | Y | Y | N |
| Archive vs delete distinct | Y | Y | ~ | Y | Y | Y |
| Bulk select by criteria | Y | Y | Y | Y | Y | ~ (visual) |

**What a complete client needs:** dual organization primitives (folders plus multi-label tagging); thread mute that routes future replies out of the inbox; saved searches as persistent queryable views; server-side rules with a client-visible config surface (so rules run while the client is closed); bulk operations anchored to search results, not just the visible page.

**Poplar today:** Folders only, no labels. Thread grouping with fold. Visual-select bulk mode. Retention sweep. No saved searches, no filters, no snooze, no mute.

**Gap (lacks):** labels/tags, saved searches/virtual folders, filters (Sieve for JMAP), snooze, thread mute. These are the core power-user features of the field.

**Decision forced:** folders-only vs folders + labels is foundational. JMAP supports mailbox multi-membership, which maps cleanly to a label model.

---

## 3. Reading, triage, navigation

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| Single-key triage from list | ~ | Y | Y | Y | Y | Y |
| Pattern-based bulk select | ~ | Y | Y | ~ | Y | ~ |
| Next-unread crosses folders | Y | Y | Y | ~ | ~ | N |
| Thread mute with archive semantics | Y | N | ~ | ~ | Y | N |
| Undo window for destructive acts | Y | N | ~ | Y | Y | Y |
| Reading pane | Y | tab | pager | Y | Y | one-pane |
| Customizable keybindings | ~ | Y | Y | N | N | N |

**What a complete client needs:** single-key triage from the list; pattern bulk selection ("select all unread", "all from sender"); next-unread that advances into the next folder with unread; mute with archive semantics; a meaningful undo window.

**Poplar today:** Single-key triage (d/a/s/./m), visual-select mode, undo window (6s default, one pending action), one-pane Pine-style reader, mouse wired. Single-key bindings only, no modifiers or sequences (a deliberate constraint).

**Gap (lacks):** next-unread across folders; pattern-based selection beyond manual visual mode; mute.

**Lead opportunity:** the constraint of modifier-free single keys is a distinctive, Pine-true design. Keep it. The strongest field model is neomutt's tag-pattern plus apply, adapted to single keys.

---

## 4. Message rendering and display

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| HTML approach | Gecko | w3m dump | mailcap/w3m | browser | sanitized DOM | HTML->markdown->blocks |
| Plain-vs-HTML default | HTML | plain | configurable | HTML | HTML | plain |
| Remote-image block by default | Y | Y (sandbox) | N | block+proxy | proxy+load | strip entirely |
| Quote folding | Y | ~ color | N | Y | Y | N |
| Table rendering | full | ASCII | ASCII | full | full | GFM pipe / flatten |
| Link extraction (footnotes) | N | Y (w3m) | Y (w3m) | N | N | Y |
| Code / syntax highlight | sender CSS | ~ diffs | N | mono | mono | compose only |
| Reflow to width | n/a | Y | Y | n/a | n/a | 72-col cap |

**Key finding:** poplar's HTML to markdown to block-model to lipgloss pipeline is *more* structured than the terminal field, which shells out to w3m/lynx for a lossy text dump. The markdown reduction is defensible and arguably best-in-class for structure. The real issues are conversion fidelity (layout-table flattening, image-alt handling) and hardcoded constants (72-col body cap, 30-cell URL threshold).

**What good rendering requires:** faithful HTML is impossible to fully reproduce in a character grid, so the wins are: block remote content by default (poplar already strips it, which is private-by-default); first-class link handling (the survey called clickable-link extraction "the unsolved problem in terminal rendering"); code-block fidelity (preserve `<pre>`, monospace, with scroll or wrap); and a clear plain-vs-HTML policy.

**Gap / lead:** add syntax highlighting in the reader (chroma is already wired in compose), patch/diff-aware rendering (aerc colorizes `[PATCH]`; a coder's client should render `git format-patch` natively), and push link handling further (copy, launch, per-link actions). Poplar is already ahead on link footnotes.

**Direction for the rendering pass:** keep markdown, raise fidelity, lock a rendering contract against a golden corpus, add dev-first code/patch/link features. A runtime LLM-cleans-HTML feature is a deferred opt-in, not the default path.

---

## 5. Composing and sending

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| Editor | rich | external/embedded | external | rich | rich | catkin (markdown) |
| Markdown -> multipart/alt | N | Y | N | N | N | Y |
| Signatures multi / per-identity | Y | Y | Y | Y | Y | Y |
| Templates / snippets | Y | Y | ~ | N | Y | N |
| Attachment reminder | ~ | Y | Y | N | N | N |
| Drafts server-sync | Y | Y | Y | Y | Y | Y |
| Send-later / scheduled | ~ | N | N | Y | Y | Y |
| Undo-send | ~ | N | N | Y | Y | Y |
| Address autocomplete | Y | Y | Y | Y | Y | Y |
| Identity auto-select on reply | Y | Y | ~ | Y | Y | Y |

**What a complete client needs:** identity management with auto-selection on reply; an attachment safety net (keyword reminder); drafts with server sync and safe resume; a send delay with a cancel path; pluggable address autocomplete.

**Poplar today:** Strong here. Catkin markdown editor with live styling, syntax-highlighted code, undo/redo, find/replace, spellcheck, AI tidy. Markdown to multipart/alternative on send. Per-identity signatures, drafts autosave plus server push, send-later, undo-send, recency-decayed autocomplete.

**Gap (lacks):** templates/snippets; attachment-forgotten reminder. Both are small, high-value adds.

**Lead opportunity:** markdown-native compose, syntax highlighting, and AI tidy are ahead of the field. Keep them central. Tidy folds into catkin (no separate package).

---

## 6. Search

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| Local full-text index | Y | ~ (notmuch) | ~ (notmuch) | server | server | Y (FTS5) |
| Typed operators | ~ | ~ | Y | Y | Y | ~ |
| Date / size operators | Y | ~ | Y | Y | Y | N |
| Saved searches | Y | Y (notmuch) | Y (notmuch) | Y web | N | N |
| Scope toggle (folder/account/all) | Y | ~ | Y | Y | ~ | ~ (folder/all) |
| Search-as-you-type | ~ | N | N | Y | Y | N |

**What a complete client needs:** a local full-text index (server IMAP SEARCH is too slow and inconsistent, and offline needs a local index); typed operators at minimum (`from:`, `subject:`, `has:attachment`, `before:`/`after:`, `is:read`); saved searches as first-class virtual folders; a scope toggle; search-as-you-type with operator suggestions.

**Poplar today:** FTS5 local index, operators (`from:`/`to:`/`cc:`/`subject:`/`in:`/`has:attachment`), folder vs all-folders scope, next/prev match. No date/size operators, no saved searches, no search-as-you-type.

**Gap (lacks):** date/size operators, saved searches (ties to virtual folders in domain 2), search-as-you-type.

---

## 7. Contacts, calendar, security

| Feature | TB | aerc | NM | FM | Gm | Pop |
|---|---|---|---|---|---|---|
| CardDAV contacts | Y | ~ (khard) | ~ (khard) | Y | Y | Y |
| Auto-collect from sent | Y | N | N | N | Y | ~ (history) |
| ICS invite display | Y | Y | ~ | N | Y | Y (display only) |
| RSVP from email | Y | Y | ~ | N | Y | N |
| PGP / OpenPGP | Y | Y | Y | N | N | N |
| S/MIME | Y | N | Y | N | ~ | N |
| DKIM/DMARC display | ~ | N | N | ~ | ~ | N |
| List-Unsubscribe one-click | ~ | N | N | ~ | Y | Y |

**What a complete client needs:** CardDAV sync with multiple books and auto-collect; inline ICS with one-action RSVP; PGP plus S/MIME with integrated key handling; a sender-verification badge that surfaces DMARC/DKIM result; List-Unsubscribe one-click as a first-class affordance.

**Poplar today:** CardDAV ingest and write-back, contact popover, full contacts mode, ICS display (no RSVP), List-Unsubscribe one-click. No PGP, no S/MIME, no DKIM/DMARC display, no RSVP.

**Gap (lacks):** RSVP for invites; sender-verification display; PGP/S-MIME (a large undertaking, likely a scope decision rather than an automatic include).

**Already ahead:** List-Unsubscribe one-click, which most terminal clients lack.

---

## Cross-cutting decisions the gaps force

These resolve in Pass 0 or the named domain pass.

1. **Multi-account and unified inbox** (Pass 1, but foundational): the largest gap; reshapes cache, backend, sidebar.
2. **Folders-only vs folders + labels/tags** (Pass 1/2, foundational): JMAP multi-membership maps to labels; affects the data model.
3. **Saved searches / virtual folders** (Pass 2 and 6): ties organization and search together.
4. **Server-side filters (Sieve)** (Pass 2): the JMAP-correct answer to rules.
5. **Snooze and thread mute** (Pass 2/3): daily-driver staples.
6. **Rendering direction** (Pass 4): keep markdown, raise fidelity, golden corpus plus AI loop, dev-first features.
7. **RSVP and sender-verification display** (Pass 7).
8. **PGP / S-MIME** (Pass 7): scope decision; large.

## Coder-first differentiators to lean into

- Syntax highlighting in the reader (not just compose).
- Patch / diff-aware rendering for `git format-patch` mail.
- First-class link handling (copy, launch, per-link actions).
- Markdown-native compose with AI tidy (already distinctive).
- JMAP-native architecture.
- Modifier-free single-key Pine-true keyboard model.
