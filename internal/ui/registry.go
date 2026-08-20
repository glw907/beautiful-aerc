package ui

import (
	"fmt"
	"reflect"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/theme"
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
// deliberately reuses global keys for buffer-scoped verbs.
type GrammarExemption int

// The grammar's exemption values. GrammarExemptNone is the default:
// most screens are fully checked against the interaction grammar.
const (
	GrammarExemptNone GrammarExemption = iota
	GrammarExemptModalConfirm
	GrammarExemptCatkinCommand
)

// grammarExemptions is the interaction grammar's two closed
// exemptions from the non-contradiction check (design language
// section 2). A GrammarExempt value absent from this map, including
// GrammarExemptNone, is not recognized, so checkGrammar still checks
// an entry that claims one.
var grammarExemptions = map[GrammarExemption]bool{
	GrammarExemptModalConfirm:  true,
	GrammarExemptCatkinCommand: true,
}

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
	PointerDragSelect
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

// pointerLegalStates enumerates, for each PointerTarget, the
// StateClass values ADR-0017's v1 vocabulary allows it to fire in.
// A surface-digit click, for instance, switches surfaces only in
// StateDigitsSwitch, exactly as the digit keys themselves do
// (ADR-0017: "a no-op in entry states and modals, exactly as the
// keys are"). Wheel scroll fires wherever a scrollable pane exists:
// lists, the reader, and the help overlay, per ADR-0017's table,
// which spans both browse and text-entry contexts (a picker's
// filtered list scrolls too) but never a modal. PointerPane and
// PointerModalAnswer are each the other's complement: a pane click
// focuses content, which a modal has none of to focus, so it is
// illegal there; a modal answer is legal nowhere else.
var pointerLegalStates = map[PointerTarget][]StateClass{
	PointerRow:           {StateDigitsSwitch, StatePrintableEntry},
	PointerRowOpen:       {StateDigitsSwitch},
	PointerSidebarEntry:  {StateDigitsSwitch},
	PointerPane:          {StateDigitsSwitch, StatePrintableEntry},
	PointerSurfaceDigit:  {StateDigitsSwitch},
	PointerFooterHint:    {StateDigitsSwitch, StatePrintableEntry, StateModal},
	PointerBannerDismiss: {StateDigitsSwitch, StatePrintableEntry, StateModal},
	PointerWheel:         {StateDigitsSwitch, StatePrintableEntry},
	PointerFieldCursor:   {StatePrintableEntry},
	PointerModalAnswer:   {StateModal},
	PointerDragSelect:    {StateDigitsSwitch},
}

// ScreenEntry is a screen's registration: its keymap, its pointer
// targets, its UX-4 switch-table state, and the committed order the
// footer renders its hints in (decision 8). Type identifies the
// screen's own Go type; Register derives it from the type parameter
// passed to Register, so a ScreenEntry literal never sets it by hand.
// Name is the entry's own state name from the design language section
// 2's switch table (e.g. "mail list", "config sections"); the
// switch-table test cross-references it against the digits-switch and
// printable-entry authority lists by SwitchState.
type ScreenEntry struct {
	Type           reflect.Type
	Name           string
	Keys           help.KeyMap
	Pointer        []PointerBinding
	SwitchState    StateClass
	FooterPriority []key.Binding
	GrammarExempt  GrammarExemption
}

var registered []ScreenEntry

// Register adds entry to poplar's package-level screen registry
// (ADR-0011), for screen type S: a screen calls Register[MailScreen]
// (entry) once, from its own init. The type parameter is what lets
// the screenregistry analyzer (tools/analyzers/screenregistry) read
// every registration statically from the instantiation itself,
// rather than interpreting an arbitrary expression a caller could
// get wrong. Register derives entry.Type from S, normalizing a
// pointer-receiver screen's type down to its named type, so
// Register[*MailScreen] and Register[MailScreen] register under the
// same identity.
func Register[S Screen](entry ScreenEntry) {
	t := reflect.TypeFor[S]()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	entry.Type = t
	registered = append(registered, entry)
}

// Registered returns every screen entry registered so far.
func Registered() []ScreenEntry {
	return slices.Clone(registered)
}

// flattenKeys returns every enabled key.Binding a KeyMap's FullHelp
// exposes, in column order. A disabled binding is skipped, since
// UX-2 promises no advertised key is ever a no-op, and a check
// built on this must see the same set the footer and help overlay
// do.
func flattenKeys(km help.KeyMap) []key.Binding {
	var all []key.Binding
	for _, group := range km.FullHelp() {
		for _, b := range group {
			if b.Enabled() {
				all = append(all, b)
			}
		}
	}
	return all
}

// bindingsEqual reports whether a and b bind the same keys to the
// same help text.
func bindingsEqual(a, b key.Binding) bool {
	return slices.Equal(a.Keys(), b.Keys()) && a.Help() == b.Help()
}

// helpContent returns entry's complete keymap, flattened from
// Keys.FullHelp: the help overlay's content surface (UX-5). It is
// always the full keymap, regardless of what the footer's
// width-limited prefix shows (decision 8).
func helpContent(entry ScreenEntry) []key.Binding {
	return flattenKeys(entry.Keys)
}

// shortHelpWithinFullHelp reports every entry whose Keys.ShortHelp
// advertises a binding Keys.FullHelp does not also carry: ShortHelp
// is meant to be a subset of the complete keymap, never a binding
// FullHelp omits.
func shortHelpWithinFullHelp(entries []ScreenEntry) []string {
	var violations []string
	for _, e := range entries {
		full := flattenKeys(e.Keys)
		for _, b := range e.Keys.ShortHelp() {
			if !b.Enabled() {
				continue
			}
			if !slices.ContainsFunc(full, func(f key.Binding) bool { return bindingsEqual(f, b) }) {
				violations = append(violations, fmt.Sprintf("%s: ShortHelp advertises %v, absent from FullHelp", e.Type, b))
			}
		}
	}
	return violations
}

// hintText renders b as the two- or three-word footer hint text
// (design language section 4): the key label, a space, and the
// description.
func hintText(b key.Binding) string {
	return b.Help().Key + " " + b.Help().Desc
}

// footerHelpHint is the pinned help hint's own rendered text, derived
// from GrammarKeys.Help rather than a literal (BACKLOG #62's defect
// class): every footer reserves space for it at every width (decision
// 8), and it never comes from a screen's own FooterPriority.
var footerHelpHint = hintText(GrammarKeys.Help)

// footerHints returns the width-maximal prefix of entry's committed
// FooterPriority that fits within width columns alongside the
// pinned help hint (decision 8): each enabled hint costs its
// rendered display width (ansi.StringWidth, never len, since a hint
// carries no guarantee of staying single-byte-per-cell) plus
// theme.GapHint from its neighbor, and the pinned help hint's own
// width is reserved at every call so the returned prefix never
// crowds it out. A disabled hint is skipped rather than stopping the
// scan, matching UX-2's no-advertised-no-op MUST.
func footerHints(entry ScreenEntry, width int) []key.Binding {
	avail := width - ansi.StringWidth(footerHelpHint) - theme.GapHint
	var kept []key.Binding
	used := 0
	for _, b := range entry.FooterPriority {
		if !b.Enabled() {
			continue
		}
		next := used + ansi.StringWidth(hintText(b))
		if len(kept) > 0 {
			next += theme.GapHint
		}
		if next > avail {
			break
		}
		used = next
		kept = append(kept, b)
	}
	return kept
}

// footerPriorityWithinKeymap reports every entry whose
// FooterPriority names a binding, by verb identity, absent from its
// own Keys.FullHelp: the footer's priority list can only ever
// promote a hint the screen's own keymap actually grants.
func footerPriorityWithinKeymap(entries []ScreenEntry) []string {
	var violations []string
	for _, e := range entries {
		full := flattenKeys(e.Keys)
		for _, b := range e.FooterPriority {
			if !slices.ContainsFunc(full, func(f key.Binding) bool { return f.Help().Desc == b.Help().Desc }) {
				violations = append(violations, fmt.Sprintf("%s: FooterPriority names %v, absent from its own keymap", e.Type, b))
			}
		}
	}
	return violations
}

// validSwitchStates returns the Type of every entry whose
// SwitchState is not one of the UX-4 switch table's three classes.
func validSwitchStates(entries []ScreenEntry) []string {
	var bad []string
	for _, e := range entries {
		switch e.SwitchState {
		case StateDigitsSwitch, StatePrintableEntry, StateModal:
		default:
			bad = append(bad, e.Type.String())
		}
	}
	return bad
}

// printableEntryScreens returns the Type of every entry in entries
// whose SwitchState is StatePrintableEntry: the set UX-4 requires to
// equal the screens that accept printable input.
func printableEntryScreens(entries []ScreenEntry) []reflect.Type {
	var out []reflect.Type
	for _, e := range entries {
		if e.SwitchState == StatePrintableEntry {
			out = append(out, e.Type)
		}
	}
	return out
}

// GrammarKeys is the interaction grammar's canonical key.Binding
// values (design language, 2026-07-27, section 2): the global verb
// table, the triage verb set (identical from list, thread view, and
// reader per LT-2), the folder-jump capitals, calendar's `t`, and
// thread fold/unfold. It is the source a screen's own KeyMap
// composes its bindings from, so the same key carries the same verb
// by construction rather than by review; checkGrammar checks a
// registered screen's bindings against it by verb identity
// (Help().Desc), never by whole-binding equality, since a screen is
// free to bundle a verb's synonym keys under one Binding.
var GrammarKeys = grammarKeymap{
	Navigate:      key.NewBinding(key.WithKeys("j", "k", "up", "down"), key.WithHelp("j/k", "navigate")),
	Page:          key.NewBinding(key.WithKeys("space", "b", "pgup", "pgdown"), key.WithHelp("space/b", "page")),
	Extremes:      key.NewBinding(key.WithKeys("home", "end", "G"), key.WithHelp("home/end", "extremes")),
	Open:          key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Back:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	MessageStep:   key.NewBinding(key.WithKeys("n", "p"), key.WithHelp("n/p", "message step")),
	Search:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Goto:          key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "goto")),
	NextUnread:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next unread")),
	Select:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "select")),
	SelectBy:      key.NewBinding(key.WithKeys(";"), key.WithHelp(";", "select by")),
	Undo:          key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
	Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:          key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	SurfaceSwitch: key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1-4", "surface switch")),

	Archive:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
	Delete:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	FlagToggle:  key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "flag toggle")),
	ReadToggle:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "read toggle")),
	Move:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "move")),
	JunkToggle:  key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "junk toggle")),
	Reply:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
	ReplyAll:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reply all")),
	Forward:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forward")),
	Compose:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "compose")),
	Yank:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank")),
	Attachments: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "attachments")),

	GotoInbox:   key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "inbox")),
	GotoDrafts:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "drafts")),
	GotoSent:    key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sent")),
	GotoArchive: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archive folder")),
	GotoJunk:    key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "junk folder")),
	GotoTrash:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "trash")),

	Today: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "today")),

	ThreadFold:   key.NewBinding(key.WithKeys("h"), key.WithHelp("h/l", "thread fold")),
	ThreadUnfold: key.NewBinding(key.WithKeys("l"), key.WithHelp("h/l", "thread unfold")),
}

// grammarKeymap is GrammarKeys's type: every field is a verb from
// the design language's section 2 tables.
type grammarKeymap struct {
	Navigate, Page, Extremes, Open, Back, MessageStep, Search, Goto,
	NextUnread, Select, SelectBy, Undo, Help, Quit, SurfaceSwitch,
	Archive, Delete, FlagToggle, ReadToggle, Move, JunkToggle, Reply,
	ReplyAll, Forward, Compose, Yank, Attachments,
	GotoInbox, GotoDrafts, GotoSent, GotoArchive, GotoJunk, GotoTrash,
	Today,
	ThreadFold, ThreadUnfold key.Binding
}

// fields returns every field of GrammarKeys, in the struct's own
// declaration order.
func (g grammarKeymap) fields() []key.Binding {
	return []key.Binding{
		g.Navigate, g.Page, g.Extremes, g.Open, g.Back, g.MessageStep, g.Search, g.Goto,
		g.NextUnread, g.Select, g.SelectBy, g.Undo, g.Help, g.Quit, g.SurfaceSwitch,
		g.Archive, g.Delete, g.FlagToggle, g.ReadToggle, g.Move, g.JunkToggle, g.Reply,
		g.ReplyAll, g.Forward, g.Compose, g.Yank, g.Attachments,
		g.GotoInbox, g.GotoDrafts, g.GotoSent, g.GotoArchive, g.GotoJunk, g.GotoTrash,
		g.Today,
		g.ThreadFold, g.ThreadUnfold,
	}
}

// grammarKeyTable maps every physical key GrammarKeys binds to the
// verb's own Binding, built once from GrammarKeys so checkGrammar
// and its test read the same data GrammarKeys itself commits to.
var grammarKeyTable = buildGrammarKeyTable()

func buildGrammarKeyTable() map[string]key.Binding {
	table := make(map[string]key.Binding)
	for _, b := range GrammarKeys.fields() {
		for _, k := range b.Keys() {
			table[k] = b
		}
	}
	return table
}

// checkGrammar reports every binding in entries that contradicts
// GrammarKeys, the design language's section 2 canonical key-to-verb
// mapping: a non-exempt, in-scope screen binding one of its keys
// must carry the same verb identity (Help().Desc), though it is
// free to bundle synonym keys under its own Binding value however it
// likes. Entries whose SwitchState is StatePrintableEntry are out of
// scope by the grammar's own scope rule (section 2: text-entry
// states are governed by section 3 instead); entries whose
// GrammarExempt names one of the two closed exemptions are skipped
// entirely.
func checkGrammar(entries []ScreenEntry) []string {
	var violations []string
	for _, e := range entries {
		if e.SwitchState == StatePrintableEntry || grammarExemptions[e.GrammarExempt] {
			continue
		}
		for _, b := range flattenKeys(e.Keys) {
			for _, k := range b.Keys() {
				want, ok := grammarKeyTable[k]
				if !ok || b.Help().Desc == want.Help().Desc {
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

// checkPointerGrammar reports every pointer binding in entries that
// names a verb, by identity (Help().Desc), absent from its own
// screen's keymap, or that fires in a state ADR-0017's v1
// vocabulary does not allow its target kind to fire in.
func checkPointerGrammar(entries []ScreenEntry) []string {
	var violations []string
	for _, e := range entries {
		keys := flattenKeys(e.Keys)
		for _, pb := range e.Pointer {
			if !slices.Contains(pointerLegalStates[pb.Target], e.SwitchState) {
				violations = append(violations, fmt.Sprintf(
					"%s: pointer target %v is illegal in state %v", e.Type, pb.Target, e.SwitchState))
				continue
			}
			if !slices.ContainsFunc(keys, func(b key.Binding) bool { return b.Help().Desc == pb.Key.Help().Desc }) {
				violations = append(violations, fmt.Sprintf(
					"%s: pointer target %v names a verb absent from its own keymap", e.Type, pb.Target))
			}
		}
	}
	return violations
}
