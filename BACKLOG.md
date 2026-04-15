# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## High

- [x] **#7** Lipgloss renderer: missing first-level blockquote wrapping `#rendering` `#mailrender` *(2026-04-10)*
  Fixed with two changes: (1) post-parse `wrapImpliedQuotes` wraps unquoted content after a `QuoteAttribution` in a `Blockquote{Level: 1}`, incrementing inner blockquote levels; (2) renderer prefix changed from `strings.Repeat("> ", b.Level)` to `"> "` so structural nesting handles depth without double-counting. Only triggers at top level to avoid compounding. Verified against Yahoo deeply-threaded HTML and plain text emails.

## Someday

- [ ] **#5** Built-in bubbletea compose editor `#poplar` `#v2` *(2026-04-10)*
  Pine-style built-in compose using `bubbles/textarea` for body + custom header fields. Alternative to `$EDITOR` for users who want a seamless, zero-dependency compose experience. Would be a bubbletea showcase piece. Design after external editor flow (Pass 9) is stable.
- [ ] **#6** Neovim companion plugin for poplar `#poplar` `#v2` *(2026-04-10)*
  Email browsing within neovim (folder list, message list, viewer as buffers), telescope pickers, compose integration, poplar command passthrough. Requires IPC/RPC interface in poplar. Design when core client is stable.

## Medium

- [ ] **#10** Evaluate migrating mail backend from aerc fork to emersion ecosystem `#improvement` `#poplar` *(2026-04-15)*
  Emersion maintains a coherent Go mail/DAV library stack — `go-imap`, `go-smtp`, `go-message`, `go-webdav`, `go-vcard` — that would eliminate the aerc fork maintenance burden (`internal/mailworker/`, ADR 0058) and give us a library-based architecture instead of a fork of another TUI's internals. **Critical blocker to investigate first:** emersion has no JMAP client as far as I know, and the Go JMAP landscape is very thin. Options if confirmed: (1) drop JMAP, use IMAP for Fastmail — loses push, efficient delta sync, atomic ops; (2) hybrid with emersion IMAP + aerc JMAP fork — worst of both; (3) find or write a Go JMAP client — big v1-derailing project. First step when picked up: WebFetch pkg.go.dev for `jmap` and check what exists. Emerged during 2026-04-15 CardDAV brainstorm when evaluating `emersion/go-vcard` for the contacts parser. Reverses ADR 0058; worth its own focused brainstorm, not a casual refactor.

- [ ] **#9** Viewer `n/N` walks filtered row set `#feature` `#poplar` *(2026-04-14)*
  While a search filter is committed and the viewer is open, `n/N` should advance to the next/previous message in the filtered row set and fetch its body into the current viewer. Deferred from Pass 2.5b-4 brainstorm (option c). Requires viewer↔msglist cursor coupling, body prefetch semantics, and filter-boundary behavior. **Bundle with Pass 3 (wire to live backend)** — prefetch semantics only become meaningful with real IMAP/JMAP latency.

- [ ] **#8** Design folder jump keybindings without multi-key sequences `#feature` `#poplar` *(2026-04-10)*
  Need single-key alternatives to g-prefix chords (gi/gd/gs/ga/gx/gt) for jumping to Inbox, Drafts, Sent, Archive, Spam, Trash. Bubbletea sends one KeyMsg per keypress — multi-key chords require a custom state machine. Options: single-key mnemonics, command mode `:go inbox`, numeric folder indices, or other approaches. Important for daily triage workflow. Deserves its own pass.

- [x] **#1** Clean up pick-link references from live docs `#improvement` `#docs` *(2026-04-09)*
  Binary was archived but `~/.claude/docs/aerc-setup.md` and `CLAUDE.md` still reference it extensively.
- [x] **#2** Clean up stale pandoc references from docs `#improvement` `#docs` *(2026-04-09)*
  pandoc is no longer part of the project but `~/.claude/docs/aerc-setup.md` still references it in the filter pipeline and compose settings.
- [ ] **#4** Investigate JMAP blob preloading for faster message open `#improvement` `#upstream` *(2026-04-09)*
  New messages are slow to open (~6s) because aerc fetches body blobs lazily from Fastmail on first open. `cache-blobs=true` only helps on second open. Investigate whether aerc's JMAP backend supports blob prefetching (e.g., preload next 2-3 messages) or if this needs an upstream aerc patch.
- [x] **#3** ~~Glamour: hanging indent for wrapped list items~~ `#upstream` `#rendering` *(2026-04-09)*
  Obsolete — glamour dependency removed in Pass 2.5-render (lipgloss migration). List items now rendered directly via lipgloss.
