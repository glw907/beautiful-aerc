# Poplar Rebuild: Spec-Hardening Review

A three-pass quality gate that runs after Pass 8 (consolidation) locks the
canonical functional spec and before the numbered build plans begin. The goal is
a spec that is complete, correct, and structured for a Claude-first build.

Canonical spec: `docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md`.
Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`.

Each pass runs in its own cleared context. Paste its starter prompt, or say
"start hardening Pass A/B/C". Passes A and B write no spec edits; they commit a
report artifact. Pass C ingests both reports, edits the spec, and reformats it.

## Decisions that frame these passes

- **Report-only for A and B.** Each writes a committed report artifact under
  `docs/poplar/research/`. No spec edits land in A or B. Pass C reads both
  reports and formulates the spec updates.
- **Flag and propose on locked decisions.** A and B may question any settled
  decision and argue for a change. They do not change it. A reopening lands only
  with Geoff's sign-off at the Pass C gate.
- **Max effort, deep research.** Both analytical passes fan out with parallel
  subagents and cite primary sources. The `deep-research` skill is a fit for the
  cataloging and verification work.

## State (2026-05-30)

- [x] Pass A: gap hunt. Artifact: `docs/poplar/research/2026-05-30-spec-gap-analysis.md` (78 confirmed gaps, 17 high priority; 22 locked-decision tensions; 7 candidates adversarially refuted and dropped).
- [ ] Pass B: critique. Artifact: `docs/poplar/research/2026-05-30-spec-critique.md`.
- [ ] Pass C: integrate and reformat. Edits the canonical spec.

## Pass A: gap hunt

Starter prompt (paste after /clear, or say "start hardening Pass A"):

```
Spec-hardening Pass A of the poplar rebuild: gap hunt. Max effort, deep
research. This pass writes NO spec edits. It produces one committed report:
docs/poplar/research/2026-05-30-spec-gap-analysis.md.

Goal: find what the locked functional spec is MISSING, by cataloging the feature
surface of the major mail clients and diffing it against the spec.

Read first: the canonical spec
docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md end to end,
the charter docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md, and the
prior field survey docs/poplar/research/2026-05-29-mail-client-gap-analysis.md.
Go deeper than that survey and diff against the now-complete spec rather than
repeating it. Also read docs/superpowers/specs/spec-hardening-STATUS.md.

Method:
- Catalog the feature surface of the major clients, weighting Thunderbird, Apple
  Mail, Outlook, mutt and neomutt, aerc, K-9 and Thunderbird for Android, Geary,
  Evolution, and alpine and pine. Disregard FairEmail and niche clients (project
  guidance). Cover account and protocol support, folders and labels, search,
  threading, compose, rendering, contacts, calendar, security, automation,
  accessibility, keyboard and UX, offline and sync, notifications, performance,
  extensibility, onboarding, import and export, and whatever else recurs.
- For each capability, mark whether the poplar spec covers it, defers it, or
  omits it. Produce a ranked gap list of capabilities no spec section addresses,
  ranked by how load-bearing each is for the coder audience and the better-Pine
  product.
- Flag any gap whose fix would reopen a locked decision; name the decision and
  the tension, and do not resolve it.
- Fan out with parallel subagents for breadth (one per client family or per
  feature domain) and cite primary sources (docs, man pages, source) where
  practical.

Artifact contents: a short executive summary, the feature catalog as a matrix,
the ranked gap list with rationale and spec cross-references, and the
locked-decision tensions. Commit it as "Spec-hardening Pass A: mail-client gap
analysis" and push. Update spec-hardening-STATUS.md to check off Pass A. Then
stop. Pass B and C run in fresh sessions.
```

## Pass B: critique

Starter prompt (paste after /clear, or say "start hardening Pass B"):

```
Spec-hardening Pass B of the poplar rebuild: critique. Max effort. This pass
writes NO spec edits. It produces one committed report:
docs/poplar/research/2026-05-30-spec-critique.md.

Goal: stress-test the locked functional spec for places it is WRONG, not just
incomplete. Internal contradictions, unsound technical claims, protocol
misunderstandings, behavior underspecified enough that two implementers diverge,
capability-gating holes, UX and keyboard conflicts, security mistakes, and
decisions that look defensible but are likely wrong for the audience.

Read first: the canonical spec end to end, the charter, Pass A's gap report
docs/poplar/research/2026-05-30-spec-gap-analysis.md for context, this tracker
docs/superpowers/specs/spec-hardening-STATUS.md, and the relevant RFCs and
library docs when checking a technical claim.

Method:
- Adversarial review at max effort. Verify the load-bearing technical claims
  against primary sources: the JMAP, IMAP, CONDSTORE and QRESYNC, Sieve and
  ManageSieve, OAuth, PGP and MIME, and iCalendar and iMIP RFCs, plus the actual
  capabilities of the locked libraries (go-jmap, go-imap v2, go-message,
  go-smtp, go-webdav, go-vcard, goldmark, chroma, arran4/golang-ical). Where the
  spec asserts a protocol behavior, confirm or refute it with a citation.
- Hunt contradictions and ambiguity across sections. The consolidation pass
  fixed many; find what remains and anything consolidation introduced.
- Question a locked decision where the evidence says it is wrong. Flag and argue;
  do not change it.
- Fan out with parallel subagents per domain (sync and protocols, organization
  and automation, rendering, compose, search, contacts and calendar and
  security, UX and keymap). Run an adversarial verification step on each finding
  to kill false positives before it enters the report.

Artifact contents: findings grouped by severity (blocking error, soundness risk,
ambiguity, minor), each with the exact spec location, the problem, the evidence
with citation, and a proposed direction (not a final edit). Commit it as
"Spec-hardening Pass B: spec critique" and push. Update spec-hardening-STATUS.md
to check off Pass B. Then stop. Pass C runs in a fresh session.
```

## Pass C: integrate and reformat

This pass edits the canonical spec. It is where the gaps get filled, the errors
get fixed, and the document gets restructured for a Claude-first build.

Starter prompt (paste after /clear, or say "start hardening Pass C"):

```
Spec-hardening Pass C of the poplar rebuild: integrate and reformat. Max effort.
This pass edits the canonical spec.

Read first: the canonical spec
docs/superpowers/specs/2026-05-29-poplar-rebuild-functional-spec.md end to end,
the charter, this tracker docs/superpowers/specs/spec-hardening-STATUS.md, and
BOTH reports: docs/poplar/research/2026-05-30-spec-gap-analysis.md (Pass A) and
docs/poplar/research/2026-05-30-spec-critique.md (Pass B).

Work:
- Triage every finding from A and B. For a finding that fits the locked
  decisions, formulate the spec update. For any finding that reopens a locked
  decision, present it to Geoff for sign-off BEFORE editing. Hold those edits at
  the gate.
- Apply the approved updates: fill gaps, fix errors, resolve ambiguities, and
  update the affected acceptance scenarios so the done-contract stays true.
- Reformat the whole spec for a Claude-first build (see the criteria below).
- Run prose-guard on the result and keep the voice clean.
- Pass-end: update this tracker and the main rebuild STATUS so the next work is
  the numbered build plans, then commit and push.

End at a review gate presenting the integrated, reformatted spec, with the
locked-decision reopenings called out for sign-off.

Claude-first structure criteria:
- Stable, numbered section and subsection anchors a build plan can cite.
- A stable ID on every acceptance scenario (for example AC-1.1 through AC-8.10)
  so a build-phase test maps to one ID.
- Each section self-contained, cross-referencing by section number, with no
  dependence on an external or archived baseline.
- Normative language (MUST, SHOULD, MAY) where a statement is binding, with
  rationale clearly marked non-normative.
- The capability tables, config-key index, glossary, and deferral register kept
  current as the single reference surfaces.
- A per-section build-plan boundary note: what the spec locks versus what the
  plan decides.
- One terminology set, enforced against the glossary.
- A short "how to read this spec" preamble aimed at an implementer subagent.
```

## Pass-end note

Each pass updates the State checklist above. After Pass C the spec phase plus the
hardening review are complete, and the next work is the numbered build plans
(charter section 9).
