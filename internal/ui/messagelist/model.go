package messagelist

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/search"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/mattn/go-runewidth"
)

// mlFlagWidth pads the flag cell to match the Nerd Font SPUA-A 2-cell glyph
// so flagged and unflagged rows keep the sender column aligned.
const mlFlagWidth = 2

const mlCursorGlyph = "▐"

// Thread-prefix tokens are each exactly 3 display cells. buildPrefix
// relies on that to keep column math stable. Edit them as a set.
const (
	mlThreadVert  = "│  " // ancestor that still has more siblings below
	mlThreadGap   = "   " // ancestor that was the last sibling
	mlThreadTee   = "├─ " // current row, more siblings below
	mlThreadElbow = "└─ " // current row, last sibling
)

// SortOrder controls the order of thread roots. Children inside a thread
// always sort chronologically ascending regardless of this setting.
type SortOrder int

const (
	SortDateDesc SortOrder = iota // newest activity first (default)
	SortDateAsc                   // oldest activity first
)

type Styles struct {
	MsgListBg            lipgloss.Style
	MsgListSelected      lipgloss.Style
	MsgListCursor        lipgloss.Style
	MsgListUnreadSender  lipgloss.Style
	MsgListUnreadSubject lipgloss.Style
	MsgListReadSender    lipgloss.Style
	MsgListReadSubject   lipgloss.Style
	MsgListDate          lipgloss.Style
	MsgListIconUnread    lipgloss.Style
	MsgListIconRead      lipgloss.Style
	MsgListFlagFlagged   lipgloss.Style
	MsgListThreadPrefix  lipgloss.Style
	MsgListPlaceholder   lipgloss.Style
}

// Row is the public projection of displayRow returned by Rows.
type Row struct {
	Msg          mail.MessageInfo
	Prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	IsThreadRoot bool
	ThreadSize   int  // set on roots only. 1 for unthreaded
	Hidden       bool // true when collapsed under a folded root
	Depth        uint8
}

type displayRow struct {
	msg          mail.MessageInfo
	prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	dateText     string // pre-rendered date column, computed in rebuild
	isThreadRoot bool
	threadSize   int   // set on roots only. 1 for unthreaded
	hidden       bool  // true when collapsed under a folded root
	depth        uint8 // 0 = root, derived during prefix computation
}

// FilterValue satisfies list.Item. The list's built-in filter stays
// disabled because the sidebar shelf owns search (ADR-0188), so this
// is never consulted.
func (r displayRow) FilterValue() string { return "" }

// searchFilter holds the parsed query plus its raw form for the
// "is filter active?" predicate. Zero value means no filter.
type searchFilter struct {
	raw   string
	query search.Query
}

// Model renders the message list panel: flags, sender, subject, date.
// Embeds bubbles/v2/list with a custom *rowDelegate (delegate.go) so
// the list owns cursor + viewport + key dispatch. Owns thread
// grouping, fold state, sort direction, and the source/derived rows
// pipeline. The embedded list sees only visible rows (hidden rows
// stay in m.rows for tests, ActionTargets thread expansion, etc.).
type Model struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	threaded bool
	styles   Styles
	icons    uicore.IconSet
	layout   uicore.LayoutMode
	width    int
	height   int
	now      time.Time

	list     list.Model
	delegate *rowDelegate
	keys     KeyMap

	filter          searchFilter
	preSearchCursor int
	savedByFilter   bool
	filterResults   int
	visualMode      bool
	marked          map[mail.UID]struct{}

	resultsMode  bool
	originByUID  map[mail.UID]string
	preResults   []mail.MessageInfo
	preThreaded  bool
	preCursorUID mail.UID
}

// New constructs a Model. layout defaults to (Sender=22, Date=5,
// FlagColumn=true) so tests that bypass WindowSizeMsg get sensible
// output before any SetLayout call.
func New(styles Styles, msgs []mail.MessageInfo, width, height int, icons uicore.IconSet) Model {
	delegate := &rowDelegate{
		styles: styles,
		layout: uicore.LayoutMode{Sender: 22, Date: 5, FlagColumn: true},
		icons:  icons,
		now:    time.Now(),
		width:  width,
	}
	ls := list.New(nil, delegate, width, height)
	ls.SetShowTitle(false)
	ls.SetShowFilter(false)
	ls.SetShowStatusBar(false)
	ls.SetShowPagination(false)
	ls.SetShowHelp(false)
	ls.SetFilteringEnabled(false)
	ls.InfiniteScrolling = false
	ls.DisableQuitKeybindings()
	ls.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(),
		CursorDown: key.NewBinding(),
	}

	m := Model{
		styles:   styles,
		icons:    icons,
		layout:   delegate.layout,
		width:    width,
		height:   height,
		folded:   map[mail.UID]bool{},
		marked:   map[mail.UID]struct{}{},
		sort:     SortDateDesc,
		threaded: true,
		now:      delegate.now,
		list:     ls,
		delegate: delegate,
		keys:     DefaultKeyMap(),
	}
	m.SetMessages(msgs)
	return m
}

// KeyMap returns the binding set Update dispatches on. Exported so
// the help popover and external test code can introspect.
func (m Model) KeyMap() KeyMap { return m.keys }

// SetKeyMap overrides the default bindings. Test seam.
func (m *Model) SetKeyMap(km KeyMap) { m.keys = km }

// Update is the canonical key-dispatch entry. account.Model
// forwards messages here after handling its own bindings (triage,
// open, fold, visual, search). Returns the updated Model and any
// Cmd; the embedded list does not produce Cmds in messagelist's
// configuration (filtering disabled, no spinner), so the result is
// always nil. The signature stays Cmd-shaped for forward
// compatibility.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Down):
		m.list.CursorDown()
	case key.Matches(keyMsg, m.keys.Up):
		m.list.CursorUp()
	case key.Matches(keyMsg, m.keys.Top):
		m.list.GoToStart()
	case key.Matches(keyMsg, m.keys.Bottom):
		m.list.GoToEnd()
	}
	return m, nil
}

// SetMessages replaces the source slice and rebuilds rows. Resets fold
// state, cursor, viewport, and any active filter. Refreshes the clock
// snapshot so newly-delivered messages get same-day relative formatting.
func (m *Model) SetMessages(msgs []mail.MessageInfo) {
	m.source = msgs
	m.folded = map[mail.UID]bool{}
	m.marked = map[mail.UID]struct{}{}
	m.visualMode = false
	m.filter = searchFilter{}
	m.savedByFilter = false
	m.preSearchCursor = 0
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.list.Select(0)
}

// rebuild runs the group→sort→flatten pipeline against m.source: bucket by
// ThreadID, pick a root per bucket (empty InReplyTo, falling back to
// earliest by date), sort threads by latest-activity in m.sort direction,
// walk each thread root-then-children to compute depth and prefix, apply
// fold state.
func (m *Model) rebuild() {
	var buckets [][]mail.MessageInfo
	if m.threaded {
		buckets = bucketByThreadID(m.source)
	} else {
		buckets = make([][]mail.MessageInfo, len(m.source))
		for i, msg := range m.source {
			buckets[i] = []mail.MessageInfo{msg}
		}
	}
	buckets = m.filterBuckets(buckets)
	if m.filter.raw != "" {
		m.filterResults = len(buckets)
	} else {
		m.filterResults = 0
	}
	// Pair each bucket with its latest-activity key so the comparator runs
	// in O(1) and the memoized value stays aligned through the in-place sort.
	type bucketSort struct {
		bucket []mail.MessageInfo
		latest mail.MessageInfo
	}
	pairs := make([]bucketSort, len(buckets))
	for i, b := range buckets {
		pairs[i] = bucketSort{bucket: b, latest: latestActivity(b)}
	}
	slices.SortStableFunc(pairs, func(a, b bucketSort) int {
		if m.sort == SortDateAsc {
			return compareMessage(a.latest, b.latest)
		}
		return compareMessage(b.latest, a.latest)
	})
	for i, p := range pairs {
		buckets[i] = p.bucket
	}

	rows := make([]displayRow, 0, len(m.source))
	for _, bucket := range buckets {
		rows = appendThreadRows(rows, bucket)
	}
	if m.filter.raw == "" {
		applyFoldState(rows, m.folded)
	}
	for i := range rows {
		rows[i].dateText = displayDate(rows[i].msg, m.now, m.layout.Date)
	}
	m.rows = rows
	m.delegate.now = m.now
	m.syncList()
}

// syncList copies the visible subset of m.rows into the embedded
// list.Model. Hidden rows (folded thread children) stay in m.rows
// for ActionTargets / Rows() / threadRootIndex but never reach the
// list's cursor or viewport.
func (m *Model) syncList() {
	visible := make([]list.Item, 0, len(m.rows))
	for _, r := range m.rows {
		if r.hidden {
			continue
		}
		visible = append(visible, r)
	}
	m.list.SetItems(visible)
}

// snapToUIDInList moves the list cursor onto the visible row whose
// msg.UID matches uid. Empty UID or no match leaves cursor at 0.
func (m *Model) snapToUIDInList(uid mail.UID) {
	if uid == "" {
		m.list.Select(0)
		return
	}
	for i, item := range m.list.Items() {
		if r, ok := item.(displayRow); ok && r.msg.UID == uid {
			m.list.Select(i)
			return
		}
	}
	m.list.Select(0)
}

// bucketByThreadID groups messages by ThreadID, preserving input order
// within and across buckets. The two-pass shape (collect IDs, then slot)
// keeps bucket order deterministic for layout-comparing tests.
func bucketByThreadID(msgs []mail.MessageInfo) [][]mail.MessageInfo {
	var order []mail.UID
	seen := make(map[mail.UID]int)
	for _, m := range msgs {
		if _, ok := seen[m.ThreadID]; ok {
			continue
		}
		seen[m.ThreadID] = len(order)
		order = append(order, m.ThreadID)
	}
	buckets := make([][]mail.MessageInfo, len(order))
	for _, m := range msgs {
		idx := seen[m.ThreadID]
		buckets[idx] = append(buckets[idx], m)
	}
	return buckets
}

// filterBuckets keeps any bucket containing at least one matching message.
// Thread-level predicate per ADR-0064.
func (m *Model) filterBuckets(buckets [][]mail.MessageInfo) [][]mail.MessageInfo {
	if m.filter.raw == "" {
		return buckets
	}
	out := buckets[:0]
	for _, bucket := range buckets {
		for _, msg := range bucket {
			if matchMessageQuery(msg, m.filter.query) {
				out = append(out, bucket)
				break
			}
		}
	}
	return out
}

// matchMessageQuery tests one message against a parsed search.Query
// whose terms have already been lowercased by the caller (see
// SetFilter). Bare terms match subject + from + to + cc; field-scoped
// operators constrain the matching field. HasAttachment + In are
// no-ops folder-locally; the cache search path covers them.
func matchMessageQuery(msg mail.MessageInfo, q search.Query) bool {
	for _, t := range q.Terms {
		if !(containsFold(msg.Subject, t) ||
			containsFold(msg.From, t) ||
			containsFold(msg.To, t) ||
			containsFold(msg.Cc, t)) {
			return false
		}
	}
	for _, t := range q.From {
		if !containsFold(msg.From, t) {
			return false
		}
	}
	for _, t := range q.To {
		if !containsFold(msg.To, t) {
			return false
		}
	}
	for _, t := range q.Cc {
		if !containsFold(msg.Cc, t) {
			return false
		}
	}
	for _, t := range q.Subject {
		if !containsFold(msg.Subject, t) {
			return false
		}
	}
	return true
}

func containsFold(haystack, lowerNeedle string) bool {
	return strings.Contains(strings.ToLower(haystack), lowerNeedle)
}

// lowerQueryTerms returns q with every term lowercased so the
// per-message match loop avoids re-lowercasing on each row.
func lowerQueryTerms(q search.Query) search.Query {
	lower := func(xs []string) []string {
		if len(xs) == 0 {
			return xs
		}
		out := make([]string, len(xs))
		for i, s := range xs {
			out[i] = strings.ToLower(s)
		}
		return out
	}
	q.Terms = lower(q.Terms)
	q.From = lower(q.From)
	q.To = lower(q.To)
	q.Cc = lower(q.Cc)
	q.Subject = lower(q.Subject)
	return q
}

// pickRoot returns the index of the thread root: the message with empty
// InReplyTo, or the earliest by sent time when the parent chain is broken.
func pickRoot(bucket []mail.MessageInfo) int {
	for i, m := range bucket {
		if m.InReplyTo == "" {
			return i
		}
	}
	earliest := 0
	for i, m := range bucket {
		if compareMessage(m, bucket[earliest]) < 0 {
			earliest = i
		}
	}
	return earliest
}

// latestActivity returns the message representing the thread's most recent
// activity. Empty bucket returns a zero-value MessageInfo so downstream
// comparisons stay safe even though callers should not pass one.
func latestActivity(bucket []mail.MessageInfo) mail.MessageInfo {
	var latest mail.MessageInfo
	for _, m := range bucket {
		if compareMessage(latest, m) < 0 {
			latest = m
		}
	}
	return latest
}

// compareMessage orders messages oldest-first. The Date-string fallback
// and zero-SentAt tie-break exist for legacy fixtures predating SentAt.
func compareMessage(a, b mail.MessageInfo) int {
	aZero := a.SentAt.IsZero()
	bZero := b.SentAt.IsZero()
	if !aZero && !bZero {
		return a.SentAt.Compare(b.SentAt)
	}
	if aZero && bZero {
		return cmp.Compare(a.Date, b.Date)
	}
	if aZero {
		return -1
	}
	return 1
}

// threadNode is a transient tree node used only during prefix computation
// inside appendThreadRows. The tree is discarded after the walk.
type threadNode struct {
	msg      mail.MessageInfo
	children []*threadNode
}

// appendThreadRows builds a transient tree from one thread bucket and
// emits displayRows in depth-first root-then-children order with the
// box-drawing prefix for each row's position.
func appendThreadRows(rows []displayRow, bucket []mail.MessageInfo) []displayRow {
	rootIdx := pickRoot(bucket)
	root := &threadNode{msg: bucket[rootIdx]}

	byUID := map[mail.UID]*threadNode{}
	for i, msg := range bucket {
		if i == rootIdx {
			byUID[msg.UID] = root
			continue
		}
		byUID[msg.UID] = &threadNode{msg: msg}
	}

	// Broken chains (InReplyTo points outside the bucket) attach to the
	// root as top-level children.
	for i, msg := range bucket {
		if i == rootIdx {
			continue
		}
		node := byUID[msg.UID]
		parent, ok := byUID[msg.InReplyTo]
		if !ok {
			parent = root
		}
		parent.children = append(parent.children, node)
	}

	var sortChildren func(n *threadNode)
	sortChildren = func(n *threadNode) {
		slices.SortStableFunc(n.children, func(a, b *threadNode) int {
			return compareMessage(a.msg, b.msg)
		})
		for _, c := range n.children {
			sortChildren(c)
		}
	}
	sortChildren(root)

	rows = append(rows, displayRow{
		msg:          root.msg,
		isThreadRoot: true,
		threadSize:   len(bucket),
		depth:        0,
	})

	for node, step := range walkThread(root) {
		rows = append(rows, displayRow{
			msg:    node.msg,
			depth:  step.depth,
			prefix: buildPrefix(step.ancestorLastFlags, step.isLast),
		})
	}
	return rows
}

// buildPrefix renders the box-drawing prefix from the per-ancestor
// "is-last-sibling" trail and the current row's own last-sibling flag.
func buildPrefix(ancestorLastFlags []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorLastFlags {
		if last {
			b.WriteString(mlThreadGap)
		} else {
			b.WriteString(mlThreadVert)
		}
	}
	if isLast {
		b.WriteString(mlThreadElbow)
	} else {
		b.WriteString(mlThreadTee)
	}
	return b.String()
}

// applyFoldState mutates rows in place. For any folded root, the root's
// prefix becomes "[N] " (N = threadSize) and every row down to the next
// root is marked hidden.
func applyFoldState(rows []displayRow, folded map[mail.UID]bool) {
	for i := range len(rows) {
		if !rows[i].isThreadRoot {
			continue
		}
		if !folded[rows[i].msg.UID] {
			continue
		}
		rows[i].prefix = fmt.Sprintf("[%d] ", rows[i].threadSize)
		for j := i + 1; j < len(rows); j++ {
			if rows[j].isThreadRoot {
				break
			}
			rows[j].hidden = true
		}
	}
}

// SetSearchResults switches the model into cross-folder results mode.
// originByUID maps each result's UID to its origin folder name; that
// powers the `[Folder] ` sender prefix in the renderer. Threading is
// suppressed for the results pane: cross-folder threads aren't
// meaningful since a single thread can span multiple folders.
// ClearSearchResults returns to the prior in-folder source.
func (m *Model) SetSearchResults(msgs []mail.MessageInfo, originByUID map[mail.UID]string) {
	if !m.resultsMode {
		m.preResults = m.source
		m.preThreaded = m.threaded
		m.preCursorUID = m.cursorUID()
	}
	m.resultsMode = true
	m.originByUID = originByUID
	m.delegate.resultsMode = true
	m.delegate.originByUID = originByUID
	m.threaded = false
	m.source = msgs
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.list.Select(0)
}

// ClearSearchResults restores the pre-search source and threading
// flag. Cursor lands on the previously selected UID when it survives;
// otherwise it falls back to row 0.
func (m *Model) ClearSearchResults() {
	if !m.resultsMode {
		return
	}
	m.resultsMode = false
	m.originByUID = nil
	m.delegate.resultsMode = false
	m.delegate.originByUID = nil
	m.source = m.preResults
	m.threaded = m.preThreaded
	m.preResults = nil
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(m.preCursorUID)
}

// ResultsMode reports whether the model is currently displaying a
// cross-folder search result set.
func (m Model) ResultsMode() bool { return m.resultsMode }

// SetFilter applies a search filter and rebuilds rows. The query is
// parsed by internal/search so operators (`from:`, `subject:`) work
// in folder-local mode too. Bare terms match subject + from + to +
// cc. The first unfiltered→filtered transition snapshots the cursor
// row so ClearFilter can restore it. Later keystrokes leave the
// snapshot alone.
func (m *Model) SetFilter(q string) {
	if !m.savedByFilter && q != "" {
		m.preSearchCursor = m.list.Index()
		m.savedByFilter = true
	}
	m.filter = searchFilter{raw: q, query: lowerQueryTerms(search.Parse(q))}
	m.rebuild()
	m.list.Select(0)
}

// ClearFilter clears the active filter, rebuilds rows, and restores the
// pre-search cursor row when one was saved (clamped to 0 if it would
// land past the new end).
func (m *Model) ClearFilter() {
	m.filter = searchFilter{}
	m.rebuild()
	if m.savedByFilter {
		idx := m.preSearchCursor
		if idx >= len(m.list.Items()) {
			idx = 0
		}
		m.list.Select(idx)
		m.savedByFilter = false
	}
}

// FilterResultCount returns the number of threads matching the active
// filter (0 when no filter is active). The filter predicate runs per
// bucket so the count is always thread-shaped, not message-shaped.
func (m Model) FilterResultCount() int {
	return m.filterResults
}

// SetSort changes the thread-level sort direction. Children inside a
// thread always sort ascending regardless of this setting.
func (m *Model) SetSort(order SortOrder) {
	m.sort = order
	m.rebuild()
}

// SetThreaded toggles thread grouping. When false the list flattens to one
// row per message: sort and filter still apply, prefixes and fold state do
// not. Per-folder [ui.folders.<name>] threading = false flips this.
func (m *Model) SetThreaded(threaded bool) {
	if m.threaded == threaded {
		return
	}
	m.threaded = threaded
	m.rebuild()
}

// ToggleFold flips the fold state of the thread the cursor is inside.
// Cursor on a child row still toggles that child's root. After folding
// the cursor snaps to the nearest visible row.
func (m *Model) ToggleFold() {
	if len(m.rows) == 0 {
		return
	}
	uid := m.cursorUID()
	rowIdx := -1
	for i, r := range m.rows {
		if r.msg.UID == uid {
			rowIdx = i
			break
		}
	}
	rootIdx := m.threadRootIndex(rowIdx)
	if rootIdx < 0 {
		return
	}
	rootUID := m.rows[rootIdx].msg.UID
	m.folded[rootUID] = !m.folded[rootUID]
	m.rebuild()
	m.snapToVisible()
}

// ToggleFoldAll is the bulk counterpart to ToggleFold. If any multi-message
// thread is unfolded it folds every thread, otherwise it unfolds
// everything. The "mixed state → fold" direction matches the typical bulk
// reset: collapse the noise, then open the specific thread you're reading.
func (m *Model) ToggleFoldAll() {
	anyUnfolded := false
	for _, r := range m.rows {
		if r.isThreadRoot && r.threadSize > 1 && !m.folded[r.msg.UID] {
			anyUnfolded = true
			break
		}
	}
	if anyUnfolded {
		for _, r := range m.rows {
			if r.isThreadRoot && r.threadSize > 1 {
				m.folded[r.msg.UID] = true
			}
		}
	} else {
		m.folded = map[mail.UID]bool{}
	}
	m.rebuild()
	m.snapToVisible()
}

// snapToVisible re-anchors the list cursor on the nearest non-hidden
// row in m.rows. Called after fold toggles where the previously
// selected UID may now be hidden under a folded root.
func (m *Model) snapToVisible() {
	var uid mail.UID
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		uid = r.msg.UID
	}
	// Find uid in m.rows; if hidden, walk backwards to the nearest
	// visible row (always a thread root for hidden children).
	if uid != "" {
		for i, r := range m.rows {
			if r.msg.UID != uid {
				continue
			}
			if !r.hidden {
				m.snapToUIDInList(uid)
				return
			}
			for j := i - 1; j >= 0; j-- {
				if !m.rows[j].hidden {
					m.snapToUIDInList(m.rows[j].msg.UID)
					return
				}
			}
			break
		}
	}
	m.list.Select(0)
}

// threadRootIndex returns the row index of the thread root that owns the
// row at idx, or -1 if no root sits above idx.
func (m Model) threadRootIndex(idx int) int {
	if idx < 0 || idx >= len(m.rows) {
		return -1
	}
	for i := idx; i >= 0; i-- {
		if m.rows[i].isThreadRoot {
			return i
		}
	}
	return -1
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.delegate.width = width
	m.list.SetSize(width, height)
}

func (m Model) Layout() uicore.LayoutMode { return m.layout }

// SetLayout updates the column widths and date/flag toggles. A Date-width
// change triggers rebuild because dateText is precomputed per row;
// sender/flag widths take effect on the next render.
func (m *Model) SetLayout(l uicore.LayoutMode) {
	prevDate := m.layout.Date
	m.layout = l
	m.delegate.layout = l
	if prevDate != l.Date {
		m.rebuild()
	}
}

// SetNow overrides the clock snapshot used during rebuild. Test seam.
func (m *Model) SetNow(now time.Time) {
	m.now = now
	m.delegate.now = now
	m.rebuild()
}

// Selected returns the list cursor index over visible rows.
func (m Model) Selected() int { return m.list.Index() }

// SelectedMessage returns the message under the cursor.
func (m Model) SelectedMessage() (mail.MessageInfo, bool) {
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		return r.msg, true
	}
	return mail.MessageInfo{}, false
}

func (m Model) MessageByUID(uid mail.UID) (mail.MessageInfo, bool) {
	for i := range m.source {
		if m.source[i].UID == uid {
			return m.source[i], true
		}
	}
	return mail.MessageInfo{}, false
}

func (m Model) Count() int                 { return len(m.source) }
func (m Model) Source() []mail.MessageInfo { return m.source }

func (m Model) Rows() []Row {
	out := make([]Row, len(m.rows))
	for i, r := range m.rows {
		out[i] = Row{
			Msg:          r.msg,
			Prefix:       r.prefix,
			IsThreadRoot: r.isThreadRoot,
			ThreadSize:   r.threadSize,
			Hidden:       r.hidden,
			Depth:        r.depth,
		}
	}
	return out
}

func (m Model) VisibleCount() int {
	n := 0
	for _, r := range m.rows {
		if !r.hidden {
			n++
		}
	}
	return n
}

func (m Model) cursorUID() mail.UID {
	if r, ok := m.list.SelectedItem().(displayRow); ok {
		return r.msg.UID
	}
	return ""
}

// IsNearBottom reports whether the cursor is within k visible rows
// of the last visible row, used to trigger lazy-load before the
// user runs out of messages.
func (m *Model) IsNearBottom(k int) bool {
	n := len(m.list.Items())
	return n > 0 && m.list.Index() >= n-k
}

// AppendMessages adds extra to the source slice, rebuilds, and restores
// the cursor by UID. Lazy-load entry point for large folders.
func (m *Model) AppendMessages(extra []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = append(m.source, extra...)
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(uid)
}

// RefreshSource replaces the source slice and rebuilds rows while
// preserving the cursor UID, fold state, marks, and any active filter.
// Use for cache-driven refreshes. SetMessages is for fresh folder loads.
func (m *Model) RefreshSource(msgs []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = msgs
	m.now = time.Now()
	m.delegate.now = m.now
	m.rebuild()
	m.snapToUIDInList(uid)
}

// MoveCursor shifts by delta visible rows and returns the resulting
// UID plus whether the cursor moved. Boundaries are inert: calling
// at the first or last visible row returns ("", false). Programmatic
// entry point. The viewer's n/N path uses it; keyboard navigation
// goes through Update.
func (m *Model) MoveCursor(delta int) (mail.UID, bool) {
	before := m.list.Index()
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	for range delta {
		if step > 0 {
			m.list.CursorDown()
		} else {
			m.list.CursorUp()
		}
	}
	after := m.list.Index()
	if after == before {
		return "", false
	}
	return m.cursorUID(), true
}

// View renders the visible window. Empty list shows the centered
// placeholder; otherwise the embedded list.Model handles cursor +
// viewport and the rowDelegate paints each row.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.list.Items()) == 0 {
		return m.renderEmpty()
	}
	out := m.list.View()
	lines := strings.Split(out, "\n")
	for len(lines) < m.height {
		lines = append(lines, m.styles.MsgListBg.Width(m.width).Render(""))
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderEmpty() string {
	label := "No messages"
	if m.filter.raw != "" {
		label = "No matches"
	}
	labelLine := m.styles.MsgListBg.Width(m.width).
		Foreground(m.styles.MsgListPlaceholder.GetForeground()).
		Align(lipgloss.Center).
		Render(label)
	blank := m.styles.MsgListBg.Width(m.width).Render("")

	mid := m.height / 2
	lines := make([]string, m.height)
	for i := range lines {
		if i == mid {
			lines[i] = labelLine
		} else {
			lines[i] = blank
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) VisualMode() bool { return m.visualMode }

// EnterVisual enters visual-select mode without disturbing the marked set.
// Marks survive ExitVisual and are consumed by the next dispatch.
func (m *Model) EnterVisual() { m.visualMode = true }

// ExitVisual leaves visual-select mode and clears any marks.
func (m *Model) ExitVisual() {
	m.visualMode = false
	m.marked = map[mail.UID]struct{}{}
}

func (m *Model) ToggleMark(uid mail.UID) {
	if _, ok := m.marked[uid]; ok {
		delete(m.marked, uid)
		return
	}
	m.marked[uid] = struct{}{}
}

// Marked returns the marked UIDs in source order, or nil when empty.
func (m Model) Marked() []mail.UID {
	if len(m.marked) == 0 {
		return nil
	}
	out := make([]mail.UID, 0, len(m.marked))
	for _, msg := range m.source {
		if _, ok := m.marked[msg.UID]; ok {
			out = append(out, msg.UID)
		}
	}
	return out
}

// ActionTargets returns the UIDs a triage action should operate on:
// marks (in source order) when any are set, otherwise the cursor UID.
// A folded thread root expands to root + all child UIDs so the action
// matches what the user sees.
func (m Model) ActionTargets() []mail.UID {
	if len(m.marked) > 0 {
		return m.Marked()
	}
	row, ok := m.list.SelectedItem().(displayRow)
	if !ok {
		return nil
	}
	if row.isThreadRoot && row.threadSize > 1 && m.folded[row.msg.UID] {
		return m.threadUIDs(row.msg.UID)
	}
	return []mail.UID{row.msg.UID}
}

// threadUIDs returns root + all children sharing its ThreadID in source
// order.
func (m Model) threadUIDs(root mail.UID) []mail.UID {
	var threadID mail.UID
	for _, msg := range m.source {
		if msg.UID == root {
			threadID = msg.ThreadID
			break
		}
	}
	if threadID == "" {
		return []mail.UID{root}
	}
	out := []mail.UID{root}
	for _, msg := range m.source {
		if msg.UID != root && msg.ThreadID == threadID {
			out = append(out, msg.UID)
		}
	}
	return out
}

// truncateCells cuts s to width display cells, appending an ellipsis when
// truncated. Inputs are plain header text without ANSI escapes, so
// runewidth measures directly without lipgloss.Width's stripping pass.
func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}

// padRight pads s on the right with spaces to width display cells.
func padRight(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// padLeft pads s on the left with spaces to width display cells.
func padLeft(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return strings.Repeat(" ", width-w) + s
	}
	return s
}

// displayDate returns the date column text. Width 0 hides the column,
// width 3 picks the compact relative format, anything else picks short
// absolute.
func displayDate(msg mail.MessageInfo, now time.Time, width int) string {
	if width == 0 {
		return ""
	}
	t := msg.SentAt
	if t.IsZero() {
		return msg.Date
	}
	switch width {
	case 3:
		return formatRelativeDateCompact(t, now)
	default:
		return formatRelativeDateShort(t, now)
	}
}

func formatRelativeDateCompact(t, now time.Time) string {
	if t.IsZero() {
		return "   "
	}
	t = t.In(now.Location())
	delta := now.Sub(t)
	switch {
	case delta < 5*time.Minute && delta >= 0:
		return "now"
	case delta < time.Hour:
		return padRight(fmt.Sprintf("%dm", int(delta.Minutes())), 3)
	case delta < 24*time.Hour:
		return padRight(fmt.Sprintf("%dh", int(delta.Hours())), 3)
	case delta < 7*24*time.Hour:
		return padRight(fmt.Sprintf("%dd", int(delta.Hours()/24)), 3)
	case delta < 28*24*time.Hour:
		return padRight(fmt.Sprintf("%dw", int(delta.Hours()/(24*7))), 3)
	case t.Year() == now.Year():
		return t.Format("Jan")
	default:
		yy := t.Year() % 100
		return fmt.Sprintf("'%02d", yy)
	}
}

func formatRelativeDateShort(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(now.Location())
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	if ty == ny && tm == nm && td == nd {
		hour := t.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		ap := "a"
		if t.Hour() >= 12 {
			ap = "p"
		}
		return fmt.Sprintf("%d:%02d%s", hour, t.Minute(), ap)
	}
	return t.Format("01-02")
}
