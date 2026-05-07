// SPDX-License-Identifier: MIT

package contacts

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Email and phone label tables. Index 0 of each is the default.
var (
	emailLabels = []string{"Work", "Home", ""}
	phoneLabels = []string{"Mobile", "Work", "Home", "Fax", ""}
)

const (
	formContentW    = 60
	formMinContentW = 40
	noteRows        = 3
)

type emailRow struct {
	input textinput.Model
	label int // index into emailLabels
}

type phoneRow struct {
	input textinput.Model
	label int // index into phoneLabels
}

// Form is the contact edit modal. Two render contexts: centered modal
// over dimmed mail chrome (FromPopover), or right-pane replacement in
// Contacts mode.
type Form struct {
	shell       uicore.ModalShell
	styles      Styles
	initial     Contact
	kind        Kind
	fromPopover bool
	saveTo      []string
	saveIdx     int

	first   textinput.Model
	last    textinput.Model
	org     textinput.Model
	title   textinput.Model
	bizName textinput.Model

	emails []emailRow
	phones []phoneRow
	note   textarea.Model

	focusIdx int
	err      string

	width, height int
}

// NewForm builds a Form pre-seeded from initial. saveTo lists the
// destinations the radio button cycles through. fromPopover selects
// the centered-modal render path. Otherwise the form replaces the
// right pane in Contacts mode.
func NewForm(s Styles, initial Contact, fromPopover bool, saveTo []string) Form {
	mk := func(value string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetValue(value)
		return ti
	}

	emails := make([]emailRow, 0, len(initial.Emails))
	for _, e := range initial.Emails {
		emails = append(emails, emailRow{
			input: mk(e.Address),
			label: labelIndex(emailLabels, e.Label),
		})
	}
	if len(emails) == 0 {
		emails = append(emails, emailRow{input: mk(""), label: 0})
	}
	phones := make([]phoneRow, 0, len(initial.Phones))
	for _, p := range initial.Phones {
		phones = append(phones, phoneRow{
			input: mk(p.E164),
			label: labelIndex(phoneLabels, p.Label),
		})
	}

	note := textarea.New()
	note.Prompt = ""
	note.SetValue(initial.Note)
	note.SetHeight(noteRows)
	note.ShowLineNumbers = false

	first := mk(initial.Given)
	last := mk(initial.Family)
	bizName := mk(initial.Name)
	if initial.Kind == KindPerson {
		// Person carries the FN in Name when we lack first/last; fall
		// back to splitting Name on first space.
		if initial.Given == "" && initial.Family == "" && initial.Name != "" {
			first.SetValue(initial.Name)
		}
	}

	f := Form{
		styles:      s,
		kind:        initial.Kind,
		fromPopover: fromPopover,
		saveTo:      saveTo,
		first:       first,
		last:        last,
		org:         mk(initial.Org),
		title:       mk(initial.Title),
		bizName:     bizName,
		emails:      emails,
		phones:      phones,
		note:        note,
		focusIdx:    1, // first text field, past the kind toggle
	}
	f = f.applyFocus()
	// Snapshot initial state from the form's own derivation so the dirty
	// check compares apples to apples (Name = Given + " " + Family for
	// Person, etc.).
	f.initial = f.currentContact()
	return f
}

func labelIndex(labels []string, want string) int {
	for i, l := range labels {
		if strings.EqualFold(l, want) {
			return i
		}
	}
	return len(labels) - 1 // blank
}

// FromPopover reports whether the form was opened from the i-popover
// (centered modal) or from Contacts mode (right-pane replacement).
func (f Form) FromPopover() bool { return f.fromPopover }

// SetSize stores the terminal dimensions and resizes children.
func (f Form) SetSize(w, h int) Form {
	f.width = w
	f.height = h
	f.shell = f.shell.SetSize(w, h)
	cw := f.contentW()
	inputW := f.inputWidth(cw)
	for i := range f.emails {
		f.emails[i].input.Width = inputW
	}
	for i := range f.phones {
		f.phones[i].input.Width = inputW
	}
	f.first.Width = inputW
	f.last.Width = inputW
	f.org.Width = inputW
	f.title.Width = inputW
	f.bizName.Width = inputW
	f.note.SetWidth(inputW)
	return f
}

func (f Form) contentW() int {
	// Right-pane mode renders without ┌─┐ chrome, so the entire width
	// is content. Modal mode reserves 2 cells for borders and clamps
	// to formContentW for visual balance against the popover.
	if !f.fromPopover {
		if f.width <= 0 {
			return formMinContentW
		}
		return f.width
	}
	cw := formContentW
	if f.width-4 < cw {
		cw = f.width - 4
	}
	if cw < formMinContentW {
		cw = formMinContentW
	}
	return cw
}

// inputWidth returns the textinput width given a row content width.
// The label column eats labelCol cells, then a space, then the input.
const labelCol = 8

func (f Form) inputWidth(contentW int) int {
	// Reserve label column + leading space + outer brackets + trailing
	// row chrome (cycler, star, minus add ~10 cells). The cycler/buttons
	// only show on email/phone rows but a uniform input width keeps
	// columns aligned and the textinput body inside the panel.
	w := contentW - labelCol - 14
	if w < 10 {
		w = 10
	}
	return w
}

// Update routes a tea.Msg into the focused widget or handles a global
// form binding (Tab, Ctrl+S, Esc, Space/← /→ on the kind toggle).
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return f, nil
	}

	// Global form keys.
	switch k.Type {
	case tea.KeyCtrlS:
		if err := f.Validate(); err != nil {
			f.err = err.Error()
			return f, nil
		}
		f.err = ""
		c := f.currentContact()
		dest := ""
		if f.saveIdx >= 0 && f.saveIdx < len(f.saveTo) {
			dest = f.saveTo[f.saveIdx]
		}
		return f, func() tea.Msg { return ContactSaveMsg{Contact: c, SaveTo: dest} }
	case tea.KeyEsc:
		dirty := f.dirty()
		return f, func() tea.Msg { return ContactCancelMsg{Dirty: dirty} }
	case tea.KeyTab:
		f = f.advanceFocus(+1)
		return f, nil
	case tea.KeyShiftTab:
		f = f.advanceFocus(-1)
		return f, nil
	}

	// Focused-widget routing.
	w := f.focusedWidget()
	switch w.kind {
	case widKind:
		switch k.Type {
		case tea.KeyLeft, tea.KeyRight:
			f.kind = toggleKind(f.kind)
			f = f.applyFocus()
		case tea.KeyRunes:
			if len(k.Runes) == 1 && k.Runes[0] == ' ' {
				f.kind = toggleKind(f.kind)
				f = f.applyFocus()
			}
		}
		return f, nil
	case widEmailCycler:
		if isCyclerKey(k) {
			f.emails[w.row].label = (f.emails[w.row].label + 1) % len(emailLabels)
		}
		return f, nil
	case widEmailStar:
		if k.Type == tea.KeyEnter && w.row > 0 {
			row := f.emails[w.row]
			f.emails = append(f.emails[:w.row], f.emails[w.row+1:]...)
			f.emails = append([]emailRow{row}, f.emails...)
			f = f.applyFocus()
		}
		return f, nil
	case widEmailMinus:
		if k.Type == tea.KeyEnter && len(f.emails) > 1 {
			f.emails = append(f.emails[:w.row], f.emails[w.row+1:]...)
			f = f.applyFocus()
		}
		return f, nil
	case widAddEmail:
		if k.Type == tea.KeyEnter {
			ti := textinput.New()
			ti.Prompt = ""
			ti.Width = f.inputWidth(f.contentW())
			f.emails = append(f.emails, emailRow{input: ti, label: 0})
			f = f.applyFocus()
		}
		return f, nil
	case widPhoneCycler:
		if isCyclerKey(k) {
			f.phones[w.row].label = (f.phones[w.row].label + 1) % len(phoneLabels)
		}
		return f, nil
	case widPhoneStar:
		if k.Type == tea.KeyEnter && w.row > 0 {
			row := f.phones[w.row]
			f.phones = append(f.phones[:w.row], f.phones[w.row+1:]...)
			f.phones = append([]phoneRow{row}, f.phones...)
			f = f.applyFocus()
		}
		return f, nil
	case widPhoneMinus:
		if k.Type == tea.KeyEnter {
			f.phones = append(f.phones[:w.row], f.phones[w.row+1:]...)
			f = f.applyFocus()
		}
		return f, nil
	case widAddPhone:
		if k.Type == tea.KeyEnter {
			ti := textinput.New()
			ti.Prompt = ""
			ti.Width = f.inputWidth(f.contentW())
			f.phones = append(f.phones, phoneRow{input: ti, label: 0})
			f = f.applyFocus()
		}
		return f, nil
	case widSaveTo:
		switch k.Type {
		case tea.KeyLeft:
			if f.saveIdx > 0 {
				f.saveIdx--
			}
		case tea.KeyRight:
			if f.saveIdx < len(f.saveTo)-1 {
				f.saveIdx++
			}
		case tea.KeyRunes:
			if len(k.Runes) == 1 && k.Runes[0] == ' ' {
				if len(f.saveTo) > 0 {
					f.saveIdx = (f.saveIdx + 1) % len(f.saveTo)
				}
			}
		}
		return f, nil
	}

	// Text-input routing for the regular widgets.
	var cmd tea.Cmd
	switch w.kind {
	case widFirst:
		f.first, cmd = f.first.Update(msg)
	case widLast:
		f.last, cmd = f.last.Update(msg)
	case widOrg:
		f.org, cmd = f.org.Update(msg)
	case widTitle:
		f.title, cmd = f.title.Update(msg)
	case widBizName:
		f.bizName, cmd = f.bizName.Update(msg)
	case widEmailInput:
		f.emails[w.row].input, cmd = f.emails[w.row].input.Update(msg)
	case widPhoneInput:
		f.phones[w.row].input, cmd = f.phones[w.row].input.Update(msg)
	case widNote:
		f.note, cmd = f.note.Update(msg)
	}
	return f, cmd
}

func isCyclerKey(k tea.KeyMsg) bool {
	if k.Type == tea.KeyLeft || k.Type == tea.KeyRight || k.Type == tea.KeyEnter {
		return true
	}
	return k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] == ' '
}

func toggleKind(k Kind) Kind {
	if k == KindPerson {
		return KindOrg
	}
	return KindPerson
}

// Validate applies the spec rules. Returns nil when the form is
// ready to save.
func (f Form) Validate() error {
	switch f.kind {
	case KindPerson:
		if strings.TrimSpace(f.first.Value()) == "" && strings.TrimSpace(f.last.Value()) == "" {
			return errors.New("name required (first or last)")
		}
	case KindOrg:
		if strings.TrimSpace(f.bizName.Value()) == "" {
			return errors.New("name required")
		}
	}
	if len(f.emails) == 0 {
		return errors.New("at least one email required")
	}
	for i, e := range f.emails {
		v := strings.TrimSpace(e.input.Value())
		if v == "" {
			return fmt.Errorf("email row %d is empty", i+1)
		}
		if _, perr := mail.ParseAddress(v); perr != nil {
			return fmt.Errorf("email row %d invalid: %v", i+1, perr)
		}
	}
	if f.saveIdx < 0 || f.saveIdx >= len(f.saveTo) {
		return errors.New("save destination required")
	}
	return nil
}

// dirty reports whether the form's current state differs from initial.
func (f Form) dirty() bool {
	return !sameContact(f.initial, f.currentContact())
}

func (f Form) currentContact() Contact {
	c := Contact{Kind: f.kind, Note: f.note.Value()}
	switch f.kind {
	case KindPerson:
		c.Given = strings.TrimSpace(f.first.Value())
		c.Family = strings.TrimSpace(f.last.Value())
		c.Org = strings.TrimSpace(f.org.Value())
		c.Title = strings.TrimSpace(f.title.Value())
		c.Name = strings.TrimSpace(strings.Join([]string{c.Given, c.Family}, " "))
	case KindOrg:
		c.Name = strings.TrimSpace(f.bizName.Value())
	}
	for _, e := range f.emails {
		v := strings.TrimSpace(e.input.Value())
		if v == "" {
			continue
		}
		c.Emails = append(c.Emails, Email{Address: v, Label: emailLabels[e.label]})
	}
	for _, p := range f.phones {
		v := strings.TrimSpace(p.input.Value())
		if v == "" {
			continue
		}
		c.Phones = append(c.Phones, Phone{E164: v, Label: phoneLabels[p.label]})
	}
	return c
}

func sameContact(a, b Contact) bool {
	if a.Kind != b.Kind || a.Name != b.Name || a.Family != b.Family ||
		a.Given != b.Given || a.Org != b.Org || a.Title != b.Title || a.Note != b.Note {
		return false
	}
	if len(a.Emails) != len(b.Emails) || len(a.Phones) != len(b.Phones) {
		return false
	}
	for i := range a.Emails {
		if a.Emails[i] != b.Emails[i] {
			return false
		}
	}
	for i := range a.Phones {
		if a.Phones[i] != b.Phones[i] {
			return false
		}
	}
	return true
}

// --- Focus model ---

type widgetKind int

const (
	widKind widgetKind = iota
	widFirst
	widLast
	widOrg
	widTitle
	widBizName
	widEmailInput
	widEmailCycler
	widEmailStar
	widEmailMinus
	widAddEmail
	widPhoneInput
	widPhoneCycler
	widPhoneStar
	widPhoneMinus
	widAddPhone
	widNote
	widSaveTo
)

type widget struct {
	kind widgetKind
	row  int // for email/phone widgets; -1 otherwise
}

// focusList builds the ordered list of focusable widgets given the
// current kind and row counts. Recomputed on every mutation so
// indices remain stable for tests via the helper methods below.
func (f Form) focusList() []widget {
	out := []widget{{kind: widKind}}
	if f.kind == KindPerson {
		out = append(out, widget{kind: widFirst}, widget{kind: widLast},
			widget{kind: widOrg}, widget{kind: widTitle})
	} else {
		out = append(out, widget{kind: widBizName})
	}
	for i := range f.emails {
		out = append(out,
			widget{kind: widEmailInput, row: i},
			widget{kind: widEmailCycler, row: i},
			widget{kind: widEmailStar, row: i},
			widget{kind: widEmailMinus, row: i},
		)
	}
	out = append(out, widget{kind: widAddEmail})
	for i := range f.phones {
		out = append(out,
			widget{kind: widPhoneInput, row: i},
			widget{kind: widPhoneCycler, row: i},
			widget{kind: widPhoneStar, row: i},
			widget{kind: widPhoneMinus, row: i},
		)
	}
	out = append(out, widget{kind: widAddPhone}, widget{kind: widNote}, widget{kind: widSaveTo})
	return out
}

func (f Form) focusedWidget() widget {
	list := f.focusList()
	if f.focusIdx < 0 || f.focusIdx >= len(list) {
		return list[0]
	}
	return list[f.focusIdx]
}

func (f Form) advanceFocus(delta int) Form {
	list := f.focusList()
	n := len(list)
	f.focusIdx = ((f.focusIdx + delta) + n) % n
	return f.applyFocus()
}

// applyFocus blurs every text input then focuses the one (if any)
// targeted by the current focusIdx. Cycler/button widgets carry
// keyboard focus implicitly via the highlight in View.
func (f Form) applyFocus() Form {
	// Clamp focusIdx in case the list shrank.
	list := f.focusList()
	if f.focusIdx >= len(list) {
		f.focusIdx = len(list) - 1
	}
	if f.focusIdx < 0 {
		f.focusIdx = 0
	}

	f.first.Blur()
	f.last.Blur()
	f.org.Blur()
	f.title.Blur()
	f.bizName.Blur()
	for i := range f.emails {
		f.emails[i].input.Blur()
	}
	for i := range f.phones {
		f.phones[i].input.Blur()
	}
	f.note.Blur()

	w := f.focusedWidget()
	switch w.kind {
	case widFirst:
		f.first.Focus()
	case widLast:
		f.last.Focus()
	case widOrg:
		f.org.Focus()
	case widTitle:
		f.title.Focus()
	case widBizName:
		f.bizName.Focus()
	case widEmailInput:
		f.emails[w.row].input.Focus()
	case widPhoneInput:
		f.phones[w.row].input.Focus()
	case widNote:
		f.note.Focus()
	}
	return f
}

func (f Form) addEmailIdx() int {
	for i, w := range f.focusList() {
		if w.kind == widAddEmail {
			return i
		}
	}
	return 0
}

func (f Form) emailRowMinusIdx(row int) int {
	for i, w := range f.focusList() {
		if w.kind == widEmailMinus && w.row == row {
			return i
		}
	}
	return 0
}

func (f Form) emailRowStarIdx(row int) int {
	for i, w := range f.focusList() {
		if w.kind == widEmailStar && w.row == row {
			return i
		}
	}
	return 0
}

// --- Rendering ---

// Box renders the form as a bordered modal (used in fromPopover mode).
func (f Form) Box(termW, _ int) string {
	cw := f.contentW()
	bodyRows := f.bodyRows(cw)
	footerRows := f.footerRows(cw)
	title := "New contact"
	if f.initial.Name != "" || f.initial.Given != "" || f.initial.Family != "" {
		title = "Edit contact"
	}
	return f.shell.Box(title, bodyRows, footerRows, cw)
}

// Position centers the box.
func (f Form) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}

// View renders the form. Right-pane mode (Contacts mode) returns the
// body + footer rows without ┌─┐ chrome since the contacts frame
// already supplies its own borders. Modal mode (popover origin) wraps
// the rows in a ModalShell box. The App composites it via PlaceOverlay.
func (f Form) View() string {
	if f.width == 0 || f.height == 0 {
		return ""
	}
	if f.fromPopover {
		return f.Box(f.width, f.height)
	}
	cw := f.contentW()
	rows := f.bodyRows(cw)
	rows = append(rows, uicore.PadOrTruncate("", cw))
	rows = append(rows, f.footerRows(cw)...)
	return strings.Join(rows, "\n")
}

func (f Form) bodyRows(cw int) []string {
	var rows []string
	rows = append(rows, uicore.PadOrTruncate("", cw))
	rows = append(rows, f.kindRow(cw))
	rows = append(rows, uicore.PadOrTruncate("", cw))

	switch f.kind {
	case KindPerson:
		rows = append(rows,
			f.fieldRow("First", f.first, f.focusedKind() == widFirst, cw),
			f.fieldRow("Last", f.last, f.focusedKind() == widLast, cw),
			f.fieldRow("Org", f.org, f.focusedKind() == widOrg, cw),
			f.fieldRow("Title", f.title, f.focusedKind() == widTitle, cw),
		)
	case KindOrg:
		rows = append(rows, f.fieldRow("Name", f.bizName, f.focusedKind() == widBizName, cw))
	}
	rows = append(rows, uicore.PadOrTruncate("", cw))

	for i, e := range f.emails {
		rows = append(rows, f.contactRow("Emails", i, e.input, emailLabels[e.label], i == 0, cw))
	}
	rows = append(rows, f.addRow("+ add email", f.focusedKind() == widAddEmail, cw))
	rows = append(rows, uicore.PadOrTruncate("", cw))

	for i, p := range f.phones {
		rows = append(rows, f.phoneContactRow("Phones", i, p.input, phoneLabels[p.label], i == 0, cw))
	}
	rows = append(rows, f.addRow("+ add phone", f.focusedKind() == widAddPhone, cw))
	rows = append(rows, uicore.PadOrTruncate("", cw))

	rows = append(rows, f.noteRows(cw)...)
	rows = append(rows, uicore.PadOrTruncate("", cw))
	rows = append(rows, f.saveToRow(cw))

	if f.err != "" {
		rows = append(rows, uicore.PadOrTruncate("", cw))
		rows = append(rows, uicore.PadOrTruncate(f.styles.Warn.Render("⚠ "+f.err), cw))
	}
	return rows
}

func (f Form) footerRows(cw int) []string {
	hint := "Tab/Shift+Tab navigate · Ctrl+S save · Esc cancel"
	return []string{uicore.PadOrTruncate(f.styles.Dim.Render(hint), cw)}
}

func (f Form) focusedKind() widgetKind { return f.focusedWidget().kind }

func (f Form) kindRow(cw int) string {
	on := f.styles.KindOn
	off := f.styles.KindOff
	person := off.Render("○ Person")
	org := off.Render("○ Business")
	if f.kind == KindPerson {
		person = on.Render("● Person")
	} else {
		org = on.Render("● Business")
	}
	prefix := uicore.PadOrTruncate("Kind:", labelCol)
	body := prefix + " " + person + "   " + org
	if f.focusedKind() == widKind {
		body = f.styles.FieldFocus.Render("▸ ") + body
	} else {
		body = "  " + body
	}
	return uicore.PadOrTruncate(body, cw)
}

func (f Form) fieldRow(label string, ti textinput.Model, focused bool, cw int) string {
	prefix := uicore.PadOrTruncate(label+":", labelCol)
	style := f.styles.FieldBlur
	if focused {
		style = f.styles.FieldFocus
	}
	box := style.Render("[" + ti.View() + "]")
	row := prefix + " " + box
	return uicore.PadOrTruncate(row, cw)
}

func (f Form) contactRow(label string, row int, ti textinput.Model, lbl string, isPrimary bool, cw int) string {
	prefix := "        "
	if row == 0 {
		prefix = uicore.PadOrTruncate(label+":", labelCol)
	}
	fk := f.focusedWidget()
	inputFocused := fk.kind == widEmailInput && fk.row == row
	cyclerFocused := fk.kind == widEmailCycler && fk.row == row
	starFocused := fk.kind == widEmailStar && fk.row == row
	minusFocused := fk.kind == widEmailMinus && fk.row == row

	inputStyle := f.styles.FieldBlur
	if inputFocused {
		inputStyle = f.styles.FieldFocus
	}
	inputBox := inputStyle.Render("[" + ti.View() + "]")

	cycler := "◀" + cyclerLabel(lbl) + "▶"
	if cyclerFocused {
		cycler = f.styles.FieldFocus.Render(cycler)
	} else {
		cycler = f.styles.Dim.Render(cycler)
	}

	star := "☆"
	if isPrimary {
		star = "★"
	}
	if starFocused {
		star = f.styles.FieldFocus.Render(star)
	} else if isPrimary {
		star = f.styles.GroupLabel.Render(star)
	} else {
		star = f.styles.Dim.Render(star)
	}

	minus := "−"
	if len(f.emails) <= 1 {
		minus = f.styles.Dim.Render(minus)
	} else if minusFocused {
		minus = f.styles.FieldFocus.Render(minus)
	} else {
		minus = f.styles.Body.Render(minus)
	}

	body := prefix + " " + inputBox + " " + cycler + " " + star + minus
	return uicore.PadOrTruncate(body, cw)
}

func (f Form) phoneContactRow(label string, row int, ti textinput.Model, lbl string, isPrimary bool, cw int) string {
	prefix := "        "
	if row == 0 {
		prefix = uicore.PadOrTruncate(label+":", labelCol)
	}
	fk := f.focusedWidget()
	inputFocused := fk.kind == widPhoneInput && fk.row == row
	cyclerFocused := fk.kind == widPhoneCycler && fk.row == row
	starFocused := fk.kind == widPhoneStar && fk.row == row
	minusFocused := fk.kind == widPhoneMinus && fk.row == row

	inputStyle := f.styles.FieldBlur
	if inputFocused {
		inputStyle = f.styles.FieldFocus
	}
	inputBox := inputStyle.Render("[" + ti.View() + "]")

	cycler := "◀" + cyclerLabel(lbl) + "▶"
	if cyclerFocused {
		cycler = f.styles.FieldFocus.Render(cycler)
	} else {
		cycler = f.styles.Dim.Render(cycler)
	}

	star := "☆"
	if isPrimary {
		star = "★"
	}
	if starFocused {
		star = f.styles.FieldFocus.Render(star)
	} else if isPrimary {
		star = f.styles.GroupLabel.Render(star)
	} else {
		star = f.styles.Dim.Render(star)
	}

	minus := "−"
	if minusFocused {
		minus = f.styles.FieldFocus.Render(minus)
	} else {
		minus = f.styles.Body.Render(minus)
	}

	body := prefix + " " + inputBox + " " + cycler + " " + star + minus
	return uicore.PadOrTruncate(body, cw)
}

func (f Form) addRow(label string, focused bool, cw int) string {
	prefix := strings.Repeat(" ", labelCol+1)
	style := f.styles.Dim
	if focused {
		style = f.styles.FieldFocus
	}
	body := prefix + style.Render("["+label+"]")
	return uicore.PadOrTruncate(body, cw)
}

func cyclerLabel(s string) string {
	return uicore.PadOrTruncate(s, 4)
}

func (f Form) noteRows(cw int) []string {
	prefix := uicore.PadOrTruncate("Note:", labelCol)
	style := f.styles.FieldBlur
	if f.focusedKind() == widNote {
		style = f.styles.FieldFocus
	}
	view := f.note.View()
	lines := strings.Split(view, "\n")
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		var p string
		if i == 0 {
			p = prefix + " "
		} else {
			p = strings.Repeat(" ", labelCol+1)
		}
		out = append(out, uicore.PadOrTruncate(p+style.Render(l), cw))
	}
	return out
}

func (f Form) saveToRow(cw int) string {
	prefix := uicore.PadOrTruncate("Save to:", labelCol)
	parts := make([]string, 0, len(f.saveTo))
	for i, dest := range f.saveTo {
		marker := "○ "
		if i == f.saveIdx {
			marker = "● "
		}
		seg := marker + dest
		if i == f.saveIdx {
			seg = f.styles.KindOn.Render(seg)
		} else {
			seg = f.styles.KindOff.Render(seg)
		}
		parts = append(parts, seg)
	}
	body := prefix + " " + strings.Join(parts, "   ")
	if f.focusedKind() == widSaveTo {
		body = f.styles.FieldFocus.Render("▸ ") + body
	} else {
		body = "  " + body
	}
	return uicore.PadOrTruncate(body, cw)
}
