## Auditor's report, pass 2 gate

Read: all 23 live-dark and 23 live-light captures (PNG plus .ansi), the four exemplar images, the sketch toast/banner/modal/help/placeholder frames in truecolor dark, truecolor light, ANSI-16 and NO_COLOR, the floor and short frames, `render.txt`, plan lines 127-300 and 426-600, design language sections 4, 6, 7, 9, and STATUS gate items 1-12.

---

## 1. Is the shell right?

The shipped shell is right. Almost every STRUCTURAL finding in the graders' set traces to the capture tool, not the product. I verified the product independently:

- **FullRegion placeholders are correct in the live build.** `mail-150x45`: `Mail` at lead 73, end 77, right margin 73. Exactly centred on the full band, no rail, no divider. `config-150x45` 72/72, `calendar-150x45` 71/71. **Zero** live captures contain U+2502; **30 of 58** sketch captures do. Gate item 1's FullRegion ruling is satisfied by the product and contradicted by the tooling.
- Vertical centring is right at every rung (title/blank/count block centred in the content band at 45, 30, 24, 20, 19, 15 rows).
- Floor state is clean at 59x24 and 100x14, both themes: centred notice, no chrome, no garble (F4 satisfied except the ASCII `x`, already a gate item).
- Pointer works: `click-digit-2` lands on Calendar, `click-help` opens help over Calendar with the title tracking the surface, `esc-back` returns to Calendar root.
- Truecolor grounds are byte-exact against the ratified palette (dark base `22,24,29`, panel `38,43,54`; light panel `255,255,255`).

**Passed, terse:** status-line anatomy and roles; footer anatomy, 2-cell inset, right-pinned `? help` ending at col 148 of 150; band grounds and the panel step on every non-help screen; placeholder composition and centring at all six rungs; floor composition; floor-height boundary 15/14; light/dark theme parity; modal anatomy (question bold, consequence muted, `y quit` / `n stay` with the default grounded) in truecolor both themes; banner anatomy in truecolor dark; toast atom content.

---

## 2. What Geoff's eye will catch that a conformance grader cannot

### A. The help screen is not one defect, it is eight, and that is why it reads as a mess

Looking at `live-dark/help-150x45.png` and `help-80x24.png` (light is identical in layout):

1. **The header smears.** Row 1 is `1 Mail  2 3 4`, row 2 is `Help · Mail`, adjacent, both bold, no gutter. The word "Mail" appears twice in two stacked bold rows. F5 put a full-width rule between them; decision 11 retired rules in favour of "whitespace gutters and alignment" and no gutter was substituted.
2. **The screen loses its frame.** The whole 45-row window is one flat `#262B36`. On every other screen the status row and footer read instantly as raised plates against `#16181D`. On help they vanish. This is gate item 9 and it is the single most damaging thing in the pack.
3. **The second column is a stranded fragment.** At 150 the split is a fixed half-width at cell 76. Global's widest row ends at cell 45. `This screen` plus a 7-cell `q  quit` sits alone in an ocean, with 30 empty rows under both columns. The 80-column single-column form is the better composition of the two, which means **the one layout boundary pass 2 consumes makes the screen worse as it grows.**
4. **The "This screen" section is 100% duplicate.** For a placeholder its only entry is `q  quit`, which is already in Global. So help shows two literally identical rows and, at 150, spends a half-width column on the duplicate. Nobody connected H4 (the duplicate) to H7 (the stranding); they are the same fact.
5. **Global's disambiguating copy is gone.** F5 has `q  quit (at a surface root)`. Shipped is bare `quit`. The parenthetical was the only thing that made the deliberate duplicate legible. Gate item 10 claims the F5 copy was restored; this line was not.
6. **Two ragged description axes.** Global pads its key column to 10 (descriptions at col 13); This screen sizes its own (descriptions at col 6). At 80 they stack and the mismatch is unmissable. F5's key rows are also indented under their section header; shipped rows sit flush with the header, so the header stops reading as a header.
7. **Key names are lowercase here and capitalised everywhere else.** `esc`, `enter`, `space/b`, `home/end`, `1-4` against the exemplar's `Enter open` and the banner's `Esc dismiss`. Worse, one row mixes both conventions: `1-4   Mail · Calendar · People · Config`.
8. **Every hint help offers is a no-op at this size.** The footer says `j/k navigate   space/b page   esc back`, and `help-wheel-150x45.ansi` is byte-identical to `help-150x45.ansi` apart from the spinner frame, because the content fits. Over a pass-2 placeholder the body also advertises `j/k`, `space/b`, `home/end`, `enter`, which F1 explicitly says are not legal there.

That is what a user calls a mess: not a broken widget, an uncomposed screen.

### B. Three of the five components have no valid painted evidence at this gate

No live capture, in either theme, raises a banner, a toast, or a modal. Every painted banner, toast and modal in the pack comes from `cmd/sketch`, and `cmd/sketch` composites the modal over surviving chrome, a phantom 22-cell rail and a half-width footer stub. **Gate item 2 ("the modal wipes the whole shell, his eye rules for real") cannot be ruled from this pack at all**: the only frame that shows the wipe is a committed text golden. The banner-dismiss click and the modal-answer click on the pointer checklist are likewise unexercised.

### C. The chrome bands are nearly empty at the wide rung

At 150 the status row is a cluster at far left, `Syncing` at far right, and roughly 135 empty cells between; the footer is `q quit` and `? help` with the same void. The exemplar at 120 fills both bands. The reference is silent because pass 3 fills the left segment with surface context, but Geoff will see two empty rails and should be told the emptiness is a pass-2 artifact before he rules on footer or status-line design.

### D. `1 2 3 4 Config` is the worst cluster case, and the graders dismissed it

LD-4 and P-8 flagged the mid-run gap asymmetry and explicitly said Config is unaffected. It is the opposite. With the active surface last there is no trailing gap at all, so the row reads `1 2 3 4 Config`: a digit run followed by a word that looks like a label for the run, not a pairing with `4`. `1 2 3 People  4` is nearly as bad.

### E. The live shell shows exactly one sync state, and it is the indeterminate one

All 46 live captures contain the string `Syncing` and nothing else: no progress fraction, no `Synced`, no `Offline`, no backing-off. Decision 7's `⠹ syncing 4,312/36,102` never appears live. The braille spinner at Monaspace Neon 11pt is a two-pixel speck. A user cannot distinguish "syncing" from "hung" anywhere in this demo. Gate item 8 can be eyeballed on one of five strings.

### F. The count line does work, and one grader group nearly convinced you otherwise

LD-6 reports zero counts on all four dark surfaces and concludes the live-store-counts half of decision 9 is unproven. The light run of the same build on the same account shows `24,700 messages in 14 folders`. The dark run was captured against a first-run/empty store. **Do not rule "counts are broken."**

### G. `0 events` is a false statement, not just a register mismatch

P-7 filed the two-register problem (facts on Mail/Calendar, sentences on People/Config) as taste. The sharper half: Calendar has no engine, so "0 events" tells the user their calendar is empty when the truth is that calendar sync does not exist. "Contact sync is not available yet." is true. Decision 9's "state store facts" rule produces a misleading claim on a surface with no store.

### H. The light captures misrepresent the light theme's ground step

`kitty.conf` sets `background_opacity 0.95`, and poplar's light base `#E4E7EC` equals the capture terminal's configured light background, so those cells composite at 95% while explicitly-coloured `#FFFFFF` panel cells stay opaque. Measured: base renders `217,219,224` instead of `228,231,236`. Consequence: **the panel/base step in every light capture reads 1.385:1 against 1.24:1 shipped, about 12% more separation than the product has.** Exemplar-4 renders `#E4E7EC` exactly, which is the cross-check. Gate item 5 is a ground-step eyeball, so this matters, and every light-theme contrast number any grader measured off these PNGs is off.

### I. Rung behaviour is unfalsifiable in this pack

Five boundary pairs were shot. `140/139`, `100/99`, `80/79` and `20/19` are identical apart from re-centring. Only `60/59` and `15/14` change anything. The manifest calls `80/79` a "compact/spartan boundary pair" and section 9 has no boundary at 80. The wide rung's split and the standard rung's sidebar are both unevidenced (correctly, per the FullRegion ruling). The wheel item is unevidenced because help fits at 150x45; it needs a short window to falsify.

### J. Quit leaves scrollback residue

`quit-q` shows `checking store integrity...` and `checking full-text index...` in the terminal's own colours above the prompt. Those also flash before the alt screen at every launch, so they are the app's first and last impression. Reference is silent.

---

## 3. The 12 gate items, as the captures show them

1. **Kitty demo.** Both themes, 23 captures each. Rungs: only 60/59 and 15/14 change anything (see I). Pointer: digit click, footer-hint click and Esc-back all verified; wheel unevidenced; banner-dismiss and modal-answer clicks not captured. FullRegion placeholders: correct in all 46 live captures, contradicted in all 56 non-floor sketch captures.
2. **Modal wipe.** No live modal exists. The sketch modal shows the opposite of the ruling. **Unrulable from this pack.**
3. **No non-color channel on chrome bands in ANSI-16/NO_COLOR.** Confirmed by eye: `mail-ansi16`, `mail-nocolor` are one flat field top to bottom; bands survive only as text position. Survivable on a two-line placeholder, not on the banner fixture (rows 1 and 2 become undifferentiated text) and not on help. No light-ground degrade capture exists in the pack at all.
4. **`NO_COLOR=` empty enables color.** No visual evidence; nothing to eyeball.
5. **codeBg step 1.088/1.099.** No pass-2 surface renders code. Only exemplar-4's table shows it, and see H for the capture distortion.
6. **Profile conflates depth with Unicode.** Well evidenced: the modal frame collapses to `+---+` that reads visibly dashed at 11pt, the divider to a broken `|`, `✕` to a lowercase `x`, and `·` to `-` colliding with the `1-4` key label in the same help row.
7. **readerContentCap=100 at 195 columns.** No reader in pass 2, widest capture 150. **No evidence.**
8. **Sync copy sentence-cased.** Only `Syncing` appears live, `Synced` only in sketch. Two of five strings visible (see E).
9. **Help ground merging with chrome bands.** Confirmed, both themes, both rungs. See A2. This is the item to rule first.
10. **F5 copy restored / ASCII x / placeholder copy.** Copy is **not** fully restored (see A5, A7; footer verbs reworded move→navigate, close→back). ASCII `x` confirmed at 59x24 and 100x14, both themes. Placeholder copy carries two registers and one false fact (see G).
11. **Vale rule.** Not a visual item.
12. **Ratification list.** FullRegion holds live and fails in every sketch frame. `? toggles help` ships, but the pin still reads `? help` on the help screen, so it advertises "help" while acting as close. Modal wipe unevidenced. GapPin and the exemplar-wins conflicts hold where visible.

---

## 4. Judging the grading

**What is wrong, or overstated:**

- **The verifiers contradict each other on one root cause.** `cmd/sketch` omitting `FullRegion` (plus App.View's stack-top bypass and `AltScreen`) produced nine findings: SK-1, SK-2, SK-3, L1, L2, L3, L4, DG-8, DG-9. SK-1, L1, L2 and DG-9 were confirmed STRUCTURAL; SK-2, SK-3, L3, L4 and DG-8 were refuted as "documented tooling scope". Same mechanism, opposite verdicts, depending on which verifier read the doc comment. My ruling: **one STRUCTURAL defect in `cmd/sketch`**, and the waiver does not cover it. The doc comment waives modal-confirm and help; it does not waive the four placeholders, and the FullRegion ruling postdates the file. Decision 10's claim that "whatever it renders is what the app renders" is currently false for the pass's own design instrument, and that instrument is the sole source of painted evidence for banner, toast, modal and all four degrade profiles at this gate.
- **The missing "synced just now" line was filed four times** (LD-1, L5, P-6, SHORT-2), twice as STRUCTURAL. It is not a defect; decision 9 names two elements and the status line carries sync state. The verifiers got this right in both refutations, but four filings for one non-defect inflates the count. The ASCII `x` was filed three times and is already gate item 10; the `? help` pin three times; the fgSubtle digits twice; the toast gap twice.
- **DG-1's severity is wrong.** I read `help-ansi16.png` directly: the description column is dim, not absent. I could read every string the grader called "a ghost", and the grader transcribed them all itself. The verifier's reclass to DESIGN-QUESTION is right, and its proposed fix (suppress `Faint` at ANSI-16 where slot 8 already carries dim, keep it at NO_COLOR where it is the only channel) is the right question to put to Geoff. DG-2 and DG-3 are the same mechanism re-filed on the cluster and the toast; DG-3 was confirmed STRUCTURAL where DG-1 and DG-2 were refuted, which is the same inconsistency again.
- **LD-4/P-8 dismissed Config**, which is the worst case (see D).
- **LD-6 implies the live counts do not work.** They do (see F).
- **DG-5's refutation is correct** (`border` is contrast-exempt by name in section 7), and I agree the "sub-contrast structural line is the degrade substitution channel" tension is a genuine question rather than a defect.
- **SK-4's refutation is correct**: the manifest parenthetical is wrong, the shell is not.

**What all five grader groups missed:** the absence of any live banner/toast/modal (B); that the wide and short-height boundaries are as unfalsifiable as 100/99 (I); that the wheel capture proves nothing (I); the empty chrome bands at 150 (C); the Config cluster (D); the light-theme 0.95 opacity distortion of every light contrast measurement (H); the dark-vs-light count discrepancy (F); that only one of five sync states appears live (E); that "0 events" is false where "Contact sync is not available yet" is true (G); and that the duplicate `q quit` and the stranded second column are the same fact (A4).

---

## 5. The three things to put in front of Geoff first

1. **The help screen, as one problem, not eight findings.** It is the only real screen pass 2 ships, it is the one he already flagged, and the 150-column form is worse than the 80-column form, which means the single layout boundary pass 2 consumes is a regression. Rule the ground step (item 9), the header gutter, the two-column trigger (width alone, with a one-entry section, is the wrong condition), the restored `quit (at a surface root)` parenthetical, one shared key column, and key-name capitalisation, in one sitting.

2. **The evidence gap, before ruling items 1, 2 and 6.** No live banner, toast or modal exists; `cmd/sketch` mis-renders composition on every placeholder, the modal and help; and the pack carries 56 sketch frames showing the empty rail the FullRegion ruling removed. Fix the one call site, reshoot the sketch group, and add live banner/toast/modal triggers plus a short-window help capture for the wheel. Item 2 is currently unrulable by eye.

3. **The two things that are actually broken in shipped code, plus one one-line doc ruling.** DG-4 is real and I confirmed it visually: in ANSI-16 the modal's default-answer pill emits reverse video and then changes foreground mid-run, so `  n` renders as a light plate and ` stay  ` as a dark plate with pale, near-unreadable text. It is in `confirm.go`, not the tooling, and it breaks selection's exclusive channel. Second, the live sync signal is the word "Syncing" with an invisible spinner and no progress in every capture, which is indistinguishable from a hang. And the doc ruling owed: decision 6 says fgSubtle "never carries reading text", while the ratified exemplar paints the sibling digits, which are also the status line's pointer targets, in fgSubtle. Either the sentence or the exemplar has to move, and it binds pass 3's row anatomy.