# ADR-0008: staged deterministic pipeline with a doc model

Date 2026-07-27. Status: accepted (Phase 4). Binds the Phase 1
verdict's settled constraints (C2).

## Context

Rendering is the central bet, validated in Phase 1: deterministic
Go, named provenance-carrying rules, fired-rule traces, honest
downgrades (RD-1, RD-2), byte-determinism per profile tuple
(QA-7). The survey confirmed the building blocks and the two
diseases with no library cure (layout tables, hidden content).

## Decision

A staged pipeline, pure function of (raw bytes, RenderContext):
decode (enmime; charset repair; defect accumulation) → part plan
(text/calendar first, then html, then plain; choice recorded) →
DOM parse (x/net/html) → rule engine (ordered named rules over DOM
and doc; every firing traced) → poplar's intermediate document
model (blocks, inlines, links, code, quotes, tables, attachments)
→ fact check (deterministic extractors for links, amounts, dates,
codes; missing facts downgrade the render) → emit (markdown for
glamour, filtered plain text, or raw). The rule engine owns
layout-table linearization and hidden-content elision as named
rules. Quote folding and link numbering are doc-model transforms.
The raw source is retained and reachable for every message; the
offline improve harness (RD-15) ships in-repo.

## Alternatives considered

- **html-to-markdown/v2 as the primary path with rule hooks**: its
  hook granularity is unverified (survey open item) and the rule
  engine would live inside another library's traversal order,
  which breaks trace ownership. It stays the baseline comparison
  in the improve loop.
- **Readability-style extraction in the pipeline**: heuristic
  tuning shifts output between releases, a direct QA-7 hazard;
  extraction stays outside the deterministic guarantee if it ever
  ships at all.
- **Direct DOM→ANSI rendering (skip markdown)**: loses glamour's
  mature terminal typography and the fallback-stack symmetry
  (RD-3 cycles renditions of the same doc model).

## Consequences

The doc model is the pipeline's stable center: rules, fact check,
fallbacks, and copy-out all speak it. Golden render tests pin
byte-determinism per declared profile. QA-9's regrade harness
runs the Phase 1 protocol against the productized pipeline.
