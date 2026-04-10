# Poplar Status

**Current state:** Pass 2.5-render done with post-migration fixes
committed (wrapping, spacing, entities). One open rendering issue:
first-level blockquote wrapping for HTML emails (BACKLOG #7). UI
design spec pending review.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 | Scaffold + Fork | done |
| 2 | Backend Adapter + Connect | done |
| 2.5-render | Lipgloss migration: block model + compiled themes | done |
| 2.5-fix | Fix first-level blockquote wrapping (BACKLOG #7) | pending |
| 2.5a | Text wireframes for all screens | pending |
| 2.5b-1 | Prototype: chrome shell | pending |
| 2.5b-2 | Prototype: sidebar | pending |
| 2.5b-3 | Prototype: message list | pending |
| 2.5b-4 | Prototype: message viewer | pending |
| 2.5b-5 | Prototype: help popover | pending |
| 2.5b-6 | Prototype: status/toast system | pending |
| 2.5b-7 | Prototype: command mode | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 7 | Command mode + search | pending |
| 8 | Gmail IMAP | pending |
| 9 | Compose + send | pending |
| 10 | Config | pending |
| 11 | Polish for daily use | pending |

## Plans

- [Design spec](../superpowers/specs/2026-04-09-poplar-design.md)
- [UI design spec](../superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md)
- [Lipgloss migration spec](../superpowers/specs/2026-04-10-mailrender-lipgloss-design.md)
- [Lipgloss migration plan](../superpowers/plans/2026-04-10-mailrender-lipgloss.md)
- [Pass 1 plan](../superpowers/plans/2026-04-09-poplar-pass1-scaffold.md)
- [Pass 2 plan](../superpowers/plans/2026-04-09-poplar-pass2-backend-adapter.md)

## Continuing Development

### Next steps

1. **Pass 2.5-fix**: Design and fix first-level blockquote wrapping
   (BACKLOG #7). Needs a design spec before implementation.
2. **User reviews UI design spec** — review
   `docs/superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md`
   and approve or request changes
3. **Write implementation plan for Pass 2.5a** (text wireframes)
4. **Execute Pass 2.5a** — draw text wireframes for all 20 UI elements

### Next starter prompt

> Fix BACKLOG #7: first-level blockquote wrapping for HTML emails.
> Read `BACKLOG.md` for the full problem description and constraints.
> Write a design spec first, then implement. Test against the Yahoo
> email "Re: Draft Survey - Boat Builder Search Committee" from
> jmnsailor@yahoo.com and the plain text email "Re: small business
> group" from geoff@907.life (which already works and must not regress).

### Pass-end checklist

1. `/simplify` — code quality review
2. Update `docs/poplar/architecture.md` — design decisions
3. Update this file — mark pass done, next starter prompt
4. Update docs appropriate to the pass stage
5. Commit all changes
6. `git push`
