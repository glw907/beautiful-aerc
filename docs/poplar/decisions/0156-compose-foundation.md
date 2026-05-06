---
title: Compose package foundation
status: accepted
date: 2026-05-05
---

## Context

Pass 9e stands up `internal/compose/`, the outbound-mail surface
that ComposeTab (Pass 9h) and the Send/Append cache ops (Pass 9g)
will share. The compose-design spec
(`docs/superpowers/specs/2026-05-04-compose-design.md`) settled the
shape; this ADR records the binding decisions in the file the
codebase reads.

## Decision

`internal/compose/` is a UI-free package owning four surfaces:

1. **`Editor` interface.** A bubbletea sub-model contract: `Init`,
   `Update(msg) (Editor, tea.Cmd)`, `View`, `SetSize`, `SetWidth`,
   `Focus`, `Blur`, `Focused`, `Value`, `SetValue`, `WordCount`,
   `CharCount`, plus `RegisterAnnotator(catkin.Annotator)`. `Update`
   returns `Editor` rather than `tea.Model` so callers chain without
   a type assertion. The neovim adapter (v1.1, ADR-0033) implements
   the same surface.

2. **`CatkinEditor`.** Adapter wrapping `catkin.Model`. v1's only
   `Editor` impl. Pointer receiver, inner model held by value and
   reassigned in `Update`.

3. **`Draft` value type.** Headers (From, To, Cc, Bcc, Subject,
   InReplyTo, References) plus body as raw CommonMark plus an
   `Attachments []string` of filesystem paths. The cache outbox
   stores assembled bytes (not the Draft), so a queued message
   ships even if attachment files move.

4. **`AssembleMIME(d Draft, now time.Time) ([]byte, error)`.** Pure
   function. Emits multipart/alternative (text/plain markdown
   verbatim plus text/html via goldmark Linkify+Table), wrapped in
   multipart/mixed when attachments are present. Goldmark setup
   lives in `internal/filter` (`MarkdownBody`/`MarkdownToHTML`)
   shared with the existing inbound converter.

5. **`SeedReply(parent, body)` / `SeedReplyAll(parent, body, self)` /
   `SeedForward(parent, body)`.** `body []byte` is the parent's raw
   RFC 5322 message; Message-Id and References parse from it via
   `net/mail`, so `mail.MessageInfo` does not grow new wire fields
   this pass. Reply quoting is depth-preserving (every line gets one
   additional `> `; existing `> ` runs deepen). Attribution row
   above; cursor lands two newlines above attribution for
   bottom-posting. Subject prefix collapses repeated `Re:` /  `Fwd:`
   case-insensitively. ReplyAll merges parent's To+Cc, dedups, and
   filters self.

Address parsing reuses `content.ParseAddressList` (exported in this
pass). `gomail.Address` (alias to `net/mail.Address`) is the address
type carried on Drafts, matching `internal/config/accounts.go` and
`internal/ui/cmds.go`.

No UI wiring, no backend `Send`/`Append`, no outbox dispatch, no
Tidy seam this pass. Those land in 9f/9g/9h/9i.

## Consequences

- ComposeTab (Pass 9h) holds `editor compose.Editor` and a `Draft`.
  At send it queues `cache.SendArgs{MIME: AssembleMIME(d, time.Now())}`
  plus an `AppendArgs` for the Sent copy.
- The neovim adapter in v1.1 implements `Editor`; the seam is
  named, single-impl status no longer triggers ADR-0141's tell.
- `internal/filter` gains `MarkdownBody` and `MarkdownToHTML` as the
  shared goldmark entry points. The inbound `ToHTML` (markdown →
  standalone document) and the outbound compose path (markdown →
  embedded body) share extension config, so changes propagate.
- `content.ParseAddressList` is now exported; future packages that
  need RFC 5322 address parsing with the bare-email fallback should
  call it rather than reach for `net/mail.ParseAddressList` directly.
- Pass 9f extends `mail.Backend` with `Send` and `Append`; the
  current `Backend.Send(from, rcpts, body io.Reader)` shape will be
  reshaped to take pre-assembled bytes. Pass 9g wires
  `cache.SendArgs`/`AppendArgs` through the drainer.
