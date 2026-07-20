# Public email corpus survey

**Date:** 2026-07-19
**Phase:** Re-founding Phase 1, the rendering bet (future-work research)
**Question:** Does the internet provide a good, public, modern corpus of
email bodies usable for rendering tests and rule evolution?
**Answer:** No ready-made corpus exists; the gap is real. The viable path
combines lore.kernel.org mbox retrieval, a dedicated capture mailbox, and
permissively licensed synthetic template sets. Web-verified 2026-07-19 by
a research agent; re-verify licensing before committing any specimen.

## 1. Classic academic spam/ham corpora — confirmed stale, useful only as structural fallback tests

**Enron Email Dataset**
- URL: https://www.cs.cmu.edu/~enron/ (canonical CMU CALO mirror), also on Internet Archive (https://archive.org/details/2011_04_02_enron_email_dataset)
- Classes: internal corporate correspondence only — no marketing/newsletter/transactional/calendar/CI mail
- Format: maildir (plain-text per-message files), ~0.5M messages, 150 mailboxes, tarball ~422MB (enron_mail_20150507.tar.gz)
- Vintage: 1999–2002 messages, released 2015 (last official refresh)
- License: released for non-commercial research; no formal open-source license grant, but broadly reused for 20+ years without pushback
- Fitness: **poor** — pre-HTML-email-era corporate plaintext, not representative of modern HTML rendering challenges. Confirmed still the canonical download; a July 2026 legal-tech blog post ("Still on Dial-Up: Why It's Time to Retire the Enron Email Corpus," craigball.net) argues it should be retired for exactly this staleness reason.

**SpamAssassin Public Corpus**
- URL: https://spamassassin.apache.org/old/publiccorpus/ (official), mirrors on Kaggle (kaggle.com/datasets/beatoa/spamassassin-public-corpus, kaggle.com/datasets/bayes2003/...), Hugging Face (huggingface.co/datasets/talby/spamassassin), GitHub (stdlib-js/datasets-spam-assassin)
- Classes: spam and "ham," some HTML present but era-appropriate (table-layout, no responsive/dark-mode CSS)
- Format: individual bzip2'd files per message, named by number+MD5; **confirmed via readme fetch**: 6,047 messages total, ~31% spam, split into easy_ham/hard_ham/easy_ham_2/spam/spam_2 buckets
- Vintage: **confirmed** — assembled October 2002 through January 2006, explicitly dated snapshots
- License: copyright stays with original senders; Apache offers it "as a service to spam filter developers," explicit instruction "do NOT send these emails into a live email system." Kaggle/HF mirrors don't add a clearer license (HF dataset card literally says license "unknown")
- Fitness: **poor-to-fair** — actively still downloadable and multiple mirrors alive, but 20+ years stale; email HTML/CSS conventions have changed completely (no responsive design, no dark-mode meta tags, table-hack-era markup only). Useful only for legacy-rendering regression, not modern coverage.

**TREC 2007 Public Spam Corpus (trec07p)**
- URL: https://plg.uwaterloo.ca/~gvcormac/treccorpus07/ (redirects through cormack.uwaterloo.ca; final target 404'd on this check — the corpus has moved/lapsed at least once, worth re-verifying live before depending on it)
- Classes: real 2007-era email, spam + ham, some HTML
- Format: mbox-style individual messages, 75,419 total (25,220 ham / 50,199 spam), ~255MB
- Vintage: April–July 2007
- License: requires reading and accepting an "Agreement for use" before download — historically these TREC spam-track corpora restrict use to spam-filter research and do **not** grant redistribution rights, because the ham side contains real private correspondence. Could not re-confirm exact current text (link now 404s); treat as **not clearable for a public repo without direct outreach to the maintainer**.
- Fitness: **poor for redistribution**, fair-only for vintage (still pre-responsive-HTML era at that).

**CEAS 2008 Live Spam Challenge Corpus**
- URL: https://plg.uwaterloo.ca/~gvcormac/ceascorpus/ (same maintainer/domain family as TREC07, same access pattern)
- Classes: spam + ham, hand-labeled
- Format: similar to TREC, 137,705 messages (27,126 ham / 110,579 spam)
- Vintage: 2008
- License: same agreement-gated access pattern as TREC — **not confirmed redistributable**
- Fitness: **poor** — same staleness and licensing caveats as TREC07, one year newer at best.

## 2. Continuously-updated spam archives

**untroubled.org (Bruce Guenter's spam archive)**
- URL: http://untroubled.org/spam/, mirrored on Internet Archive (archive.org/details/untroubled_spam_archive)
- Classes: raw spam, historically includes modern HTML marketing-style spam
- Format: per-day archives, raw message format
- Vintage: **confirmed stale for its main feed** — the site itself states the primary bruce-guenter.dyndns.org source domain expired in April 2018, and "the archive may not be of much use going forward." The historical archive back to 1998 remains browsable but new capture volume/quality is unclear post-2018.
- License: stated purpose is "researching behavior of spammers and development of new spam management techniques" — permissive intent but no formal license text found
- Fitness: **poor-to-unclear** — worth a direct live check of current update cadence before relying on it (I could not confirm 2025/2026 activity), but even if alive, spam ≠ well-formed legitimate marketing/newsletter mail, so its rendering-fidelity value is limited regardless.

## 3. Modern marketing/newsletter galleries — good for eyeballing, bad for redistribution

**Really Good Emails**
- URL: https://reallygoodemails.com/
- Classes: marketing, promotional, newsletter (Substack/Mailchimp/beehiiv-era and beyond)
- Format: web gallery of real captured sends
- Vintage: continuously updated, current as of 2026
- License: **confirmed via fetch** — no visible "view source"/export feature on the category page checked, and the footer states "©2026 BEE Content Design, Inc. All rights reserved," with linked Terms of Service and a DMCA takedown page. No redistribution grant found.
- Fitness: **poor for a committable corpus** — good for visual reference/inspiration only, not a legally clean source of raw HTML to check into an open-source repo.

**Milled.com**
- URL: https://milled.com/
- Classes: e-commerce/marketing newsletters, claims 46M+ emails from 100K+ brands
- Format: web archive/search engine, no documented raw-HTML export or public API found
- License: unclear, no API/export terms surfaced
- Fitness: **poor** — same problem as Really Good Emails, worse (no confirmed way to even get raw source out).

**MJML sample template sets (mjmlio/email-templates, Mailteorite/mjml-email-templates, franklindesign/MJML-templates)**
- URL: https://github.com/mjmlio/email-templates, https://github.com/Mailteorite/mjml-email-templates, https://mjml.io/templates
- Classes: newsletter/marketing/transactional layout patterns
- Format: `.mjml` source (compiles to HTML, not itself raw production HTML) plus some rendered HTML
- Vintage: actively maintained
- License: Mailteorite's set explicitly MIT; mjmlio's official set is on GitHub (typical MIT-style OSS license, not independently re-verified line-by-line here)
- Fitness: **fair, but caveat** — these are hand-authored demonstration templates, not real captured production mail. Useful as clean, permissively-licensed synthetic specimens of "how a well-formed responsive newsletter should render," not as "messy real-world HTML" regression fodder — the stated goal (messy HTML → markdown) is exactly what these clean templates under-represent.

**caniemail test cases / caniemail/testing-templates**
- URL: https://github.com/hteumeuleu/caniemail (main data/site repo, has a LICENSE file — type not independently confirmed in this pass), https://github.com/caniemail/testing-templates (companion template repo)
- Classes: HTML/CSS email-client-support edge cases (not marketing/newsletter content per se — feature-isolation tests)
- Format: individual `.html` test files, ~10+ files, "HTML 100%" per language stats
- Vintage: actively maintained (caniemail.com is a live, current reference)
- License: LICENSE file present on hteumeuleu/caniemail but exact terms not independently confirmed; recommend a direct license-file read before use
- Fitness: **fair as a supplement** — excellent for isolated CSS/HTML-feature edge cases (e.g., "does this client support `background-image` in emails"), not for realistic full-message regression specimens. Content is synthetic single-feature tests, not real captured mail.

**Litmus community resources / email framework sample sets (Cerberus, Foundation for Emails/Zurb Ink, mailgun/transactional-email-templates, mailpace/templates, sendgrid/email-templates)**
- URLs: https://github.com/tedgoas/Cerberus (also mirrored under emailmonday/Cerberus, coryasilva/Cerberus), https://github.com/mailgun/transactional-email-templates, https://github.com/mailpace/templates, https://github.com/sendgrid/email-templates
- Classes: transactional email layout patterns (receipts, order confirmations, welcome emails) plus general responsive-email boilerplate
- Format: raw `.html`, small counts (a handful to a few dozen files per repo)
- Vintage: Cerberus and Zurb Ink are older but still referenced; mailgun/sendgrid/mailpace sets vary in freshness
- License: generally MIT or similarly permissive (Cerberus and most of these boilerplate repos are open-source by design)
- Fitness: **fair for transactional class specifically** — these are the closest thing to open, redistributable transactional-email HTML available, but they are hand-authored *templates*, not real captured receipts from real vendors (Amazon, Stripe, etc.), so they won't exercise the same messy real-world markup a genuine vendor receipt would.

## 4. Public mailing-list archives with raw mbox access

**lore.kernel.org / public-inbox (Linux kernel and related lists)**
- URL: https://lore.kernel.org/, mirroring docs at https://korg.docs.kernel.org/lore.html, mirror instructions e.g. https://lore.kernel.org/linux-arm-kernel/_/text/mirror/
- Classes: mailing-list/patch mail — **excellent fit**, this is real developer-workflow email at volume
- Format: **confirmed** git-backed public-inbox archives; per-thread mbox export via URL+curl, full git-clonable epoch repos (each ~1GB), Atom feeds, NNTP/IMAP read access. Tools like `b4` and `get-lore-mbox` script bulk retrieval.
- Vintage: continuously updated, decades of kernel-list history, live today
- License: mailing-list postings are generally public domain/public archive by convention (GPL project mailing lists); individual message copyright technically remains with authors but redistribution of public list traffic is universally accepted practice in this ecosystem
- Fitness: **excellent** — best-in-class source for the mailing-list/patch-mail class specifically: live, huge volume, scriptable raw-mbox retrieval, effectively unlimited redistribution precedent.

**Debian mailing lists**
- URL: https://lists.debian.org/, mbox retrieval pattern **confirmed**: `https://lists.debian.org/cgi-bin/mbox/<list>-YYYYMM`
- Classes: mailing-list/patch mail, bug-tracking-flavored technical correspondence
- Format: one-mbox-per-list-per-month, downloadable via the cgi-bin URL pattern above
- Vintage: continuously updated, decades of history
- License: public archive, same open-list-traffic convention as kernel.org
- Fitness: **good** — confirmed working download mechanism, good secondary source for mailing-list class diversity (different quoting/threading conventions than kernel lists).

**W3C mailing lists (lists.w3.org)**
- URL: https://lists.w3.org/
- Classes: mailing-list/patch mail (web-standards discussion, less code-patch-heavy than kernel/Debian)
- Format: browsable HTML archive only
- Vintage: continuously updated, huge historical depth
- License: **confirmed negative finding** — W3C staff have explicitly stated mbox source files are not intended to be shared publicly and asked people not to redistribute them; only the rendered HTML archive is meant for public access
- Fitness: **poor for raw-mbox harvesting** — usable only via scraping the HTML archive pages (fragile, and arguably against stated intent), not a clean raw-format source.

**marc.info (MARC — cross-project mailing list archive)**
- URL: https://marc.info/
- Classes: mailing-list/patch mail, aggregates 2,400+ lists, ~320K new messages/month
- Format: raw/partially-raw mail accessible per-message via `?i=<message-id>` URLs
- Vintage: continuously updated, very high volume
- License/mechanics: no documented bulk-download API found in this pass; per-message raw access exists but a scripted bulk-mbox mechanism wasn't confirmed
- Fitness: **fair** — good breadth (covers lists lore.kernel.org/Debian don't), but bulk-retrieval mechanics are murkier than the git-based public-inbox model; lower priority than lore.kernel.org.

## 5. Purpose-built rendering-test corpora on GitHub/HuggingFace/Kaggle

- **jamesmacwhite/Email-Client-Testing** (https://github.com/jamesmacwhite/Email-Client-Testing) — hand-authored HTML/CSS capability-probe templates (acid-test style), not real mail; useful only as a synthetic edge-case supplement.
- **Hugging Face marketing-email datasets** (marketeam/Marketing-Emails, emailmarketingdataset/open-email-marketing-dataset, dvilasuero/marketing, FourthBrainGenAI/MarketMail-AI) — checked several; these are explicitly **synthetically generated** text/JSONL datasets built for NLP fine-tuning, not raw captured HTML email bodies. **Not usable** for this task's stated purpose (rendering messy real HTML), only for text-content modeling.
- No purpose-built "HTML email rendering regression corpus" analogous to what's being sought (real, modern, license-clean, spanning all six classes) was found to already exist as a maintained GitHub project. This appears to be a genuine gap — a poplar-specific corpus repo would be filling real white space, not duplicating prior art.

## 6. Calendar invites and transactional receipts — confirmed hardest, largely absent publicly

- No public corpus of real modern **calendar invites** (`.ics`/`text/calendar` MIME parts as actually generated by Google Calendar, Outlook, Fastmail/JMAP, etc.) was found. GitHub search surfaced only hand-authored `.ics` samples (gists, library test fixtures like `icsinviter`'s `testdata/arbitrary.ics`) — synthetic, not real client output, and not wrapped in a real multipart/mixed email envelope with REQUEST/REPLY method headers as real invite mail actually looks.
- No public corpus of real **transactional receipts/order confirmations** (Stripe, Amazon, Airbnb-style real vendor mail) was found; only hand-authored template repos (section 3 above), which are clean and don't exercise real-world quirks (vendor-specific tracking pixels, proprietary CSS resets, broken markup from ESP-of-the-month).
- **Conclusion for this class: self-generation is the realistic path.** Sending real invites from Google Calendar/Outlook/Fastmail to a harvest account, and signing up for real receipts/notifications from a spread of real vendors (Stripe test mode, Amazon, GitHub Actions/CI notifications, Airbnb, etc.) to a dedicated capture mailbox, is the only way to get real, modern, structurally-correct specimens for these two classes. GitHub/CI notification mail (Actions run summaries, PR review requests, Dependabot alerts) falls in the same bucket — no third-party archive of real GitHub notification `.eml` files was found; harvesting from a real GitHub account against a test repo is the practical route.

---

## Ranked shortlist (best 3–5 sources to actually build the corpus from)

1. **lore.kernel.org / public-inbox** — for mailing-list/patch mail. Confirmed live, huge volume, scriptable raw-mbox retrieval, essentially unrestricted redistribution norm for this ecosystem. Highest confidence of the whole survey.
2. **Self-harvested modern mail via a dedicated capture mailbox** (sign up for real Substack/beehiiv newsletters, real e-commerce marketing lists, real Stripe/Amazon/service receipts, real Google Calendar/Outlook invites sent to the account, real GitHub Actions/PR notifications from a scratch repo) — the only realistic source for transactional, calendar, and GitHub/CI classes, and arguably the *best* source even for marketing/newsletter given the Really Good Emails/Milled licensing dead-ends. Geoff's own `geoff@907.life` Fastmail account (already wired for poplar testing per the JMAP memory) is a natural capture point.
3. **Debian mailing lists (`cgi-bin/mbox` export)** — secondary mailing-list source, different threading/quoting conventions from kernel.org, confirmed working download mechanism, good redistribution norm.
4. **caniemail/testing-templates + MJML official/Mailteorite template sets** — permissively licensed, actively maintained synthetic specimens for isolated CSS-feature edge cases and clean well-formed newsletter/transactional structure; supplements but doesn't replace real-world messiness.
5. **SpamAssassin public corpus** — lowest tier, kept only for legacy/structural regression (table-layout-era HTML), given confirmed 2002–2006 vintage and murky HF/Kaggle mirror licensing; do not treat as representative of modern email.

## Coverage gaps requiring self-generation or scrubbed private mail

- **Calendar invites**: no viable public corpus of real client-generated `.ics`-in-email found. Self-generation only.
- **Transactional receipts/notifications**: no viable public corpus of real vendor receipts found; template repos are clean synthetic stand-ins, not messy real-world specimens. Self-generation (real signups) or scrubbed personal mail (with PII stripped) are the only paths.
- **GitHub/CI notification mail**: no third-party archive found; self-generation against a scratch GitHub repo (Actions runs, PR events, Dependabot) is the practical route.
- **Modern marketing/newsletter raw HTML with clean redistribution rights**: Really Good Emails and Milled both confirmed to lack an export path or a redistribution grant; self-subscribing a harvest mailbox to real newsletters (Substack, beehiiv, Mailchimp-sent lists) is likely necessary, with attention to each sender's own terms before committing specimens to an open-source repo (a defensible position is "small excerpted/redacted specimens for interoperability testing," but this needs its own judgment call per sender, not a blanket assumption).
- **TREC07/CEAS licensing**: flagged as likely non-redistributable (agreement-gated, contains real private ham) — do not commit raw messages from these without directly confirming current terms with the maintainer; the download page itself 404'd during this check, so even access is unverified as of today.