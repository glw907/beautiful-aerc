package ui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/cache"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/config"
	corecontacts "github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/tidy"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/helppopover"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/outbox"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/sidebar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// pendingEmptyConfirm holds the parameters needed to emit
// EmptyFolderConfirmedMsg when the user accepts the confirm modal. The
// zero value means no empty-folder confirm is pending.
type pendingEmptyConfirm struct {
	folder string
	source string
}

// pendingReschedule holds the App-owned schedule picker and the op it
// targets. Zero value means no reschedule is in progress.
type pendingReschedule struct {
	picker *uicompose.SchedulePicker
	opID   int64
}

// App is the root bubbletea model.
type App struct {
	acct               account.Model
	icons              uicore.IconSet
	styles             Styles
	topLine            TopLine
	statusBar          StatusBar
	footer             Footer
	keys               GlobalKeys
	viewerOpen         bool
	helpOpen           bool
	help               helppopover.Model
	linkPicker         reader.LinkPicker
	attachPicker       reader.AttachPicker
	movePicker         movepicker.Model
	downloadDir        string
	confirm            ConfirmModal
	pendingEmpty       pendingEmptyConfirm
	outbox             OutboxOverlay
	outboxOpen         bool
	outboxView         *outbox.Model // non-nil while the Outbox folder is selected
	outboxPrevFolder   string        // canonical folder to restore on outbox close
	reschedule         pendingReschedule
	conflict           ConflictOverlay
	conflictOpen       bool
	lastOutboxDepth    cache.OutboxDepth
	offlineHinted      bool
	lastErr            ErrorMsg
	toast              pendingAction
	pendingUnsub       *reader.OpenUnsubscribeConfirmMsg
	lastNotice         string
	lastNoticeDeadline time.Time
	undoSeconds        int
	undoSendWindow     time.Duration
	now                func() time.Time // test seam, defaults to time.Now
	opener             URLOpener        // test seam, defaults to xdgOpenURL

	tidyEnabled bool
	tidyAPIKey  string
	tidyCfg     tidy.Config

	identities      []mailcompose.Identity
	contactsCfg     *corecontacts.ClientConfig
	contactsRefresh time.Duration

	lastBackfillDone   int
	lastBackfillTotal  int
	lastBackfillPaused bool
	lastBackfillWarn   bool

	theme                *theme.CompiledTheme
	compose              *uicompose.Model
	pendingComposeSave   bool // Save? modal is open for a dirty compose
	popover              *contacts.Popover
	contactsMode         bool
	contactsSidebar      contacts.Sidebar
	contactsList         contacts.List
	contactsStyles       contacts.Styles
	form                 *contacts.Form
	pendingFormDiscard   bool
	pendingContactDelete string
	width                int
	height               int
}

// WithOpener replaces the URL-opener seam.
func (m App) WithOpener(opener URLOpener) App {
	m.opener = opener
	return m
}

// NewApp creates the root model. Folder loading runs in Init's Cmd chain,
// not synchronously.
func NewApp(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons uicore.IconSet, contactsCfg *config.ContactsConfig, identities []config.Identity) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	cStyles := contacts.NewStyles(t)
	cFixtures := contacts.Fixtures()
	app := App{
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
		undoSendWindow:  uiCfg.UndoSendWindow,
		now:             time.Now,
		opener:          xdgOpenURL,
		tidyEnabled:     uiCfg.Tidy.Enabled,
		tidyAPIKey:      tidy.ResolveAPIKey(uiCfg.Tidy.Config),
		tidyCfg:         uiCfg.Tidy.Config,
		identities:      uicompose.IdentitiesFromConfig(identities),
		contactsStyles:  cStyles,
		contactsSidebar: contacts.NewSidebar(cStyles, cFixtures),
		contactsList:    contacts.NewList(cStyles, cFixtures, contacts.SortFirstName),
	}
	if contactsCfg != nil {
		pw, err := contactsCfg.ResolvePassword()
		if err == nil {
			cfg := &corecontacts.ClientConfig{
				URL:         contactsCfg.URL,
				Username:    contactsCfg.Username,
				Password:    pw,
				InsecureTLS: contactsCfg.InsecureTLS,
			}
			cl, cerr := corecontacts.NewClient(cfg.URL, cfg.Username, cfg.Password, cfg.InsecureTLS)
			if cerr != nil {
				app.lastErr = ErrorMsg{Op: "contacts init", Err: cerr}
			} else {
				acct.ContactsWriter = cl
				app.contactsCfg = cfg
				app.contactsRefresh = contactsCfg.RefreshInterval
			}
		}
	}
	return app
}

// suggestAddresses adapts cache.Account.SuggestAddresses to the SuggestFn
// signature compose expects (synchronous, returns rows only). Errors degrade
// silently; autocomplete is best-effort, not a blocking I/O surface.
func (m *App) suggestAddresses(prefix string) []contacts.Suggestion {
	out, err := m.acct.Cache().SuggestAddresses(context.Background(), prefix)
	if err != nil {
		return nil
	}
	return out
}

// Init kicks off the account tab's initial folder fetch and starts the
// backend update pump.
func (m App) Init() tea.Cmd {
	cmds := []tea.Cmd{m.acct.Init(), pumpUpdatesCmd(m.acct.Backend())}
	if m.contactsCfg != nil {
		cmds = append(cmds,
			syncContactsCmd(m.acct.Cache(), m.contactsCfg),
			scheduleSyncCmd(m.contactsRefresh),
		)
	}
	return tea.Batch(cmds...)
}

// deriveChromeFromAcct re-reads AccountTab state into the App-owned
// chrome (footer, status bar, viewerOpen).
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

func (m App) refreshBackfillSegment() App {
	done, total, warn, _ := m.acct.Cache().BackfillProgress()
	paused := m.statusBar.ConnectionState() != Connected
	if done == m.lastBackfillDone && total == m.lastBackfillTotal &&
		paused == m.lastBackfillPaused && warn == m.lastBackfillWarn {
		return m
	}
	m.lastBackfillDone = done
	m.lastBackfillTotal = total
	m.lastBackfillPaused = paused
	m.lastBackfillWarn = warn
	m.statusBar = m.statusBar.SetBackfill(done, total, paused, warn)
	return m
}

// Update handles global keys and delegates everything else to the
// account tab. Chrome (footer, status bar, link picker) is re-read from
// AccountTab accessors after each delegation.
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
		if m.outboxView != nil {
			w, h := m.rightPaneSize()
			m.outboxView.SetSize(w, h)
		}
		if m.reschedule.picker != nil {
			m.reschedule.picker.SetSize(m.width, m.height)
		}
		if m.contactsMode {
			m.contactsSidebar, m.contactsList = m.sizedContactsChildren()
		}
		if m.form != nil {
			w, h := m.formSize(m.form.FromPopover())
			next := m.form.SetSize(w, h)
			m.form = &next
		}
		// Sizing alone does not change viewer state or folder counts, so
		// no chrome derivation is needed here.
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
		match, _, found := m.acct.Cache().LookupContact(context.Background(), msg.Email)
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
		f := contacts.NewForm(m.contactsStyles, msg.Initial, msg.FromPopover, saveTo).
			WithExistingUID(msg.UID)
		w, h := m.formSize(msg.FromPopover)
		f = f.SetSize(w, h)
		m.form = &f
		return m, nil

	case contacts.OpenContactDeleteConfirmMsg:
		m.pendingContactDelete = msg.UID
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Delete contact?",
			Body:  msg.DisplayName + " will be removed from this address book.",
		})
		return m, nil

	case contacts.ContactSaveMsg:
		uid := ""
		if m.form != nil {
			uid = m.form.ExistingUID()
		}
		m.form = nil
		return m, queueContactPutCmd(m.acct.Cache(), uid, msg.Contact)

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

	case reader.OpenUnsubscribeConfirmMsg:
		host := unsubscribeHost(msg.Unsub)
		stash := msg
		m.pendingUnsub = &stash
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Unsubscribe",
			Body:  "Send unsubscribe request to " + host + "?",
		})
		return m, nil

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
		case m.pendingContactDelete != "":
			uid := m.pendingContactDelete
			m.pendingContactDelete = ""
			m.form = nil
			return m, queueContactDeleteCmd(m.acct.Cache(), uid)
		case m.pendingComposeSave:
			m.pendingComposeSave = false
			if m.compose == nil {
				return m, nil
			}
			// Persist the current draft and queue a server push.
			draftsFolder := resolveDraftsFolder(m.acct.Cache())
			d := m.compose.CurrentDraft()
			draftID := m.compose.DraftID()
			prevUID := mail.UID(m.compose.PrevServerUID())
			m.compose = nil
			if draftsFolder != "" {
				return m, upsertAndPushDraftCmd(m.acct.Cache(), draftID, draftsFolder, d, prevUID, m.identities)
			}
			return m, nil
		case m.pendingUnsub != nil:
			pu := *m.pendingUnsub
			m.pendingUnsub = nil
			return m, m.dispatchUnsubscribe(pu.Unsub)
		case m.pendingEmpty.folder != "":
			folder, source := m.pendingEmpty.folder, m.pendingEmpty.source
			m.pendingEmpty = pendingEmptyConfirm{}
			return m, func() tea.Msg {
				return account.EmptyFolderConfirmedMsg{Folder: folder, Source: source}
			}
		}
		return m, nil

	case ConfirmModalNoMsg:
		m.pendingUnsub = nil
		if m.pendingFormDiscard {
			m.pendingFormDiscard = false
			return m, nil
		}
		if m.pendingContactDelete != "" {
			m.pendingContactDelete = ""
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
		// Esc on discard-changes keeps the form mounted.
		if m.pendingFormDiscard {
			m.pendingFormDiscard = false
			m.confirm = m.confirm.Close()
			return m, nil
		}
		if m.pendingContactDelete != "" {
			m.pendingContactDelete = ""
			m.confirm = m.confirm.Close()
			return m, nil
		}
		// Esc on save-draft keeps compose mounted.
		if m.pendingComposeSave {
			m.pendingComposeSave = false
			m.confirm = m.confirm.Close()
			return m, nil
		}
		if m.pendingUnsub != nil {
			m.pendingUnsub = nil
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

	case undoCountdownTickMsg:
		if m.toast.op != opSendUndo || m.now().After(m.toast.deadline) {
			return m, nil
		}
		return m, undoCountdownTickCmd()

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

	case UnsubscribeDoneMsg:
		m.lastNotice = "Unsubscribed from " + msg.Host
		m.lastNoticeDeadline = time.Now().Add(5 * time.Second)
		return m, clearNoticeAfter(5 * time.Second)

	case noticeExpireMsg:
		if !m.lastNoticeDeadline.IsZero() && !time.Now().Before(m.lastNoticeDeadline) {
			m.lastNotice = ""
			m.lastNoticeDeadline = time.Time{}
		}
		return m, nil

	case ErrorMsg:
		// An error clears any pending toast and any success notice.
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m.lastNotice = ""
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
		if msg.Name == mail.CanonicalOutbox {
			if m.outboxView == nil {
				m.outboxPrevFolder = m.acct.CurrentFolderName()
				ob := outbox.New(m.theme)
				w, h := m.rightPaneSize()
				ob.SetSize(w, h)
				m.outboxView = &ob
			}
			return m, loadOutboxScheduledCmd(m.acct.Cache())
		}
		// A fresh folder load commits any in-flight toast (msglist was
		// reset by selectionChangedCmds).
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
			m.acct.NotifyConnState(cs == Connected)
			m = m.refreshBackfillSegment()
		}
		// Non-ConnState Update types delegate to AccountTab in a later pass.
		return m, tea.Batch(cmds...)

	case account.CacheEventMsg:
		cmds := []tea.Cmd{refreshOutboxDepthCmd(m.acct.Cache())}
		if m.outboxOpen {
			cmds = append(cmds, loadOutboxSummaryCmd(m.acct.Cache()))
		}
		if m.conflictOpen {
			cmds = append(cmds, loadOutboxConflictsCmd(m.acct.Cache()))
		}
		if m.outboxView != nil {
			cmds = append(cmds, loadOutboxScheduledCmd(m.acct.Cache()))
		}
		if msg.Event.Note != "" {
			m.lastErr = ErrorMsg{Op: "draft", Err: errors.New(msg.Event.Note)}
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		cmds = append(cmds, fcmd)
		m = m.refreshBackfillSegment()
		return m, tea.Batch(cmds...)

	case outboxScheduledMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox", Err: msg.err}
			return m, nil
		}
		if m.outboxView != nil {
			m.outboxView.SetRows(msg.rows)
		}
		return m, nil

	case outboxCancelledMsg:
		if msg.err != nil && !errors.Is(msg.err, cache.ErrNotPending) {
			m.lastErr = ErrorMsg{Op: "cancel op", Err: msg.err}
		}
		return m, tea.Batch(loadOutboxScheduledCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache()))

	case rescheduleOpMsg:
		if msg.err != nil {
			if errors.Is(msg.err, cache.ErrNotPending) {
				m.lastErr = ErrorMsg{Op: "reschedule", Err: errors.New("op already dispatched")}
			} else {
				m.lastErr = ErrorMsg{Op: "reschedule", Err: msg.err}
			}
		}
		return m, tea.Batch(loadOutboxScheduledCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache()))

	case OpenConflictsFromOutboxMsg:
		m.outboxOpen = false
		m.conflictOpen = true
		m.conflict = m.conflict.Open(nil)
		return m, loadOutboxConflictsCmd(m.acct.Cache())

	case outboxDepthMsg:
		prev := m.lastOutboxDepth
		m.lastOutboxDepth = msg.depth
		inflight := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed
		m.statusBar = m.statusBar.SetOutboxDepth(inflight, msg.depth.Conflict)
		total := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed + msg.depth.Conflict
		prevTotal := prev.Pending + prev.Executing + prev.Failed + prev.Conflict
		if total != prevTotal {
			m.acct.SetOutboxCount(total)
		}
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

	case outbox.CloseMsg:
		m.outboxView = nil
		// Restore the previous folder. When outboxPrevFolder is empty (the
		// Outbox was the first selection), fall back to Inbox.
		prev := m.outboxPrevFolder
		if prev == "" {
			prev = mail.CanonicalInbox
		}
		m.outboxPrevFolder = ""
		acct, cmd := m.acct.Update(account.JumpFolderMsg{Canonical: prev})
		m.acct = acct
		m = m.deriveChromeFromAcct()
		return m, cmd

	case outbox.CancelMsg:
		return m, cancelOutboxOpCmd(m.acct.Cache(), msg.OpID)

	case outbox.RescheduleMsg:
		p := uicompose.NewSchedulePicker(m.theme, m.now(), msg.Initial)
		p.SetSize(m.width, m.height)
		m.reschedule = pendingReschedule{picker: &p, opID: msg.OpID}
		return m, nil

	case outbox.EditAsDraftMsg:
		if msg.Draft == nil {
			return m, nil
		}
		return m, editAsDraftCmd(m.acct.Cache(), msg.OpID, msg.Draft)

	case uicompose.ScheduleAcceptedMsg:
		if m.reschedule.picker != nil {
			opID := m.reschedule.opID
			m.reschedule = pendingReschedule{}
			return m, rescheduleOpCmd(m.acct.Cache(), opID, msg.When)
		}
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd
		}
		return m, nil

	case uicompose.ScheduleCancelledMsg:
		if m.reschedule.picker != nil {
			m.reschedule = pendingReschedule{}
			return m, nil
		}
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd
		}
		return m, nil

	case uicompose.SendMsg:
		sent := resolveSentFolder(m.acct.Cache())
		if sent == "" {
			if m.compose != nil {
				m.compose.SetErr("no Sent folder configured")
			}
			return m, nil
		}
		d := msg.Draft
		acct := m.acct.Cache()
		cmds := []tea.Cmd{composeSendCmd(acct, sent, d, m.identities, m.undoSendWindow, msg.ScheduledFor)}
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
		if msg.ScheduledFor.IsZero() || !msg.ScheduledFor.After(m.now()) {
			// No hold window: drainer is already eligible. No banner.
			return m, nil
		}
		deadline := msg.ScheduledFor
		m.toast = pendingAction{
			op:        opSendUndo,
			deadline:  deadline,
			sendOpIDs: msg.OpIDs,
			sendDraft: msg.Draft,
		}
		cmds := []tea.Cmd{
			tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
				return toastExpireMsg{deadline: deadline}
			}),
			undoCountdownTickCmd(),
		}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case uicompose.SeededMsg:
		w, h := m.rightPaneSize()
		m.compose = uicompose.New(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses)
		m.compose.SetSize(w, h)
		m.compose.SetIdentities(m.identities)
		m.compose.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
		m.compose.Seed(msg.Draft)
		return m, m.compose.Init()

	case RestoreFromDraftMsg:
		w, h := m.rightPaneSize()
		m.compose = uicompose.New(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses)
		m.compose.SetSize(w, h)
		m.compose.SetIdentities(m.identities)
		m.compose.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
		m.compose.Seed(msg.Draft)
		return m, m.compose.Init()

	case openDraftMsg:
		// Drafts-folder Enter: wire cache/target, then open compose.
		w, h := m.rightPaneSize()
		row := msg.row
		c := uicompose.Open(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), row.DraftID, msg.draft, m.suggestAddresses)
		c.SetSize(w, h)
		c.SetIdentities(m.identities)
		c.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
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

	case uicompose.AttachAcceptedMsg, uicompose.AttachCancelledMsg:
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd
		}
		return m, nil

	case contactsTickMsg:
		if m.contactsCfg == nil {
			return m, nil
		}
		return m, tea.Batch(
			syncContactsCmd(m.acct.Cache(), m.contactsCfg),
			scheduleSyncCmd(m.contactsRefresh),
		)

	case contactsSyncedMsg:
		return m, nil

	case tea.KeyMsg:
		m.acct.NotifyActivity()
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
		if m.reschedule.picker != nil {
			p, cmd := m.reschedule.picker.Update(msg)
			m.reschedule.picker = &p
			return m, cmd
		}
		if m.outboxView != nil {
			next, cmd := m.outboxView.Update(msg)
			m.outboxView = &next
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
		// In the Drafts folder, Enter opens compose instead of the viewer.
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
			m.compose = uicompose.New(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses)
			m.compose.SetSize(w, h)
			m.compose.SetIdentities(m.identities)
			m.compose.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
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
			// Undo is only live while a toast is active. Otherwise 'u'
			// falls through to AccountTab so other consumers can claim it.
			switch m.toast.op {
			case opNone:
				// no-op, fall through
			case opSendUndo:
				opIDs := m.toast.sendOpIDs
				draft := m.toast.sendDraft
				hadBanner := m.hasBannerRow()
				m.toast = pendingAction{}
				var rcmd tea.Cmd
				m, rcmd = m.maybeResizeChild(hadBanner)
				return m, tea.Batch(rcmd, undoSendCmd(m.acct.Cache(), opIDs, draft))
			default:
				return m, func() tea.Msg { return undoRequestedMsg{} }
			}
		case key.Matches(msg, m.keys.Quit):
			if m.viewerOpen {
				// q closes the viewer, not the app. Delegate so AccountTab
				// routes into viewer.handleKey.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(msg)
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			if m.acct.SearchState() != sidebar.SearchIdle {
				// Steal q while search is active so it doesn't quit the
				// app mid-search; route a typed clear msg to AccountTab.
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

// renderFrame builds the full-screen layout string, extracted from View
// so it can be dimmed and composited under overlays.
func (m App) renderFrame() string {
	if m.contactsMode {
		return m.renderContactsFrame()
	}
	var rawContent string
	switch {
	case m.outboxView != nil:
		rawContent = m.acct.RenderWithRightPane(m.outboxView.View())
	case m.compose != nil:
		rawContent = m.acct.RenderWithRightPane(m.compose.View())
	default:
		rawContent = m.acct.View()
	}
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	// AccountTab.View honors its width contract (each line is exactly
	// m.width-1 display cells), so the right border can be appended
	// without per-line measure-and-pad.
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := uicore.ComputeLayout(m.width).Sidebar
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, dividerCol)
	var foot string
	if m.compose != nil {
		tidyVisible := m.compose.TidyEnabled() && m.compose.IsFocusBody()
		foot = m.footer.ViewGroups(composeFooterGroups(m.compose.HasSignatures(), m.compose.IsFocusFrom(), tidyVisible), m.width)
	} else if m.viewerOpen && m.acct.Viewer().Unsubscribe().Available() {
		foot = m.footer.ViewGroups(viewerFooterGroupsWithUnsub(), m.width)
	} else {
		foot = m.footer.View(m.width)
	}

	parts := []string{topLine, content}
	// Precedence: error banner wins, then toast, then the chrome row
	// collapses entirely.
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, status, foot)
	// strings.Join over lipgloss.JoinVertical: JoinVertical pads to the
	// widest row using lipgloss.Width, which undercounts SPUA-A glyphs by
	// 1 cell each and would push those rows outside the terminal. Content
	// already honors the terminal-width contract. See ADR-0084.
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
			// Terminal too narrow for the popover; center the notice instead.
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

	if m.compose != nil && m.compose.AttachPickerIsOpen() {
		box := m.compose.AttachPickerView()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.compose != nil && m.compose.SchedulePickerIsOpen() {
		box := m.compose.SchedulePickerView()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
		dimmed := uicore.DimANSI(frame)
		return uicore.PlaceOverlay(x, y, box, dimmed)
	}

	if m.reschedule.picker != nil {
		box := m.reschedule.picker.View()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
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

func (m App) IsLinkPickerOpen() bool { return m.linkPicker.IsOpen() }
func (m App) IsConfirmOpen() bool    { return m.confirm.IsOpen() }

// contentHeight returns the height available for content. The chrome
// banner row above the status bar adds one row when either an error
// banner or an active toast is showing.
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

// rightPaneSize mirrors the geometry AccountTab derives in its
// WindowSizeMsg handler.
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

func (m App) selectedMessage() (mail.MessageInfo, bool) {
	return m.acct.SelectedMessage()
}

// hasBannerRow reports whether the chrome row above the status bar is
// occupied by an error banner, a success notice, or an active toast.
func (m App) hasBannerRow() bool {
	if m.lastErr.Err != nil || !m.toast.IsZero() {
		return true
	}
	return m.lastNotice != "" && !m.lastNoticeDeadline.IsZero() && time.Now().Before(m.lastNoticeDeadline)
}

// maybeResizeChild re-forwards a WindowSizeMsg to the child when the
// chrome banner row's occupancy has changed since hadBanner was
// captured.
func (m App) maybeResizeChild(hadBanner bool) (App, tea.Cmd) {
	if hadBanner == m.hasBannerRow() || m.width <= 0 || m.height <= 0 {
		return m, nil
	}
	contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
	acct, cmd := m.acct.Update(contentMsg)
	m.acct = acct
	return m, cmd
}

// parseSender splits the From display string into (displayName, email)
// via content.ParseAddressList. Falls back to (from, from) when parsing
// fails or yields no addresses.
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

// chromeBannerRow renders the chrome row above the status bar. Error
// banner wins, then success notice, then send-undo countdown, then
// triage toast, else "" collapses the row.
func (m App) chromeBannerRow(width int) string {
	if banner := renderErrorBanner(m.lastErr, width, m.styles); banner != "" {
		return banner
	}
	if m.lastNotice != "" && !m.lastNoticeDeadline.IsZero() && time.Now().Before(m.lastNoticeDeadline) {
		return m.styles.Toast.Render(uicore.TruncateToWidth(m.lastNotice, width))
	}
	if m.toast.op == opSendUndo {
		remaining := time.Until(m.toast.deadline).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		text := fmt.Sprintf("Sending in %ds — u undo", int(remaining.Seconds()))
		return m.styles.Toast.Render(uicore.TruncateToWidth(text, width))
	}
	if !m.toast.IsZero() {
		return renderToast(m.toast, width, m.styles)
	}
	return ""
}

// unsubscribeHost returns the user-visible host string for a confirm-modal
// prompt. Picks from the action that will fire (one-click → mailto → http)
// and falls back to a fixed label when every URL is malformed.
func unsubscribeHost(u content.Unsubscribe) string {
	switch {
	case u.OneClick != "":
		if p, err := url.Parse(u.OneClick); err == nil && p.Host != "" {
			return p.Host
		}
	case u.Mailto != "":
		if p, err := url.Parse(u.Mailto); err == nil && p.Opaque != "" {
			at := strings.IndexByte(p.Opaque, '?')
			if at < 0 {
				return p.Opaque
			}
			return p.Opaque[:at]
		}
	case u.HTTP != "":
		if p, err := url.Parse(u.HTTP); err == nil && p.Host != "" {
			return p.Host
		}
	}
	return "this list"
}

// dispatchUnsubscribe routes a confirmed unsubscribe by RFC 8058
// precedence: one-click POST > mailto compose seed > plain http via
// URLOpener.
func (m App) dispatchUnsubscribe(u content.Unsubscribe) tea.Cmd {
	switch {
	case u.OneClick != "":
		return unsubscribePostCmd(u.OneClick)
	case u.Mailto != "":
		d, err := mailcompose.SeedFromMailto(u.Mailto, m.acct.AccountEmail())
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Op: "unsubscribe (mailto)", Err: err}
			}
		}
		return func() tea.Msg { return uicompose.SeededMsg{Draft: d} }
	case u.HTTP != "":
		return launchURLCmd(m.opener, u.HTTP)
	}
	return nil
}

// contactsColumnWidths returns (sidebarW, listW, detailW) for the
// contacts three-column layout. Below 60 cells the layout collapses to
// list-only with sidebarW=detailW=0.
func (m App) contactsColumnWidths() (sidebarW, listW, detailW int) {
	contentW := m.width - 1 // one cell for the right border App appends
	if contentW < 60 {
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

// sizedContactsChildren returns sidebar and list sized for the current
// terminal; the contacts frame's header row reserves one line.
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

// updateContactsKey handles a key press in contacts mode. M returns to
// mail, q quits, j/k/J/K and a–z route to sidebar/list.
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

	// Scroll the list to match when the sidebar letter changed.
	newLetter := m.contactsSidebar.SelectionLetter()
	if newLetter != prevLetter && newLetter != 0 {
		m.contactsList = m.contactsList.SetSelectionLetter(newLetter)
	}

	return m, tea.Batch(sbCmd, listCmd)
}

func (m App) renderContactsFrame() string {
	sbW, listW, detailW := m.contactsColumnWidths()
	contentH := m.contactsBodyHeight()

	var content string
	if sbW == 0 {
		// Narrow: list only.
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

// formSize returns the (width, height) the contact form should occupy.
// Modal mode (fromPopover) uses the full terminal so the form's internal
// width budget can clamp itself; right-pane mode mirrors the detail
// column width.
func (m App) formSize(fromPopover bool) (int, int) {
	if fromPopover {
		return m.width, m.height
	}
	_, _, detailW := m.contactsColumnWidths()
	return detailW, m.contactsBodyHeight()
}
