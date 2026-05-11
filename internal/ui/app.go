package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	corecontacts "github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailcompose"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/tidytext"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/helppopover"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/outbox"
	"github.com/glw907/poplar/internal/ui/reader"
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
	measurer           ansix.Measurer
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
	tidyCfg     tidytext.Config

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
func NewApp(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons uicore.IconSet, m ansix.Measurer, contactsCfg *config.ContactsConfig, identities []config.Identity) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	cStyles := contacts.NewStyles(t)
	cFixtures := contacts.Fixtures()
	app := App{
		acct:            account.New(t, acct, uiCfg, icons, m),
		icons:           icons,
		measurer:        m,
		styles:          styles,
		theme:           t,
		topLine:         NewTopLine(styles),
		statusBar:       sb,
		footer:          NewFooter(styles),
		keys:            NewGlobalKeys(),
		linkPicker:      reader.NewLinkPicker(reader.NewStyles(t), m),
		attachPicker:    reader.NewAttachPicker(reader.NewStyles(t), icons, m),
		movePicker:      movepicker.New(movepicker.NewStyles(t), m),
		downloadDir:     uiCfg.DownloadDir,
		confirm:         NewConfirmModal(styles),
		outbox:          NewOutboxOverlay(styles),
		conflict:        NewConflictOverlay(styles),
		undoSeconds:     uiCfg.UndoSeconds,
		undoSendWindow:  uiCfg.UndoSendWindow,
		now:             time.Now,
		opener:          xdgOpenURL,
		tidyEnabled:     uiCfg.Tidytext.Enabled,
		tidyAPIKey:      tidytext.ResolveAPIKey(uiCfg.Tidytext.Config),
		tidyCfg:         uiCfg.Tidytext.Config,
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

// Update dispatches by message type. Size + keys are special-cased;
// everything else flows through per-domain dispatchers, with the
// account tab as the fallback consumer.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.updateSize(msg)
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	// Chrome runs first: backendUpdateMsg and account.CacheEventMsg
	// fire on every drainer/idle cycle and would otherwise walk every
	// other dispatcher.
	if m2, cmd, ok := m.updateChromeMsg(msg); ok {
		return m2, cmd
	}
	if m2, cmd, ok := m.updateOutboxMsg(msg); ok {
		return m2, cmd
	}
	if m2, cmd, ok := m.updateComposeMsg(msg); ok {
		return m2, cmd
	}
	if m2, cmd, ok := m.updateModalsMsg(msg); ok {
		return m2, cmd
	}
	if m2, cmd, ok := m.updateContactsMsg(msg); ok {
		return m2, cmd
	}
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	m = m.deriveChromeFromAcct()
	return m, cmd
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
