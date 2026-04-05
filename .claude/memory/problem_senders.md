---
name: Problem sender patterns
description: Email sender types that stress the HTML filter pipeline -- useful for regression testing
type: project
---

Sender types that produce challenging HTML for the pipeline:

- **Marketing emails** -- layout tables, tracking images, nbsp padding,
  responsive duplicates in display:none divs
- **Bank of America** -- zero-size tracking pixels inline between URL
  fragments, unclosed `<strong>` tags producing consecutive bold
- **Apple receipts** -- complex nested tables, heavy inline styles
- **GitHub notifications** -- image-links, multi-line link text,
  tracking redirects with empty URLs
- **Google Calendar** -- image buttons for RSVP, empty-URL links
- **Thunderbird senders** -- `class="moz-*"` attributes pollute output
- **Yahoo mail** -- various DOCTYPE formats, non-standard structures
- **Microsoft Outlook** -- platform-specific markup, MSO conditionals
- **Newsletters** -- HTML with no paragraph breaks in text/plain part
  (reason HTML MIME part is preferred over text/plain)
- **Remind.com** -- bare domain URLs without https:// scheme
- **Callcentric** -- angle-bracket autolink URLs
- **ClouDNS** -- empty mailto: links
- **Spotify/dbrand** -- responsive HTML with duplicate content in
  display:none sections

**Why:** These patterns recur. New pipeline fixes should be tested
against this list to catch regressions.

**How to apply:** When making pipeline changes, mentally check whether
the change could affect any of these sender types. The `corpus/`
directory and `scripts/audit.sh` can test against real examples.
