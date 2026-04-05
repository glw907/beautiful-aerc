---
name: Edge case fix workflow
description: When the user flags rendering issues, fix the filter and ship autonomously
type: feedback
---

When the user reports formatting issues (via corpus, pasted output,
or description), fix the beautiful-aerc filter code autonomously,
then run /ship when done. No need to ask for confirmation -- diagnose,
fix, and ship.

For batch fixes, use the /fix-corpus skill which previews each corpus
email via tmux, triages by pattern, fixes holistically, and ships.

**Why:** The user tests beautiful-aerc by reading real email and
spotting rendering problems. They want quick turnaround, not
discussion.

**How to apply:** On seeing reported issues, immediately start
diagnosing. Trace through the pipeline, fix the Go code, and ship.
