# Rendering readability principles

**Date:** 2026-07-19
**Phase:** Re-founding Phase 1, the rendering bet
**Source:** Distilled from 21 hand-authored ideal renders across seven
message classes (github-ci, newsletter, transactional, marketing,
personal, calendar, list-patch; three per class). The corpus and the
ideal renders contain private mail and stay uncommitted. This document
states the standard they define, with every example abstracted or
invented, so it stands alone.

An ideal render answers one question: if the sender had written this
message as markdown for a terminal instead of as HTML for a webmail
pane, what would they have written? The ideal preserves every fact a
reader might act on, presents it in the structure the sender intended,
and carries nothing else. HTML email buries a small amount of content
under a large amount of delivery apparatus. Rendering is the act of
recovering the content and discarding the apparatus.

## What excellent means per class

### github-ci

Automated developer notifications reduce to a status heading, one
context line (repository, branch, short commit), a compact table when
the payload is genuinely tabular (jobs and their results), and a single
canonical action link with tracking parameters stripped. Redundant
links that resolve to the same destination collapse into one. When the
message is a human comment delivered through the platform, the standard
inverts to preservation: the author's prose, inline code, blockquotes,
and real links survive verbatim, including the author's own
punctuation, with only the platform chrome (avatars, reply footers,
beacons, embedded metadata scripts) removed. Attribution survives as a
bold name lead-in with a permalink. Facts the body assumes from the
Subject header (a quota figure, a branch name) get pulled into the
render so the body is self-sufficient.

### newsletter

An excellent newsletter render keeps the editorial contract and drops
the commercial wrapper. Stories become real headings. House-style
devices that carry meaning (bold lead-in labels, section-marking emoji,
a word-count line, a table of contents, per-story bylines and
sign-offs) survive because the reader expects them. Sponsor blocks,
share buttons, self-promotion, and footers go, even when they sit
inside the editorial flow rather than below it. Headings that the HTML
carries only as images or styled cells are inferred and written as
text. Links that are pure tracking redirects dressed as citations are
dropped entirely; links a reader would act on (register, vote, donate)
stay inline even when the wrapped URL is enormous, with shouting
button-case labels demoted to sentence-level link text. Numbered
paragraphs become ordered lists. Notices become blockquotes. Invisible
characters are stripped.

### transactional

A receipt, report, or verification code renders as its facts and
nothing else. Key-value blocks (order metadata, billing identity)
become bold-label lists. A line-items-and-total grid stays a real
table, with the total as a bolded final row because that is where the
HTML puts it. Multi-row addresses merge into the one line a person
would write. A verification code is promoted to a heading set in
inline code so it dominates the render and copies cleanly,
substituting structure for the display type the HTML spent on it.
Identical-every-time boilerplate condenses rather than transcribes,
and when the real payload lives in an attachment, the render says so
explicitly, because that pointer is the one thing the reader needs.
Every billable or actionable fact survives: amounts, quotas,
percentages, reset and renewal dates, policy sentences a reader might
act on.

### marketing

Marketing renders as offer, deadline, action. The headline offer
becomes the H1 or a bold lead line, with a deadline the HTML buried in
fine print surfaced into the lede. When most of the message lives in
image alt text, the render is reconstructed from the alts. Sections
that all funnel to one shop action collapse into a single link, and a
long opaque redirect URL still earns its place when it is the
message's one action. Legal terms split by audience: for a broad
consumer blast they compress into one short fine-print paragraph that
keeps dates, channels, and exclusions; for an invited-participant
offer the terms are the actionable substance, so they become headed
sections with FAQ facts folded into the restrictions they clarify.
When the message is a short human letter in a marketing wrapper, the
work is subtraction and restraint: strip the platform shell, keep the
sender's prose and sign-off verbatim.

### personal

Personal mail defines the floor: a one-line note or a bare shared URL
renders as exactly that, with zero added chrome and no framing the
sender never wrote. Forwards open with a compact attribution block
(From, Date, Subject, To) recovered from the client's markup, then
present the embedded original at top level when the forwarder added no
prose. Replies put the new prose first and demote the quoted history
below a rule as a blockquote with a bolded attribution line.
Client-mangled text is repaired: word-per-span markup is stitched back
into sentences, symbol-font bullet paragraphs become real lists,
double-spaced non-breaking runs collapse. Signatures compress to name,
title or firm, and direct phone; confidentiality notices, office
addresses, and disclaimer blocks go. Facts inside a quoted message (a
date, a renewal URL, a contact address) stay when a reader may still
act on them.

### calendar

An invite renders as the event name in a heading, bold-label fact
lines (organizer, when with timezone, where, guests), a join section
(meeting URL, phone bridge with PIN), and an RSVP line. The RSVP
action URLs are enormous tokenized blobs and are also the invite's
entire point, so they stay inline behind Yes / No / Maybe link text
while duplicate same-destination calendar links are shed. An update
renders the diff as the message: a bold lead naming what changed, the
new value, and the old value in strikethrough with a "was" cue. A
bodyless programmatic response (an acceptance with an empty HTML body)
is synthesized from headers into one declarative sentence, with an
explicit no-message note so the emptiness reads as fact rather than as
a rendering failure. Honest synthesis beats echoing a blank body.

### list-patch

Mailing-list traffic renders as the thread, not the platform. Nested
reply levels become real markdown blockquotes with compact attribution
lines, keeping every level needed to make the top reply intelligible.
List footers (view, reply, mute, unsubscribe), hashtag chrome, and
signature-delimiter dashes go. Duplicate URLs merge into one inline
link; a bullet list of bold labels over bare URLs becomes a list of
linked items. A list event announcement renders like a calendar
invite: title heading, fact lines, fee and deadline warnings promoted
from footnote asterisks into a proper list, and one link per distinct
action (register, add to calendar, details, map). An announcement
letter from a project keeps its prose and its sign-off untouched.

## Recurring moves

The same operations recur across all seven classes. These six are the
standard's core.

### Structure inference

The render recovers the document the sender meant, which the HTML
often only implies. Headings are inferred from images, styled table
cells, or giant display text and written as markdown headings under a
sane, uniform hierarchy (parallel sections at one level, not the
HTML's accidental mix). Nested layout tables, spacer cells, and
wrapper divs collapse into linear prose. Numbered paragraphs become
ordered lists; symbol-font and asterisk devices become real lists;
warning banners become blockquotes; shredded span-per-word text is
stitched back into sentences. When the body is empty or the payload
is elsewhere, the render synthesizes the one honest sentence the
headers support rather than emitting a blank.

### Noise shedding

Delivery apparatus never survives: logos, avatars, hero images,
spacer gifs, tracking pixels, preheader and hidden-preview text,
embedded metadata and script blocks, social-icon rows, share buttons,
sponsor blocks, view-in-browser links, unsubscribe and mailing-list
footers, postal-address blocks, confidentiality notices, and platform
CSS. Boilerplate that repeats identically in every message of a
stream condenses to its actionable core. The test is whether a reader
could ever act on the element; decoration and compliance chrome fail
that test, while a policy sentence, an eligibility rule, or a
fine-print deadline can pass it.

### Link and image policy

Links are kept per distinct action, not per anchor tag. Duplicate
links to one destination merge into one. Pure tracking redirects
posing as citations are dropped. Tracking and campaign query
parameters are stripped when the base URL is self-evidently the
destination; an opaque redirect that cannot be unwrapped is kept
whole when it is the message's action, because losing the action is
worse than keeping an ugly URL. Link text is sentence-level prose,
with button-case labels demoted. A message may correctly end with
zero links or with exactly one bare URL. Images never survive as
images; an image that carries content contributes its alt text or its
URL-derived caption to the render, and a decorative image contributes
nothing.

### Table intent

A markdown table appears only where the content is genuinely tabular:
line items with a total, jobs with results. Everything the HTML put
in tables for layout is flattened. Key-value infoboxes render as
bold-label lists, not tables. A total row that lives in the source
grid stays in the rendered table as a bolded final row. Progress
bars, meters, and other graphical figures render as one bold
fact line stating the numbers the graphic encoded.

### Quote handling

New prose comes first; quoted history is demoted below a rule into
blockquotes. Quote depth is preserved to the level needed for the
top message to be intelligible, with each level carrying a compact
attribution line. Forward and reply attribution blocks are rebuilt as
bold-label lines from the client's markup. A forward that adds no
prose opens directly with the attribution block and presents the
original at top level. Quoted footers and styling are stripped, while
facts inside the quote that a reader may still act on stay.

### Fact preservation

Zero tolerance for lost actionable facts. Amounts, quotas, dates,
deadlines, timezones, codes, identifiers, names, phone numbers,
eligibility rules, and policy consequences all survive, including
facts the HTML hid in fine print, alt text, footnotes, or the Subject
header. Emphasis follows the facts: the fact the message exists to
deliver gets the render's strongest structure (a heading, a bold lead
line, a strikethrough diff), and surfaced fine print keeps its dates
and exclusions even when condensed. The sender's own words are never
paraphrased when they can be preserved; condensation is reserved for
boilerplate, and synthesis for bodies that carry no prose at all.

## Grade definitions

These four grades apply to every class for the whole phase. Each is
defined by what a grader can observe in the output, not by how the
output was produced.

**Excellent.** Reads as if the sender wrote it as markdown natively.
Every actionable fact is present, and the message's central fact
carries the render's strongest structure. No noise: no chrome, no
tracking artifacts, no layout residue, no duplicate links, no invented
framing. Structure matches sender intent, including inferred headings,
real lists, correct quote depth, and tables only where content is
tabular. A minimal message renders minimally; an empty body renders as
an honest synthesized statement.

**Usable.** Every actionable fact is present and findable on a single
read. Minor noise or structural awkwardness a reader tolerates
without effort: a stray boilerplate line, a surviving duplicate link,
a heading at the wrong level, a key-value block rendered flat, an
unstripped tracking parameter. The reader never has to guess at a
fact or reconstruct structure, only to overlook small flaws.

**Degraded.** The facts can be extracted, but the reader must fight
for them. Observable symptoms: actionable content interleaved with
surviving chrome or raw URLs, layout-table residue breaking prose
flow, quote levels collapsed so attribution is ambiguous, the central
fact present but stripped of all emphasis and buried mid-body, lists
or tables rendered as run-on text. Reading requires scanning past
noise or mentally reassembling structure, and a hurried reader might
miss a fact that is technically present.

**Fail.** An actionable fact is lost or buried beyond practical
recovery, or the output is unreadable. Observable symptoms: a
deadline, code, amount, or action link absent from the output; the
one action link dropped as if it were tracking noise; a blank or
near-blank render of a message that carried content; raw HTML, base64,
or CSS in the output; content so scrambled that the sender's meaning
cannot be reconstructed. Honest emptiness is not failure when the
source was empty; silent emptiness when the source had content always
is.
