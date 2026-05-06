# Pass 9e — `internal/compose/` foundation

**Date:** 2026-05-05
**Pass:** 9e
**Spec:** `docs/superpowers/specs/2026-05-04-compose-design.md`

## Scope

Stand up `internal/compose/` — package skeleton, the `Editor` seam,
the `Draft` value type, MIME assembly, and reply/forward seeders. No
UI wiring (Pass 9h), no backend `Send` (Pass 9f), no outbox dispatch
(Pass 9g).

## Settled (no brainstorm)

The compose-design spec already decided every starter-prompt open
question. Inheriting verbatim:

| Question | Resolution (spec line) |
|---|---|
| Editor surface | `tea.Model` + accessors (`SetSize`, `Focus`, `Blur`, `Focused`, `Value`, `SetValue`); annotation seam = `SetAnnotations` per ADR-0150. (§Architecture, §Annotation pipeline) |
| Draft layout | headers + body as raw markdown source string. Attachments are filesystem paths. (§Architecture lines 127–135) |
| Reply quoting | full body quoted, depth-preserving prefix walk; attribution line above; cursor lands above attribution (bottom-post). (§Reply / Forward seeding) |
| Attachment threading | `Attachments []string` paths on the Draft; MIME assembler reads from disk. (§Architecture) |

## Inline picks (not in spec, decided by codebase convention)

- **Address type.** Use `gomail "github.com/emersion/go-message/mail"`
  alias to match `internal/ui/cmds.go` and `internal/config/accounts.go`.
  `gomail.Address` is the stdlib `net/mail.Address`.
- **Message-Id / References extraction.** `mail.MessageInfo` does not
  carry `Message-Id` or `References`. The seed signatures take
  `body []byte` (the raw RFC 5322 message); parse headers via
  `net/mail.ReadMessage` to pull `Message-Id` + `References`. Keeps
  `MessageInfo` unchanged.
- **Goldmark wiring.** Inline in `assemble.go`. `internal/filter/tohtml.go`
  produces a standalone HTML document; compose embeds the inner HTML
  in a MIME part — different output shapes, no clean reuse seam yet.
- **Plain-text body.** v1 emits the markdown source verbatim as the
  `text/plain` part. Stripping markers for a "plain reading" version
  is a post-1.0 polish.
- **Date stamping.** `AssembleMIME(d Draft, now time.Time) ([]byte, error)`
  takes a clock so tests are deterministic.
- **Message-Id generation.** `<random@host>` shape, `host` = `From`
  domain, `random` = 16 hex bytes from `crypto/rand`. Time goes in
  `Date:` header.

## Files

```
internal/compose/
  editor.go        # Editor interface + CatkinEditor adapter
  draft.go         # Draft, Mode
  assemble.go      # AssembleMIME (markdown → multipart/alternative)
  seed.go          # SeedReply, SeedReplyAll, SeedForward
  editor_test.go
  assemble_test.go
  seed_test.go
```

## Tasks

1. Plan doc (this file).
2. `editor.go` — Editor interface + CatkinEditor adapter.
3. `draft.go` — Draft + Mode.
4. `assemble.go` — AssembleMIME.
5. `seed.go` — SeedReply / SeedReplyAll / SeedForward.
6. Unit tests across the three implementation files.
7. Pass-end ritual (ADR for compose-package shape; invariants; archive
   plan + spec; commit/push/install).

## ADR draft

One ADR — "compose package foundation":

- Editor seam shape (tea.Model + accessors + SetAnnotations) — codifies
  the v1.1 neovim-adapter contract.
- Draft as raw-markdown body + paths slice (no parsed parts in v1).
- AssembleMIME signature `(Draft, time.Time) → ([]byte, error)`; pure
  function; goldmark `autolink + table`.
- Seed signatures take `body []byte` so Message-Id / References parse
  from bytes — no `MessageInfo` wire extension this pass.

## Out of scope

ComposeTab UI, `Tidy` interface (both 9h), backend `Send` / `Append`
(9f), outbox dispatch (9g), spellcheck wiring into compose (already
landed Catkin-side in 9d).
