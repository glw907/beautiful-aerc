# Pass 8.11 — Voice cleanup III (comment prose-rhythm)

## Goal

Eliminate the prose-rhythm AI tells (T33 em-dash, T34 semicolon
clause-joiner, T35 doc labels) the 8.8/8.9 string audit missed.
Then activate the deferred grep checks in `scripts/voice-check.sh`
so `make check` becomes the regression gate.

## Scope

Comments only across `cmd/` and `internal/`. No semantic code
changes — except for one in-passing fix: `%v` → `%w` on the dozen
`fmt.Errorf` lines in `internal/mailjmap/jmap.go` where the cache
drainer needs `errors.Is(err, mail.ErrAuth)` /
`mail.ErrNotFound` to route correctly.

Pre-existing audit:
- T33 (em-dash): 211 hits across ~80 files.
- T34 (semicolon clause-joiner): 266 hits across ~95 files.
- T35 (doc label): 2 hits — `internal/content/headers.go:65`
  (`Fallback:`), `internal/mail/classify.go:36` (`Priority:`).

T36 (long parens) and T37 (multi-clause rhythm) are not
grep-detectable. The `/simplify` voice lens (Agent 4) catches
those during the pass-end ritual.

## Approach

Mechanical first. Per package, walk the grep hits and rewrite
each comment:

- `// X — Y` → `// X. Y.` or one shorter line.
- `// X; Y` → same. Inside lists, `; ` is fine — only flag
  clause-joiners.
- `// Label: Y` → drop the label, write prose.

Acceptable em dashes after the sweep: short comma-like asides
("Stub — Pass 9.6 will implement"). Acceptable semicolons:
inside lists or short parentheticals.

After per-package sweeps, run a tree-wide `grep` to confirm zero
hits. Then activate the three deferred `scan` calls in
`scripts/voice-check.sh` and run `make check`.

`/simplify` (pass-end step 1) sweeps T36/T37 across the diff.

## File-by-file checklist

Grouped by package (lowest-touched first to build momentum).

### Trivial: T35 (2 hits)

- [ ] `internal/content/headers.go:65` — `Fallback:` label
- [ ] `internal/mail/classify.go:36` — `Priority:` label

### internal/term (T33, T34)

- [ ] `internal/term/cpr.go`
- [ ] `internal/term/font.go`
- [ ] `internal/term/font_test.go`
- [ ] `internal/term/probe.go`
- [ ] `internal/term/probe_test.go`
- [ ] `internal/term/resolve.go`

### internal/theme (T34)

- [ ] `internal/theme/palette.go`
- [ ] `internal/theme/theme_test.go`

### internal/mailauth (T33, T34)

- [ ] `internal/mailauth/xoauth2.go`

### internal/mail (T33, T34)

- [ ] `internal/mail/backend.go`
- [ ] `internal/mail/changes.go`
- [ ] `internal/mail/classify.go` (also T35)
- [ ] `internal/mail/classify_test.go`
- [ ] `internal/mail/mock.go`
- [ ] `internal/mail/types.go`

### internal/mailimap (T33, T34)

- [ ] `internal/mailimap/actions.go`
- [ ] `internal/mailimap/attachments.go`
- [ ] `internal/mailimap/auth.go`
- [ ] `internal/mailimap/changes.go`
- [ ] `internal/mailimap/client.go`
- [ ] `internal/mailimap/fake_test.go`
- [ ] `internal/mailimap/folders.go`
- [ ] `internal/mailimap/folders_test.go`
- [ ] `internal/mailimap/idle.go`
- [ ] `internal/mailimap/idle_test.go`
- [ ] `internal/mailimap/imap.go`
- [ ] `internal/mailimap/messages.go`
- [ ] `internal/mailimap/messages_test.go`
- [ ] `internal/mailimap/realclient.go`

### internal/mailjmap (T33, T34, plus %v→%w)

- [ ] `internal/mailjmap/attachments.go`
- [ ] `internal/mailjmap/attachments_test.go`
- [ ] `internal/mailjmap/changes.go`
- [ ] `internal/mailjmap/fake_test.go`
- [ ] `internal/mailjmap/jmap.go` (also `%v` → `%w` on 12 lines)
- [ ] `internal/mailjmap/jmap_test.go`
- [ ] `internal/mailjmap/push.go`

### internal/config (T33, T34)

- [ ] `internal/config/accounts.go`
- [ ] `internal/config/cache.go`
- [ ] `internal/config/loader.go`
- [ ] `internal/config/ui.go`
- [ ] `internal/config/writer.go`

### internal/content (T33, T34, plus T35)

- [ ] `internal/content/blocks.go`
- [ ] `internal/content/headers.go` (also T35)
- [ ] `internal/content/parse.go`
- [ ] `internal/content/parse_test.go`
- [ ] `internal/content/render.go`
- [ ] `internal/content/render_footnote.go`
- [ ] `internal/content/render_footnote_test.go`
- [ ] `internal/content/render_test.go`
- [ ] `internal/content/url_trim.go`

### internal/filter (T33, T34)

- [ ] `internal/filter/convert.go`
- [ ] `internal/filter/html.go`
- [ ] `internal/filter/html_test.go`

### internal/tidy (T33)

- [ ] `internal/tidy/prompt.go`
- [ ] `internal/tidy/tidy.go`

### internal/cache (T33, T34)

- [ ] `internal/cache/account.go`
- [ ] `internal/cache/attachments.go`
- [ ] `internal/cache/attachments_test.go`
- [ ] `internal/cache/bodies.go`
- [ ] `internal/cache/bodies_test.go`
- [ ] `internal/cache/cache_test.go`
- [ ] `internal/cache/drainer.go`
- [ ] `internal/cache/integration_test.go`
- [ ] `internal/cache/ops.go`
- [ ] `internal/cache/outbox_reads.go`
- [ ] `internal/cache/reads.go`
- [ ] `internal/cache/schema.go`
- [ ] `internal/cache/syncer.go`

### internal/catkin (T33, T34)

- [ ] `internal/catkin/autopair.go`
- [ ] `internal/catkin/blocks.go`
- [ ] `internal/catkin/buffer.go`
- [ ] `internal/catkin/catkin.go`
- [ ] `internal/catkin/find.go`
- [ ] `internal/catkin/indent.go`
- [ ] `internal/catkin/match.go`
- [ ] `internal/catkin/paste.go`
- [ ] `internal/catkin/reflow.go`
- [ ] `internal/catkin/reflow_test.go`
- [ ] `internal/catkin/render.go`
- [ ] `internal/catkin/render_test.go`
- [ ] `internal/catkin/scrolloff.go`
- [ ] `internal/catkin/style.go`
- [ ] `internal/catkin/undo.go`
- [ ] `internal/catkin/wordnav.go`

### internal/ui (T33, T34)

- [ ] `internal/ui/account_tab.go`
- [ ] `internal/ui/account_tab_test.go`
- [ ] `internal/ui/app.go`
- [ ] `internal/ui/app_test.go`
- [ ] `internal/ui/attachpicker.go`
- [ ] `internal/ui/cmds.go`
- [ ] `internal/ui/confirm_modal.go`
- [ ] `internal/ui/conflict_overlay.go`
- [ ] `internal/ui/date_format.go`
- [ ] `internal/ui/error_banner.go`
- [ ] `internal/ui/footer.go`
- [ ] `internal/ui/footer_test.go`
- [ ] `internal/ui/golden_test.go`
- [ ] `internal/ui/help_popover.go`
- [ ] `internal/ui/help_popover_test.go`
- [ ] `internal/ui/icons.go`
- [ ] `internal/ui/icons_test.go`
- [ ] `internal/ui/iconwidth.go`
- [ ] `internal/ui/layout.go`
- [ ] `internal/ui/layout_test.go`
- [ ] `internal/ui/linkpicker.go`
- [ ] `internal/ui/modal_shell.go`
- [ ] `internal/ui/movepicker.go`
- [ ] `internal/ui/movepicker_test.go`
- [ ] `internal/ui/msglist.go`
- [ ] `internal/ui/msglist_test.go`
- [ ] `internal/ui/outbox_overlay.go`
- [ ] `internal/ui/overlay.go`
- [ ] `internal/ui/overlay_test.go`
- [ ] `internal/ui/sidebar.go`
- [ ] `internal/ui/sidebar_column.go`
- [ ] `internal/ui/sidebar_column_test.go`
- [ ] `internal/ui/sidebar_search.go`
- [ ] `internal/ui/sidebar_search_test.go`
- [ ] `internal/ui/status_bar.go`
- [ ] `internal/ui/styles.go`
- [ ] `internal/ui/toast.go`
- [ ] `internal/ui/viewer.go`
- [ ] `internal/ui/viewer_test.go`

### cmd/poplar (T33)

- [ ] `cmd/poplar/cache.go`

## After mechanical sweep

1. Confirm zero hits for T33, T34, T35 across the tree.
2. Activate the three deferred `scan` calls in
   `scripts/voice-check.sh`.
3. `%v` → `%w` sweep on the 12 `fmt.Errorf` lines in
   `internal/mailjmap/jmap.go`.
4. `make check` — must be green (this is also the new regression
   gate).
5. `/simplify` — Agent 4 voice lens catches T36/T37 across the
   diff.
6. ADR-0148 — supersedes ADR-0142, locks the grep gate, adds
   T33–T37 to the scope.
7. Update invariants.md (Build & verification block — name the
   T33/T34/T35 scans).
8. STATUS.md update + plan/spec archival.
9. Commit + push + install.
