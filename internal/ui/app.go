// SPDX-License-Identifier: MIT

package ui

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/helppopover"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/sidebar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// pendingEmptyConfirm carries the parameters App needs to emit
// EmptyFolderConfirmedMsg when the user accepts the active confirm
// modal. Zero value (empty folder) means no empty-folder confirm is
// pending.
type pendingEmptyConfirm struct {
	folder string
	source string
}

// TidyFn rewrites the markdown body before MIME assembly. The default
// is identity passthrough. Callers swap in a real implementation.
type TidyFn func(ctx context.Context, body string) (string, error)

func identityTidy(_ context.Context, body string) (string, error) {
	return body, nil
}

// App is the root bubbletea model for poplar.
type App struct {
	acct            account.Model
	icons           uicore.IconSet
	styles          Styles
	topLine         TopLine
	statusBar       StatusBar
	footer          Footer
	keys            GlobalKeys
	viewerOpen      bool
	helpOpen        bool
	help            helppopover.Model
	linkPicker      reader.LinkPicker
	attachPicker    reader.AttachPicker
	movePicker      movepicker.Model
	downloadDir     string
	confirm         ConfirmModal
	pendingEmpty    pendingEmptyConfirm
	outbox          OutboxOverlay
	outboxOpen      bool
	conflict        ConflictOverlay
	conflictOpen    bool
	lastOutboxDepth cache.OutboxDepth
	offlineHinted   bool
	lastErr         ErrorMsg
	toast           pendingAction
	undoSeconds     int
	// now returns the wall clock. Test seam, defaults to time.Now.
	now func() time.Time
	// opener launches URLs. Test seam, defaults to xdgOpenURL.
	opener URLOpener
	// tidy rewrites the markdown body before MIME assembly. Test seam,
	// defaults to identityTidy.
	tidy               TidyFn
	theme              *theme.CompiledTheme
	compose            *uicompose.Model
	pendingComposeSave bool // Save? modal is open for a dirty compose
	popover            *contacts.Popover
	contactsMode       bool
	contactsSidebar    contacts.Sidebar
	contactsList       contacts.List
	contactsStyles     contacts.Styles
	form               *contacts.Form
	pendingFormDiscard bool
	width              int
	height             int
}

// WithOpener returns a copy of m with the URL opener replaced.
func (m App) WithOpener(opener URLOpener) App {
	m.opener = opener
	return m
}

// WithTidy returns a copy of m with the body tidy seam replaced.
func (m App) WithTidy(fn TidyFn) App {
	m.tidy = fn
	return m
}

// NewApp creates the root model with a single account.Model. Folder
// loading happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons uicore.IconSet) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	cStyles := contacts.NewStyles(t)
	cFixtures := contacts.Fixtures()
	return App{
		acct:            account.New(t, acct, uiCfg, icons),
		icons:           icons,
		styles:          styles,
		theme:           t,
		topLine:         NewTopLine(styles),
		statusBar:       sb,
		footer:          NewFooter(styles),
		keys:            NewGlobalKeys(),
		linkPicker:      reader.NewLinkPicker(reader.NewStyles(t)),
		attachPicker:    reader.NewAttachPicker(reader.NewStyles(t), icons),
		movePicker:      movepicker.New(movepicker.NewStyles(t)),
		downloadDir:     uiCfg.DownloadDir,
		confirm:         NewConfirmModal(styles),
		outbox:          NewOutboxOverlay(styles),
		conflict:        NewConflictOverlay(styles),
		undoSeconds:     uiCfg.UndoSeconds,
		now:             time.Now,
		opener:          xdgOpenURL,
		tidy:            identityTidy,
		contactsStyles:  cStyles,
		contactsSidebar: contacts.NewSidebar(cStyles, cFixtures),
		contactsList:    contacts.NewList(cStyles, cFixtures, contacts.SortFirstName),
	}
}

// Init delegates to the account tab so the initial folder fetch fires,
// and starts the backend update pump.
func (m App) Init() tea.Cmd {
	return tea.Batch(m.acct.Init(), pumpUpdatesCmd(m.acct.Backend()))
}

// deriveChromeFromAcct re-reads AccountTab state and propagates it
// to App-owned chrome (footer, status bar, viewerOpen, linkPicker).
func (m App) deriveChromeFromAcct() App {
	prevViewer := m.viewerOpen
	m.viewerOpen = m.acct.ViewerOpen()
	exists, unseen := m.acct.SelectedFolderCounts()
	m.statusBar = m.statusBar.SetCounts(exists, unseen)
	if m.viewerOpen {
		if !prevViewer {
			m.footer = m.footer.SetContext(ViewerContext)
			m.statusBar = m.statusBar.SetMode(StatusViewer).SetScrollPct(0)
		} else {
			m.statusBar = m.statusBar.SetScrollPct(m.acct.ViewerScrollPct())
		}
	} else if prevViewer {
		m.footer = m.footer.SetContext(AccountContext)
		m.statusBar = m.statusBar.SetMode(StatusAccount)
	}
	m.footer = m.footer.SetCounter(m.acct.WindowCounter())
	return m
}

// Update handles global keys and delegates everything else to the
// account tab. Chrome (footer, status bar, link picker) is derived
// by reading AccountTab accessors after each delegation.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var cmds []tea.Cmd
		var cmd tea.Cmd
		m.linkPicker = m.linkPicker.SetSize(m.width, m.height)
		m.linkPicker, cmd = m.linkPicker.Update(msg)
		cmds = append(cmds, cmd)
		m.attachPicker = m.attachPicker.SetSize(m.width, m.height)
		m.attachPicker, cmd = m.attachPicker.Update(msg)
		cmds = append(cmds, cmd)
		m.movePicker = m.movePicker.SetSize(m.width, m.height)
		m.movePicker, cmd = m.movePicker.Update(msg)
		cmds = append(cmds, cmd)
		m.confirm = m.confirm.SetSize(m.width, m.height)
		m.confirm, cmd = m.confirm.Update(msg)
		cmds = append(cmds, cmd)
		m.outbox = m.outbox.SetSize(m.width, m.height)
		m.conflict = m.conflict.SetSize(m.width, m.height)
		m.help = m.help.SetSize(m.width, m.height)
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		m.acct, cmd = m.acct.Update(contentMsg)
		cmds = append(cmds, cmd)
		if m.compose != nil {
			w, h := m.rightPaneSize()
			m.compose.SetSize(w, h)
		}
		if m.contactsMode {
			m.contactsSidebar, m.contactsList = m.sizedContactsChildren()
		}
		if m.form != nil {
			w, h := m.formSize(m.form.FromPopover())
			next := m.form.SetSize(w, h)
			m.form = &next
		}
		// WindowSizeMsg only forwards sizing. Chrome derivation is not
		// needed (sizing alone does not change viewer open/close state
		// or folder counts).
		return m, tea.Batch(cmds...)

	case reader.OpenLinkPickerMsg:
		m.linkPicker = m.linkPicker.Open(msg.Links)
		return m, nil

	case reader.LinkPickerClosedMsg:
		m.linkPicker = m.linkPicker.Close()
		return m, nil

	case movepicker.OpenMsg:
		m.movePicker = m.movePicker.Open(msg.UIDs, msg.Src, msg.Folders)
		return m, nil

	case movepicker.ClosedMsg:
		m.movePicker = m.movePicker.Close()
		return m, nil

	case movepicker.PickedMsg:
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		m = m.deriveChromeFromAcct()
		return m, cmd

	case reader.OpenAttachPickerMsg:
		m.attachPicker = m.attachPicker.Open(msg.UID, msg.Items)
		return m, nil

	case reader.AttachPickerClosedMsg:
		m.attachPicker = m.attachPicker.Close()
		return m, nil

	case reader.OpenAttachmentMsg:
		return m, openAttachmentCmd(m.acct.Cache(), m.opener, msg.UID, msg.Att)

	case reader.SaveAttachmentMsg:
		return m, saveAttachmentCmd(m.acct.Cache(), m.downloadDir, msg.UID, msg.Att)

	case contacts.OpenPopoverMsg:
		p := contacts.NewPopover(m.contactsStyles)
		p.SetSize(m.width, m.height)
		match, found := contacts.LookupByEmail(contacts.Fixtures(), msg.Email)
		p.SetMatch(msg.DisplayName, msg.Email, match, found)
		m.popover = &p
		return m, nil

	case contacts.ClosePopoverMsg:
		m.popover = nil
		return m, nil

	case contacts.OpenFormMsg:
		m.popover = nil
		saveTo := []string{"Local file"}
		if email := m.acct.AccountEmail(); email != "" {
			saveTo = append(saveTo, email)
		}
		f := contacts.NewForm(m.contactsStyles, msg.Initial, msg.FromPopover, saveTo)
		w, h := m.formSize(msg.FromPopover)
		f = f.SetSize(w, h)
		m.form = &f
		return m, nil

	case contacts.ContactSaveMsg:
		m.form = nil
		return m, nil

	case contacts.ContactCancelMsg:
		if m.form == nil {
			return m, nil
		}
		if !msg.Dirty {
			m.form = nil
			return m, nil
		}
		m.pendingFormDiscard = true
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Discard changes?",
			Body:  "Unsaved edits to this contact will be lost.",
		})
		return m, nil

	case contacts.EnterContactsModeMsg:
		m.contactsMode = true
		m.contactsSidebar, m.contactsList = m.sizedContactsChildren()
		return m, nil

	case contacts.ExitContactsModeMsg:
		m.contactsMode = false
		return m, nil

	case reader.AttachmentSavedMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m.toast = pendingAction{
			op:       opSaveAttachment,
			dest:     msg.Path,
			deadline: deadline,
		}
		cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
			return toastExpireMsg{deadline: deadline}
		})}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case account.OpenConfirmEmptyMsg:
		body := strconv.Itoa(msg.Total) + " messages will be permanently deleted."
		m.pendingEmpty = pendingEmptyConfirm{folder: msg.Folder, source: msg.Source}
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Empty " + msg.Folder,
			Body:  body,
		})
		return m, nil

	case ConfirmModalYesMsg:
		switch {
		case m.pendingFormDiscard:
			m.pendingFormDiscard = false
			m.form = nil
			return m, nil
		case m.pendingComposeSave:
			m.pendingComposeSave = false
			if m.compose == nil {
				return m, nil
			}
			// Save path: persist current draft and queue a server push.
			draftsFolder := resolveDraftsFolder(m.acct.Cache())
			d := m.compose.CurrentDraft()
			draftID := m.compose.DraftID()
			prevUID := mail.UID(m.compose.PrevServerUID())
			m.compose = nil
			if draftsFolder != "" {
				return m, upsertAndPushDraftCmd(m.acct.Cache(), draftID, draftsFolder, d, prevUID)
			}
			return m, nil
		case m.pendingEmpty.folder != "":
			folder, source := m.pendingEmpty.folder, m.pendingEmpty.source
			m.pendingEmpty = pendingEmptyConfirm{}
			return m, func() tea.Msg {
				return account.EmptyFolderConfirmedMsg{Folder: folder, Source: source}
			}
		}
		return m, nil

	case ConfirmModalNoMsg:
		if m.pendingFormDiscard {
			m.pendingFormDiscard = false
			return m, nil
		}
		if m.pendingComposeSave {
			m.pendingComposeSave = false
			if m.compose != nil {
				draftID := m.compose.DraftID()
				prevUID := mail.UID(m.compose.PrevServerUID())
				draftsFolder := resolveDraftsFolder(m.acct.Cache())
				m.compose = nil
				return m, discardDraftCmd(m.acct.Cache(), draftID, draftsFolder, prevUID)
			}
		}
		return m, nil

	case ConfirmModalClosedMsg:
		// Esc on the discard-changes modal keeps the form mounted.
		if m.pendingFormDiscard {
			m.pendingFormDiscard = false
			m.confirm = m.confirm.Close()
			return m, nil
		}
		// When the save-draft modal was Esc'd, keep compose mounted.
		if m.pendingComposeSave {
			m.pendingComposeSave = false
			m.confirm = m.confirm.Close()
			return m, nil
		}
		m.pendingEmpty = pendingEmptyConfirm{}
		m.confirm = m.confirm.Close()
		return m, nil

	case account.EmptyFolderConfirmedMsg:
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		m = m.deriveChromeFromAcct()
		return m, cmd

	case reader.LaunchURLMsg:
		return m, launchURLCmd(m.opener, msg.URL)

	case account.TriageStartedMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m.toast = pendingAction{
			op:       msg.Op,
			n:        msg.N,
			dest:     msg.Dest,
			inverse:  msg.Inverse,
			deadline: deadline,
		}
		cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
			return toastExpireMsg{deadline: deadline}
		})}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case toastExpireMsg:
		if m.toast.IsZero() || !msg.deadline.Equal(m.toast.deadline) {
			return m, nil
		}
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m, rcmd := m.maybeResizeChild(hadBanner)
		return m, rcmd

	case undoRequestedMsg:
		if m.toast.IsZero() {
			return m, nil
		}
		cmd := m.toast.inverse
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		cmds := []tea.Cmd{}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case ErrorMsg:
		// Errors clear any pending toast. The cache holds the
		// optimistic flip and the user must fire u to revert.
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m.lastErr = msg
		cmds := make([]tea.Cmd, 0, 2)
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		m = m.deriveChromeFromAcct()
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...)

	case account.FolderLoadedMsg:
		// A fresh folder load (msglist reset by selectionChangedCmds)
		// commits any in-flight toast.
		if !m.toast.IsZero() && m.acct.MessageListCount() == 0 {
			hadBanner := m.hasBannerRow()
			m.toast = pendingAction{}
			var rcmd tea.Cmd
			m, rcmd = m.maybeResizeChild(hadBanner)
			acct, fcmd := m.acct.Update(msg)
			m.acct = acct
			m = m.deriveChromeFromAcct()
			cmds := []tea.Cmd{fcmd}
			if rcmd != nil {
				cmds = append(cmds, rcmd)
			}
			return m, tea.Batch(cmds...)
		}

	case backendUpdateMsg:
		cmds := []tea.Cmd{pumpUpdatesCmd(m.acct.Backend())} // re-arm pump
		if msg.update.Type == mail.UpdateConnState {
			var cs ConnectionState
			switch msg.update.ConnState {
			case mail.ConnConnected:
				cs = Connected
				m.offlineHinted = false
			case mail.ConnReconnecting:
				cs = Reconnecting
			default:
				cs = Offline
				if !m.offlineHinted {
					d := m.lastOutboxDepth
					if d.Pending+d.Executing+d.Failed+d.Conflict > 0 {
						m.lastErr = ErrorMsg{
							Op:  "connection",
							Err: errors.New("offline — queued ops will sync on reconnect"),
						}
						m.offlineHinted = true
					}
				}
			}
			m.statusBar = m.statusBar.SetConnectionState(cs)
		}
		// Other Update types (UpdateNewMail, UpdateFlagsChanged, etc.)
		// delegate to AccountTab in a later pass.
		return m, tea.Batch(cmds...)

	case account.CacheEventMsg:
		cmds := []tea.Cmd{refreshOutboxDepthCmd(m.acct.Cache())}
		if m.outboxOpen {
			cmds = append(cmds, loadOutboxSummaryCmd(m.acct.Cache()))
		}
		if m.conflictOpen {
			cmds = append(cmds, loadOutboxConflictsCmd(m.acct.Cache()))
		}
		if msg.Event.Note != "" {
			m.lastErr = ErrorMsg{Op: "draft", Err: errors.New(msg.Event.Note)}
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...)

	case OpenConflictsFromOutboxMsg:
		m.outboxOpen = false
		m.conflictOpen = true
		m.conflict = m.conflict.Open(nil)
		return m, loadOutboxConflictsCmd(m.acct.Cache())

	case outboxDepthMsg:
		m.lastOutboxDepth = msg.depth
		inflight := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed
		m.statusBar = m.statusBar.SetOutboxDepth(inflight, msg.depth.Conflict)
		return m, nil

	case outboxSummaryMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox summary", Err: msg.err}
			return m, nil
		}
		if m.outboxOpen {
			m.outbox = m.outbox.SetGroups(msg.groups)
		}
		return m, nil

	case outboxConflictsMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox conflicts", Err: msg.err}
			return m, nil
		}
		if m.conflictOpen {
			m.conflict = m.conflict.SetRows(msg.rows)
			if len(msg.rows) == 0 {
				m.conflict = m.conflict.Close()
				m.conflictOpen = false
			}
		}
		return m, nil

	case RetryConflictMsg:
		return m, retryConflictCmd(m.acct.Cache(), msg.OpID)

	case DiscardConflictMsg:
		return m, discardConflictCmd(m.acct.Cache(), msg.OpID)

	case conflictResolvedMsg:
		if msg.err != nil && !errors.Is(msg.err, cache.ErrNotConflict) {
			m.lastErr = ErrorMsg{Op: "resolve conflict", Err: msg.err}
		}
		return m, tea.Batch(loadOutboxConflictsCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache()))

	case uicompose.SendMsg:
		sent := resolveSentFolder(m.acct.Cache())
		if sent == "" {
			if m.compose != nil {
				m.compose.SetErr("no Sent folder configured")
			}
			return m, nil
		}
		d := msg.Draft
		tidy := m.tidy
		acct := m.acct.Cache()
		cmds := []tea.Cmd{composeSendCmd(acct, sent, tidy, d)}
		if m.compose != nil && m.compose.DraftID() != "" {
			draftID := m.compose.DraftID()
			prevUID := mail.UID(m.compose.PrevServerUID())
			draftsFolder := resolveDraftsFolder(acct)
			cmds = append(cmds, discardDraftCmd(acct, draftID, draftsFolder, prevUID))
		}
		m.compose = nil
		return m, tea.Batch(cmds...)

	case uicompose.SentMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(2 * time.Second)
		m.toast = pendingAction{
			op:       opSending,
			deadline: deadline,
		}
		cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
			return toastExpireMsg{deadline: deadline}
		})}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case uicompose.SeededMsg:
		w, h := m.rightPaneSize()
		m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail())
		m.compose.SetSize(w, h)
		m.compose.Seed(msg.Draft)
		return m, m.compose.Init()

	case openDraftMsg:
		// Opened from Drafts-folder Enter. Wire cache/target then open.
		w, h := m.rightPaneSize()
		row := msg.row
		c := uicompose.Open(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), row.DraftID, msg.draft)
		c.SetSize(w, h)
		c.SetCache(m.acct.Cache())
		c.SetDraftTarget(row.ServerFolder, string(row.ServerUID))
		m.compose = c
		return m, m.compose.Init()

	case uicompose.EnqueuePushDraftMsg:
		return m, enqueuePushDraftCmd(m.acct.Cache(), msg.DraftID, msg.Folder, msg.MIME, mail.UID(msg.PrevServerUID))

	case uicompose.CancelMsg:
		if m.compose == nil {
			return m, nil
		}
		if !msg.Dirty {
			m.compose = nil
			return m, nil
		}
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Save draft?",
			Body:  "[y] Save and close   [n] Discard   [Esc] Keep editing",
		})
		m.pendingComposeSave = true
		return m, nil

	case tea.KeyMsg:
		if m.helpOpen {
			if key.Matches(msg, m.keys.CloseHelp) {
				m.helpOpen = false
			}
			return m, nil
		}
		if m.confirm.IsOpen() {
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			return m, cmd
		}
		if m.conflictOpen {
			var cmd tea.Cmd
			m.conflict, cmd = m.conflict.Update(msg)
			if !m.conflict.IsOpen() {
				m.conflictOpen = false
			}
			return m, cmd
		}
		if m.outboxOpen {
			var cmd tea.Cmd
			m.outbox, cmd = m.outbox.Update(msg)
			if !m.outbox.IsOpen() {
				m.outboxOpen = false
			}
			return m, cmd
		}
		if m.linkPicker.IsOpen() {
			var cmd tea.Cmd
			m.linkPicker, cmd = m.linkPicker.Update(msg)
			return m, cmd
		}
		if m.attachPicker.IsOpen() {
			var cmd tea.Cmd
			m.attachPicker, cmd = m.attachPicker.Update(msg)
			return m, cmd
		}
		if m.movePicker.IsOpen() {
			var cmd tea.Cmd
			m.movePicker, cmd = m.movePicker.Update(msg)
			return m, cmd
		}
		if m.form != nil {
			next, cmd := m.form.Update(msg)
			m.form = &next
			return m, cmd
		}
		if m.popover != nil {
			next, cmd := m.popover.Update(msg)
			m.popover = &next
			return m, cmd
		}
		if m.compose != nil {
			next, cmd := m.compose.Update(msg)
			m.compose = next
			return m, cmd
		}
		if m.contactsMode {
			return m.updateContactsKey(msg)
		}
		// Intercept Enter in the Drafts folder to open compose instead of
		// the viewer.
		if msg.Type == tea.KeyEnter && !m.viewerOpen {
			if info, ok := m.acct.SelectedMessage(); ok {
				draftsFolder := resolveDraftsFolder(m.acct.Cache())
				if draftsFolder != "" && m.acct.CurrentFolderName() == draftsFolder {
					if id, ok := draftLocalID(info.UID); ok {
						return m, openLocalDraftCmd(m.acct.Cache(), id)
					}
					return m, openDraftFromServerUIDCmd(m.acct.Cache(), info.UID, draftsFolder)
				}
			}
		}
		switch {
		case key.Matches(msg, m.keys.Compose):
			w, h := m.rightPaneSize()
			m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail())
			m.compose.SetSize(w, h)
			m.compose.SetCache(m.acct.Cache())
			draftsFolder := resolveDraftsFolder(m.acct.Cache())
			m.compose.SetDraftTarget(draftsFolder, "")
			return m, m.compose.Init()
		case key.Matches(msg, m.keys.Reply), key.Matches(msg, m.keys.ReplyAll), key.Matches(msg, m.keys.Forward):
			parent, ok := m.selectedMessage()
			if !ok {
				break
			}
			kind := uicompose.SeedReply
			if key.Matches(msg, m.keys.ReplyAll) {
				kind = uicompose.SeedReplyAll
			} else if key.Matches(msg, m.keys.Forward) {
				kind = uicompose.SeedForward
			}
			return m, composeSeedCmd(m.acct.Cache(), parent, m.acct.AccountEmail(), kind)
		case key.Matches(msg, m.keys.Undo):
			// Undo is only live while a toast is active. Otherwise the
			// 'u' key falls through to AccountTab so other meanings can
			// take over later.
			if !m.toast.IsZero() {
				return m, func() tea.Msg { return undoRequestedMsg{} }
			}
		case key.Matches(msg, m.keys.Quit):
			if m.viewerOpen {
				// Viewer-open: q closes the viewer, not the app.
				// Delegate so AccountTab routes to viewer.handleKey.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(msg)
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			if m.acct.SearchState() != sidebar.SearchIdle {
				// Steal q while search is active so it doesn't quit
				// the app mid-search. Send a typed clear msg to
				// AccountTab.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(sidebar.ClearSearchMsg{})
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.OutboxOverlay):
			m.outboxOpen = true
			m.outbox = m.outbox.Open(nil)
			return m, loadOutboxSummaryCmd(m.acct.Cache())
		case key.Matches(msg, m.keys.ConflictOverlay):
			m.conflictOpen = true
			m.conflict = m.conflict.Open(nil)
			return m, loadOutboxConflictsCmd(m.acct.Cache())
		case key.Matches(msg, m.keys.Help):
			m.helpOpen = true
			ctx := helppopover.Account
			if m.viewerOpen {
				ctx = helppopover.Viewer
			}
			m.help = helppopover.New(helppopover.NewStyles(m.theme), ctx).SetSize(m.width, m.height)
			return m, nil
		case key.Matches(msg, m.keys.SenderPopover):
			if !m.viewerOpen {
				info, ok := m.acct.SelectedMessage()
				if ok {
					displayName, email := parseSender(info.From)
					return m, func() tea.Msg {
						return contacts.OpenPopoverMsg{DisplayName: displayName, Email: email}
					}
				}
			}
		case key.Matches(msg, m.keys.ContactsMode):
			return m, func() tea.Msg { return contacts.EnterContactsModeMsg{} }
		}
	}

	// Delegate everything else to the account tab.
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	m = m.deriveChromeFromAcct()
	return m, cmd
}

// renderFrame builds the full-screen layout string. It is extracted
// from View so it can be dimmed and composited under overlays.
func (m App) renderFrame() string {
	if m.contactsMode {
		return m.renderContactsFrame()
	}
	var rawContent string
	if m.compose != nil {
		rawContent = m.acct.RenderWithRightPane(m.compose.View())
	} else {
		rawContent = m.acct.View()
	}
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	// AccountTab.View honors its width contract: every line is exactly
	// m.width-1 display cells. Append the right border directly without
	// per-line measure-and-pad. See TestAccountTabView_HonorsAssignedWidth.
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := uicore.ComputeLayout(m.width).Sidebar
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, dividerCol)
	foot := m.footer.View(m.width)

	parts := []string{topLine, content}
	// Precedence: error banner wins, then toast, then the
	// chrome row collapses entirely.
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, status, foot)
	// Use strings.Join rather than lipgloss.JoinVertical. JoinVertical pads
	// all rows to the widest row using lipgloss.Width, which undercounts
	// SPUA-A Nerd Font glyphs by 1 cell each. Content rows already have the
	// correct terminal width (guaranteed by AccountTab's width contract);
	// JoinVertical would add spurious 1-cell padding to any row with SPUA-A
	// content, causing those rows to land 1 cell outside the terminal width.
	return strings.Join(parts, "\n")
}

// View composes the full-screen layout. When the help popover is open the
// underlying account frame is rendered, dimmed via DimANSI, and then the
// popover box is composited over it via PlaceOverlay so the underlying
// context remains visible but recedes visually.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	frame := m.renderFrame()

	if m.helpOpen {
		box, tooNarrow := m.help.Box(m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		if tooNarrow != "" {
			// Terminal too narrow for the full popover: show the notice
			// centered over the dimmed frame.
			x, y := (m.width-lipgloss.Width(tooNarrow))/2, m.height/2
			if x < 0 {
				x = 0
			}
			return uicore.PlaceOverlay(x, y, tooNarrow, dimmed)
		}
		x, y := m.help.Position(box, m.width, m.height)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.confirm.IsOpen() {
		box := m.confirm.Box(m.width, m.height)
		x, y := m.confirm.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.conflictOpen {
		body := m.conflict.View()
		x, y := uicore.CenterOverlay(body, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, body, dimmed)
	}

	if m.outboxOpen {
		body := m.outbox.View()
		x, y := uicore.CenterOverlay(body, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, body, dimmed)
	}

	if m.linkPicker.IsOpen() {
		box := m.linkPicker.Box(m.width, m.height)
		x, y := m.linkPicker.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.attachPicker.IsOpen() {
		box := m.attachPicker.Box(m.width, m.height)
		x, y := m.attachPicker.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.movePicker.IsOpen() {
		box := m.movePicker.Box(m.width, m.height)
		x, y := m.movePicker.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.form != nil && m.form.FromPopover() {
		box := m.form.Box(m.width, m.height)
		x, y := m.form.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.popover != nil {
		box := m.popover.Box(m.width, m.height)
		x, y := m.popover.Position(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	return frame
}

// IsLinkPickerOpen reports whether the link picker overlay is visible.
func (m App) IsLinkPickerOpen() bool { return m.linkPicker.IsOpen() }

// IsConfirmOpen reports whether the confirm modal overlay is visible.
func (m App) IsConfirmOpen() bool { return m.confirm.IsOpen() }

// contentHeight returns the height available for the content area.
// The chrome banner row (error banner or toast) takes one extra row
// when either is present. The row collapses when both are absent.
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	if m.lastErr.Err != nil || !m.toast.IsZero() {
		chrome++
	}
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}

// rightPaneSize returns the width and height available for the right pane,
// mirroring the geometry AccountTab derives in its WindowSizeMsg handler.
func (m App) rightPaneSize() (w, h int) {
	contentW := m.width - 1 // one cell for the right border App appends
	layout := uicore.ComputeLayout(contentW)
	sw := layout.Sidebar
	if sw > contentW/2 {
		sw = contentW / 2
	}
	w = max(1, contentW-sw-1) // -1 for divider
	h = m.contentHeight()
	return w, h
}

// selectedMessage returns the currently-selected message from the
// account tab's message list, forwarding to the viewer's current
// message when the viewer is open.
func (m App) selectedMessage() (mail.MessageInfo, bool) {
	return m.acct.SelectedMessage()
}

// hasBannerRow reports whether the chrome row above the status bar is
// occupied (either by the error banner or by an active toast).
func (m App) hasBannerRow() bool {
	return m.lastErr.Err != nil || !m.toast.IsZero()
}

// maybeResizeChild re-forwards a WindowSizeMsg to the child when the
// chrome banner row's occupancy has changed since hadBanner was
// captured. Returns the (possibly-updated) App and the resize Cmd, or
// the input App and nil when no resize is needed.
func (m App) maybeResizeChild(hadBanner bool) (App, tea.Cmd) {
	if hadBanner == m.hasBannerRow() || m.width <= 0 || m.height <= 0 {
		return m, nil
	}
	contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
	acct, cmd := m.acct.Update(contentMsg)
	m.acct = acct
	return m, cmd
}

// parseSender splits the From display string into a (displayName, email)
// pair using the RFC 5322 address parser. Falls back to (from, from) when
// the string cannot be parsed or contains no addresses.
func parseSender(from string) (displayName, email string) {
	addrs := content.ParseAddressList(from)
	if len(addrs) == 0 {
		return from, from
	}
	a := addrs[0]
	if a.Name != "" {
		return a.Name, a.Email
	}
	return a.Email, a.Email
}

// chromeBannerRow renders the single chrome row above the status bar.
// Error banner wins precedence. Otherwise the toast renders. Otherwise
// the empty string collapses the row.
func (m App) chromeBannerRow(width int) string {
	if banner := renderErrorBanner(m.lastErr, width, m.styles); banner != "" {
		return banner
	}
	if !m.toast.IsZero() {
		return renderToast(m.toast, width, m.styles)
	}
	return ""
}

// contactsColumnWidths returns (sidebarW, listW, detailW) for the contacts
// three-column layout at the current terminal width. Falls back to a single
// column when content width is below 60 cells. detailW may be zero.
func (m App) contactsColumnWidths() (sidebarW, listW, detailW int) {
	contentW := m.width - 1 // one cell for the right border App appends
	if contentW < 60 {
		// Narrow: single-column fallback. List only, no sidebar or detail.
		return 0, contentW, 0
	}
	const sidebarFloor = 14
	const dividers = 2
	listMin := 30
	detail := contentW - sidebarFloor - dividers - listMin
	if detail < 0 {
		detail = 0
		listMin = contentW - sidebarFloor - dividers
		if listMin < 1 {
			listMin = 1
		}
	}
	return sidebarFloor, listMin, detail
}

// sizedContactsChildren returns sidebar and list sized for the current terminal.
// The contacts frame adds one header row, so children get contentHeight-1.
func (m App) sizedContactsChildren() (contacts.Sidebar, contacts.List) {
	h := m.contactsBodyHeight()
	sbW, listW, _ := m.contactsColumnWidths()
	sb := m.contactsSidebar.SetSize(sbW, h)
	ls := m.contactsList.SetSize(listW, h)
	return sb, ls
}

func (m App) contactsBodyHeight() int {
	h := m.contentHeight() - 1
	if h < 1 {
		return 1
	}
	return h
}

// updateContactsKey handles a key press while contacts mode is active.
// M returns to mail mode. q quits. j/k/J/K and a–z route to sidebar/list.
func (m App) updateContactsKey(msg tea.KeyMsg) (App, tea.Cmd) {
	if key.Matches(msg, m.keys.MailMode) {
		return m, func() tea.Msg { return contacts.ExitContactsModeMsg{} }
	}
	if key.Matches(msg, m.keys.Quit) || key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	prevLetter := m.contactsSidebar.SelectionLetter()
	var sbCmd, listCmd tea.Cmd
	m.contactsSidebar, sbCmd = m.contactsSidebar.Update(msg)
	m.contactsList, listCmd = m.contactsList.Update(msg)

	// When the sidebar letter changed, scroll the list to match.
	newLetter := m.contactsSidebar.SelectionLetter()
	if newLetter != prevLetter && newLetter != 0 {
		m.contactsList = m.contactsList.SetSelectionLetter(newLetter)
	}

	return m, tea.Batch(sbCmd, listCmd)
}

// renderContactsFrame builds the full-screen contacts layout string.
func (m App) renderContactsFrame() string {
	sbW, listW, detailW := m.contactsColumnWidths()
	contentH := m.contactsBodyHeight()

	var content string
	if sbW == 0 {
		// Narrow fallback: list only.
		content = m.contactsList.View()
	} else {
		sbLines := strings.Split(m.contactsSidebar.View(), "\n")
		listLines := strings.Split(m.contactsList.View(), "\n")
		divLine := m.styles.FrameBorder.Render("│")

		var detailLines []string
		if m.form != nil && !m.form.FromPopover() {
			detailLines = strings.Split(m.form.View(), "\n")
		} else {
			cursor := m.contactsList.Cursor()
			detailLines = strings.Split(contacts.RenderDetailCard(cursor, detailW, m.contactsStyles), "\n")
		}

		assembled := make([]string, contentH)
		for i := range contentH {
			sb := ""
			if i < len(sbLines) {
				sb = sbLines[i]
			}
			sb = uicore.PadOrTruncate(sb, sbW)
			ls := ""
			if i < len(listLines) {
				ls = listLines[i]
			}
			ls = uicore.PadOrTruncate(ls, listW)
			dl := ""
			if detailW > 0 {
				if i < len(detailLines) {
					dl = detailLines[i]
				}
				dl = uicore.PadOrTruncate(dl, detailW)
			}
			assembled[i] = sb + divLine + ls + divLine + dl
		}
		content = strings.Join(assembled, "\n")
	}

	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(content, "\n")
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	body := strings.Join(contentLines, "\n")

	header := uicore.PadOrTruncate("CONTACTS · All sources", m.width-2) + rightBorder
	footerLine := m.footer.SetContext(ContactsContext).View(m.width)

	parts := []string{m.topLine.View(m.width, sbW+1), header, body}
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, m.statusBar.View(m.width, sbW+1), footerLine)
	return strings.Join(parts, "\n")
}

// formSize returns the (width, height) the contact form should be sized
// to. Modal mode (fromPopover) uses the full terminal so the form's
// internal width budget can clamp itself. Right-pane mode mirrors the
// detail column width in Contacts mode.
func (m App) formSize(fromPopover bool) (int, int) {
	if fromPopover {
		return m.width, m.height
	}
	_, _, detailW := m.contactsColumnWidths()
	return detailW, m.contactsBodyHeight()
}
