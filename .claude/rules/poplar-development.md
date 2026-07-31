---
description: Development workflow for poplar, routes to the active re-founding track
---

Poplar has one active track: the re-founding. Charter:
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.

When the user says "continue", "continue development", "next phase",
"next pass", "run pass N", or any phase or pass trigger, read
`docs/superpowers/specs/poplar-refounding-STATUS.md` for the current
phase or pass and its starter prompt, then the charter. On any phase
close, run the phase-end ritual at the top of that STATUS; updating
the STATUS is its first and non-optional step.

The build follows the requirements' spine: six numbered passes
(foundation; design language and shell; mail read path; compose;
calendar; hardening). Each pass is one plan doc under
`docs/superpowers/plans/`, authored in the pass's execution session
from the spine and the specs, and executed subagent-driven with
per-task review. A screen pass adds a design ritual: a text wireframe
per screen per responsive class that changes its layout, pointer
targets included, reviewed by Geoff before the screen's tasks
dispatch. A pass ends with consolidation (simplify, reviewer fan-out,
STATUS update, plan archival) and closes at a pass gate with Geoff.

**A pass can be split at a logical point, and the numbered spine does
not forbid it** (pass 1b exists because pass 1 burst). The signal to
propose one is repeated task splits: splitting a task keeps the work
inside the pass, and only splitting the pass lets work leave, so
task-splitting looks like sizing discipline while changing nothing about
the commitment. A second task split inside one pass is the prompt; a
third means the proposal is overdue. Propose it with a named cut point,
usually the last clean self-contained task, and give the follow-up pass
a letter so its work has a home rather than being homeless. Geoff rules
on the split; the orchestrator owes him the proposal unprompted, because
he sees per-item summaries and the orchestrator holds the dispatch list.

Two closed tracks remain in the repo as reference only: the dogfood
client (tag `poplar-legacy`, branch `legacy`) and the 2026-05-29
rebuild spec track (`docs/superpowers/specs/poplar-rebuild-STATUS.md`,
`spec-hardening-STATUS.md`, the 2026-05-29 charter and functional
spec). Their documents bind nothing; the rebuild track's functional
spec and gap analysis are research inputs per the re-founding charter.
Do not resume either on a generic "continue".
