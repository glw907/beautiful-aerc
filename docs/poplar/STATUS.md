# Poplar Status

**Current pass:** Pass 16d — `log/slog` adoption + logging-convention
ADR. Last of four modernization passes (16a infra ✓, 16b sweep ✓,
16c `iter.Seq` ✓, 16d `log/slog`) before the bubbles-adoption
remainder (17a sidebar tree, 17b messagelist on `bubbles/v2/list`,
17c help audit). Polish II (18) and v0.9.0 prep (19) follow.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Background body sync + status indicator (substrate for #38; ADR-0187) | done |
| 13.1 | Search (#38; ADR-0188) | done |
| 13.1.5 | Claude infrastructure v2 prep (skills, conventions, pass ritual) | done |
| 13.2a | `charm.land/v2` substrate — mechanical migration + AdaptiveColor (ADR-0189a) | done |
| 13.2b | `charm.land/v2` reframes — paste; chrome + cursor absorbed by 13.2a (ADR-0189b) | done |
| 14a | Probe + config substrate (#27 part 1, #29; ADR-0190) | done |
| 14b | Wizard domain + huh UI (#27 part 2; ADR-0191) | done |
| 14c | First-run integration (#27 part 3; ADR-0192) | done |
| 14.1 | OAuth refresh (#42; ADR-0193) | done |
| 15a | `bubbles/v2/list` adoption — `movepicker`, `reader/linkpicker`, `reader/attachpicker` (ADR-0194) | done |
| 15a.5 | `compose/attachpicker` filepicker-lessons — symlink + atomic id + size column; ADR-0195 names the deviation | done |
| 16a | Claude infra: modern-Go defaults (skill bump + simplify Agent 3 + `modern-go-check.sh`; ADR-0196) | done |
| 16b | Mechanical Go modernization sweep — `slices.SortFunc`, `slices.Sort`, `for range N`, `sync.OnceValue`, `slices.Chunk`, `slices.Sorted+maps.Keys`; ~60 sites, no ADR | done |
| 16c | `iter.Seq` for `catkin/style.go` `walkSpans` + 3 callers; no ADR | done |
| **16d** | **`log/slog` adoption + logging-convention ADR (mailjmap push-loop, error transcript shape)** | **pending — next** |
| 17a | Sidebar folder hierarchy on a v2 tree component (plan + spec already on disk; ADR-0197) | pending |
| 17b | `messagelist` on `bubbles/v2/list` (custom item renderer; absorbs BACKLOG #46 `iter.Seq2`; ADR if it survives) | pending |
| 17c | `bubbles/v2/help` audit + ADRs for bubbles deviations that survive 15a/17a/17b (`helppopover`, `schedulepicker`) | pending |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 16d)

> **Goal.** Adopt `log/slog` for poplar's diagnostic logging.
> Land a logging-convention ADR that binds where slog is used,
> which handler/format, and which sites stay on stderr.
>
> **Scope.** `internal/mailjmap/push.go` push-loop transcript
> (currently `fmt.Fprintf(os.Stderr, ...)`), `internal/mailimap`
> reconnect/idle transcript, error-transcript shape across
> backend probes. `cmd/poplar/` user-facing startup errors stay
> on stderr (not slog). Test seam for capturing log output in
> backend tests.
>
> **Settled (do not re-brainstorm):** Use stdlib `log/slog` only,
> no third-party logger. Default handler is `slog.NewTextHandler`
> on stderr at `slog.LevelInfo`; `POPLAR_LOG=debug` raises to
> `LevelDebug`. Per-component logger via `slog.With("component",
> "mailjmap")` etc.
>
> **Still open — brainstorm these:**
> - Where does the root logger live and how does it thread into
>   backends (App-owned vs. package-global vs. context-carried)?
> - Test-time capture: `slog.SetDefault` swap in `TestMain`, or
>   per-test `slog.New(slog.NewTextHandler(&buf, ...))` threaded
>   through a logger seam on `Backend`?
> - Migration scope: convert *all* `fmt.Fprintf(os.Stderr, ...)`
>   in `internal/` this pass, or only the loop-shaped log sites
>   from the 16a audit appendix?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/2026-05-1X-slog-adoption.md`, then
> implement. ADR for the logging convention. Standard pass-end
> ritual; `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh` stays
> at exit 0; `make check` green.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list.
