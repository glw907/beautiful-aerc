# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## High

- [x] **#7** Lipgloss renderer: missing first-level blockquote wrapping `#rendering` `#mailrender` *(2026-04-10)*
  Fixed with two changes: (1) post-parse `wrapImpliedQuotes` wraps unquoted content after a `QuoteAttribution` in a `Blockquote{Level: 1}`, incrementing inner blockquote levels; (2) renderer prefix changed from `strings.Repeat("> ", b.Level)` to `"> "` so structural nesting handles depth without double-counting. Only triggers at top level to avoid compounding. Verified against Yahoo deeply-threaded HTML and plain text emails.

## Someday

- [ ] **#15** Help popover: responsive layout for narrow terminals `#improvement` `#poplar` *(2026-04-25)*
  Popover has a fixed natural width derived from content (~62 cols account, ~58 viewer). On terminals narrower than the popover, `lipgloss.Place` clips gracefully but the layout breaks visually. A future polish pass could reflow into a single column or drop columns based on terminal width. Surfaced during Pass 2.5b-5; out of scope per ADR-0071.

- [ ] **#14** Help popover: background dim for the underlying view `#improvement` `#poplar` *(2026-04-25)*
  Wireframe (§5) called for dimmed content behind the popover. Skipped in Pass 2.5b-5 because lipgloss has no native opacity and ANSI-level color stripping of the underlying view is fragile (ADR-0071). Revisit if user testing flags the no-dim approach as confusing. Implementation paths: (1) hand-roll a "dim every fg color" transform on the rendered chrome+content before composing under the popover; (2) wait for an upstream lipgloss dim helper.

- [ ] **#13** Drop dead `blockKind` / `spanKind` enums from `internal/content/` `#improvement` `#poplar` *(2026-04-25)*
  The `Block` and `Span` interfaces require unexported marker methods (`blockType() blockKind`, `spanType() spanKind`) returning private kind constants. The sealed-sum-type pattern. Consumers never inspect the kind values — discrimination always happens via Go type switches. The enum constants and the kind return values are compile-but-unused machinery (~30 LOC). Reduce the marker methods to no-args (`isBlock()`, `isSpan()`) and delete the kindParagraph...,kindText... constants. Audit-2 explicitly notes this is **not blocking** and should ride along with the next pass that touches `internal/content/` — no dedicated commit.

- [ ] **#5** Built-in bubbletea compose editor `#poplar` `#v2` *(2026-04-10)*
  Pine-style built-in compose using `bubbles/textarea` for body + custom header fields. Alternative to `$EDITOR` for users who want a seamless, zero-dependency compose experience. Would be a bubbletea showcase piece. Design after external editor flow (Pass 9) is stable.
- [ ] **#6** Neovim companion plugin for poplar `#poplar` `#v2` *(2026-04-10)*
  Email browsing within neovim (folder list, message list, viewer as buffers), telescope pickers, compose integration, poplar command passthrough. Requires IPC/RPC interface in poplar. Design when core client is stable.

## Medium

- [ ] **#12** Pass 9.5 prereq: collapse `internal/tidy/` to drop CLI machinery `#improvement` `#poplar` *(2026-04-25)*
  Audit-2 verdict on `internal/tidy/` was **collapse** — the core algorithm (`SplitQuoted`/`Reassemble`/`BuildPrompt`/`CallAPI`/`Tidy`) fits the Pass 9.5 compose consumer well, but the package carries CLI ergonomics from a previous standalone-binary lineage that won't fit poplar's unified config. When Pass 9.5 lands, delete `LoadConfig`, `ApplyRuleOverrides`, `ApplyStyleOverrides`, `ConfigString`, `ResolveAPIKey`, and their tests (~100–150 LOC across source + tests). Move any surviving validators next to the unified config decode site in `internal/config/`. Optionally unexport `CallAPI` and `BuildPrompt` if no test reaches them after the trim. Goal: tidy exposes only `Config`, `DefaultConfig()`, `Tidy()`, `Result`, and the status constants. **Don't pre-emptively collapse** — wait until Pass 9.5 surfaces concrete needs that may reshape the trim. Findings: `docs/poplar/audits/2026-04-25-library-packages-findings.md`.

- [ ] **#11** Pass 3 prereq: MIME-aware body fetch for filter dispatch `#improvement` `#poplar` *(2026-04-25)*
  Today `internal/ui/cmds.go::loadBodyCmd` reads `mail.Backend.FetchBody` bytes and pipes them straight into `content.ParseBlocks`, which expects pre-cleaned markdown. The mock backend hides the gap by handing back markdown. Real IMAP/JMAP bodies are `text/html` or raw `text/plain` and must run through `filter.CleanHTML` / `filter.CleanPlain` before `ParseBlocks`. The `mail.Backend.FetchBody(uid) (io.Reader, error)` signature has no MIME signal for the call site to dispatch on. Pass 3 needs either (a) a signature change to return MIME alongside the body, or (b) a sniff helper (`detectHTML` already exists privately in `filter/plain.go` — could be promoted). Surfaced by Audit-2; findings at `docs/poplar/audits/2026-04-25-library-packages-findings.md`.

- [ ] **#10** Evaluate migrating mail backend from aerc fork to emersion ecosystem `#improvement` `#poplar` *(2026-04-15)*
  Emersion maintains a coherent Go mail/DAV library stack — `go-imap`, `go-smtp`, `go-message`, `go-webdav`, `go-vcard` — that would eliminate the aerc fork maintenance burden (`internal/mailworker/`, ADR 0058) and give us a library-based architecture instead of a fork of another TUI's internals. **Critical blocker to investigate first:** emersion has no JMAP client as far as I know, and the Go JMAP landscape is very thin. Options if confirmed: (1) drop JMAP, use IMAP for Fastmail — loses push, efficient delta sync, atomic ops; (2) hybrid with emersion IMAP + aerc JMAP fork — worst of both; (3) find or write a Go JMAP client — big v1-derailing project. First step when picked up: WebFetch pkg.go.dev for `jmap` and check what exists. Emerged during 2026-04-15 CardDAV brainstorm when evaluating `emersion/go-vcard` for the contacts parser. Reverses ADR 0058; worth its own focused brainstorm, not a casual refactor.

- [ ] **#9** Viewer `n/N` walks filtered row set `#feature` `#poplar` *(2026-04-14)*
  While a search filter is committed and the viewer is open, `n/N` should advance to the next/previous message in the filtered row set and fetch its body into the current viewer. Deferred from Pass 2.5b-4 brainstorm (option c). Requires viewer↔msglist cursor coupling, body prefetch semantics, and filter-boundary behavior. **Bundle with Pass 3 (wire to live backend)** — prefetch semantics only become meaningful with real IMAP/JMAP latency.

- [x] **#8** ~~Design folder jump keybindings without multi-key sequences~~ `#feature` `#poplar` *(2026-04-10)*
  Resolved 2026-04-25 — design call landed as uppercase single-key mnemonics (I/D/S/A/X/T), codified in invariants U5 and the help popover wireframe. Wiring is bundled into Pass 2.5b-4.5 per Audit-3.

- [x] **#1** Clean up pick-link references from live docs `#improvement` `#docs` *(2026-04-09)*
  Binary was archived but `~/.claude/docs/aerc-setup.md` and `CLAUDE.md` still reference it extensively.
- [x] **#2** Clean up stale pandoc references from docs `#improvement` `#docs` *(2026-04-09)*
  pandoc is no longer part of the project but `~/.claude/docs/aerc-setup.md` still references it in the filter pipeline and compose settings.
- [ ] **#4** Investigate JMAP blob preloading for faster message open `#improvement` `#upstream` *(2026-04-09)*
  New messages are slow to open (~6s) because aerc fetches body blobs lazily from Fastmail on first open. `cache-blobs=true` only helps on second open. Investigate whether aerc's JMAP backend supports blob prefetching (e.g., preload next 2-3 messages) or if this needs an upstream aerc patch.
- [x] **#3** ~~Glamour: hanging indent for wrapped list items~~ `#upstream` `#rendering` *(2026-04-09)*
  Obsolete — glamour dependency removed in Pass 2.5-render (lipgloss migration). List items now rendered directly via lipgloss.
