# BACKLOG

> Project issue tracker. Managed by `/log-issue`.

## High

- [ ] **#7** Lipgloss renderer: missing first-level blockquote wrapping `#rendering` `#mailrender` *(2026-04-10)*
  HTML emails from Yahoo, Outlook, and other clients often lack a `<blockquote>` tag on the first reply level. The html-to-markdown library produces correct `>` prefixes for nested `<blockquote>` elements, but the outermost replied-to content appears as unquoted paragraphs after an attribution line ("On ... wrote:"). The parser detects the attribution but doesn't wrap the following unquoted content as a blockquote.

  **Root cause:** The content parser (`internal/content/parse.go`) treats "On ... wrote:" as a `QuoteAttribution` block but has no mechanism to infer that unquoted text following it should be quoted. Naive approaches (prefix remaining lines with `>` and re-parse) cause nesting to compound exponentially — each inner attribution triggers another wrapping pass, producing 60+ levels of `>` on deeply threaded emails.

  **What works now:** Text wrapping at 78 chars (via `ansi.Wordwrap`), paragraph spacing within blockquotes, HTML entity unescaping, ANSI color output when piped. Inner `<blockquote>` nesting renders correctly.

  **What's broken:** First-level reply content after "On ... wrote:" renders without `>` prefix. Only affects HTML emails where the first reply level lacks `<blockquote>` (common with Yahoo Mail, some Outlook versions). Plain text emails with explicit `>` prefixes are unaffected.

  **Approach needed:** Design spec first. Key constraint: must not compound nesting on recursive attributions. Options include: (1) single-level wrapping that only triggers at `quoteLevel == 1`, (2) handling in the filter layer (`CleanHTML`) by detecting `<div>` structures after attribution patterns and injecting `>` prefixes before markdown conversion, (3) a post-parse fixup that walks the block tree.

  **Test emails:** "Re: Draft Survey - Boat Builder Search Committee" from jmnsailor@yahoo.com (Yahoo HTML, deeply threaded, multiple attributions), "Re: small business group" from geoff@907.life (plain text with correct `>` prefixes — should remain unaffected).

## Someday

- [ ] **#5** Built-in bubbletea compose editor `#poplar` `#v2` *(2026-04-10)*
  Pine-style built-in compose using `bubbles/textarea` for body + custom header fields. Alternative to `$EDITOR` for users who want a seamless, zero-dependency compose experience. Would be a bubbletea showcase piece. Design after external editor flow (Pass 9) is stable.
- [ ] **#6** Neovim companion plugin for poplar `#poplar` `#v2` *(2026-04-10)*
  Email browsing within neovim (folder list, message list, viewer as buffers), telescope pickers, compose integration, poplar command passthrough. Requires IPC/RPC interface in poplar. Design when core client is stable.

## Medium

- [x] **#1** Clean up pick-link references from live docs `#improvement` `#docs` *(2026-04-09)*
  Binary was archived but `~/.claude/docs/aerc-setup.md` and `CLAUDE.md` still reference it extensively.
- [x] **#2** Clean up stale pandoc references from docs `#improvement` `#docs` *(2026-04-09)*
  pandoc is no longer part of the project but `~/.claude/docs/aerc-setup.md` still references it in the filter pipeline and compose settings.
- [ ] **#4** Investigate JMAP blob preloading for faster message open `#improvement` `#upstream` *(2026-04-09)*
  New messages are slow to open (~6s) because aerc fetches body blobs lazily from Fastmail on first open. `cache-blobs=true` only helps on second open. Investigate whether aerc's JMAP backend supports blob prefetching (e.g., preload next 2-3 messages) or if this needs an upstream aerc patch.
- [x] **#3** ~~Glamour: hanging indent for wrapped list items~~ `#upstream` `#rendering` *(2026-04-09)*
  Obsolete — glamour dependency removed in Pass 2.5-render (lipgloss migration). List items now rendered directly via lipgloss.
