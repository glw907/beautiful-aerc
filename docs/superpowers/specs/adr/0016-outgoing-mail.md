# ADR-0016: the outgoing-mail path

Date 2026-07-27. Status: accepted (Phase 4). Added in revision 2:
the adversarial review found compose's three MUSTs (CO-2, CO-3,
CO-5) had no design home.

## Context

Reply seeding carries the largest fixture matrix in the compose
path (recipient math, reference chains, prefix idempotence,
attachment carry-through). The live probe proved identity
auto-selection is poplar-owned: Fastmail's Identity list is
curated, with no wildcard or catch-all entries. CO-5 requires a
committed tag/attribute allowlist fixed in Phase 4 and a plain
part that never reflows code or diffs.

## Decision

Three pure subsystems in `internal/mail`, plus assembly:

- **Reply seeding**: a pure function from (source message, mode,
  identities) to a compose seed, covering CO-2's whole matrix;
  quoting operates over the rendered doc model so quote depth is
  preserved from what the user actually read.
- **Identity matching**: exact delivered-to match → configured
  alias patterns → domain-suffix match → account default;
  per-recipient last-identity memory rides `sent_history`.
  Signature materialization uses Catkin's buffer-mutation API
  (one buffer-undo entry) and the byte-identical swap rule.
- **Assembly**: goldmark AST → text/plain at fixed 72-column
  wrap with verbatim passthrough for fenced regions and unfenced
  diff-shaped runs, and → the conservative HTML part restricted
  to the committed allowlist
  (`2026-07-27-poplar-html-allowlist.md`), inline styles only,
  built with go-message. A text-only toggle omits the HTML part.

## Alternatives considered

- **format=flowed for the plain part**: Gmail has never honored
  it and practitioner consensus abandoned it; fixed wrap with
  diff-aware passthrough meets CO-5's no-reflow criterion with
  less machinery. (Inbound un-flowing is separate and required;
  technical design section 9.)
- **A template engine (hermes-style) for the HTML part**: built
  for a different product shape (marketing/transactional
  layouts); poplar's markdown-shaped output needs a renderer,
  not templates.
- **CSS inlining via go-premailer**: solves authoring-time CSS
  the inline-first design never writes.
- **Server-side identity data as the matcher**: the probe result
  forecloses it; the server does not know the aliases.

## Appendix: the `internal/when` ruling

The survey did not cover natural-language date parsing (a C9
gap). Candidates checked at decision time: olebedev/when,
araddon/dateparse, tj/go-naturaldate. Ruling: hand-rolled. The
shared parser backs acceptance-tested behaviors ("next wed",
"aug 14") where deterministic, en-only, documented outcomes
matter more than breadth, and the C6 obligation (one parser
behind every date surface) makes its grammar poplar API, not an
implementation detail.

## Consequences

The compose path is testable without a terminal: seeding,
matching, and assembly are pure functions with fixture matrices.
The allowlist is data with a validation test, so recipient-
compatibility changes are documented amendments, not code
archaeology. Build-order step 4 opens with designed subsystems.
