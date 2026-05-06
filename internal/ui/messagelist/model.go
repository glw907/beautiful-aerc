// SPDX-License-Identifier: MIT

package messagelist

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/mattn/go-runewidth"
)

// mlFlagWidth is the width of the flag/status icon cell in display cells.
// Nerd Font SPUA-A glyphs render as 2 cells in real terminals, and the
// no-flag case pads to match (see renderFlagCell).
const mlFlagWidth = 2

// mlCursorGlyph is the cursor indicator in column 0.
const mlCursorGlyph = "▐"

// Box-drawing tokens for thread prefixes. Each string is exactly 3
// display cells. buildPrefix relies on that to keep column math
// stable. Edit them as a set.
const (
	mlThreadVert  = "│  " // ancestor that still has more siblings below
	mlThreadGap   = "   " // ancestor that was the last sibling
	mlThreadTee   = "├─ " // current row, more siblings below
	mlThreadElbow = "└─ " // current row, last sibling
)

// SortOrder is the thread-level sort direction. Children inside a
// thread always sort chronologically ascending. SortOrder controls
// only the order of thread roots (and of unthreaded messages, which
// are single-message threads).
type SortOrder int

const (
	SortDateDesc SortOrder = iota // newest activity first (default)
	SortDateAsc                   // oldest activity first
)

// Styles holds the subset of UI styles the message list needs.
// Populated from ui.Styles at construction time.
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

// Row is a rendered row in the message list. Exported for cross-package
// test assertions in account_tab_test (package ui). Production code
// does not use this type.
type Row struct {
	Msg          mail.MessageInfo
	Prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	IsThreadRoot bool
	ThreadSize   int  // set on roots only. 1 for unthreaded
	Hidden       bool // true when collapsed under a folded root
	Depth        uint8
}

// displayRow is the internal row type, kept unexported. Rows() converts
// it to Row for external consumers.
type displayRow struct {
	msg          mail.MessageInfo
	prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	dateText     string // pre-rendered date column, computed in rebuild
	isThreadRoot bool
	threadSize   int   // set on roots only. 1 for unthreaded
	hidden       bool  // true when collapsed under a folded root
	depth        uint8 // 0 = root, derived during prefix computation
}

// searchFilter holds the active filter's query and mode. The zero
// value (empty query, uicore.SearchModeName) means "no filter."
type searchFilter struct {
	query string
	mode  uicore.SearchMode
}

// Model renders the message list panel: flags, sender, subject,
// and date columns. Hand-rolled (not bubbles/list) to match the
// sidebar pattern and allow the ▐ cursor + selection background.
//
// Model owns thread grouping, fold state, and sort direction.
// The source slice is preserved alongside a derived []displayRow so
// fold mutations re-flatten without a backend refetch.
type Model struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	threaded bool
	selected int
	offset   int
	styles   Styles
	icons    uicore.IconSet
	layout   uicore.LayoutMode
	width    int
	height   int
	// now is the clock snapshot fed into displayDate during rebuild.
	// Captured at construction and refreshed on SetMessages so View
	// never has to call time.Now() itself (keeps I/O out of the
	// render path). Tests assign directly to freeze the clock.
	now             time.Time
	filter          searchFilter
	preSearchCursor int
	savedByFilter   bool
	filterResults   int
	// Visual-select mode state.
	visualMode bool
	marked     map[mail.UID]struct{}
}

// New creates a Model with the given messages and size.
// layout defaults to a legacy-compatible value (Sender=22, Date=5,
// FlagColumn=true) so callers that haven't yet called SetLayout (e.g.
// tests that bypass WindowSizeMsg) get sensible output.
func New(styles Styles, msgs []mail.MessageInfo, width, height int, icons uicore.IconSet) Model {
	m := Model{
		styles:   styles,
		icons:    icons,
		layout:   uicore.LayoutMode{Sender: 22, Date: 5, FlagColumn: true},
		width:    width,
		height:   height,
		folded:   map[mail.UID]bool{},
		marked:   map[mail.UID]struct{}{},
		sort:     SortDateDesc,
		threaded: true,
		now:      time.Now(),
	}
	m.SetMessages(msgs)
	return m
}

// SetMessages replaces the source slice and rebuilds the displayRow
// list. Resets fold state, cursor, viewport, and any active filter.
// Also refreshes the clock snapshot so newly-delivered messages get
// the same-day relative formatting.
func (m *Model) SetMessages(msgs []mail.MessageInfo) {
	m.source = msgs
	m.folded = map[mail.UID]bool{}
	m.marked = map[mail.UID]struct{}{}
	m.visualMode = false
	m.selected = 0
	m.offset = 0
	m.filter = searchFilter{}
	m.savedByFilter = false
	m.preSearchCursor = 0
	m.now = time.Now()
	m.rebuild()
}

// rebuild runs the group → sort → flatten pipeline against m.source
// and applies fold state, producing m.rows. Called from SetMessages
// and from any fold-mutating method.
//
// Pipeline:
//
//  1. Bucket by ThreadID.
//  2. Pick a root per bucket (empty InReplyTo, fallback earliest by date).
//  3. Sort threads by latest-activity in m.sort direction.
//  4. Walk each thread, emit displayRows root-then-children,
//     computing depth and box-drawing prefix.
//  5. Apply fold state.
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
	if m.filter.query != "" {
		m.filterResults = len(buckets)
	} else {
		m.filterResults = 0
	}
	// Precompute each bucket's latest-activity message so the
	// comparator runs in O(1). Pairing with the bucket keeps the
	// memoized value aligned across the in-place sort's swaps.
	type bucketSort struct {
		bucket []mail.MessageInfo
		latest mail.MessageInfo
	}
	pairs := make([]bucketSort, len(buckets))
	for i, b := range buckets {
		pairs[i] = bucketSort{bucket: b, latest: latestActivity(b)}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if m.sort == SortDateAsc {
			return lessMessage(pairs[i].latest, pairs[j].latest)
		}
		return lessMessage(pairs[j].latest, pairs[i].latest)
	})
	for i, p := range pairs {
		buckets[i] = p.bucket
	}

	rows := make([]displayRow, 0, len(m.source))
	for _, bucket := range buckets {
		rows = appendThreadRows(rows, bucket)
	}
	if m.filter.query == "" {
		applyFoldState(rows, m.folded)
	}
	for i := range rows {
		rows[i].dateText = displayDate(rows[i].msg, m.now, m.layout.Date)
	}
	m.rows = rows
}

// bucketByThreadID groups messages by their ThreadID, preserving
// input order within each bucket. Iterates the input twice (once to
// collect ThreadIDs in encounter order, once to slot messages) so the
// bucket order is deterministic, important for tests that compare
// against a specific layout.
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

// filterBuckets is the filter step of the build pipeline. When the
// filter query is empty, it returns buckets unchanged. When non-empty,
// it keeps any bucket containing at least one matching message. The
// thread-level predicate from ADR 0064.
func (m *Model) filterBuckets(buckets [][]mail.MessageInfo) [][]mail.MessageInfo {
	if m.filter.query == "" {
		return buckets
	}
	q := strings.ToLower(m.filter.query)
	out := buckets[:0]
	for _, bucket := range buckets {
		for _, msg := range bucket {
			if m.matchMessage(msg, q) {
				out = append(out, bucket)
				break
			}
		}
	}
	return out
}

// matchMessage tests one message against a pre-lowercased query
// under the active filter mode. [name] matches subject + sender;
// [all] additionally matches the rendered date text the user sees
// in the date column (not the wire RFC2822 string). Each field is
// lowercased once per call.
func (m *Model) matchMessage(msg mail.MessageInfo, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(msg.Subject), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(msg.From), lowerQuery) {
		return true
	}
	if m.filter.mode == uicore.SearchModeAll {
		dateText := displayDate(msg, m.now, m.layout.Date)
		if strings.Contains(strings.ToLower(dateText), lowerQuery) {
			return true
		}
	}
	return false
}

// pickRoot returns the index within bucket of the message that should
// be treated as the thread root. Preference: the message with empty
// InReplyTo. Fallback: the earliest message by Sent time (or Date lex
// for legacy fixtures without a Sent time). The fallback handles
// broken parent chains (message references a parent that wasn't
// fetched) without crashing. The synthetic root and any other
// top-level orphans become depth-1 children in the renderer.
func pickRoot(bucket []mail.MessageInfo) int {
	for i, m := range bucket {
		if m.InReplyTo == "" {
			return i
		}
	}
	earliest := 0
	for i, m := range bucket {
		if lessMessage(m, bucket[earliest]) {
			earliest = i
		}
	}
	return earliest
}

// latestActivity returns the message representing the thread's most
// recent activity. Used as the inter-thread sort key in step 5 of the
// build pipeline. Empty bucket returns a zero-value MessageInfo, a
// caller should not invoke on an empty bucket but the total-function
// return keeps downstream comparisons safe.
func latestActivity(bucket []mail.MessageInfo) mail.MessageInfo {
	var latest mail.MessageInfo
	for _, m := range bucket {
		if lessMessage(latest, m) {
			latest = m
		}
	}
	return latest
}

// lessMessage returns true if a is older than b. Uses SentAt when
// both messages carry a non-zero SentAt. Falls back to lexicographic
// comparison of the display Date for legacy fixtures that leave
// SentAt unset. Mixed cases (one has SentAt, one doesn't) sort the
// zero-SentAt message as the older of the pair, arbitrary but
// deterministic. Real workers always populate SentAt so this branch
// only fires for older unit-test fixtures.
func lessMessage(a, b mail.MessageInfo) bool {
	aZero := a.SentAt.IsZero()
	bZero := b.SentAt.IsZero()
	if !aZero && !bZero {
		return a.SentAt.Before(b.SentAt)
	}
	if aZero && bZero {
		return a.Date < b.Date
	}
	return aZero
}

// threadNode is a transient tree node used during prefix computation.
// The tree exists only for the duration of one appendThreadRows call;
// after the walk produces displayRows it's discarded.
type threadNode struct {
	msg      mail.MessageInfo
	children []*threadNode
}

// appendThreadRows builds a transient tree from one thread bucket,
// then emits displayRows in depth-first root-then-children order with
// the right prefix for each row's position. The tree never escapes
// this function. It's a scratch structure for prefix computation.
func appendThreadRows(rows []displayRow, bucket []mail.MessageInfo) []displayRow {
	rootIdx := pickRoot(bucket)
	root := &threadNode{msg: bucket[rootIdx]}

	// Index every message by UID so children can find their parent.
	byUID := map[mail.UID]*threadNode{}
	for i, msg := range bucket {
		if i == rootIdx {
			byUID[msg.UID] = root
			continue
		}
		byUID[msg.UID] = &threadNode{msg: msg}
	}

	// Hook each non-root child to its parent. If the parent is missing
	// (broken chain: InReplyTo references a UID outside the bucket),
	// fall back to attaching it to the root as a top-level child.
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

	// Sort children chronologically ascending at every level.
	var sortChildren func(n *threadNode)
	sortChildren = func(n *threadNode) {
		sort.SliceStable(n.children, func(i, j int) bool {
			return lessMessage(n.children[i].msg, n.children[j].msg)
		})
		for _, c := range n.children {
			sortChildren(c)
		}
	}
	sortChildren(root)

	// Emit the root.
	rows = append(rows, displayRow{
		msg:          root.msg,
		isThreadRoot: true,
		threadSize:   len(bucket),
		depth:        0,
	})

	// Walk children depth-first, building the prefix from the trail
	// of "is-last-sibling" flags at each ancestor level.
	var walk func(node *threadNode, ancestorLastFlags []bool)
	walk = func(node *threadNode, ancestorLastFlags []bool) {
		for i, child := range node.children {
			isLast := i == len(node.children)-1
			rows = append(rows, displayRow{
				msg:          child.msg,
				isThreadRoot: false,
				threadSize:   0,
				depth:        uint8(len(ancestorLastFlags) + 1),
				prefix:       buildPrefix(ancestorLastFlags, isLast),
			})
			walk(child, append(ancestorLastFlags, isLast))
		}
	}
	walk(root, nil)

	return rows
}

// buildPrefix constructs the box-drawing prefix string for a row.
// ancestorLastFlags has one entry per ancestor level above this row,
// indicating whether that ancestor was the last sibling at its own
// level. isLast reports whether the current row is the last sibling.
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

// applyFoldState mutates rows in place: for any folded thread root,
// every subsequent row up to the next root is marked hidden, and the
// root's prefix is replaced with "[N] " where N is threadSize.
func applyFoldState(rows []displayRow, folded map[mail.UID]bool) {
	for i := 0; i < len(rows); i++ {
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

// SetFilter applies a search filter to the message list, rebuilding
// the display rows through the filterBuckets pipeline step. On the
// first transition from unfiltered to filtered, saves the pre-search
// cursor row so ClearFilter can restore it. Subsequent keystrokes do
// not overwrite the saved row. The save gate stays armed until clear.
func (m *Model) SetFilter(q string, mode uicore.SearchMode) {
	if !m.savedByFilter && q != "" {
		m.preSearchCursor = m.selected
		m.savedByFilter = true
	}
	m.filter = searchFilter{query: q, mode: mode}
	m.rebuild()
	m.clampOffset()
}

// ClearFilter removes any active filter, rebuilds rows, and restores
// the pre-search cursor row if one was saved. A cursor that points
// past the new end of rows clamps to 0.
func (m *Model) ClearFilter() {
	m.filter = searchFilter{}
	m.rebuild()
	if m.savedByFilter {
		m.selected = m.preSearchCursor
		if m.selected >= len(m.rows) {
			m.selected = 0
		}
		m.savedByFilter = false
	}
	m.clampOffset()
}

// FilterResultCount returns the number of threads matching the
// active filter, or 0 if no filter is active. Thread count, not
// message count, because the filter predicate runs per bucket and
// keeps whole threads as units.
func (m Model) FilterResultCount() int {
	return m.filterResults
}

// SetSort changes the thread-level sort direction and re-runs the
// build pipeline. Children inside a thread always sort ascending
// regardless of this setting.
func (m *Model) SetSort(order SortOrder) {
	m.sort = order
	m.rebuild()
}

// SetThreaded toggles thread grouping. When true (the default),
// messages are bucketed by ThreadID and the rebuild pipeline emits a
// thread tree per bucket. When false, every message is its own bucket
// Flat display: one row per message, no prefixes, no fold
// state) but sort and filter still apply. Per-folder
// `[ui.folders.<name>] threading = false` flips this.
func (m *Model) SetThreaded(threaded bool) {
	if m.threaded == threaded {
		return
	}
	m.threaded = threaded
	m.rebuild()
}

// ToggleFold flips the fold state of the thread the cursor is
// currently inside. If the cursor is on a child row, the toggle still
// operates on that child's thread root. After folding, the cursor
// snaps to the nearest visible row so it doesn't land on a hidden one.
func (m *Model) ToggleFold() {
	if len(m.rows) == 0 {
		return
	}
	rootIdx := m.threadRootIndex(m.selected)
	if rootIdx < 0 {
		return
	}
	rootUID := m.rows[rootIdx].msg.UID
	m.folded[rootUID] = !m.folded[rootUID]
	m.rebuild()
	m.snapToVisible()
}

// ToggleFoldAll is the bulk toggle counterpart to ToggleFold: if any
// multi-message thread is currently unfolded it folds every thread,
// otherwise it unfolds everything. The "mixed state → fold" direction
// matches what users usually want from a bulk reset (collapse the
// noise, then open the specific thread you're reading).
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

// snapToVisible walks the cursor backwards to the nearest visible row
// after a rebuild. Children always sit below their thread root in the
// slice, so walking back from a hidden child lands on the root that
// owns it. Re-clamps the viewport.
func (m *Model) snapToVisible() {
	if m.selected < len(m.rows) && !m.rows[m.selected].hidden {
		m.clampOffset()
		return
	}
	for i := m.selected; i >= 0; i-- {
		if i < len(m.rows) && !m.rows[i].hidden {
			m.selected = i
			break
		}
	}
	m.clampOffset()
}

// threadRootIndex returns the row index of the thread root that owns
// the row at idx. Walks backwards from idx until it finds a row with
// isThreadRoot == true. Returns -1 if no root is found above idx.
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

// SetSize updates the panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.clampOffset()
}

// Layout returns the current layout settings. Used by cross-package tests.
func (m Model) Layout() uicore.LayoutMode { return m.layout }

// SetLayout updates the column widths and date/flag toggles. Date
// width changes trigger a rebuild because dateText is precomputed
// per row. Sender/flag widths take effect at next render.
func (m *Model) SetLayout(l uicore.LayoutMode) {
	prevDate := m.layout.Date
	m.layout = l
	if prevDate != l.Date {
		m.rebuild()
	}
}

// SetNow overrides the cached clock snapshot used by displayDate
// during rebuild. Tests use this to freeze time. Production code
// relies on the SetMessages-driven refresh.
func (m *Model) SetNow(now time.Time) {
	m.now = now
	m.rebuild()
}

// Selected returns the index of the currently selected message.
func (m Model) Selected() int { return m.selected }

// SelectedMessage returns the currently selected message. ok is false
// if the list is empty.
func (m Model) SelectedMessage() (mail.MessageInfo, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return mail.MessageInfo{}, false
	}
	return m.rows[m.selected].msg, true
}

// MessageByUID returns the message info for uid, or ok=false when not
// found in the source set.
func (m Model) MessageByUID(uid mail.UID) (mail.MessageInfo, bool) {
	for i := range m.source {
		if m.source[i].UID == uid {
			return m.source[i], true
		}
	}
	return mail.MessageInfo{}, false
}

// Count returns the number of source messages in the list.
func (m Model) Count() int { return len(m.source) }

// Source returns the underlying source message slice (read-only).
func (m Model) Source() []mail.MessageInfo { return m.source }

// Rows returns the rendered displayRow slice as exported Row values.
// Used by cross-package tests that need to inspect fold/thread state.
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

// VisibleCount returns the number of non-hidden rows.
func (m Model) VisibleCount() int {
	n := 0
	for _, r := range m.rows {
		if !r.hidden {
			n++
		}
	}
	return n
}

// cursorUID returns the UID under the cursor, or empty if no rows.
// Used as an anchor across rebuild.
func (m *Model) cursorUID() mail.UID {
	if len(m.rows) == 0 || m.selected >= len(m.rows) {
		return ""
	}
	return m.rows[m.selected].msg.UID
}

// snapToUID positions the cursor on the row whose UID matches uid.
// Falls back to clamp at len(rows)-1 when not found.
func (m *Model) snapToUID(uid mail.UID) {
	if uid == "" || len(m.rows) == 0 {
		m.selected = 0
		return
	}
	for i, r := range m.rows {
		if r.msg.UID == uid {
			m.selected = i
			return
		}
	}
	m.selected = len(m.rows) - 1
}

// IsNearBottom reports whether the cursor is within k rows of the
// last row. Used by AccountTab to trigger lazy-load before the user
// runs out of messages.
func (m *Model) IsNearBottom(k int) bool {
	return len(m.rows) > 0 && m.selected >= len(m.rows)-k
}

// AppendMessages adds extra to the message list, re-runs the
// group→sort→flatten pipeline, and restores the cursor by UID.
// Used for lazy-loading the next window of a large folder. Safe
// against duplicate UIDs (rebuild dedups).
func (m *Model) AppendMessages(extra []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = append(m.source, extra...)
	m.now = time.Now()
	m.rebuild()
	m.snapToUID(uid)
}

// RefreshSource replaces the source slice and rebuilds rows while
// preserving the cursor on the same UID, fold state, marks, and any
// active filter. Use this for cache-driven refreshes that should not
// disturb the user's view. SetMessages is for fresh folder loads.
func (m *Model) RefreshSource(msgs []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = msgs
	m.now = time.Now()
	m.rebuild()
	m.snapToUID(uid)
}

// moveBy shifts the cursor by delta visible rows, walking past any
// hidden rows in the requested direction. Empty list is a no-op.
func (m *Model) moveBy(delta int) {
	if len(m.rows) == 0 {
		return
	}
	if delta == 0 {
		m.clampOffset()
		return
	}

	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}

	idx := m.selected
	for delta > 0 {
		next := idx + step
		for next >= 0 && next < len(m.rows) && m.rows[next].hidden {
			next += step
		}
		if next < 0 || next >= len(m.rows) {
			break
		}
		idx = next
		delta--
	}
	m.selected = idx
	m.clampOffset()
}

// MoveDown advances the cursor by one visible row.
func (m *Model) MoveDown() { m.moveBy(1) }

// MoveUp retreats the cursor by one visible row.
func (m *Model) MoveUp() { m.moveBy(-1) }

// MoveCursor shifts the cursor by delta visible rows (negative for up,
// positive for down) and returns the resulting UID and whether the
// cursor actually moved. Boundaries are inert: at first/last visible
// row, calling with the corresponding direction returns ("", false).
// Hidden (folded) rows are skipped.
func (m *Model) MoveCursor(delta int) (mail.UID, bool) {
	before := m.selected
	m.moveBy(delta)
	if m.selected == before {
		return "", false
	}
	return m.cursorUID(), true
}

// MoveToTop jumps the cursor to the first visible row.
func (m *Model) MoveToTop() {
	for i := 0; i < len(m.rows); i++ {
		if !m.rows[i].hidden {
			m.selected = i
			m.offset = 0
			m.clampOffset()
			return
		}
	}
}

// MoveToBottom jumps the cursor to the last visible row.
func (m *Model) MoveToBottom() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.rows[i].hidden {
			m.selected = i
			m.clampOffset()
			return
		}
	}
}

// HalfPageDown moves the cursor down by half the visible height.
func (m *Model) HalfPageDown() { m.moveBy(max(1, m.height/2)) }

// HalfPageUp moves the cursor up by half the visible height.
func (m *Model) HalfPageUp() { m.moveBy(-max(1, m.height/2)) }

// PageDown moves the cursor down by one full visible page.
func (m *Model) PageDown() { m.moveBy(max(1, m.height)) }

// PageUp moves the cursor up by one full visible page.
func (m *Model) PageUp() { m.moveBy(-max(1, m.height)) }

// clampOffset adjusts the viewport so the cursor stays visible.
func (m *Model) clampOffset() {
	if m.height <= 0 {
		m.offset = 0
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.height {
		m.offset = m.selected - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// View renders the visible window of message rows. Empty state shows
// a centered "No messages" placeholder.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.rows) == 0 {
		return m.renderEmpty()
	}

	plainBg := m.styles.MsgListBg
	selectedBg := m.styles.MsgListSelected

	lines := make([]string, 0, m.height)
	visible := 0
	for i := m.offset; i < len(m.rows) && visible < m.height; i++ {
		if m.rows[i].hidden {
			continue
		}
		bg := plainBg
		if i == m.selected {
			bg = selectedBg
		}
		lines = append(lines, m.renderRow(i, bg))
		visible++
	}
	for len(lines) < m.height {
		lines = append(lines, m.renderBlankLine())
	}
	return strings.Join(lines, "\n")
}

// renderRow renders one message row at the configured width.
func (m Model) renderRow(idx int, bgStyle lipgloss.Style) string {
	row := m.rows[idx]
	msg := row.msg
	isSelected := idx == m.selected
	isUnread := msg.Flags&mail.FlagSeen == 0

	var cursor string
	if isSelected {
		cursor = uicore.ApplyBg(m.styles.MsgListCursor, bgStyle).Render(mlCursorGlyph)
	} else {
		cursor = bgStyle.Render(" ")
	}

	senderStyle := m.styles.MsgListReadSender
	subjectStyle := m.styles.MsgListReadSubject
	if isUnread {
		senderStyle = m.styles.MsgListUnreadSender
		subjectStyle = m.styles.MsgListUnreadSubject
	}

	senderText := padRight(truncateCells(msg.From, m.layout.Sender), m.layout.Sender)
	sender := uicore.ApplyBg(senderStyle, bgStyle).Render(senderText)

	var date string
	if m.layout.Date > 0 {
		dateText := padLeft(truncateCells(row.dateText, m.layout.Date), m.layout.Date)
		date = uicore.ApplyBg(m.styles.MsgListDate, bgStyle).Render(dateText)
	}

	// fixed: cursor(1) + sp2 + sp2(sender→subject) + sp2(subject→date) + sp(trail) = 8;
	// +flag(2) + sp2(flag→sender) = 12 with flag column. When Date=0 the trailing
	// sp2+date block is omitted. fillRowToWidth absorbs the 3-cell slack.
	var flag string
	fixed := 8
	if m.layout.FlagColumn {
		flag = m.renderFlagCell(msg, isUnread, bgStyle)
		fixed = 12
	}

	// When a SPUA-A glyph is in the flag cell, lipgloss.Width undercounts
	// it by (spuaCellWidth-1). Subtract the under-count from the subject
	// budget so displayCells(assembled row) == m.width regardless of flag
	// content.
	flagAdjust := 0
	if uicore.SPUACellWidth() > 1 && m.layout.FlagColumn {
		flagAdjust = uicore.SpuaCount(flag) * (uicore.SPUACellWidth() - 1)
	}
	subjectWidth := max(1, m.width-fixed-m.layout.Sender-m.layout.Date-flagAdjust)
	prefixCells := lipgloss.Width(row.prefix)
	subjectCells := max(0, subjectWidth-prefixCells)

	prefixStyled := uicore.ApplyBg(m.styles.MsgListThreadPrefix, bgStyle).Render(row.prefix)
	subjectText := padRight(truncateCells(msg.Subject, subjectCells), subjectCells)
	subjectStyled := uicore.ApplyBg(subjectStyle, bgStyle).Render(subjectText)
	subject := prefixStyled + subjectStyled

	sp2 := bgStyle.Render("  ")
	sp1 := bgStyle.Render(" ")

	var rowStr string
	if m.layout.FlagColumn {
		rowStr = cursor + sp2 + flag + sp2 + sender + sp2 + subject
	} else {
		rowStr = cursor + sp2 + sender + sp2 + subject
	}
	if m.layout.Date > 0 {
		rowStr += sp2 + date
	}
	rowStr += sp1

	return uicore.FillRowToWidth(rowStr, m.width, bgStyle)
}

// renderFlagCell renders the flag column. Priority: flagged > answered >
// unread > none. Read state wins over flag state for color. Only the
// unread+flagged case escalates to the warning accent. Read rows always
// use the dim icon style so the glyph dims with the rest of the row.
//
// The rendered output is always exactly mlFlagWidth display cells. In
// simple-icon mode, narrow glyphs (1 cell) are padded with one trailing
// space so flagged and unflagged rows keep the sender column aligned.
func (m Model) renderFlagCell(msg mail.MessageInfo, isUnread bool, bgStyle lipgloss.Style) string {
	iconStyle := m.styles.MsgListIconRead
	if isUnread {
		iconStyle = m.styles.MsgListIconUnread
	}
	var glyph string
	switch {
	case msg.Flags&mail.FlagFlagged != 0:
		glyph = m.icons.FlagFlagged
		if isUnread {
			iconStyle = m.styles.MsgListFlagFlagged
		}
	case msg.Flags&mail.FlagAnswered != 0:
		glyph = m.icons.FlagAnswered
	case isUnread:
		glyph = m.icons.FlagUnread
	default:
		return bgStyle.Render("  ")
	}
	rendered := uicore.ApplyBg(iconStyle, bgStyle).Render(glyph)
	// Pad with background spaces until the cell is exactly mlFlagWidth display
	// cells wide. In fancy mode the SPUA-A glyph already occupies 2 cells
	// (spuaCellWidth == 2), so displayCells == mlFlagWidth and the loop is a
	// no-op. In simple mode the narrow glyph is 1 cell, so one space is added.
	for uicore.DisplayCells(rendered) < mlFlagWidth {
		rendered += bgStyle.Render(" ")
	}
	return rendered
}

// renderBlankLine returns a blank line at panel width with the base
// message-list background.
func (m Model) renderBlankLine() string {
	return m.styles.MsgListBg.Width(m.width).Render("")
}

// renderEmpty renders the centered placeholder. Wording depends on
// why the list is empty: "No messages" when the source has no
// messages at all, "No matches" when a filter is active and matched
// nothing.
func (m Model) renderEmpty() string {
	label := "No messages"
	if m.filter.query != "" {
		label = "No matches"
	}
	labelLine := m.styles.MsgListBg.Width(m.width).
		Foreground(m.styles.MsgListPlaceholder.GetForeground()).
		Align(lipgloss.Center).
		Render(label)

	mid := m.height / 2
	lines := make([]string, m.height)
	for i := range lines {
		if i == mid {
			lines[i] = labelLine
		} else {
			lines[i] = m.renderBlankLine()
		}
	}
	return strings.Join(lines, "\n")
}

// VisualMode reports whether the list is in visual-select mode.
func (m Model) VisualMode() bool { return m.visualMode }

// EnterVisual enters visual-select mode. The marked set is unchanged.
func (m *Model) EnterVisual() { m.visualMode = true }

// ExitVisual leaves visual-select mode and clears the marked set.
func (m *Model) ExitVisual() {
	m.visualMode = false
	m.marked = map[mail.UID]struct{}{}
}

// ToggleMark flips membership of uid in the marked set.
func (m *Model) ToggleMark(uid mail.UID) {
	if _, ok := m.marked[uid]; ok {
		delete(m.marked, uid)
		return
	}
	m.marked[uid] = struct{}{}
}

// Marked returns the marked UIDs in source order. Returns nil when none
// are marked.
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

// ActionTargets returns the UIDs a triage action should operate on.
// If any UIDs are marked, those are returned in source order.
// Otherwise the cursor UID is returned. For a folded thread root,
// the cursor case expands to root + all child UIDs (WYSIWYG).
func (m Model) ActionTargets() []mail.UID {
	if len(m.marked) > 0 {
		return m.Marked()
	}
	if m.selected < 0 || m.selected >= len(m.rows) {
		return nil
	}
	row := m.rows[m.selected]
	if row.isThreadRoot && row.threadSize > 1 && m.folded[row.msg.UID] {
		return m.threadUIDs(row.msg.UID)
	}
	return []mail.UID{row.msg.UID}
}

// threadUIDs returns the root UID followed by all child UIDs in source
// order. Children are identified by matching ThreadID.
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

// NewStyles builds a messagelist.Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		MsgListBg:            lipgloss.NewStyle().Background(t.BgBase),
		MsgListSelected:      lipgloss.NewStyle().Background(t.BgSelection),
		MsgListCursor:        lipgloss.NewStyle().Foreground(t.AccentPrimary),
		MsgListUnreadSender:  lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		MsgListUnreadSubject: lipgloss.NewStyle().Foreground(t.FgBright),
		MsgListReadSender:    lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListReadSubject:   lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListDate:          lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListIconUnread:    lipgloss.NewStyle().Foreground(t.FgBright),
		MsgListIconRead:      lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListFlagFlagged:   lipgloss.NewStyle().Foreground(t.ColorWarning),
		MsgListThreadPrefix:  lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListPlaceholder:   lipgloss.NewStyle().Foreground(t.FgDim),
	}
}

// truncateCells cuts s to fit width display cells, appending an
// ellipsis when truncated. Inputs are plain mail header text (no ANSI
// escapes), so runewidth handles cell measurement directly without
// the ANSI-stripping pass that lipgloss.Width would do.
func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}

// padRight right-pads s with spaces to width display cells. Input is
// plain text (post-truncateCells), so runewidth measures directly.
func padRight(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// padLeft left-pads s with spaces to width display cells. Input is
// plain text (post-truncateCells), so runewidth measures directly.
func padLeft(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return strings.Repeat(" ", width-w) + s
	}
	return s
}

// displayDate returns the date column text for a message at the given
// width. Width 0 means no column. Width 3 uses compact relative format;
// other widths use short absolute format.
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

// formatRelativeDateCompact returns a 3-cell relative date string.
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

// formatRelativeDateShort returns a 5-cell short date string.
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
