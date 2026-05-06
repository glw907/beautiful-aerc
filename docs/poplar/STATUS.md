# Poplar Status

**Current pass:** Pass 9g next — cache outbox Send/Append dispatch.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9f | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append (ADRs 0001–0157) | done |
| 9g | Cache outbox Send/Append dispatch | pending |
| 9h | ComposeTab UI + `c` wiring + tidy seam | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending (after 9i) |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9g)

> **Goal.** Wire the cache outbox drainer to dispatch `KindSend` and
> `KindAppend` ops through the backend's new `Send`/`Append`
> primitives. Land `SendArgs` and `AppendArgs` (currently reserved
> in the OpArgs sum) and the drainer cases that route them.
>
> **Scope.** `internal/cache/outbox.go` (or wherever drainer cases
> live), `internal/cache/types.go` (SendArgs/AppendArgs concrete
> shapes), `(*Account).QueueOp` accepting the new args, and the
> conflict-matrix entries for send/append (the existing
> `recoverExecuting` pathway already routes both kinds to
> `OpConflict crashed-mid-execute`). The IMAP path enqueues two
> ops (Send then Append-to-Sent); JMAP enqueues one Send (server
> handles atomic Sent placement). No ComposeTab UI (Pass 9h).
>
> **Settled.** Backend Send + Append shape (ADR-0157). `Envelope =
> { From, Rcpts }` + `mime []byte` is the wire shape. JMAP collapses
> Send + Sent atomically; IMAP needs a separate Append. Outbox
> kinds `KindSend`/`KindAppend` already exist as enum values
> (Cache III). Crash-recovery for both already lands in
> `OpConflict crashed-mid-execute`.
>
> **Still open — brainstorm before coding:** SendArgs envelope
> shape (store the assembled MIME bytes in the outbox row, or
> store a Draft + reassemble on dispatch?); AppendArgs flag
> encoding in the outbox JSON args column (raw `Flag` uint vs.
> string list); two-step IMAP Send semantics on partial failure
> (Send succeeded but Append-to-Sent failed — does the user see
> a conflict or a silent missing-from-Sent?).
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-outbox-send-dispatch.md`,
> then implement. Pass size budget applies. Standard pass-end
> checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
