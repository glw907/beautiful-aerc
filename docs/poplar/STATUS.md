# Poplar Status

**Current pass:** Pass 45 landed catkin render-time soft wrap
(ADR-0243). Pass 46+ is rolling dogfood-driven work.

**Dogfood phase, pre-beta rules in force.** Geoff is the sole
user; soak enters when a second user lands or Geoff calls feature
freeze. Pass 35.1 still pending Gmail/Outlook creds.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 45 | Scaffold through catkin render-time soft wrap (ADRs 0001–0243) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 46+ | Dogfood-driven fixes + quality (rolling) | next |
| Beta soak | Gated on second user or feature freeze | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 46)

> **Goal.** Roll the next dogfood-driven fix or quality
> improvement Geoff queues. No fixed scope; pass cadence reverts
> to issue-driven until a larger initiative is named.
>
> **Scope.** Whatever the next session opens with. Adjacent fixes
> inline per pre-beta rules. If the work fans out beyond ~12
> tasks, split the pass before coding.
>
> **Settled.** Pass 45's render-time soft wrap is the new wrap
> contract; treat it as a binding fact. `viewportTop` is a
> visual-row index. List-item continuation styling and code-fence
> wrap are the named future follow-ups from ADR-0243; pick them up
> if they come up organically.
>
> **Open — brainstorm if any.** Set by the session's opening
> prompt.
>
> **Approach.** Read STATUS, invariants, and the relevant
> rule-scoped invariants. Standard pass-end checklist applies.
