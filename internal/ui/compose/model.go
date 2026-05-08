package compose

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gomail "github.com/emersion/go-message/mail"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/google/uuid"
)

// CacheStore is the subset of cache.Account compose needs. Defined here
// so tests can inject a fake without importing internal/cache.
type CacheStore interface {
	CreateDraft(ctx context.Context, draftID string, payload []byte) error
	UpdateDraft(ctx context.Context, draftID string, payload []byte) error
	LoadDraft(ctx context.Context, draftID string) ([]byte, error)
}

// Model is the inline compose surface. Send and discard surface as
// tea.Msg values that App translates into cache ops.
type Model struct {
	styles Styles

	from string

	to      textinput.Model
	cc      textinput.Model
	bcc     textinput.Model
	subject textinput.Model
	editor  mailcompose.Editor

	focus int
	err   string

	width   int
	height  int
	divider string

	cache         CacheStore
	draftID       string
	draftsFolder  string
	prevServerUID string
	localDirty    bool
	pushDirty     bool
	lastEditAt    time.Time

	suggest Dropdown
}

type autosaveTickMsg struct{}
type serverPushTickMsg struct{}

const (
	autosaveDelay   = 1 * time.Second
	serverPushDelay = 5 * time.Minute
)

const (
	focusTo = iota
	focusCc
	focusBcc
	focusSubject
	focusBody
)

// labelWidth fits "Subject:" (8 cells) plus a separating space.
const labelWidth = 9

// chromeRows counts the 5 headers plus the divider; the error banner
// adds one more when set.
const chromeRows = 6

func newModel(styles Styles, self string, suggest SuggestFn) *Model {
	mk := func() textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = ""
		return ti
	}
	c := &Model{
		styles:  styles,
		from:    self,
		to:      mk(),
		cc:      mk(),
		bcc:     mk(),
		subject: mk(),
		editor:  mailcompose.NewCatkinEditor(),
		suggest: NewDropdown(suggest).WithStyles(styles),
	}
	c.to.Focus()
	c.focus = focusTo
	return c
}

func New(styles Styles, self string, suggest SuggestFn) *Model {
	c := newModel(styles, self, suggest)
	c.draftID = uuid.NewString()
	return c
}

// Open returns a Model wired to an existing draftID, pre-seeded with d.
// Both dirty flags start clear because the cache and server images match.
func Open(styles Styles, self string, draftID string, d mailcompose.Draft, suggest SuggestFn) *Model {
	c := newModel(styles, self, suggest)
	c.draftID = draftID
	c.Seed(d)
	return c
}

// SetCache wires the autosave store. When set, Init seeds an empty row
// so the draft appears in ListDrafts immediately.
func (c *Model) SetCache(cache CacheStore) { c.cache = cache }

func (c *Model) DraftID() string { return c.draftID }

// SetDraftTarget records the Drafts folder and the last known server UID
// for the push path.
func (c *Model) SetDraftTarget(folder, prevUID string) {
	c.draftsFolder = folder
	c.prevServerUID = prevUID
}

func (c *Model) scheduleAutosaveCmd() tea.Cmd {
	return tea.Tick(autosaveDelay, func(time.Time) tea.Msg { return autosaveTickMsg{} })
}

func (c *Model) scheduleServerPushCmd() tea.Cmd {
	return tea.Tick(serverPushDelay, func(time.Time) tea.Msg { return serverPushTickMsg{} })
}

func (c *Model) createDraftCmd() tea.Cmd {
	id := c.draftID
	cache := c.cache
	d := c.currentDraft()
	return func() tea.Msg {
		payload, err := mailcompose.EncodeDraft(d)
		if err != nil {
			return uicore.ErrorMsg{Op: "encode draft", Err: err}
		}
		if err := cache.CreateDraft(context.Background(), id, payload); err != nil {
			return uicore.ErrorMsg{Op: "save draft", Err: err}
		}
		return DraftPersistedMsg{DraftID: id}
	}
}

// updateDraftCmd persists the autosave snapshot. UPDATE-only so a deleted
// row makes the in-flight cmd a benign zero-row no-op.
func (c *Model) updateDraftCmd() tea.Cmd {
	id := c.draftID
	cache := c.cache
	d := c.currentDraft()
	return func() tea.Msg {
		payload, err := mailcompose.EncodeDraft(d)
		if err != nil {
			return uicore.ErrorMsg{Op: "encode draft", Err: err}
		}
		if err := cache.UpdateDraft(context.Background(), id, payload); err != nil {
			return uicore.ErrorMsg{Op: "save draft", Err: err}
		}
		return DraftPersistedMsg{DraftID: id}
	}
}

// currentDraft snapshots the current inputs as a Draft without address
// validation; partial input is normal during editing.
func (c *Model) currentDraft() mailcompose.Draft {
	return mailcompose.Draft{
		From:    gomail.Address{Address: c.from},
		Subject: c.subject.Value(),
		Body:    c.editor.Value(),
	}
}

func (c *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		c.editor.Init(),
		c.scheduleAutosaveCmd(),
		c.scheduleServerPushCmd(),
	}
	if c.cache != nil {
		cmds = append(cmds, c.createDraftCmd())
	}
	return tea.Batch(cmds...)
}

func (c *Model) SetSize(w, h int) {
	c.width = w
	c.height = h
	c.divider = strings.Repeat("─", w)

	inputW := w - labelWidth - 1
	if inputW < 1 {
		inputW = 1
	}
	c.to.Width = inputW
	c.cc.Width = inputW
	c.bcc.Width = inputW
	c.subject.Width = inputW

	bodyHeight := h - chromeRows
	if c.err != "" {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	c.editor.SetSize(w, bodyHeight)
}

// View enforces the size contract: every line is exactly c.width cells
// and the output is exactly c.height rows.
func (c *Model) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	rows := make([]string, 0, c.height)
	rows = append(rows, c.headerRow("From:", c.from))
	addrFields := []struct {
		label string
		view  string
		focus int
	}{
		{"To:", c.to.View(), focusTo},
		{"Cc:", c.cc.View(), focusCc},
		{"Bcc:", c.bcc.View(), focusBcc},
	}
	showDropdown := c.dropdownActive()
	var dropRows []string
	if showDropdown {
		dropRows = c.dropdownRows()
	}
	for _, f := range addrFields {
		rows = append(rows, c.headerRow(f.label, f.view))
		if showDropdown && c.focus == f.focus {
			rows = append(rows, dropRows...)
		}
	}
	rows = append(rows, c.headerRow("Subject:", c.subject.View()))
	if c.err != "" {
		rows = append(rows, c.padRow(c.styles.ErrorBanner.Render(c.err)))
	}
	rows = append(rows, c.padRow(c.divider))
	for _, line := range strings.Split(c.editor.View(), "\n") {
		rows = append(rows, c.padRow(line))
	}
	for len(rows) < c.height {
		rows = append(rows, c.padRow(""))
	}
	if len(rows) > c.height {
		rows = rows[:c.height]
	}
	return strings.Join(rows, "\n")
}

// dropdownRows pads each suggest line to c.width so splicing into View
// preserves the width contract.
func (c *Model) dropdownRows() []string {
	view := c.suggest.View()
	if view == "" {
		return nil
	}
	lines := strings.Split(view, "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = c.padRow(l)
	}
	return out
}

func (c *Model) headerRow(label, value string) string {
	pad := labelWidth - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}
	return c.padRow(label + strings.Repeat(" ", pad) + value)
}

func (c *Model) padRow(s string) string {
	w := lipgloss.Width(s)
	if w >= c.width {
		return truncate(s, c.width)
	}
	return s + strings.Repeat(" ", c.width-w)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	cells := 0
	for i, r := range s {
		w := lipgloss.Width(string(r))
		if cells+w > n {
			return s[:i]
		}
		cells += w
	}
	return s
}

// SendMsg fires on Ctrl+X with a valid draft. App assembles MIME and
// queues the outbox op.
type SendMsg struct {
	Draft mailcompose.Draft
}

// CancelMsg fires on Ctrl+C. App opens a discard ConfirmModal when
// Dirty; clean drafts close immediately.
type CancelMsg struct {
	Dirty bool
}

func (c *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autosaveTickMsg:
		if c.cache != nil && c.localDirty && time.Since(c.lastEditAt) >= autosaveDelay {
			c.localDirty = false
			return c, tea.Batch(c.updateDraftCmd(), c.scheduleAutosaveCmd())
		}
		return c, c.scheduleAutosaveCmd()

	case serverPushTickMsg:
		if c.pushDirty && c.draftsFolder != "" {
			c.pushDirty = false
			folder := c.draftsFolder
			prevUID := c.prevServerUID
			id := c.draftID
			d := c.currentDraft()
			return c, tea.Batch(
				func() tea.Msg {
					mime, err := mailcompose.AssembleMIME(d, time.Now())
					if err != nil {
						// Partial drafts can fail to assemble; the next tick retries.
						return nil
					}
					return EnqueuePushDraftMsg{
						DraftID:       id,
						Folder:        folder,
						MIME:          mime,
						PrevServerUID: prevUID,
					}
				},
				c.scheduleServerPushCmd(),
			)
		}
		return c, c.scheduleServerPushCmd()

	case tea.KeyMsg:
		if c.dropdownActive() {
			switch msg.Type {
			case tea.KeyTab, tea.KeyEnter:
				c.acceptSuggestion()
				return c, nil
			case tea.KeyEsc:
				c.suggest = c.suggest.Clear()
				return c, nil
			case tea.KeyUp, tea.KeyDown:
				c.suggest, _ = c.suggest.Update(msg)
				return c, nil
			}
		}
		switch msg.Type {
		case tea.KeyCtrlX:
			d, err := c.Draft()
			if err != nil {
				return c, nil
			}
			return c, func() tea.Msg { return SendMsg{Draft: d} }
		case tea.KeyCtrlC:
			dirty := c.IsDirty()
			return c, func() tea.Msg { return CancelMsg{Dirty: dirty} }
		case tea.KeyTab:
			c.advanceFocus(+1)
			return c, nil
		case tea.KeyShiftTab:
			c.advanceFocus(-1)
			return c, nil
		case tea.KeyEsc:
			if c.focus == focusBody {
				c.setFocus(focusSubject)
			} else {
				c.setFocus(focusBody)
			}
			return c, nil
		}
	}

	var cmd tea.Cmd
	switch c.focus {
	case focusTo:
		c.to, cmd = c.to.Update(msg)
	case focusCc:
		c.cc, cmd = c.cc.Update(msg)
	case focusBcc:
		c.bcc, cmd = c.bcc.Update(msg)
	case focusSubject:
		c.subject, cmd = c.subject.Update(msg)
	case focusBody:
		c.editor, cmd = c.editor.Update(msg)
	}

	if isEditMsg(msg) {
		c.localDirty = true
		c.pushDirty = true
		c.lastEditAt = time.Now()
	}

	if _, isKey := msg.(tea.KeyMsg); isKey {
		c.refreshSuggest()
	}
	return c, cmd
}

// dropdownActive reports whether the dropdown should consume navigation
// keys.
func (c *Model) dropdownActive() bool {
	if c.suggest.Empty() {
		return false
	}
	switch c.focus {
	case focusTo, focusCc, focusBcc:
		return true
	}
	return false
}

func (c *Model) refreshSuggest() {
	ti := c.focusedAddrField()
	if ti == nil {
		c.suggest = c.suggest.Clear()
		return
	}
	c.suggest = c.suggest.SetPrefix(trailingFragment(ti.Value()))
}

func (c *Model) focusedAddrField() *textinput.Model {
	switch c.focus {
	case focusTo:
		return &c.to
	case focusCc:
		return &c.cc
	case focusBcc:
		return &c.bcc
	}
	return nil
}

// trailingFragment returns the text after the last comma, with leading
// whitespace trimmed. The caller is typing into this fragment; earlier
// addresses are already committed.
func trailingFragment(s string) string {
	if i := strings.LastIndex(s, ","); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimLeft(s, " \t")
}

// acceptSuggestion rewrites the focused field's trailing fragment as
// `Name <email>, ` so the next address types in cleanly.
func (c *Model) acceptSuggestion() {
	sel, ok := c.suggest.Selected()
	if !ok {
		return
	}
	ti := c.focusedAddrField()
	if ti == nil {
		return
	}
	prefix := ""
	if i := strings.LastIndex(ti.Value(), ","); i >= 0 {
		prefix = ti.Value()[:i+1] + " "
	}
	rendered := fmt.Sprintf("%s <%s>, ", sel.Name, sel.Email)
	ti.SetValue(prefix + rendered)
	ti.CursorEnd()
	c.suggest = c.suggest.Clear()
	c.localDirty = true
	c.pushDirty = true
	c.lastEditAt = time.Now()
}

// isEditMsg reports whether msg should mark the draft dirty. Navigation
// and control messages (Tab, Esc, Ctrl chords) are excluded.
func isEditMsg(msg tea.Msg) bool {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch k.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyEnter, tea.KeyBackspace, tea.KeyDelete:
		return true
	}
	return false
}

func (c *Model) advanceFocus(delta int) {
	const fields = 5
	c.setFocus(((c.focus + delta) + fields) % fields)
}

func (c *Model) setFocus(target int) {
	c.to.Blur()
	c.cc.Blur()
	c.bcc.Blur()
	c.subject.Blur()
	c.editor.Blur()
	switch target {
	case focusTo:
		_ = c.to.Focus()
	case focusCc:
		_ = c.cc.Focus()
	case focusBcc:
		_ = c.bcc.Focus()
	case focusSubject:
		_ = c.subject.Focus()
	case focusBody:
		_ = c.editor.Focus()
	}
	c.focus = target
}

// Draft rebuilds a mailcompose.Draft from the current inputs. Address
// parse errors are written to c.err and also returned.
func (c *Model) Draft() (mailcompose.Draft, error) {
	to, err := parseAddrField(c.to.Value(), "To")
	if err != nil {
		c.err = err.Error()
		return mailcompose.Draft{}, err
	}
	cc, err := parseAddrField(c.cc.Value(), "Cc")
	if err != nil {
		c.err = err.Error()
		return mailcompose.Draft{}, err
	}
	bcc, err := parseAddrField(c.bcc.Value(), "Bcc")
	if err != nil {
		c.err = err.Error()
		return mailcompose.Draft{}, err
	}
	c.err = ""
	return mailcompose.Draft{
		From:    gomail.Address{Address: c.from},
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: c.subject.Value(),
		Body:    c.editor.Value(),
	}, nil
}

func (c *Model) SetErr(msg string) {
	c.err = msg
}

func (c *Model) Err() string { return c.err }

func (c *Model) SetTo(s string) { c.to.SetValue(s) }

func (c *Model) SetSubject(s string) { c.subject.SetValue(s) }

func (c *Model) SubjectValue() string { return c.subject.Value() }

func (c *Model) SetBody(s string) { c.editor.SetValue(s) }

func (c *Model) IsDirty() bool {
	return c.to.Value() != "" || c.cc.Value() != "" || c.bcc.Value() != "" ||
		c.subject.Value() != "" || c.editor.Value() != ""
}

// CurrentDraft returns the draft built from current inputs without
// validation. Use Draft for the send path where addresses must parse.
func (c *Model) CurrentDraft() mailcompose.Draft { return c.currentDraft() }

// HasContent gates the save-on-close path: an empty compose opened for
// a Drafts-row can be discarded silently without a confirm.
func (c *Model) HasContent() bool {
	d := c.currentDraft()
	return len(d.To) > 0 || len(d.Cc) > 0 || len(d.Bcc) > 0 ||
		d.Subject != "" || d.Body != "" || len(d.Attachments) > 0
}

// PrevServerUID returns the server UID of the last pushed image, or ""
// if the draft has never been pushed. App queues a Destroy on the stale
// server copy when discarding or sending.
func (c *Model) PrevServerUID() string { return c.prevServerUID }

// AllocDraftID returns a fresh draft UUID for reconstructing a server-
// side draft that has no matching local row.
func AllocDraftID() string { return uuid.NewString() }

func (c *Model) Seed(d mailcompose.Draft) {
	c.to.SetValue(joinAddresses(d.To))
	c.cc.SetValue(joinAddresses(d.Cc))
	c.bcc.SetValue(joinAddresses(d.Bcc))
	c.subject.SetValue(d.Subject)
	c.editor.SetValue(d.Body)
}

func parseAddrField(raw, label string) ([]gomail.Address, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	addrs, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	out := make([]gomail.Address, len(addrs))
	for i, a := range addrs {
		out[i] = gomail.Address{Name: a.Name, Address: a.Address}
	}
	return out, nil
}

func joinAddresses(addrs []gomail.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a.Name != "" {
			parts = append(parts, fmt.Sprintf("%q <%s>", a.Name, a.Address))
		} else {
			parts = append(parts, a.Address)
		}
	}
	return strings.Join(parts, ", ")
}
