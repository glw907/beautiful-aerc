---
name: Fix the cause not the symptoms
description: When debugging, identify the source of the problem before trying display-layer fixes
type: feedback
---

When something unwanted is showing up in output, trace it back to the
source (the tool/server producing it) rather than trying to suppress
it at every display layer.

**Why:** Wasted rounds trying to suppress issues at the display layer
when the fix was configuring the producer.

**How to apply:** Ask "what is producing this?" first. Configure the
producer, not the consumer. In the pipeline context: if pandoc
produces bad output, check if a pre-pandoc HTML cleanup can prevent
it rather than adding another post-pandoc regex.
