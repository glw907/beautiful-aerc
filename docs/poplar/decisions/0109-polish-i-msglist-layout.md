---
title: Formula-driven message-list / sidebar layout (Spartan / Intermediate / Full)
status: accepted
date: 2026-05-02
---

## Context

ADR-0096 set the responsive sidebar to `clamp(termWidth - 56, 24,
30)` — a continuous slope from 24 cells at termWidth=80 up to 30
at termWidth>=86. That was correct for the time but left
message-list column allocation static (sender=22, date=14,
flag=2). At 80×24 — the polish bar — the subject column collapsed
to 6 cells, making the entire pane unreadable for the dominant
case where users have long sender names and threaded prefixes.

A second-look at column allocation found three independent levers
worth varying with termWidth: sender width (real-world coverage
cliffs at 22/28/32 cells), date format (none/3/5 cells), and
chrome toggles (flag column, sidebar icons). Real-data sampling
of 2000 inbox messages shaped each cliff; terminal-size research
shaped the breakpoints.

## Decision

A pure function `ComputeLayout(termWidth) LayoutMode` is the
single source of truth for sidebar/sender/date/flag/icons sizing.
Continuous formulas govern sidebar and sender; discrete thresholds
gate the chrome toggles:

```
sidebar = clamp(round(14 + (W - 80) * 0.2),  14, 30)
sender  = clamp(round(22 + (W - 80) * 0.125), 22, 32)
flags   = (W >= 90)
date    = 0 if W < 90 else (3 if W < 100 else 5)
icons   = (sidebar >= 20)   # ≈ W >= 108
```

Three coherent UI tiers emerge: Spartan (W=80–89), Intermediate
(W=90–107), Full (W=108+). The 14-cell ISO date format is
removed — row position conveys recency on the default `date-desc`
sort, and the cells are better spent on subject.

## Consequences

- 80×24 polish bar: subject 6→34 cells (5.7×). Spartan tier
  gracefully degrades to name + subject only.
- W=90 brings flags + 3-cell relative date back; W=100 upgrades
  to 5-cell absolute; W=108 brings sidebar icons.
- Three small transitions on resize (3-, 4-, 1-cell subject
  costs) — well below the threshold where a CSS-style media-query
  snap is jarring.
- ADR-0096 superseded. The continuous slope is preserved (sidebar
  formula matches at the endpoints W=80→14 and W=160→30) but
  extended to cover the full range and paired with a sender slope
  + chrome thresholds.
- Sender-width cliffs are based on a 2000-message sample of one
  user's mail. Other users may have different distributions, but
  the cliffs (22/28/32) match common sender-name patterns
  (personal "First Last", brand "Bank of America", group@list).
