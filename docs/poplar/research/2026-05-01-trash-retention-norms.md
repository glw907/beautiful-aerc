# Trash retention & manual empty: reference-app norms

**Date:** 2026-05-01
**Pass:** 6.7
**Scope.** How comparable mail clients handle Trash auto-purge,
manual empty, permanent-delete bypass, and confirmation copy.
Compares to ADR-0092 (Destroy primitive), ADR-0093 (per-session
sweep), ADR-0094 (ConfirmModal + manual empty).

## Verification preamble (Task 1)

Tmux verification of Pass 6.6 against the mock backend at 120×40.
Captures live in
`docs/poplar/research/captures/2026-05-01-retention/`.

| Case | Result | Capture |
|---|---|---|
| `T` jumps to Trash | pass | `02-T-trash.txt` |
| `E` opens confirm modal | **fail then pass** | `03-E-confirm.txt` (bug) → `03-E-confirm-fixed.txt` |
| `n` dismisses modal | pass | `04-n-dismiss.txt` |
| `E` then `y` empties + toast (no `[u undo]`) | pass | `05-E-y-empty.txt` |
| `E` on Inbox is inert | pass | `06-E-inbox-inert.txt` |
| `trash_retention_days = 0`: no sweep | pass (implicit; first run) | n/a |
| `trash_retention_days = 10`: sweep destroys all 14 mock msgs | pass | `07-sweep-on.txt` |
| Help popover advertises `E empty` | pass | `08-help-E.txt` |

**Bug found and fixed:** `ConfirmModal.Box` rendered the top
border one cell wider than the body rows because
`rest = boxW - 2 - lipgloss.Width(title)` produced a top-row width
of `boxW + 1` rather than `boxW`. Fixed by changing the constant
to 3 (`internal/ui/confirm_modal.go:125`). Existing
`TestConfirmModal_ViewWidthContract` only asserted `width ≤ 80`,
so it passed despite the drift; strengthened to assert
all-rows-equal-width.

No other failures. Sweep runs invisibly (silent on success per
ADR-0093) and is correctly bounded: with `retention_days = 10`
against mock fixtures whose `SentAt` values are 18–26 days old,
all 14 messages are destroyed in a single sweep on first Trash
visit.

## Per-app survey

### mutt / NeoMutt

`$trash` variable points to a folder. Default `delete-message`
(`d`) marks for deletion, `purge-message` (unbound by default,
typically `bind index D purge-message`) bypasses Trash and
permanently deletes. `$close_purge` controls whether expunge
happens on folder close. **No native retention.** The standard
recipe is a cron job invoking IMAP commands or `notmuch` to age
out the Trash mailbox — there is no per-folder "delete after N
days" knob.

Sources: [NeoMutt trash feature](https://neomutt.org/feature/trash), [Auto-purge issue #217](https://github.com/neomutt/neomutt/issues/217).

### aerc

`:delete` on a message *moves* it to a folder with the `trash`
role (JMAP) or to the `trash` folder configured in
`accounts.conf` (IMAP). If the current mailbox is itself the
trash mailbox, `:delete` destroys server-side. There is no
`:empty-trash`, no retention config, and no `:purge`. Issue
~sircmpwn/aerc2#332 (configurable :delete folder) and the
"introduce the 'trash' command" thread on aerc-discuss show the
project has actively *resisted* a separate destructive primitive,
preferring the role-based move-or-destroy semantics.

Sources: [aerc-jmap(5)](https://man.archlinux.org/man/aerc-jmap.5.en), [aerc2#332](https://todo.sr.ht/~sircmpwn/aerc2/332), [aerc-discuss "trash functionality"](https://lists.sr.ht/~rjarry/aerc-discuss/%3CCZUGSXI26EJR.24CD5T5QFZ68Y@disroot.org%3E).

### himalaya

`himalaya message delete <id>` flags + expunges (via the underlying
library's `delete` semantics; configurable per backend). No
explicit `:empty` command — the documented pattern is shell
composition: `himalaya message delete $(himalaya envelope list -o
json --folder Trash | jq -r .[].id)`. **No native retention.**
Per the project's stateless-CLI design, retention is left to
external automation (cron + jq + himalaya).

Sources: [himalaya GitHub](https://github.com/pimalaya/himalaya), [issue #585 copy/move/delete by query](https://github.com/pimalaya/himalaya/issues/585).

### meli

`delete-message` follows IMAP's two-step model: marks `\Deleted`,
then `expunge` (or folder-close expunge if configured) actually
removes. No retention knob, no manual-empty primitive. The trash
workflow is the same as mutt: configure a trash folder and
`MOVE` there, then expunge that folder out of band.

Sources: [Cyrus IMAP "When is What… Deleted, Expired, Expunged or Purged?"](https://www.cyrusimap.org/3.4/imap/reference/faqs/o-deleted-expired-expunged-purged.html) (general two-step semantics — meli's own docs do not surface this).

### Thunderbird (representative GUI)

Three orthogonal settings:

1. **Account-level "Empty Trash on Exit"** — Account Settings →
   Server Settings → "Empty Trash on Exit" checkbox. Triggers a
   permanent-delete sweep on application quit.
2. **Per-folder Retention Policy** — right-click folder →
   Properties → Retention Policy → "Delete messages more than N
   days old". Per-folder, opt-in, days-based. Available on Trash
   like any other folder.
3. **Junk auto-delete** — Account Settings → Junk Settings →
   "Automatically delete junk mail older than N days". Only
   applies to the Junk folder; opt-in.

Confirmation: Thunderbird's "Empty Trash" menu action prompts
once with a yes/no dialog per session.

Sources: [Mozilla Support "Delete trash"](https://support.mozilla.org/en-US/questions/1312686), [Mozilla Support "Why is trash emptied on exit?"](https://support.mozilla.org/en-US/questions/1522573), [Mozilla Support "Functionality 'Delete messages older than X days' doesn't work"](https://support.mozilla.org/en-US/questions/1488672).

### Apple Mail (second GUI data point)

Mail → Settings → Accounts → \<account\> → Mailbox Behaviors:

- **"Erase deleted messages"** for the Trash mailbox: Never / After
  one day / After one week / After one month / **When quitting
  Mail** (default on iCloud).
- **"Erase junk messages"** for the Junk mailbox: same set.

Two retention modes coexist: time-based (1d / 1w / 1m) and
quit-triggered. Per-account; not per-folder.

Sources: [Apple Support "Change Mailbox Behaviors settings in Mail on Mac"](https://support.apple.com/en-ca/guide/mail/cpmlprefacctmbox/mac), [GreenNet "Automatically purge or expunge deleted messages in Apple Mail"](https://www.greennet.org.uk/support/automatically-purge-or-expunge-deleted-messages-apple-mail).

## Pattern synthesis

Two distinct camps:

| Camp | Retention | Manual empty | Permanent-delete bypass |
|---|---|---|---|
| TUI (mutt, aerc, himalaya, meli) | **none — out of band** (cron + script) | none — shell composition | yes (`purge-message`, role-based destroy) |
| GUI (Thunderbird, Apple Mail) | **opt-in, days-based, per-account or per-folder** | yes — menu action with confirm dialog | yes — Shift+Delete or "Delete" from Trash |

The TUI camp consistently externalizes retention to cron / scripts
because TUI clients are not always running. The GUI camp embeds
retention into the client because the client is (typically) the
long-running mail process. **Apple Mail's "When quitting Mail"
trigger is the closest analogue to poplar's "per-session" model.**

Confirmation copy is universally minimal in the GUI camp: a
single yes/no prompt with a count. None of the surveyed clients
require typed-out confirmation (`type EMPTY to confirm`) — that
pattern shows up in destructive ops on shared infrastructure
(GitHub repo deletion, AWS resource purges), not in mail clients.

## Comparison to ADR-0092 / 0093 / 0094

### ADR-0092 — `Backend.Destroy` permanent-delete primitive

**Pattern match.** Every surveyed client exposes a
permanent-delete bypass distinct from the move-to-trash default:
mutt's `purge-message`, aerc's role-based destroy when already in
the trash mailbox, himalaya's `delete` in the trash folder
context, GUI clients' Shift+Delete. Poplar's `Destroy` is the
direct analogue. **Ratify.**

### ADR-0093 — per-session retention sweep

**Pattern match — but uniquely positioned.** TUI peers don't
embed retention; GUI peers do but key it on quit (Apple) or
exit (Thunderbird). Poplar keys it on **first folder visit per
session**, which is structurally midway: the user has expressed
intent to look at Trash (analogous to "they care about it
enough to navigate there"), so the sweep cost attaches to a
load they already initiated. This is a defensible novelty — the
quit-triggered model wastes the sweep when the user never opens
Trash, and a timer-driven sweep contradicts poplar's
"event-driven, no background goroutines" stance.

The "per-session, no on-disk ledger" rule also matches
GUI norms — Thunderbird and Apple Mail both reset their
retention bookkeeping on each launch. **Ratify.**

### ADR-0094 — ConfirmModal + manual empty

**Pattern match.** Every GUI client surveyed has a manual
"Empty Trash" with a yes/no confirm. TUI clients do not, but
poplar's design goal is "better Pine," which lands closer to a
GUI-shaped UX than mutt-shaped UX. The confirm copy
(`<N> messages will be permanently deleted.` + `[y] yes / [n]
no / [esc] cancel`) is consistent with Thunderbird's prompt.
**Ratify.**

## Open questions — answers

### Q1: Retention default — 0 (opt-in) vs 30 (opt-out)?

**Ratify default 0 (opt-in).** Apple Mail defaults vary by
account type but the *generic* IMAP default is "Never" (= 0).
Thunderbird's per-folder retention is unset by default. Both GUI
clients err on the side of *no* automatic destruction unless the
user opts in, because an automatic destructive default in a mail
client risks violating users' implicit retention assumptions
(legal, compliance, "I'll get to it later"). Poplar's 0-default
is the right call.

The one non-trivial counterargument: Gmail's Spam folder
auto-purges at 30 days *server-side*, which means Spam is
already a shorter-lived bucket on most accounts. Poplar's
matching `spam_retention_days` knob lets users tighten that
floor below the provider's default if they want to, but the
default-off remains correct because the server is already
applying its own.

### Q2: Sweep trigger — first-visit / every visit / timer / on-quit?

**Ratify first-visit-per-session.** Of the four candidates:

- *Every visit* — wasteful (sweep repeatedly does nothing after
  the first run within a session).
- *Timer-driven* — contradicts the "no background goroutines"
  stance and adds a class of bug (sweep firing while a
  ConfirmModal is open, or mid-search).
- *On-quit* — matches Apple Mail's most common setting, but
  poplar uses `tea.Quit` which terminates the program; a
  blocking destroyCmd at quit-time has nowhere clean to surface
  errors.
- *First-visit-per-session* — sweep cost attaches to a load the
  user initiated; errors surface in the existing error banner;
  no background work; idempotent within a session.

The current design is the best fit. **Ratify.**

### Q3: "Type EMPTY" affordance for very large folders?

**Ratify simple y/n confirm; no typed confirmation.** None of
the surveyed mail clients require typed confirmation — the
typed-confirm pattern is reserved for destructive ops on shared
infrastructure where the cost of a misclick is catastrophic and
non-recoverable. For an individual user emptying their own
Trash, the existing y/n with a count (`14 messages will be
permanently deleted.`) is sufficient. The ADR-0094 design is
already aligned with how every GUI client surveyed handles this.

If a future user reports having emptied 50,000 messages by
accident, revisit. Until then, the simpler design wins.

## Recommendation

**No revisions needed. Ratify ADR-0092/0093/0094 as-is.** The
verification surfaced one rendering bug (ConfirmModal title row
width) which was fixed in this pass and back-stopped by a
strengthened test. The research confirms the design is well
positioned: it borrows the GUI clients' retention/empty surface
and the TUI clients' permanent-delete primitive, and the
"first-visit-per-session" sweep trigger is a defensible novelty
rather than a deviation that needs walking back.

No Pass 6.9 starter prompt is generated. The next pass remains
**Pass 7 — popover narrow-terminal polish + small render drift
cleanup** as already listed in STATUS.
