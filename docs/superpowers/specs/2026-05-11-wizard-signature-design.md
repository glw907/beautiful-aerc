# Wizard signature step — design

Status: draft
Date: 2026-05-11

## Goal

The first-run setup wizard collects everything needed for a working
account except a sending signature. Users currently have to hand-edit
`config.toml` after the wizard exits to add one. This design inserts a
signature step into the wizard, backed by the same catkin markdown
editor compose uses, so the wizard produces a complete account in one
pass.

Scope is one signature per account. Multi-signature setups remain a
config-file power feature.

## User flow

Stage order becomes:

```
provider → email → credentials → probe → identity → signature → label → done
```

The signature step renders:

```
Email signature — optional

Markdown is supported and will be rendered as HTML on send.
Leave blank to skip.

-- 
┃Geoff Wright
 geoff@907.life

Markdown  ^B bold · ^I italic · ^K link · ^L list · ^Q quote · ^Space task
Wizard    ^X save · Esc skip · ^P back
```

`┃` marks catkin's cursor. The `-- ` row is chrome — it is not in
catkin's buffer and the cursor cannot reach it.

The description block below the title sets two expectations the user
cannot derive from the editor itself: that markdown syntax is honored
(rather than going out verbatim), and that the wire format on send is
HTML (the multipart/alternative `text/html` part `AssembleMIME` builds
via `filter.MarkdownToHTML`). The plaintext alternative receives the
markdown source unchanged, which keeps the `-- ` boundary intact for
the plaintext reader's signature-stripping heuristics — worth knowing
later when documenting, but not a fact the wizard surfaces.

Keys:

- `Ctrl+X` — save catkin's value into `Model.Signature`, emit `AdvanceMsg`.
- `Esc` — leave `Signature` empty, emit `AdvanceMsg`.
- `Ctrl+P` — emit `BackMsg`, returns to the identity-name stage.
- Anything else — delegated to catkin (`Ctrl+B/I/K/L/Q/Space` produce
  the same markdown edits compose's body honors).

## Domain layer (`internal/wizard/`)

Add one field to `Model`:

```go
// Signature is the raw markdown body the user typed into the wizard's
// catkin editor. It does not carry the RFC 3676 "-- \n" sentinel —
// config's decoder injects that on the next load. Empty means the
// identity ships with no signatures.
Signature string
```

`Apply` change: when `m.Signature != ""`, attach one entry to the
synthesized identity.

```go
if m.IdentityName != "" || m.Signature != "" {
    id := config.Identity{Name: m.IdentityName, Email: m.Email}
    if m.Signature != "" {
        id.Signatures = []config.Signature{
            {Name: "default", Text: m.Signature},
        }
    }
    cfg.Identities = []config.Identity{id}
}
```

The existing branch that synthesized an identity only when
`IdentityName != ""` widens to cover the signature-only case.
`config.AccountConfig` requires `len(Identities) >= 1` only when
identity blocks exist in TOML; an absent block synthesizes from
top-level `From`, so the wider branch does not violate any invariant.

`FromAccount` reverse: when the first identity has signatures, strip
the leading `-- \n` from `Signatures[0].Text` and seed `m.Signature`.
The sentinel is always present on decoded signatures (per
`injectSentinel`), so the strip is unconditional.

## UI layer (`internal/ui/wizard/section_signature.go`)

New file. Modeled on `probe_screen.go` — a custom non-huh section.

```go
type signatureSection struct {
    parent *Model
    editor catkin.Model
}

func newSignatureSection(parent *Model) *signatureSection {
    ed := catkin.New()
    if parent.State.Signature != "" {
        ed.SetValue(parent.State.Signature)
    }
    return &signatureSection{parent: parent, editor: ed}
}
```

`Update` routes `Ctrl+X` / `Esc` / `Ctrl+P` to the wizard's message
types and delegates every other key to `s.editor.Update`. `View`
composes title, blank line, two-line description block, blank line,
sentinel chrome row, catkin viewport, blank line, two hint rows. The
description ("Markdown is supported and will be rendered as HTML on
send. / Leave blank to skip.") renders through `Styles.Help` so it
sits visually below the bright title. The sentinel row is a literal `"-- "` string
rendered through `Styles.Help` — a comment cites ADR-0177 so a future
reader does not "fix" the trailing space.

Sections list (`internal/ui/wizard/sections.go`) adds the signature
section between account and theme. The signature step is part of the
account section's stage machine, not a top-level section, so it
inherits the existing back/forward navigation without a new
section-level transition.

`sections.go` itself does not change; the new stage lands inside
`accountSection`. Add `stageSignature` to the `accountStage` enum
between `stageIdentity` and `stageLabel`, and extend `buildForm` to
construct the signature sub-screen (parallel to how `stageProbe`
constructs `probeScreen`). `accountSection` grows a
`signature *signatureSection` field, threaded through `Update` /
`View` / `Init` the same way `probe` and `oauthSub` are.

## Writer (`internal/config/writer.go`)

Multi-line signature bodies render today as escaped one-liners
(`text = "Geoff\nWright"`). Switch to TOML triple-quoted multi-line
literals when the body contains a newline:

```toml
[[account.identity.signature]]
name = "default"
text = """
Geoff Wright
geoff@907.life
"""
```

Implementation: new `multilineQuoted(s string) string` helper that
emits `"""\n<body>\n"""` when `strings.Contains(s, "\n")`, otherwise
falls back to `quoted(s)`. The signature `text` writer at
`writer.go:247` is the only caller for v1. Single-line bodies keep the
existing `quoted` form.

The TOML decoder handles both forms natively; no decoder change.

## Tests

`internal/wizard/apply_test.go`:

- Signature-only state (no identity name) produces one identity with
  one signature.
- Signature plus identity name produces one identity with name +
  signature.
- Empty signature produces an identity with no signatures (when name
  is set) or no identities (when both are empty).

`internal/wizard/model_test.go` (FromAccount):

- Decoded signature with sentinel round-trips into `m.Signature`
  without the sentinel.
- Identity with zero signatures leaves `m.Signature` empty.

`internal/config/writer_test.go`:

- Multi-line signature text renders as a `"""..."""` block.
- Single-line signature text keeps the basic-string form.
- Round-trip (`Render` → `Parse` → compare) preserves
  `Signature.Text` (modulo sentinel injection).

`internal/ui/wizard/section_signature_test.go`:

- Empty editor + `Esc` advances with empty `parent.State.Signature`.
- Non-empty editor + `Ctrl+X` writes catkin's value to
  `parent.State.Signature` and advances.
- `Ctrl+P` emits `BackMsg`.
- Other keys (e.g., `a`, `Ctrl+B`) reach catkin and mutate its buffer.

## Out of scope

- Multi-signature loop UI. Power users edit `config.toml`.
- File-backed signatures (`signature.file`). Wizard always emits
  inline `text`.
- Live markdown preview. Catkin's editor view is the working surface.
- Advertising catkin's markdown shortcuts in compose's footer. The
  signature step shows them because it is the user's first encounter
  with catkin; compose-footer policy is a separate decision and stays
  unchanged here.

## ADR coverage

No new ADR required. The signature-name "default", sentinel injection
at decode time, and `len(Identities) >= 1` round-trip rules are all
codified in ADR-0177. The wizard's stage-machine structure is
ADR-0191. The writer change is mechanical and falls under
ADR-0177's existing remit.
