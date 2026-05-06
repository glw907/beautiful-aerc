# Poplar Status

**Current pass:** Pass 9e next — `internal/compose/` Editor interface + CatkinEditor adapter.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9d.4 | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding (ADRs 0001–0155) | done |
| 9e | `internal/compose/` — Editor interface, CatkinEditor adapter, Draft, AssembleMIME, Seed{Reply,ReplyAll,Forward} | pending |
| 9f | Mail backend Send + Append — JMAP submission, IMAP+SMTP, `[account.smtp]` config | pending |
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

## Next starter prompt (Pass 9e)

> **Goal.** Stand up `internal/compose/` — the message-composition
> layer. Define an `Editor` interface, wrap Catkin as the first
> implementation (`CatkinEditor`), introduce a `Draft` value type,
> and implement `AssembleMIME` plus `Seed{Reply,ReplyAll,Forward}`.
>
> **Scope.** New package `internal/compose/`. No UI wiring yet
> (that's Pass 9h). No backend Send (Pass 9f). No outbox dispatch
> (Pass 9g). The MIME assembly path stays inside the package and
> writes only to a `Draft`.
>
> **Settled.** Catkin is the editor (ADR-0146 / 0147). Backends
> `Append`/`Send` arrive in Pass 9f. Outbox already carries Op
> sums (ADR-0114).
>
> **Still open — brainstorm before coding:** Editor interface
> shape (sync surface vs. tea.Model surface); Draft layout
> (headers + body in markdown vs. headers + body parts);
> reply quoting strategy (top-quote vs. inline vs. configurable);
> attachment threading from `mail.Attachment` into the draft.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-compose-foundation.md`,
> then implement. Pass size budget applies — stop at the
> Editor + CatkinEditor + Draft + AssembleMIME + Seed surface,
> defer ComposeTab UI to Pass 9h. Standard pass-end checklist
> applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
