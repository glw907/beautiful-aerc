# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. This is separate from the live poplar tracker at `docs/poplar/STATUS.md`.

## Current state (2026-05-29)

Pass 0 (Charter) and Pass I (Claude infrastructure refresh) complete.

Pass 0 artifacts:
- Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`.
- Gap analysis: `docs/poplar/research/2026-05-29-mail-client-gap-analysis.md`.
- Infra-refresh research: `docs/poplar/research/2026-05-29-infra-refresh-research.md`.

Pass I outcomes:
- `CLAUDE_CODE_SUBAGENT_MODEL` set to `inherit`, so each agent's frontmatter model resolves normally (verified against the model-config doc). The five user agents pin full model IDs; the reviewers run Opus again, which is what their frontmatter always declared.
- Rebuild agents in `.claude/agents/`: `poplar-implementer` (Sonnet, dispatch `model:opus` for judgment-heavy), `poplar-reviewer` (Opus, spec plus quality, effort high), `poplar-go-reviewer` (Sonnet, conventions).
- Gate artifacts in `docs/superpowers/specs/rebuild-gate/`: `Makefile.gate`, `.golangci.yml` (v2 schema, validated; golangci-lint joins the gate, gofumpt replaces gofmt, modern-go-check runs strict), and `README.md` (gate, model-per-role table, orchestration default).
- Go-idiom catalogue extended: tells T42 (reflexive pointer receivers) and T43 (goroutine without lifecycle coordination), plus a `cmp.Or` coalescing voice-lens note. Mirrored in the go-conventions skill.
- Plan and execution record: `docs/superpowers/plans/2026-05-29-pass-I-infra-refresh.md`.
- Greenfield tree location stays deferred to the spec-to-build boundary (charter §11), affirmed this pass.

Orchestration default: `subagent-driven-development` for the domain passes; Workflow for wide independent fan-out; `ultracode` as the middle path.

## Next: Pass 1, accounts, protocols, sync

Starter prompt (paste after /clear, or say "start Pass 1"):

```
Start Pass 1 of the poplar rebuild (accounts, protocols, sync). This is a
domain spec pass: it writes a spec section, it does not build. Read first:
docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md (esp. section 5
seams 1 and 2, section 6 decisions 1 and 2) and
docs/poplar/research/2026-05-29-mail-client-gap-analysis.md.

Brainstorm and settle the two foundational decisions, then write the spec
section and append it to the canonical functional spec:
1. Multiple accounts plus unified inbox: cache layout, credential storage,
   sidebar shape, per-account vs unified views.
2. Labels/tags vs folders-only: whether labels are a poplar-side overlay or
   strictly server-backed (JMAP multi-membership maps cleanly).
Then specify the Backend interface (JMAP and IMAP coequal), identities, the
OAuth flow, and the cache/sync contract. End with numbered acceptance
scenarios.

Pass-end: update the rebuild STATUS and append the execution record. The
Pass I gate and agents are live; domain passes that build later use them.
```

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [done] -> 1 Accounts/sync [next] -> 2 Organization -> 3 Reading/triage -> 4 Rendering -> 5 Compose -> 6 Search -> 7 Contacts/calendar/security -> 8 Consolidation -> build plans + clean build.
