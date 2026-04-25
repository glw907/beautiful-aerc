# Poplar Status

**Current pass:** Pass 2.5b-4 (message viewer prototype) shipped
2026-04-25. ADRs 0065–0069 cover the viewer model, body width
correction (78→72), footnote harvesting, modifier-free
keybindings, and optimistic mark-read. Audit-1 (invariants vs.
code) ran the same day; doc fixes landed in this commit and the
five code fixes are queued as Pass 2.5b-4.5.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 | Scaffold + Fork | done |
| 2 | Backend Adapter + Connect | done |
| 2.5-render | Lipgloss migration | done |
| 2.5a | Text wireframes for all screens | done |
| 2.5b-1..3.6, 2.5b-7 | Chrome / sidebar / msglist / threading / search | done |
| 2.5b-4 | Prototype: message viewer | done |
| 2.5b-4.5 | Audit-1 fixes: wire dead bindings, consume threading config | next |
| 2.5b-5 | Prototype: help popover | pending |
| 2.5b-6 | Prototype: status/toast system | pending |
| 2.5b-train | Tooling: mailrender training capture system | pending (after Pass 3) |
| 2.9 | Research: JMAP/IMAP/SMTP/parser library survey | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 8 | Gmail IMAP | pending |
| 9 | Compose + send (Catkin editor) | pending |
| 9.5 | Tidytext in compose | pending |
| 10 | Config | pending |
| 11 | Polish for daily use | pending |
| 1.1 | Neovim embedding (nvim --embed RPC) | pending |

## Next starter prompt (Pass 2.5b-4.5)

> **Goal.** Audit-1 follow-up: wire the defined-but-undispatched
> folder jump keys, consume the dead per-folder threading config,
> drop the dead `:` binding, decide an offline color, bump go.mod
> to match the workstation toolchain.
>
> **Scope.** Five mechanical fixes from the Audit-1 findings at
> `docs/poplar/audits/2026-04-25-invariants-findings.md`:
>
> - **U3** — Delete `Cmd` field from `GlobalKeys` and its
>   initialization (`internal/ui/keys.go:8,16`).
> - **U5** — Dispatch `I/D/S/A/X/T` in `AccountTab.handleKey`.
>   Each key looks up the canonical folder in the sidebar's
>   classified list and fires `selectionChangedCmds` (no-op when
>   the canonical isn't present in this account). Plus tests.
> - **U6** — Read `fc.Threading` in the `folderLoadedMsg` handler;
>   add `MessageList.SetThreaded(bool)` that short-circuits
>   `bucketByThreadID` to one bucket per message when false.
>   Plus tests.
> - **U14** — Pick a color for the offline state. Code currently
>   uses `FgDim`; the invariant says red. Either change
>   `styles.go:131-133` to `ColorError`, or update U14's invariant
>   text to "○ dim hollow offline." Update `styling.md` either way.
> - **B4** — Bump `go.mod` directive from `go 1.25.0` to
>   `go 1.26.0`.
>
> **Settled.** Doc-side tightenings already landed (A7, A10, A15,
> U4, U11). U14 is the only design call left in this pass.
>
> **Approach.** Implement directly — no brainstorm needed. After
> 2.5b-4.5 ships, regenerate the 2.5b-5 (help popover) starter
> prompt. Pass-end via `poplar-pass`.

## Queued audits

Independent verification tasks driven by the Opus 4.7 model
upgrade. Run sequentially with fresh sessions — findings from
each may reshape the next.

1. ~~Invariants vs. code drift~~ — done 2026-04-25; findings at
   [audits/2026-04-25-invariants-findings.md](audits/2026-04-25-invariants-findings.md);
   doc fixes landed; code fixes queued as Pass 2.5b-4.5.
2. [Library package shape](audits/2026-04-25-library-packages.md)
   — `filter/`, `content/`, `tidy/` fit-for-consumer review.
3. [Plan shape (forward look)](audits/2026-04-25-plan-shape.md)
   — STATUS.md + BACKLOG.md sequencing review.
