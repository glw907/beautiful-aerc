# Pass 2 gallery freeze stills (F1, F2)

**Status:** Committed for the design record (survey amendment F,
task 12). These four renders are what
`docs/poplar/design/2026-08-19-shell-exemplar/README.md` names as the
gallery's eventual replacement for the exemplar's interim role, now
that pass 2's theme and render seam exist.

## Files

- `f1-mail-80x24-{dark,light}.ansi` / `.txt`, wireframe F1 (shell
  frame, mail placeholder, spartan 80×24): the committed gallery
  files `internal/ui/testdata/gallery/mail-80x24-truecolor-{dark,light}.txt`,
  unescaped back to raw bytes. Byte-identical to what `make gallery`
  itself checks; a still, not a second source of truth.
- `f2-mail-toast-100x30-{dark,light}.ansi` / `.txt`, wireframe F2
  (status line with an undo toast): the `MailToast` fixture rendered
  through `ui.Render` directly at both themes, at 100×30 (the
  fixture's gallery size, not the wireframe mock's 120-column
  sketch). The gallery itself narrows `MailToast` to truecolor-dark
  and NO_COLOR only (`gallery_test.go`'s `chromeStateProfiles`: the
  content pane beneath a chrome-state fixture never varies, so a
  third and fourth profile add no distinct rendering path); the light
  variant here exists for this design record alone and is not part of
  the gated matrix.

`.ansi` is the captured color output; `cat` it in a truecolor
terminal. `.txt` is ANSI-stripped geometry, diffable.

## Regenerating

These are stills, not a `make` target: a one-off render through
`ui.Render` and `internal/ui/fixtures`, the same seam the gallery
itself calls. F1's bytes stay pinned to whatever `make gallery`
commits; F2's light variant would need re-running by hand if
`MailToast`'s fixture state ever changes.
