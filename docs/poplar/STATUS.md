# Poplar Status

**Current pass:** Pass 41.1 landed the Audit Final remediation
batch (ADR-0239): `config.AtomicWrite` collapses three temp-file
write sites with chmod-on-handle, five fake-backend `*Err` seams,
`TestCmdClient_AuthDialFailure` completes the IMAP cmd-path
ErrAuth → drainer coverage, T15 renames
(`cache.CacheEvent → Event`, `compose.CacheStore → Store`), ADR
em-dash trim, six line-level voice fixes, three invariant
doc-drift repairs.

**Dogfood phase, pre-beta rules still in force.** Geoff is the
sole user; soak (as `release-stance.md` defines it — stability
first, no schema breaks, features queued to `1.1`) is the wrong
posture while there's no one to protect. Pre-beta refactor
license stays open. Beta soak enters when a second user lands or
Geoff explicitly calls the tree feature-frozen.

Pass 35.1 still pending Gmail/Outlook creds.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 41.1 | Scaffold through Audit Final remediation (ADRs 0001–0239) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 42+ | Dogfood-driven fixes + quality (rolling) | active |
| Beta soak | Gated on second user or explicit feature freeze | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (rolling — Pass 42+)

> **Goal.** Fix what daily-driver use surfaces; keep tightening
> code quality while there are no other users to protect.
>
> **Scope.** Driven by what Geoff hits in real use. No fixed
> task list. Each session may be one bug, one refactor, or a
> small theme of related fixes. Adjacent quality wins land
> inline per pre-beta `CLAUDE.md` rules.
>
> **Settled.**
> - Pre-beta rules stay in force; the documented beta-soak
>   posture is not entered.
> - Schema breaks are fair game; the only sacred data is the
>   mail cache contents (and those rebuild from server anyway).
> - Bug reports from Geoff are authoritative without external
>   repro.
>
> **Open.** Whatever surfaces in use.
>
> **Approach.** Each session opens with a bug or itch from
> Geoff. Fix it, fix adjacent issues inline, run the pass-end
> ritual when the change set has cohered into something
> commit-shaped. Soak entry is deferred until a second user
> lands or Geoff calls feature freeze.
