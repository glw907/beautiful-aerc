# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## High

- [x] **#25** ~~Some emails render with no body text~~ `#bug` `#poplar` *(2026-04-29)* (closed 2026-04-29)
  Resolved 2026-04-29: root cause was a missing `_ "github.com/emersion/go-message/charset"` blank import in `internal/ui/cmds.go`. go-message's charset registry only carries UTF-8 by default; MIME parts declaring `charset="iso-8859-1"` (Outlook/Exchange default) failed to decode and `mr.NextPart()` errored on the first part, exiting the extraction loop with both plain and html unset. The blank import registers all standard email charsets (iso-8859-1, windows-1252, koi8-r, gb2312, shift_jis, big5, ...) side-effectfully.

- [ ] **#29** Config template includes a placeholder `[[account]]` that fails validation `#bug` `#poplar` `#config` *(2026-05-03)*
  `config.Template()` writes an example `[[account]]` block with `provider = "fastmail"`, `email = "you@yourdomain.com"`, `password-cmd = "op read ..."` but no `name` field. The validator at `internal/config/accounts.go:74` requires `name` (empty → `account 0: name is required`), so any user who edits only `email`/`password-cmd` (the obvious customizations from the inline comments) still hits a hard error. Two fixes in scope: (a) make `name` truly optional and default to `email` (the inline comment already promises this); or (b) include `name = "Your Name"` in the template block. Prefer (a) — simpler config, matches the documented behavior. Touches `internal/config/accounts.go` (drop the empty-name check, set `cfg.Name = e.Email` when blank) and the matching test in `accounts_test.go`.

- [ ] **#27** First-run setup wizard (v1) `#feature` `#poplar` `#config` `#v1` *(2026-05-02, extended 2026-05-03)* — **scheduled: Pass 9.6 (must land before beta soak / Pass 11)**
  Interactive in-TUI flow for new users to configure their first account. Builds on Pass 8.5's config infrastructure: pick provider from a list, prompt for email, prompt for password (or detect a secret manager and offer to use it), test the connection, write the [[account]] block into config.toml. Pass 8.5 ships the self-documenting TOML and template; the wizard is a follow-on pass that makes onboarding smooth without leaving the terminal. Bundled into Pass 9.6 so it can reuse Pass 9's editor primitives for the input fields.
  **Extension (2026-05-03):** the wizard handles the *missing* config case, but the *malformed/incomplete* case also needs work before beta. Today the validator emits terse messages like `account 0: name is required` with no pointer to which file/line is wrong or how to fix it. Wizard-adjacent scope: when `config.toml` exists but fails to decode or validate, surface a helpful error that names the offending field, shows the resolved config path, and (where possible) suggests the fix or offers to re-run the wizard for the broken account. Should land in the same pass as the wizard so the entire first-launch UX — missing, malformed, valid — is coherent before Pass 11 freeze.

- [ ] **#24** Attachments support (v1 blocker) `#feature` `#poplar` `#v1` *(2026-04-28)* — **scheduled: Pass 8.6 (backend) + 8.7 (viewer) + 9.5 (compose-side attach)**
  poplar v1 needs attachment support end-to-end. Scope spans: backend (JMAP attachment metadata + blob fetch via the existing Download path; equivalent IMAP path when that backend lands), UI (per-row attachment indicator, attachment list/preview in the viewer, save-to-disk action with a path picker or default Downloads dir), and compose (attach files when composing, with size limits and MIME detection). Split across three passes: 8.6 backend (JMAP+IMAP fetch path), 8.7 viewer (indicator, list/preview, save-to-disk), 9.5 compose-side (attach files when sending). Each pass plans its own task breakdown.

- [x] **#23** ~~HTML→plain-text fuses words across element boundaries~~ `#bug` `#poplar` *(2026-04-28)* (closed 2026-05-02)
  Resolved 2026-05-02 by commit `9174f85`. `prepareHTML` now calls `inlineBoundaryPad` (`internal/filter/html.go`) which surrounds the inline boundary tag set (`a`, `b`, `code`, `em`, `i`, `span`, `strong`, `u`) with spaces and replaces `<br>` with a single space before tag stripping; the existing `normalizeWhitespace` collapses the resulting runs. Regression cases live in `TestCleanHTML_InlineBoundaryFusion` and `TestCleanHTML_FusionFixtures` (real fixtures: `safari-update-fragment.html`, `dave-johnson-fragment.html`). Pass 8.5d closeout: backlog only.

- [x] **#22** ~~Auto-link bare URLs in parsed bodies~~ `#bug` `#poplar` *(2026-04-28)* (closed 2026-04-28)
  `internal/content/parse.go` only emits `Link` spans for markdown `[text](url)`. Bare `https://...` in real bodies renders as plain `Text`, so neither the long-bare-URL footnote path (Pass 2.5b-4b Phase 1) nor the `Tab` link picker fires for messages that aren't markdown-formatted. Need an autolink pass over each text run that tokenizes bare http(s) URLs (and probably `mailto:` / bare email addresses) into `Link{Text: url, URL: url}` so the existing harvest path picks them up. Discovered during Pass 2.5b-4b live tmux verification — viewer body shows bare URLs untouched, `v.links` empty, Tab inert. Fix is parser-local; no harvest or UI changes needed.
  Resolved 2026-04-28: added `splitBareURLs` post-processor in `parseSpans` that splits bare https/http/mailto URLs out of Text runs into Link{Text==URL} spans. Harvest path picks them up unchanged; Tab picker now works on real plain-text emails.

- [x] **#20** ~~SPUA-A cell-width policy: needs robust cross-terminal solution~~ `#bug` `#poplar` `#bubbletea-norms` *(2026-04-27)*
  Resolved 2026-04-27 by ADR-0084 / pass `2026-04-27-spua-cell-width-policy`. Three-mode iconography (`[ui] icons = "auto" | "simple" | "fancy"`, default auto) with sysfont-based Nerd Font detection and CPR cell-width probe. ADR-0079 superseded; ADR-0083 narrowed. New `poplar diagnose` subcommand records the empirical receipt; manual matrix in `docs/poplar/testing/icon-modes.md`.

- [x] **#17** ~~Migrate AccountTab + Viewer key dispatch to `key.Matches`~~ `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)* (closed 2026-05-02)
  Resolved 2026-04-30 by Pass 5 commit `ec0984a` ("Migrate UI key dispatch to key.Matches with per-component KeyMaps"). `AccountKeys` + `ViewerKeys` structs in `internal/ui/keys.go`, threaded through component constructors. Pass 8.2 closeout: backlog only — no code change, the work shipped in Pass 5.

- [x] **#7** Lipgloss renderer: missing first-level blockquote wrapping `#rendering` `#mailrender` *(2026-04-10)*
  Fixed with two changes: (1) post-parse `wrapImpliedQuotes` wraps unquoted content after a `QuoteAttribution` in a `Blockquote{Level: 1}`, incrementing inner blockquote levels; (2) renderer prefix changed from `strings.Repeat("> ", b.Level)` to `"> "` so structural nesting handles depth without double-counting. Only triggers at top level to avoid compounding. Verified against Yahoo deeply-threaded HTML and plain text emails.

## Someday

- [ ] **#21** View raw message content `#feature` `#poplar` `#v2` *(2026-04-28)* — **scheduled: Pass 1.2 (post-v1)**
  Toggle in viewer to show the unparsed RFC822 source — headers, MIME structure, raw HTML/text body — instead of the rendered block view. Diagnostic / power-user feature; useful for debugging filter pipeline regressions and inspecting what the server actually sent. Post-1.0.

- [x] **#19** ~~Refactor `App.View` to trust `AccountTab.View` line widths~~ `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)* (closed 2026-05-02)
  Resolved 2026-05-01 by Pass 5 commit `2b520c9` ("Trust AccountTab.View width contract; drop App per-line padding"). Per-line measure-and-pad loop in `App.renderFrame` replaced with direct border append; covered by `TestAccountTabView_HonorsAssignedWidth`. Pass 8.2 closeout: backlog only.

- [x] **#18** ~~Replace zero-latency intra-model `tea.Cmd` signals with direct delegation~~ `#improvement` `#poplar` `#bubbletea-norms` *(2026-04-26)* (closed 2026-05-02)
  Resolved 2026-04-30 by Pass 5 commit `1797927` ("Replace intra-model Cmd signals with delegate-then-read accessors"). `FolderChangedMsg` / `ViewerOpenedMsg` / `ViewerClosedMsg` / `ViewerScrollMsg` / `LinkPickerOpenMsg` removed; `App.deriveChromeFromAcct` reads `AccountTab.ViewerOpen()`, `SelectedFolderCounts()`, `ViewerScrollPct()`, `SearchState()`, `LinkPickerRequest()` after each delegation. Pass 8.2 closeout: backlog only.

- [x] **#15** ~~Help popover: responsive layout for narrow terminals~~ `#improvement` `#poplar` *(2026-04-25)* (closed 2026-05-01)
  Resolved 2026-05-01 by Pass 7 / ADR-0097. 80×24 is the design polish bar; sub-80 widths handled by `HelpPopover.Box`'s existing `tooNarrow` fallback. The popover's natural width (≤62 account, ≤58 viewer) fits within the message-list pane at 80 cols once the sidebar narrows (ADR-0096). Pass 7 also resolved the underlying drift (threaded-row date clipping at 80×24) by introducing the responsive sidebar.

- [ ] **#14** Help popover: background dim for the underlying view `#improvement` `#poplar` *(2026-04-25)* — **scheduled: Pass 10 (decide v1 vs defer)**
  Wireframe (§5) called for dimmed content behind the popover. Skipped in Pass 2.5b-5 because lipgloss has no native opacity and ANSI-level color stripping of the underlying view is fragile (ADR-0071). Revisit if user testing flags the no-dim approach as confusing. Implementation paths: (1) hand-roll a "dim every fg color" transform on the rendered chrome+content before composing under the popover; (2) wait for an upstream lipgloss dim helper.

- [x] **#13** ~~Drop dead `blockKind` / `spanKind` enums from `internal/content/`~~ `#improvement` `#poplar` *(2026-04-25)* (closed 2026-05-03)
  Resolved 2026-05-03 by Pass 8.5d / ADR-0131. Marker methods reduced to `isBlock()` / `isSpan()` (no return); `blockKind` / `spanKind` types and their `kind*` constants deleted. `parse_test.go` now asserts block sequences via `[]string` of concrete type names (`fmt.Sprintf("%T", b)`) — discrimination at non-test call sites was already type-switch based. ~30 LOC removed.

- [ ] **#5** Catkin: built-in bubbletea compose editor `#poplar` `#v1` *(2026-04-10)* — **scheduled: Pass 9.5**
  Pine-style built-in compose using `bubbles/textarea` for body + custom header fields. Alternative to `$EDITOR` for users who want a seamless, zero-dependency compose experience. Would be a bubbletea showcase piece. Lands in Pass 9.5 after Pass 9's external `$EDITOR` flow is stable; this is the "Catkin" editor referenced in the pass roadmap.
- [ ] **#6** Neovim companion plugin for poplar `#poplar` `#v2` *(2026-04-10)* — **scheduled: Pass 1.1 (post-v1)**
  Email browsing within neovim (folder list, message list, viewer as buffers), telescope pickers, compose integration, poplar command passthrough. Requires IPC/RPC interface in poplar. Design when core client is stable.

## Medium

- [ ] **#30** Render cache for `Sidebar.View()` `#improvement` `#poplar` *(2026-05-03)*
  `Sidebar.View()` rebuilds the full folder list on every `tea.Msg` (60–80 lipgloss renders per frame at typical folder counts). `AccountTab.View()` runs on every Msg, so during spinner-active periods the rebuild fires continuously. Apply the same `*cache` pointer + dirty flag pattern as `movePickerCache` and `helpPopoverCache` (Pass 8.5c). Cache key: dirty flag set by `SetFolders` / `SetSize` / `SetLayout` / `MoveUp` / `MoveDown` / `MoveToTop` / `MoveToBottom` / `SelectByCanonical`. Out of scope for Pass 8.5c (limited to MovePicker + HelpPopover); flagged by `/simplify` efficiency reviewer.

- [ ] **#28** CONDSTORE-aware IMAP ChangeTracker `#improvement` `#poplar` *(2026-05-02)* — **scheduled: post-8.4c, after profiling**
  Current `internal/mailimap/changes.go` is scan-and-diff (`UID SEARCH ALL`, diff against prior maxuid encoded in the SyncToken). Modified UIDs are always nil, so flag-only changes round-trip through `Backend.FetchHeaders` for the affected set. Replace with `UID FETCH 1:* CHANGEDSINCE <modseq>` per RFC 7162, plus the CONDSTORE assertion at `finishConnect` (NOMODSEQ → fail Connect with a clear error). SyncToken is forward-compatible: bytes 0-3 reserved for uidvalidity, bytes 4-11 hold maxuid; add a third field (modseq) by re-laying the 12-byte token. Wire VANISHED detection so Removed UIDs aren't best-effort. ADR-0120.

- [ ] **#26** Pass 7 follow-up: further responsive polish for message-list at narrow terminals `#improvement` `#poplar` *(2026-05-01)* — **scheduled: Pass 8.3**
  Pass 7 met the 80×24 polish bar via the responsive sidebar (ADR-0096), but at 80 cols the message-list pane still truncates sender names and squeezes subjects. Three interacting budgets to revisit: (1) **date column adapts to width** — switch to a narrower format (e.g. `04-30` / `3:41p`) at intermediate widths, and consider dropping the date column entirely at 80 cols; (2) **subject vs sender column balance** — currently sender gets a fixed allocation, should rebalance so subject takes more cells when sender names are short or when subject is the higher-signal column; (3) **sidebar floor** — current floor is 24 cells, explore whether 22 or 20 is acceptable when paired with label truncation, freeing more cells for the message list. Goal: 80×24 looks great even with long sender names + threaded prefixes.

- [x] **#16** ~~Sidebar rows mis-sized: Nerd Font SPUA-A icons render double-width but `lipgloss.Width` reports 1~~ `#bug` `#poplar` *(2026-04-26)*
  Resolved 2026-04-26 by Pass 4 audit-A1. New `displayCells` helper in `internal/ui/iconwidth.go` corrects the SPUA-A undercount (+1 per U+F0000–U+FFFFD codepoint); `fillRowToWidth`, sidebar `leftWidth`, and the message-list flag column now use it. Flag column bumped from 1 to 2 cells with a matching no-flag pad. See audit `docs/poplar/audits/2026-04-26-bubbletea-conventions.md` finding A1.
  Re-audit 2026-04-27: the original "1-cell undercount" framing was workstation-specific (kitty + JetBrainsMonoNL + symbol_map), not universal as ADR-0079 claimed. The `displayCells +1` fix landed here was an over-correction whose visible defect was masked until Pass 4.1 F2. ADR-0084 replaces the static rule with a runtime probe; see that ADR's "Context" section for the institutional record.

- [ ] **#12** Pass 9.5 prereq: collapse `internal/tidy/` to drop CLI machinery `#improvement` `#poplar` *(2026-04-25)* — **scheduled: Pass 9.5**
  Audit-2 verdict on `internal/tidy/` was **collapse** — the core algorithm (`SplitQuoted`/`Reassemble`/`BuildPrompt`/`CallAPI`/`Tidy`) fits the Pass 9.5 compose consumer well, but the package carries CLI ergonomics from a previous standalone-binary lineage that won't fit poplar's unified config. When Pass 9.5 lands, delete `LoadConfig`, `ApplyRuleOverrides`, `ApplyStyleOverrides`, `ConfigString`, `ResolveAPIKey`, and their tests (~100–150 LOC across source + tests). Move any surviving validators next to the unified config decode site in `internal/config/`. Optionally unexport `CallAPI` and `BuildPrompt` if no test reaches them after the trim. Goal: tidy exposes only `Config`, `DefaultConfig()`, `Tidy()`, `Result`, and the status constants. **Don't pre-emptively collapse** — wait until Pass 9.5 surfaces concrete needs that may reshape the trim. Findings: `docs/poplar/audits/2026-04-25-library-packages-findings.md`.

- [x] **#11** ~~Pass 3 prereq: MIME-aware body fetch for filter dispatch~~ `#improvement` `#poplar` *(2026-04-25)*
  Resolved 2026-04-26 by Pass 3 commit `e948edd` ("MIME-aware body fetch + Email/get state probe"). `loadBodyCmd` in `internal/ui/cmds.go` now sniffs the body format (`looksLikeRFC822`) and walks MIME parts via `extractDisplayText` before handing off to `content.ParseBlocks`. Mock backend still returns markdown; real JMAP path uses the new sniff/walk.

- [x] **#10** ~~Evaluate migrating mail backend from aerc fork to emersion ecosystem~~ `#improvement` `#poplar` *(2026-04-15)*
  Resolved 2026-04-25 by Pass 2.9 research and ADR-0075. The "Go JMAP landscape too thin" premise was wrong — `rockorager/go-jmap` covers the full JMAP surface and is already a dep. Adopting direct-on-libraries: `emersion/go-imap` v1 + `rockorager/go-jmap`, with `emersion/go-smtp/webdav/vcard` queued for later passes. ADR-0075 supersedes 0002, 0006, 0008, 0010, 0012. Research: `docs/poplar/research/2026-04-25-mail-library-stack.md`.

- [ ] **#9** Viewer `n/N` walks filtered row set `#feature` `#poplar` *(2026-04-14)* — **scheduled: Pass 8.3**
  While a search filter is committed and the viewer is open, `n/N` should advance to the next/previous message in the filtered row set and fetch its body into the current viewer. Deferred from Pass 2.5b-4 brainstorm (option c). Requires viewer↔msglist cursor coupling, body prefetch semantics, and filter-boundary behavior. **Bundle with Pass 3 (wire to live backend)** — prefetch semantics only become meaningful with real IMAP/JMAP latency.

- [x] **#8** ~~Design folder jump keybindings without multi-key sequences~~ `#feature` `#poplar` *(2026-04-10)*
  Resolved 2026-04-25 — design call landed as uppercase single-key mnemonics (I/D/S/A/X/T), codified in invariants U5 and the help popover wireframe. Wiring is bundled into Pass 2.5b-4.5 per Audit-3.

- [x] **#1** Clean up pick-link references from live docs `#improvement` `#docs` *(2026-04-09)*
  Binary was archived but `~/.claude/docs/aerc-setup.md` and `CLAUDE.md` still reference it extensively.
- [x] **#2** Clean up stale pandoc references from docs `#improvement` `#docs` *(2026-04-09)*
  pandoc is no longer part of the project but `~/.claude/docs/aerc-setup.md` still references it in the filter pipeline and compose settings.
- [ ] **#4** Investigate JMAP blob preloading for faster message open `#improvement` `#upstream` *(2026-04-09)* — **superseded by Pass 8.4–8.4c (local mail cache; see ADR-0105 for release model context)**
  New messages are slow to open (~6s) because aerc fetches body blobs lazily from Fastmail on first open. `cache-blobs=true` only helps on second open. Investigate whether aerc's JMAP backend supports blob prefetching (e.g., preload next 2-3 messages) or if this needs an upstream aerc patch.
- [x] **#3** ~~Glamour: hanging indent for wrapped list items~~ `#upstream` `#rendering` *(2026-04-09)*
  Obsolete — glamour dependency removed in Pass 2.5-render (lipgloss migration). List items now rendered directly via lipgloss.
