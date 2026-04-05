# Batch Email Audit for Filter Quality

## Problem

The current workflow discovers filter issues one email at a time - read an email in aerc, spot a rendering problem, fix it, ship. This produces reactive, narrow fixes without visibility into patterns across the corpus. A batch approach lets us see the full picture, find root causes, and make principled pipeline improvements.

## Design

### Phase 1: Audit Script

A shell script at `scripts/audit.sh` that:

1. Reads raw RFC 2822 emails from `~/.cache/aerc/907.life/blobs/`
2. For each blob, extracts the sender domain and subject from headers
3. Selects a diverse sample biased toward maximum sender domain variety - at most 1-2 emails per sender domain to maximize breadth of HTML generators encountered
4. Extracts the HTML body from each selected email, handling MIME multipart boundaries and Content-Transfer-Encoding (base64, quoted-printable)
5. Pipes the HTML through `beautiful-aerc html`
6. Writes rendered output to `audit-output/<hash>.txt`, prefixed with sender and subject for context
7. Writes `audit-output/index.txt` listing all processed emails with sender, subject, and output filename

The `audit-output/` directory is gitignored. The script is kept in the project as a reusable developer tool for future batches.

### Phase 2: Review and Catalog

Read all rendered outputs. For each, evaluate:

- Does this look like well-formatted markdown?
- Structural problems: broken lists, orphaned markup, stray ANSI codes, bleeding emphasis, bad whitespace, unstripped HTML entities
- Aesthetic problems: ugly but technically correct rendering

Catalog every issue found. Group by pattern - which pipeline stage is responsible, which HTML input patterns trigger it, how many emails are affected.

### Phase 3: Fix and Refactor

With the full catalog:

- Fix issues at their root cause, not at the symptom level
- Identify cleanup stages doing overlapping or redundant work
- Consolidate where multiple narrow fixes can become one principled approach
- Simplify pipeline stages that have accumulated band-aid patches
- Re-run the audit to verify fixes and confirm no regressions

## Constraints

- The audit script handles MIME correctly - multipart/alternative, multipart/mixed, nested multipart, Content-Transfer-Encoding
- Emails without an HTML part are skipped
- No automated quality checks - the value is in human (Claude) review of rendered output
- The script must work with the current blob cache; no aerc interaction required

## Out of Scope

- CI integration
- Automated lint/quality scoring
- Changes to the audit script beyond basic functionality
