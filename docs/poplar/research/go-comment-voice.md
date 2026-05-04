# Go Comment Voice — synthesis output

The synthesized style guide moved to `~/.claude/docs/go-comment-voice.md`
so it is accessible from any Go project on this workstation, not just
poplar.

This directory keeps the provenance:

- `2026-05-04-synthesis-brief.md` — the structural brief the synthesis
  agent worked from.
- `sources/2026-05-04-authoritative-docs.md` — Effective Go, Russ Cox's
  Go Doc Comments, Code Review Comments, Google Go Style Guide, Uber.
- `sources/2026-05-04-stdlib-exemplars.md` — verbatim excerpts from
  `net/http`, `encoding/json`, `io`, `sync`, `os/exec`, `errors`,
  `context`.
- `sources/2026-05-04-third-party-exemplars.md` — Charm libs, HashiCorp,
  Kubernetes, Prometheus, emersion, rockorager.
- `sources/2026-05-04-essays-and-proverbs.md` — Pike, Cox, Cheney,
  Gerrand, Effective Go.

ADR-0141 (Pass 8.8) codifies the policy and points at
`~/.claude/docs/go-comment-voice.md` as the binding artifact.
