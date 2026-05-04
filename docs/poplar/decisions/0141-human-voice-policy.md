---
title: Human-voice policy — research-grounded style guide, persona, /simplify voice lens
status: accepted
date: 2026-05-04
---

## Context

Contributors recognize AI-generated Go on sight and disengage. That's
the threat model: every AI-shaped tell in poplar's code is a small
deterrent to outside contribution. By the end of Pass 8.7 the code was
showing the full catalogue — uniform godoc shape on every method,
`failed to X: %w` choruses, sentence-form test names, reflexive
`<thing>.go` file layout, pass-label task framing inside production
docs. None of it was wrong; all of it was uniform.

A targeted fix per finding wouldn't hold; the texture has to be
prevented at write time. That requires a binding style guide that
catches tells before they land, calibrated against actual stdlib and
third-party Go.

## Decision

The load-bearing artifact is `~/.claude/docs/go-comment-voice.md`:

- A research-synthesized style guide built from four parallel surveys
  (authoritative docs; stdlib exemplars — `net/http`, `database/sql`,
  `encoding/json`, `time`, `io`; third-party exemplars — `bbolt`,
  `cobra`, `viper`, `zap`, `gin`; essays + proverbs from Pike,
  Cheney, Gerrand). Sources live under
  `docs/poplar/research/sources/`.
- A 32-tell catalogue (§7) that names each AI fingerprint by number
  with mechanical avoidance rules.
- A poplar-specific voice palette (§4): stdlib-formal base,
  Gerrand-welcoming for package docs, Pike-aphoristic for errors.
- A propagation plan (§9) that points at the `go-conventions` skill,
  `/simplify` voice lens, and `CLAUDE.md` pointer as the enforcement
  surfaces.

Three production-side hooks make the guide enforceable:

1. **`go-conventions` skill** carries the §7 catalogue inline and an
   experienced-Go-developer persona preamble. Mandatory invocation
   before any Go file edit (per global CLAUDE.md).
2. **`/simplify` voice lens** is a fourth parallel reviewer agent
   that scans diffs against the §7 catalogue by tell number; each
   finding cites the tell and the avoidance rule.
3. **`CLAUDE.md`** (poplar root) Human-voice section keeps the short
   rules and points at the guide as the binding artifact.

The audit + apply runs as a two-pass split, both against the same
frozen Phase 2 triage:

- **Pass 8.8 Phase 3a** (this pass) — string-only fixes: C1 (comment
  rot), C7 (error phrasing), C4-prose (uniform doc-comment shape that
  the fix is a comment edit). 91 applies across three commits;
  `make check` green between.
- **Pass 8.9 Phase 3b** — structural fixes: C2 (defensive cruft on
  internal callers), C5 (renames), C6 (test boilerplate + table-case
  rewrites), C8 (file collapses), C4-structural (chorus elimination
  by removing or reshaping symbols, not just edits). 62 applies; tmux
  render at 80×24 + 120×40 after C5 and C3.

The split reflects risk profile, not finding count: 3a touches no
function shape and lands behind a comment-only diff; 3b carries
behavior risk (nil-check removals, test renames, file moves) and
soaks the style guide one pass before applying at scale.

Two taste-calls landed as keeps:

- **`config.ExpandHome` name.** The pure-passthrough behavior for
  non-tilde paths could read as over-promising, but the name encodes
  a real promise (leading `~` expands), and the passthrough is a
  graceful-degradation contract worth flagging in the name.
- **`internal/cache/attachments.go` as a noun-swap clone of
  `bodies.go`.** The two caches have separate size backstops
  (ADR-0135) and separate eviction policies; the file split reflects
  real domain divergence even when the doc shape was reflexive. The
  doc-shape fix (C4) still applies; the file split stays.

## Consequences

- Future Go work in poplar (and on this workstation) carries the
  guide's voice palette by default. The `go-conventions` skill loads
  the 32-tell catalogue inline before every edit.
- `/simplify` will flag voice findings on every diff. The fourth
  reviewer is structural — it doesn't depend on a human noticing.
- The audit findings (`docs/poplar/audits/2026-05-04-*.md`) and
  triage (`docs/poplar/audits/2026-05-04-triage.md`) stay in place
  until Pass 8.9 closes. 8.9 reads from the same triage; archival
  happens at end of 8.9.
- Pre-beta posture endorses structural changes that were *not* shaped
  by a current consumer (renames, file collapses, dropped reflexive
  `doc.go` files). 8.9's apply set is correctness-preserving but
  cosmetic in user-visible terms; it lands on master.

### Before / after

```go
// Before — internal/mailimap/realclient.go
// imapUID converts imap.UID (uint32) to mail.UID (decimal string).
func imapUID(u imap.UID) mail.UID { ... }

// After
func imapUID(u imap.UID) mail.UID { ... }
```

```go
// Before — internal/cache/schema.go
return fmt.Errorf("migrate v1: %w", err)
return fmt.Errorf("migrate v2: %w", err)
return fmt.Errorf("migrate v3: %w", err)
return fmt.Errorf("migrate v4: %w", err)
return fmt.Errorf("migrate v5: %w", err)

// After
return fmt.Errorf("install cache schema: %v", err)
return fmt.Errorf("add outbox.next_eligible_at: %v", err)
return fmt.Errorf("add folders exists/unseen counts: %v", err)
return fmt.Errorf("narrow bodies table: %v", err)
return fmt.Errorf("add attachments table: %v", err)
```

```go
// Before — internal/mailjmap/jmap.go (× 15 methods)
// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

// After
// Updates returns a nil channel before Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }
```

The diffs are small per-site; the cumulative texture shift is what
the threat model targets.
