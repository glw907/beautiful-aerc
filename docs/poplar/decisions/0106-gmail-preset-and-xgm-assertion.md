---
title: Gmail preset and X-GM-EXT-1 assertion
status: accepted
date: 2026-05-02
---

## Context

ADR-0098 / ADR-0101 set the provider-registry shape and named
Gmail as a Pass 8.1 preset. The IMAP backend (ADR-0099/0100) is
generic; Gmail's IMAP server has well-known eccentricities
(EXPUNGE-only-deletes-in-Trash, label-as-folder semantics) that
must be gated to Gmail accounts only.

## Decision

`config.Provider` gains `GmailQuirks bool`. The `gmail` preset is:

```
"gmail": {
    Name: "gmail", Backend: "imap",
    Host: "imap.gmail.com", Port: 993,
    AuthHint: "xoauth2", GmailQuirks: true,
}
```

The flag is copied onto `AccountConfig` during preset resolution
(mirroring `InsecureTLS`). The IMAP backend's `capSet` gains
`XGM bool`, populated from `caps["X-GM-EXT-1"]`. When
`b.cfg.GmailQuirks && !cs.XGM`, `finishConnect` returns an error
— a Gmail account on a server without X-GM-EXT-1 is a
misconfiguration, not a fallback case.

## Consequences

- Other Gmail-specific paths (Destroy routing in ADR-0107) gate on
  the same flag with the same name in both places.
- Non-Gmail backends still populate `cs.XGM` (from `caps[...]`),
  but it is never read outside Gmail-quirks code paths today.
- Future presets that need quirks add another bool rather than
  reusing `GmailQuirks`. Pre-beta cleanups can unify if a pattern
  emerges; we do not abstract on a sample size of one.
