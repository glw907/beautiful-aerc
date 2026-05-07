# Pass 9k.2 — Comment sweep: cache + outbound chain

Second slice of the comment voice sweep. Bring the cache + outbound
chain into compliance with the §0 write-time rubric and tells
T38–T40 (ADR-0168), with the density-floor exemption from ADR-0170
applied where the package shape warrants it.

## Scope

| Package              | Pre-sweep cmt | Density | Notes |
|----------------------|---------------|---------|-------|
| `internal/cache/`    | 344 | 14.0% | Largest single package; drainer + outbox + body store |
| `internal/compose/`  | 86  | 11.7% | Editor seam, Draft, AssembleMIME, Seed* |
| `internal/content/`  | 152 | 12.8% | RFC 5322 parsing, address lists |
| `internal/filter/`   | 130 | 21.4% | Goldmark wrappers — highest density in slice |
| `internal/tidy/`     | 46  | 6.8%  | Already at target — spot-check only |
| `internal/humanize/` | 2   | 7.7%  | Tiny; spot-check |
| `internal/backoff/`  | 8   | 28.6% | 28-line file; ADR-0170 small-public-API exempt |
| `internal/theme/`    | 20  | 4.7%  | Already at target — spot-check only |

Target: ≤9% density per package, except where the package is mostly
public-API declarations (ADR-0170 exemption). For `backoff` and
`humanize`, qualitative gate only.

## Method

For each package, in order:

1. Walk every `.go` file (skip `*_test.go` unless a markdown-shaped
   godoc trips T40).
2. For each comment apply §0(a)/(b)/(c). (a)/(b) → delete.
   (c) → keep but rewrite if shape trips T39/T40 semantically.
3. Run paraphrase test on every in-function comment.
4. Compress label-colon godocs and reference-stuffed paragraphs
   into prose. ≤1 ADR/RFC cite per godoc.
5. Don't add new comments.
6. Commit per-package; `make check` at every commit.

## Tasks

1. `internal/cache/` — sweep + commit (largest, may want sub-commits per file group).
2. `internal/compose/` — sweep + commit.
3. `internal/content/` — sweep + commit.
4. `internal/filter/` — sweep + commit (highest density in slice).
5. `internal/tidy/` — spot-check + commit if needed.
6. `internal/humanize/` — spot-check + commit if needed.
7. `internal/backoff/` — spot-check (ADR-0170 exempt).
8. `internal/theme/` — spot-check + commit if needed.
9. Pass-end ritual: invariants/STATUS update, archive this plan,
   push + install. New ADR only if a recurring shape surfaces.

## Acceptance

- Per-package density ≤9% except ADR-0170-exempt packages.
- Zero paraphrase comments (semantic spot-check).
- Zero label-colon godocs (T40).
- ≤1 ADR/RFC cite per godoc.
- `make check` green at every commit.
