# Poplar Status

**Current pass:** Pass 38.1 — Audit E remediation. Pass 38 closed
ADR-0225: walked 212 active ADRs → 121 chosen / 79
defaulted-still-right / 12 defaulted-and-wrong. Three P1
findings queue (F2 movepicker arrow-keys, F4 outbox payload
unbounded, F6 OAuth no device-code) plus F1 doc-hygiene bundled
free. F6 may split to 38.2 if 38.1 oversizes. Pass 35.1 still
pending Gmail/Outlook creds.

**Beta soak deferred.** Pre-beta rules apply.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 34 | Scaffold through cross-pane mouse | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 36 / 36.1 | Audit C + remediation (ADR-0221/0222) | done |
| 37 / 37.1 | Audit D + remediation (ADR-0223/0224) | done |
| 38 | Audit E (ADR-0225) | done |
| 38.1 | **Audit E remediation — F1/F2/F4/F6** | next |
| 39 | Audit F — sharp edges + insecure defaults | gate |
| 40 | Audit G — test assertion meaningfulness | gate |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 38.1)

> **Goal.** Land Audit E's three P1 findings plus F1's bundled
> doc-hygiene fix per ADR-0225, gating Audit F.
>
> **Scope.**
>
> - **F1.** Set `docs/poplar/decisions/0003-external-editor-only.md`
>   frontmatter to `status: superseded by 0034`; link from 0003's
>   Consequences section. Doc-only.
> - **F2.** `internal/ui/movepicker/`: mirror the sidebar-search
>   pattern (ADR-0064) — `Tab` cycles filter/nav, `j`/`k` navigate
>   in nav mode. Update `docs/poplar/keybindings.md` and
>   `.claude/rules/ui-invariants.md`.
> - **F4.** `[cache] max-outbox-bytes` config field (default
>   unlimited, like ADR-0122's `max-size`). `insertFolderOp` /
>   `QueueOutbound` validates per-row before INSERT; surface as
>   `cache.ErrOutboxRowTooLarge`.
> - **F6.** RFC 8628 device-code Authorize mode in
>   `internal/mailauth/`. Wizard offers it as fallback when
>   loopback PKCE fails or the user picks remote/SSH. Reuses
>   keyring/age-file token store. Update ADR-0193's Consequences;
>   no new ADR.
>
> **Settled.** F12 already fixed by ADR-0165 — skip. P2 findings
> (F3, F5, F7–F11) not in scope.
>
> **Open — brainstorm.** F6 wizard surface: separate stage vs.
> credential-strategy radio (ADR-0190 projection). Probably the
> radio; confirm before coding. Split F6 to Pass 38.2 if 38.1
> task count exceeds 12 per CLAUDE.md pass-size budget.
>
> **Approach.** Brainstorm F6's wizard shape, plan at
> `docs/superpowers/plans/YYYY-MM-DD-audit-e-remediation.md`,
> then implement. Standard pass-end checklist.
