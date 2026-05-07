# Pass 9k.1 — Comment sweep: mail wire + config

Bring the mail-stack and config packages into compliance with the
§0 write-time rubric and tells T38–T40 installed in Pass 9j.

## Scope

| Package           | Pre-sweep cmt | Density | Notes |
|-------------------|---------------|---------|-------|
| `internal/mail/`      | ~156 | 21.1% | Wire types + classifier, highest density |
| `internal/mailimap/`  | ~287 | 13.7% | IMAP backend |
| `internal/mailjmap/`  | ~198 | 11.4% | JMAP backend |
| `internal/mailauth/`  | —    | 28.8% | Vendored — light touch only |
| `internal/config/`    | ~152 | 10.8% | TOML loader, providers, validation |
| `internal/term/`      | ~51  | 15.9% | Capability detection |
| `cmd/poplar/`         | ~21  | 2.6%  | Already at target |

Target: ≤9% per package (vendored mailauth excluded). Zero
paraphrase comments, zero label-colon godocs, ≤1 ADR/RFC cite per
godoc.

## Method

For each package, in order:

1. Walk every `.go` file (skip `*_test.go` unless a markdown-shaped
   godoc trips T40).
2. For each comment apply §0(a)/(b)/(c). (a)/(b) → delete.
   (c) → keep but rewrite if shape trips T39/T40 semantically.
3. Run paraphrase test on every in-function comment.
4. Compress label-colon godocs and reference-stuffed paragraphs
   into prose. One ADR/RFC cite per godoc.
5. Don't touch vendored provenance blocks. Don't add comments.
6. Commit per-package; `make check` at every commit.

## Tasks

1. `internal/mail/` — sweep + commit.
2. `internal/mailimap/` — sweep + commit.
3. `internal/mailjmap/` — sweep + commit.
4. `internal/mailauth/` — light sweep (non-vendored helpers only) + commit.
5. `internal/config/` — sweep + commit.
6. `internal/term/` — sweep + commit.
7. `cmd/poplar/` — spot-check + commit (or skip if clean).
8. Pass-end ritual: optional ADR-0170 if a recurring shape
   surfaces, invariants/STATUS update, archive this plan, push +
   install.

## Acceptance

- Per-package density ≤9% (vendored excluded).
- Zero paraphrase comments (spot-check, not grep).
- Zero label-colon godocs (semantic, beyond grep).
- ≤1 ADR/RFC cite per godoc.
- `make check` green at every commit.
