# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## High

- [ ] **#24** Attachments support (v1 blocker) `#feature` `#poplar` `#v1` *(2026-04-28)*
  poplar v1 needs attachment support end-to-end. Scope spans: backend (JMAP attachment metadata + blob fetch via the existing Download path; equivalent IMAP path when that backend lands), UI (per-row attachment indicator, attachment list/preview in the viewer, save-to-disk action with a path picker or default Downloads dir), and compose (attach files when composing, with size limits and MIME detection). Likely needs one or more dedicated planning + implementation passes — break into smaller backlog items in a future planning pass before starting.

- [ ] **#23** HTML→plain-text fuses words across element boundaries `#bug` `#poplar` *(2026-04-28)*
  Visible in the 1Password Safari update email ("Safari toSafari 18.6", "increasing the  minimum") and Dave Johnson reply ("to:Dave_99504@yahoo.comThanks,Dave Johnson"). The html→markdown converter drops inter-element whitespace when adjacent inline elements (`<br>`, `<a>`, `<span>`) abut without a separating text node — the joined text loses the implicit word boundary the rendered HTML had. Affects readability of any HTML email with non-trivial inline structure. Likely fix in `internal/filter/html.go`: insert a space at element boundaries before tag stripping, or post-process to re-introduce spaces around fused alphanumeric runs. Discovered during 2026-04-28 viewer bug-fix work.

- [x] **#22** ~~Auto-link bare URLs in parsed bodies~~ `#bug` `#poplar` *(2026-04-28)* (closed 2026-04-28)
  `internal/content/parse.go` only emits `Link` spans for markdown `[text](url)`. Bare `https://...` in real bodies renders as plain `Text`, so neither the long-bare-URL footnote path (Pass 2.5b-4b Phase 1) nor the `Tab` link picker fires for messages that aren't markdown-formatted. Need an autolink pass over each text run that tokenizes bare http(s) URLs (and probably `mailto:` / bare email addresses) into `Link{Text: url, URL: url}` so the existing harvest path picks them up. Discovered during Pass 2.5b-4b live tmux verification — viewer body shows bare URLs untouched, `v.links` empty, Tab inert. Fix is parser-local; no harvest or UI changes needed.
  Resolved 2026-04-28: added `splitBareURLs` post-processor in `parseSpans` that splits bare https/http/mailto URLs out of Text runs into Link{Text==URL} spans. Harvest path picks them up unchanged; Tab picker now works on real plain-text emails.

- [x] **#20** ~~SPUA-A cell-width policy: needs robust cross-terminal solution~~ `#bug` `#poplar` `#bubbletea-norms` *(2026-04-27)*
  Resolved 2026-04-27 by ADR-0084 / pass `2026-04-27-spua-cell-width-policy`. Three-mode iconography (`[ui] icons = "auto" | "simple" | "fancy"`, default auto) with sysfont-based Nerd Font detection and CPR cell-width probe. ADR-0079 superseded; ADR-0083 narrowed. New `poplar diagnose` subcommand records the empirical receipt; manual matrix in `docs/poplar/testing/icon-modes.md`.

- [ ] **#17** Migrate AccountTab + Viewer key dispatch to `key.Matches` `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)*
  Pass 4 audit-A3 (remainder). The App.Update slice was migrated in Pass 4; the AccountTab and Viewer handlers still use `switch msg.String()` for ~30 dispatch sites. The conventions doc and ref-apps §3/§8 establish `key.Matches` as the production norm — string switches are invisible to `bubbles/help` integration, prevent `Enabled()`-gated bindings, and block any future rebinding feature. Suggested approach: introduce an `AccountKeys` struct in `keys.go` (parallel to `GlobalKeys`) and a `ViewerKeys` struct, wire them through the model constructors, replace each dispatch chain. Should land as a dedicated structural-cleanup pass; sized at ~3-4 commits when broken up by component. See audit `docs/poplar/audits/2026-04-26-bubbletea-conventions.md` finding A3.

- [x] **#7** Lipgloss renderer: missing first-level blockquote wrapping `#rendering` `#mailrender` *(2026-04-10)*
  Fixed with two changes: (1) post-parse `wrapImpliedQuotes` wraps unquoted content after a `QuoteAttribution` in a `Blockquote{Level: 1}`, incrementing inner blockquote levels; (2) renderer prefix changed from `strings.Repeat("> ", b.Level)` to `"> "` so structural nesting handles depth without double-counting. Only triggers at top level to avoid compounding. Verified against Yahoo deeply-threaded HTML and plain text emails.

## Someday

- [ ] **#21** View raw message content `#feature` `#poplar` `#v2` *(2026-04-28)*
  Toggle in viewer to show the unparsed RFC822 source — headers, MIME structure, raw HTML/text body — instead of the rendered block view. Diagnostic / power-user feature; useful for debugging filter pipeline regressions and inspecting what the server actually sent. Post-1.0.

- [ ] **#19** Refactor `App.View` to trust `AccountTab.View` line widths `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)*
  Pass 4 audit-A10. `App.View` currently iterates every line of `m.acct.View()` to measure and pad it before appending the right border — parent-side post-processing on child output. The conventions doc lists this as an anti-pattern (§8). Once #17 lands and AccountTab fully honors its width contract, App.View can append the border without per-line measurement. Land after #17. See audit `docs/poplar/audits/2026-04-26-bubbletea-conventions.md` finding A10.

- [ ] **#18** Replace zero-latency intra-model `tea.Cmd` signals with direct delegation `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)*
  Pass 4 audit-A9. `viewerOpenedCmd` / `viewerClosedCmd` / `viewerScrollCmd` / `folderChangedCmd` emit messages from AccountTab to App as zero-latency tea.Cmds. The bubbletea source explicitly flags this as an anti-pattern (`tea.go:62-64`): "there's almost never a reason to use a command to send a message to another part of your program. That can almost always be done in the update function." Refactor: App.Update inspects AccountTab state directly after delegation (e.g. expose `IsViewerOpen()` / `SelectedFolderInfo()` on AccountTab) and updates chrome fields without a Cmd round-trip. Removes one frame of lag from the footer/status bar. See audit `docs/poplar/audits/2026-04-26-bubbletea-conventions.md` finding A9.

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

- [x] **#16** ~~Sidebar rows mis-sized: Nerd Font SPUA-A icons render double-width but `lipgloss.Width` reports 1~~ `#bug` `#poplar` *(2026-04-26)*
  Resolved 2026-04-26 by Pass 4 audit-A1. New `displayCells` helper in `internal/ui/iconwidth.go` corrects the SPUA-A undercount (+1 per U+F0000–U+FFFFD codepoint); `fillRowToWidth`, sidebar `leftWidth`, and the message-list flag column now use it. Flag column bumped from 1 to 2 cells with a matching no-flag pad. See audit `docs/poplar/audits/2026-04-26-bubbletea-conventions.md` finding A1.
  Re-audit 2026-04-27: the original "1-cell undercount" framing was workstation-specific (kitty + JetBrainsMonoNL + symbol_map), not universal as ADR-0079 claimed. The `displayCells +1` fix landed here was an over-correction whose visible defect was masked until Pass 4.1 F2. ADR-0084 replaces the static rule with a runtime probe; see that ADR's "Context" section for the institutional record.

- [ ] **#12** Pass 9.5 prereq: collapse `internal/tidy/` to drop CLI machinery `#improvement` `#poplar` *(2026-04-25)*
  Audit-2 verdict on `internal/tidy/` was **collapse** — the core algorithm (`SplitQuoted`/`Reassemble`/`BuildPrompt`/`CallAPI`/`Tidy`) fits the Pass 9.5 compose consumer well, but the package carries CLI ergonomics from a previous standalone-binary lineage that won't fit poplar's unified config. When Pass 9.5 lands, delete `LoadConfig`, `ApplyRuleOverrides`, `ApplyStyleOverrides`, `ConfigString`, `ResolveAPIKey`, and their tests (~100–150 LOC across source + tests). Move any surviving validators next to the unified config decode site in `internal/config/`. Optionally unexport `CallAPI` and `BuildPrompt` if no test reaches them after the trim. Goal: tidy exposes only `Config`, `DefaultConfig()`, `Tidy()`, `Result`, and the status constants. **Don't pre-emptively collapse** — wait until Pass 9.5 surfaces concrete needs that may reshape the trim. Findings: `docs/poplar/audits/2026-04-25-library-packages-findings.md`.

- [x] **#11** ~~Pass 3 prereq: MIME-aware body fetch for filter dispatch~~ `#improvement` `#poplar` *(2026-04-25)*
  Resolved 2026-04-26 by Pass 3 commit `e948edd` ("MIME-aware body fetch + Email/get state probe"). `loadBodyCmd` in `internal/ui/cmds.go` now sniffs the body format (`looksLikeRFC822`) and walks MIME parts via `extractDisplayText` before handing off to `content.ParseBlocks`. Mock backend still returns markdown; real JMAP path uses the new sniff/walk.

- [x] **#10** ~~Evaluate migrating mail backend from aerc fork to emersion ecosystem~~ `#improvement` `#poplar` *(2026-04-15)*
  Resolved 2026-04-25 by Pass 2.9 research and ADR-0075. The "Go JMAP landscape too thin" premise was wrong — `rockorager/go-jmap` covers the full JMAP surface and is already a dep. Adopting direct-on-libraries: `emersion/go-imap` v1 + `rockorager/go-jmap`, with `emersion/go-smtp/webdav/vcard` queued for later passes. ADR-0075 supersedes 0002, 0006, 0008, 0010, 0012. Research: `docs/poplar/research/2026-04-25-mail-library-stack.md`.

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
