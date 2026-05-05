# Poplar Status

**Current pass:** Pass 9d next — Catkin annotation pipeline + spellcheck.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge (ADR-0132–0134) | done |
| 8.5 – 8.5d | Overengineering audit, Elm conformance, UI structural cleanup, content/filter cleanup (ADR-0125–0131) | done |
| 8.6 – 8.7 | Attachments I (backend) + II (viewer) (ADR-0135–0140) | done |
| 8.8 – 8.9 | Human-voice audit I (string-only) + II (structural) | done |
| 8.10 | First-sync header population — JMAP per-folder baseline pull (ADR-0143) | done |
| 9 – 9c | Catkin — core, live styling, command vocabulary, power-user QoL (ADR-0144–0147) | done |
| 8.11 | Voice cleanup III — prose-rhythm sweep + grep gate for T33/T34/T35 (ADR-0148) | done |
| 9d | Annotation pipeline + spellcheck (Track 1 SymSpell + bundled lists; Track 2 hunspell subprocess) | pending |
| 9e | `internal/compose/` — Editor interface, CatkinEditor adapter, Draft, AssembleMIME, Seed{Reply,ReplyAll,Forward} | pending |
| 9f | Mail backend Send + Append — JMAP submission, IMAP+SMTP, `[account.smtp]` config | pending |
| 9g | Cache outbox Send/Append dispatch | pending |
| 9h | ComposeTab UI + `c` wiring + tidy seam | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) — multi-attach, attach-from-cache | pending (after 9i) |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9d)

> **Goal.** Wire Catkin's annotation pipeline and add inline
> spellcheck. Catkin already owns its render (live markdown
> styling + chroma fences); next it needs an annotation layer
> that overlays decorations (spellcheck squiggles to start,
> later: lint marks, link previews, comment author colors).
>
> **Scope.** `internal/catkin/` only. The annotation pipeline
> is generic (`Annotation` interface + `Annotator` registry +
> per-frame composition into the styled overlay). Spellcheck is
> the first consumer: Track 1 ships SymSpell with bundled
> wordlists (en-US baseline + project-specific allowlist for
> "Catkin", "JMAP", etc.); Track 2 (defer to follow-up pass)
> wraps a hunspell subprocess for users who want richer
> dictionaries.
>
> **Settled (do not re-brainstorm):**
>
> - Catkin owns rendering — annotations compose with the existing
>   styled overlay, they don't replace it.
> - Squiggle visuals: single underline in `Styles.Error` (red),
>   matching iA-Writer's restraint.
> - Spellcheck runs on idle (not per-keystroke) to keep the typing
>   path cheap. Reuse `bubbletea`'s tick pattern — no new tickers.
> - Word boundary detection reuses Catkin's existing word-nav rune
>   classification.
>
> **Still open — brainstorm these:**
>
> - Annotation interface shape: range-based vs token-based?
>   Range avoids re-tokenizing per annotator.
> - Annotation precedence when two annotators flag the same span
>   (spellcheck + grammar) — composition rule?
> - SymSpell wordlist storage — embed via `//go:embed` or load from
>   XDG data dir? Embed avoids first-run friction; XDG allows
>   user-extension without a rebuild.
> - Project allowlist source — checked-in `wordlist.txt` vs
>   per-user `~/.config/poplar/wordlist.txt`? Probably both.
> - Suggestion UI — popover? footer hint? deferred entirely to a
>   follow-up pass?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-catkin-annotations.md`,
> then implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
