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

// CacheStore is the subset of cache.Account compose needs. The
// interface lives here so model_test.go can inject a fake without
// importing internal/cache.
type CacheStore interface {
	CreateDraft(ctx context.Context, draftID string, payload []byte) error
	UpdateDraft(ctx context.Context, draftID string, payload []byte) error
	LoadDraft(ctx context.Context, draftID string) ([]byte, error)
}

// Model is the inline compose surface. Send and discard surface
// as tea.Msg values that App translates into cache ops.
type Model struct {
	styles Styles

	from string

	to      textinput.Model
	cc      textinput.Model
	bcc     textinput.Model
	subject textinput.Model
	editor  mailcompose.Editor

	focus int
	// err is rendered as a one-line banner above the divider when set.
	err string

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

// labelWidth is the column reserved for header labels. "Subject:"
// is 8 cells, plus one space of separation.
const labelWidth = 9

// chromeRows is the fixed row count above the body: 5 headers
// plus the divider rule. The error banner adds one more when set.
const chromeRows = 6

func newModel(styles Styles, self string) *Model {
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
	}
	c.to.Focus()
	c.focus = focusTo
	return c
}

// New returns a fresh, empty Model with focus on To.
func New(styles Styles, self string) *Model {
	c := newModel(styles, self)
	c.draftID = uuid.NewString()
	return c
}

// Open returns a Model wired to an existing draftID, with d pre-seeded.
// localDirty and pushDirty start cleared because the draft is already in
// the cache and the server image is current.
func Open(styles Styles, self string, draftID string, d mailcompose.Draft) *Model {
	c := newModel(styles, self)
	c.draftID = draftID
	c.Seed(d)
	return c
}

// SetCache wires the autosave store. When set, Init seeds an empty row
// so the draft appears in ListDrafts immediately.
func (c *Model) SetCache(cache CacheStore) { c.cache = cache }

// DraftID returns the UUID that identifies this draft in the cache.
func (c *Model) DraftID() string { return c.draftID }

// SetDraftTarget records the Drafts folder name and the last known
// server UID for the push path. Callers supply these from the
// classified-folder list and the cache DraftRow.
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

// createDraftCmd inserts the row on compose Init.
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

// updateDraftCmd persists the autosave snapshot. UPDATE-only: a
// deleted row makes the in-flight cmd a benign 0-row no-op.
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
// validation. Partial input is normal during editing.
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

// View enforces the size contract: every returned line is exactly
// c.width display cells and the result is exactly c.height rows. The
// parent joins panes without defensive clipping.
func (c *Model) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	rows := make([]string, 0, c.height)
	rows = append(rows, c.headerRow("From:", c.from))
	rows = append(rows, c.headerRow("To:", c.to.View()))
	rows = append(rows, c.headerRow("Cc:", c.cc.View()))
	rows = append(rows, c.headerRow("Bcc:", c.bcc.View()))
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

// truncate clips s to n display cells.
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

// SendMsg is emitted when the user presses Ctrl+X with a valid
// draft. App assembles MIME and queues the outbox op.
type SendMsg struct {
	Draft mailcompose.Draft
}

// CancelMsg is emitted when the user presses Ctrl+C. App opens
// a discard ConfirmModal when Dirty. Clean drafts close immediately.
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
						// AssembleMIME on a partial draft can fail cleanly;
						// the next tick will retry.
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

	return c, cmd
}

// isEditMsg reports whether msg represents user content input that
// should mark the draft dirty. Navigation and control messages
// (Tab, Esc, Ctrl chords) are excluded.
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
// fields are parsed via net/mail; a parse error is stored in the
// inline err row and returned to the caller.
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

// IsDirty reports whether any input field contains user-entered content.
func (c *Model) IsDirty() bool {
	return c.to.Value() != "" || c.cc.Value() != "" || c.bcc.Value() != "" ||
		c.subject.Value() != "" || c.editor.Value() != ""
}

// CurrentDraft returns the draft built from the current inputs without
// address validation. Partial input (incomplete addresses, empty fields)
// is preserved as-is. Use Draft for the send path where valid addresses
// are required.
func (c *Model) CurrentDraft() mailcompose.Draft { return c.currentDraft() }

// HasContent reports whether any meaningful content has been entered,
// including attachment paths (checked on Draft). It mirrors IsDirty but
// is the gate for the save-on-close path: an empty compose opened for a
// Drafts-row can be discarded silently without a confirm.
func (c *Model) HasContent() bool {
	d := c.currentDraft()
	return len(d.To) > 0 || len(d.Cc) > 0 || len(d.Bcc) > 0 ||
		d.Subject != "" || d.Body != "" || len(d.Attachments) > 0
}

// PrevServerUID returns the server UID of the last pushed image, or ""
// if the draft has never been pushed. App uses this to queue a Destroy
// on the stale server copy when discarding or sending.
func (c *Model) PrevServerUID() string { return c.prevServerUID }

// AllocDraftID returns a fresh draft UUID. App calls this when
// reconstructing a server-side draft that has no matching local row.
func AllocDraftID() string { return uuid.NewString() }

// Seed populates the inputs from d.
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
