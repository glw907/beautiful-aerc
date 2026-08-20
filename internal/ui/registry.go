// Package ui is poplar's bubbletea v2 UI layer (technical design
// section 12), built out across pass 2. This file holds the screen
// registry (ADR-0011): the package-level table every screen adds
// itself to at init, and the mechanism the footer, the help overlay,
// and the grammar and switch-table tests all read instead of their
// own copy of a screen's keymap.
package ui

import (
	"fmt"
	"reflect"
	"slices"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// Screen is a bubbletea model that also reports the registry entry
// it registered under. The root model calls Entry to read the
// active screen's keymap, pointer targets, and footer priority
// without a second, hand-maintained copy of the same data.
type Screen interface {
	tea.Model
	Entry() ScreenEntry
}

// StateClass is one of the UX-4 switch table's three categories a
// registered screen resolves to (design language section 2):
// StateDigitsSwitch and StatePrintableEntry partition every browse
// or entry context, and StateModal is neither, since a modal answers
// y/n/Esc and is left before switching.
type StateClass int

// The three UX-4 state classes.
const (
	StateDigitsSwitch StateClass = iota
	StatePrintableEntry
	StateModal
)

// GrammarExemption names one of the interaction grammar's two closed
// exemptions from the non-contradiction check (design language
// section 2): the modal confirm, where y/n/Esc answer a question
// and y is not yank, and Catkin's command state, whose buffer keymap
// deliberately reuses global keys for buffer-scoped verbs. The list
// is closed: a value outside these three constants is not
// recognized, so an entry that claims one is checked as if it
// claimed none.
type GrammarExemption int

// The grammar's exemption values. GrammarExemptNone is the default:
// most screens are fully checked against the interaction grammar.
const (
	GrammarExemptNone GrammarExemption = iota
	GrammarExemptModalConfirm
	GrammarExemptCatkinCommand
)

// PointerTarget names one of ADR-0017's v1 pointer-target kinds.
type PointerTarget int

// The pointer targets ADR-0017's v1 vocabulary defines.
const (
	PointerRow PointerTarget = iota
	PointerRowOpen
	PointerSidebarEntry
	PointerPane
	PointerSurfaceDigit
	PointerFooterHint
	PointerBannerDismiss
	PointerWheel
	PointerFieldCursor
	PointerModalAnswer
)

// PointerBinding binds a pointer target to the keyboard verb it
// accelerates (ADR-0017): a click or wheel action reaches the same
// grammar verb Key names, under the same state rules the key itself
// obeys. Key should be one of the bindings the screen's own Keys
// already advertises; the pointer-grammar test asserts exactly that.
type PointerBinding struct {
	Target PointerTarget
	Key    key.Binding
}

// ScreenEntry is a screen's registration: its keymap, its pointer
// targets, its UX-4 switch-table state, and the committed order the
// footer renders its hints in (decision 8). Type identifies the
// screen's own Go type, which the reflection test compares against
// every type in internal/ui/... shaped like a Screen.
type ScreenEntry struct {
	Type           reflect.Type
	Keys           help.KeyMap
	Pointer        []PointerBinding
	SwitchState    StateClass
	FooterPriority []key.Binding
	GrammarExempt  GrammarExemption
}

var registered []ScreenEntry

// Register adds entry to poplar's package-level screen registry
// (ADR-0011). A screen calls this once, from its own init, so the
// footer, the help overlay, and the grammar and switch-table tests
// all read the same data a second, hand-maintained copy could drift
// from. It panics on a nil entry.Type, since a registry entry the
// reflection test can never match back to a source type defeats the
// registry's purpose.
func Register(entry ScreenEntry) {
	if entry.Type == nil {
		panic("ui: Register called with a nil ScreenEntry.Type")
	}
	registered = append(registered, entry)
}

// Registered returns every screen entry registered so far.
func Registered() []ScreenEntry {
	return slices.Clone(registered)
}

// flattenKeys returns every key.Binding a KeyMap's FullHelp exposes,
// in column order.
func flattenKeys(km help.KeyMap) []key.Binding {
	var all []key.Binding
	for _, group := range km.FullHelp() {
		all = append(all, group...)
	}
	return all
}

// HelpContent returns entry's complete keymap, flattened from
// Keys.FullHelp: the help overlay's content surface (UX-5). It is
// always the full keymap, regardless of what the footer's
// width-limited prefix shows (decision 8).
func HelpContent(entry ScreenEntry) []key.Binding {
	return flattenKeys(entry.Keys)
}

// bindingsEqual reports whether a and b bind the same keys to the
// same help text. Two independently constructed key.Binding values
// for the same verb compare equal by this, which is what lets
// CheckGrammar and CheckPointerGrammar compare a screen's own
// bindings against a canonical reference without requiring identity.
func bindingsEqual(a, b key.Binding) bool {
	return slices.Equal(a.Keys(), b.Keys()) && a.Help() == b.Help()
}

// hintText renders b as the two- or three-word footer hint text
// (design language section 4): the key label, a space, and the
// description.
func hintText(b key.Binding) string {
	return b.Help().Key + " " + b.Help().Desc
}

// footerHelpHint is the pinned "help" hint every footer reserves
// space for at every width (decision 8); it never comes from a
// screen's own FooterPriority.
const footerHelpHint = "? help"

// footerGap is the three-cell gap decision 8 places between footer
// hints, and between the last priority hint and the pinned help hint.
const footerGap = 3

// FooterHints returns the width-maximal prefix of entry's committed
// FooterPriority that fits within width columns alongside the
// pinned help hint (decision 8): each hint costs its rendered text
// plus a three-cell gap from its neighbor, and the pinned help
// hint's own width is reserved at every call so the returned prefix
// never crowds it out.
func FooterHints(entry ScreenEntry, width int) []key.Binding {
	avail := width - len(footerHelpHint) - footerGap
	var kept []key.Binding
	used := 0
	for _, b := range entry.FooterPriority {
		next := used + len(hintText(b))
		if len(kept) > 0 {
			next += footerGap
		}
		if next > avail {
			break
		}
		used = next
		kept = append(kept, b)
	}
	return kept
}

// CheckGrammar reports every binding in entries that contradicts
// table, the canonical key-to-verb mapping the design language's
// section 2 commits: a screen binding one of table's keys must use
// exactly that key's Help, since the grammar promises one verb per
// key everywhere the verb exists. Entries whose SwitchState is
// StatePrintableEntry are out of scope by the grammar's own scope
// rule (section 2: text-entry states are governed by section 3
// instead); entries whose GrammarExempt names one of the two closed
// exemptions are skipped entirely.
func CheckGrammar(entries []ScreenEntry, table map[string]key.Binding) []string {
	var violations []string
	for _, e := range entries {
		if e.SwitchState == StatePrintableEntry {
			continue
		}
		if e.GrammarExempt == GrammarExemptModalConfirm || e.GrammarExempt == GrammarExemptCatkinCommand {
			continue
		}
		for _, b := range flattenKeys(e.Keys) {
			for _, k := range b.Keys() {
				want, ok := table[k]
				if !ok || bindingsEqual(b, want) {
					continue
				}
				violations = append(violations, fmt.Sprintf(
					"%s: key %q bound to %q, contradicts the grammar's %q",
					e.Type, k, b.Help().Desc, want.Help().Desc))
			}
		}
	}
	return violations
}

// pointerLegalStates enumerates, for each PointerTarget, the
// StateClass values ADR-0017's v1 vocabulary allows it to fire in.
// A surface-digit click, for instance, switches surfaces only in
// StateDigitsSwitch, exactly as the digit keys themselves do
// (ADR-0017: "a no-op in entry states and modals, exactly as the
// keys are").
var pointerLegalStates = map[PointerTarget][]StateClass{
	PointerRow:           {StateDigitsSwitch, StatePrintableEntry},
	PointerRowOpen:       {StateDigitsSwitch},
	PointerSidebarEntry:  {StateDigitsSwitch},
	PointerPane:          {StateDigitsSwitch, StatePrintableEntry, StateModal},
	PointerSurfaceDigit:  {StateDigitsSwitch},
	PointerFooterHint:    {StateDigitsSwitch, StatePrintableEntry, StateModal},
	PointerBannerDismiss: {StateDigitsSwitch, StatePrintableEntry, StateModal},
	PointerWheel:         {StateDigitsSwitch},
	PointerFieldCursor:   {StatePrintableEntry},
	PointerModalAnswer:   {StateModal},
}

// CheckPointerGrammar reports every pointer binding in entries that
// names a verb absent from its own screen's keymap, or that fires in
// a state ADR-0017's v1 vocabulary does not allow its target kind to
// fire in.
func CheckPointerGrammar(entries []ScreenEntry) []string {
	var violations []string
	for _, e := range entries {
		keys := flattenKeys(e.Keys)
		for _, pb := range e.Pointer {
			if !slices.Contains(pointerLegalStates[pb.Target], e.SwitchState) {
				violations = append(violations, fmt.Sprintf(
					"%s: pointer target %v is illegal in state %v", e.Type, pb.Target, e.SwitchState))
				continue
			}
			if !slices.ContainsFunc(keys, func(b key.Binding) bool { return bindingsEqual(b, pb.Key) }) {
				violations = append(violations, fmt.Sprintf(
					"%s: pointer target %v names a verb absent from its own keymap", e.Type, pb.Target))
			}
		}
	}
	return violations
}

// PrintableEntryScreens returns the Type of every entry in entries
// whose SwitchState is StatePrintableEntry: the set UX-4 requires to
// equal the screens that accept printable input.
func PrintableEntryScreens(entries []ScreenEntry) []reflect.Type {
	var out []reflect.Type
	for _, e := range entries {
		if e.SwitchState == StatePrintableEntry {
			out = append(out, e.Type)
		}
	}
	return out
}
