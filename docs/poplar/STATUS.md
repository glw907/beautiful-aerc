# Poplar Status

**Current pass:** Pass 13 — Search (#38). Pass 12 landed the
`.ics` invite viewer; #37 closed.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13)

> **Goal.** Full-account / cross-folder search (BACKLOG #38).
> Operator string typed at the existing `/` shelf, parsed to
> typed `FilterCondition`, dispatched via `mail.Backend.Search`,
> rendered as a virtual folder in the sidebar.
>
> **Scope.** Restore `Search` on `mail.Backend` (removed in
> Pass 8.5 / ADR-0125) with typed filter shape; impl against
> JMAP `Email/query` and IMAP `SEARCH`. Operator grammar:
> `from:`, `to:`, `subject:`, `is:unread`, `has:attachment`,
> `before:`, `after:`, plus bare subject/body match. Results
> render through the existing message-list surface; cache last
> N result sets.
>
> **Settled.** Fastmail/Gmail operator surface (typed parser).
> Virtual-folder rendering reuses message-list. Pass 13 is
> display + dispatch; ranking is post-1.0.
>
> **Open — brainstorm:** parser location
> (`internal/search/` vs `internal/content/`); shelf input
> mode; result-set lifetime; IMAP fallback when `SEARCH`
> exceeds cache window; cross-virtual-folder `n`/`N`.
>
> **Approach.** Brainstorm, plan + spec under
> `docs/superpowers/`, implement. Watch pass size — split
> 13a (backend) / 13b (UI) if tasks > 12.
