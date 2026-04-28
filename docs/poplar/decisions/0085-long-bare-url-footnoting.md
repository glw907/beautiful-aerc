---
title: Long bare URL footnoting
status: accepted
date: 2026-04-28
---

## Context

Bare URL spans (`Link.Text == Link.URL`) previously rendered inline
unchanged, regardless of length. Real bodies routinely contain bare
URLs > 60 cells (mailing-list moderation links, tracking URLs,
calendar invites). At `maxBodyWidth = 72` these consume an entire
content row and push other text out of view.

ADR-0066 established footnote harvesting for text-bearing links
(`Text != URL`); auto-linked bare URLs were intentionally exempt so
they could render inline in link style without a marker. That carve-
out works for short bare URLs but fails on the long ones.

## Decision

Bare URLs whose `lipgloss.Width(URL) > 30` cells get the long-URL
footnote treatment: the harvester rewrites the inline span text to
`trimURL(URL) + nbsp + [^N]` and adds the URL to the footnote list.
Short bare URLs continue to pass through unchanged.

`trimURL` (in `internal/content/url_trim.go`) produces a compact
inline form: strips the scheme, keeps host (with port), and appends
`/<first-segment>` when present. A trailing `/` is preserved only
when it terminates the URL. `…` is appended when anything was
removed; otherwise the result is identical to the host (or
`host/segment`).

Threshold and trim rule live in `internal/content/render_footnote.go`
(`longBareURLThreshold`) and `internal/content/url_trim.go`. The
30-cell cutoff was chosen so single-segment URLs like
`https://example.com/foo` (24 cells) pass through, while two-segment
or query-bearing URLs trigger the trim. The same `markerFor` dedup
ensures that a long bare URL appearing alongside a text-bearing link
to the same target shares one footnote slot.

## Consequences

- Long bare URLs now render as e.g. `example.com/very… [^3]` inline,
  with the full URL appearing in the footnote list at the bottom of
  the body and addressable by the viewer's `1`-`9` quick-launch and
  the `Tab` link picker.
- `trimURL` is a pure function — no allocations beyond two short
  string slices per call.
- Implementation depends on the parser emitting `Link{Text: url, URL:
  url}` for bare URLs. The current parser only emits `Link` for
  markdown `[text](url)` syntax — bare URLs in plaintext bodies render
  as `Text` spans and never reach `harvestFootnotes`. BACKLOG #22
  tracks the autolink pass needed to make this footnoting effective
  in real bodies.
- The `…` ellipsis is U+2026 (single cell). All comparisons use
  `lipgloss.Width`, never `len()`.
