// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/mattn/go-runewidth"
)

// Column widths for the message list. Subject takes whatever remains
// after the fixed columns. The flag cell is 2 cells wide: Nerd Font
// SPUA-A glyphs render as 2 cells in real terminals, and the no-flag
// case pads to match (see renderFlagCell).
const (
	mlSenderWidth = 22
	mlDateWidth   = 14
	mlFlagWidth   = 2
	// cursor + sp×2 + flag(2) + sp×2 + sender + sp×2 + subject-pad + sp×2 + date + sp
	mlFixedWidth = 1 + 2 + mlFlagWidth + 2 + mlSenderWidth + 2 + 2 + mlDateWidth + 1
)

// mlCursorGlyph is the cursor indicator in column 0.
const mlCursorGlyph = "▐"

// Box-drawing tokens for thread prefixes. Each string is exactly 3
// display cells; buildPrefix relies on that to keep column math
// stable. Edit them as a set.
const (
	mlThreadVert  = "│  " // ancestor that still has more siblings below
	mlThreadGap   = "   " // ancestor that was the last sibling
	mlThreadTee   = "├─ " // current row, more siblings below
	mlThreadElbow = "└─ " // current row, last sibling
)

// SortOrder is the thread-level sort direction. Children inside a
// thread always sort chronologically ascending; SortOrder controls
// only the order of thread roots (and of unthreaded messages, which
// are single-message threads).
type SortOrder int

const (
	SortDateDesc SortOrder = iota // newest activity first (default)
	SortDateAsc                   // oldest activity first
)

// displayRow is one rendered row in the message list. The slice of
// these is computed from the source []MessageInfo by the build
// pipeline (group, sort, flatten). Hidden rows still occupy indices
// in the slice; the renderer skips them and j/k navigation walks
// past them.
type displayRow struct {
	msg          mail.MessageInfo
	prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	dateText     string // pre-rendered date column; computed in rebuild
	isThreadRoot bool
	threadSize   int   // set on roots only; 1 for unthreaded
	hidden       bool  // true when collapsed under a folded root
	depth        uint8 // 0 = root; derived during prefix computation
}

// MessageList renders the message list panel: flags, sender, subject,
// and date columns. Hand-rolled (not bubbles/list) to match the
// sidebar pattern and allow the ▐ cursor + selection background.
//
// MessageList owns thread grouping, fold state, and sort direction.
// The source slice is preserved alongside a derived []displayRow so
// fold mutations re-flatten without a backend refetch.
type MessageList struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	threaded bool
	selected int
	offset   int
	styles   Styles
	icons    IconSet
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

// searchFilter holds the active filter's query and mode. The zero
// value (empty query, SearchModeName) means "no filter."
type searchFilter struct {
	query string
	mode  SearchMode
}

// NewMessageList creates a MessageList with the given messages and size.
func NewMessageList(styles Styles, msgs []mail.MessageInfo, width, height int, icons IconSet) MessageList {
	m := MessageList{
		styles:   styles,
		icons:    icons,
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
func (m *MessageList) SetMessages(msgs []mail.MessageInfo) {
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
func (m *MessageList) rebuild() {
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
	// comparator runs in O(1); pairing with the bucket keeps the
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
		rows[i].dateText = displayDate(rows[i].msg, m.now)
	}
	m.rows = rows
}

// bucketByThreadID groups messages by their ThreadID, preserving
// input order within each bucket. Iterates the input twice (once to
// collect ThreadIDs in encounter order, once to slot messages) so the
// bucket order is deterministic — important for tests that compare
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
// it keeps any bucket containing at least one matching message — the
// thread-level predicate from ADR 0064.
func (m *MessageList) filterBuckets(buckets [][]mail.MessageInfo) [][]mail.MessageInfo {
	if m.filter.query == "" {
		return buckets
	}
	q := strings.ToLower(m.filter.query)
	out := buckets[:0]
	for _, bucket := range buckets {
		for _, msg := range bucket {
			if matchMessage(msg, q, m.filter.mode) {
				out = append(out, bucket)
				break
			}
		}
	}
	return out
}

// matchMessage tests one message against a pre-lowercased query under
// the given mode. [name] matches subject + sender; [all] additionally
// matches the date text. Each field is lowercased once per call.
func matchMessage(msg mail.MessageInfo, lowerQuery string, mode SearchMode) bool {
	if strings.Contains(strings.ToLower(msg.Subject), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(msg.From), lowerQuery) {
		return true
	}
	if mode == SearchModeAll && strings.Contains(strings.ToLower(msg.Date), lowerQuery) {
		return true
	}
	return false
}

// pickRoot returns the index within bucket of the message that should
// be treated as the thread root. Preference: the message with empty
// InReplyTo. Fallback: the earliest message by Sent time (or Date lex
// for legacy fixtures without a Sent time). The fallback handles
// broken parent chains (message references a parent that wasn't
// fetched) without crashing — the synthetic root and any other
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
// build pipeline. Empty bucket returns a zero-value MessageInfo — a
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
// both messages carry a non-zero SentAt; falls back to lexicographic
// comparison of the display Date for legacy fixtures that leave
// SentAt unset. Mixed cases (one has SentAt, one doesn't) sort the
// zero-SentAt message as the older of the pair — arbitrary but
// deterministic; real workers always populate SentAt so this branch
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
// this function — it's a scratch structure for prefix computation.
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
	// (broken chain — InReplyTo references a UID outside the bucket),
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
// not overwrite the saved row — the save gate stays armed until clear.
func (m *MessageList) SetFilter(q string, mode SearchMode) {
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
func (m *MessageList) ClearFilter() {
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
// active filter, or 0 if no filter is active. Thread count — not
// message count — because the filter predicate runs per bucket and
// keeps whole threads as units.
func (m MessageList) FilterResultCount() int {
	return m.filterResults
}

// SetSort changes the thread-level sort direction and re-runs the
// build pipeline. Children inside a thread always sort ascending
// regardless of this setting.
func (m *MessageList) SetSort(order SortOrder) {
	m.sort = order
	m.rebuild()
}

// SetThreaded toggles thread grouping. When true (the default),
// messages are bucketed by ThreadID and the rebuild pipeline emits a
// thread tree per bucket. When false, every message is its own bucket
// — display becomes flat (one row per message, no prefixes, no fold
// state) but sort and filter still apply. Per-folder
// `[ui.folders.<name>] threading = false` flips this.
func (m *MessageList) SetThreaded(threaded bool) {
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
func (m *MessageList) ToggleFold() {
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
func (m *MessageList) ToggleFoldAll() {
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
func (m *MessageList) snapToVisible() {
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
func (m MessageList) threadRootIndex(idx int) int {
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
func (m *MessageList) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.clampOffset()
}

// Selected returns the index of the currently selected message.
func (m MessageList) Selected() int { return m.selected }

// SelectedMessage returns the currently selected message. ok is false
// if the list is empty.
func (m MessageList) SelectedMessage() (mail.MessageInfo, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return mail.MessageInfo{}, false
	}
	return m.rows[m.selected].msg, true
}

// MessageByUID returns the message info for uid, or ok=false when not
// found in the source set.
func (m MessageList) MessageByUID(uid mail.UID) (mail.MessageInfo, bool) {
	for i := range m.source {
		if m.source[i].UID == uid {
			return m.source[i], true
		}
	}
	return mail.MessageInfo{}, false
}

// Count returns the number of source messages in the list.
func (m MessageList) Count() int { return len(m.source) }

// Source returns the underlying source message slice. The returned slice
// is a direct reference to internal state; callers must not modify it.
func (m MessageList) Source() []mail.MessageInfo { return m.source }

// cursorUID returns the UID under the cursor, or empty if no rows.
// Used as an anchor across rebuild.
func (m *MessageList) cursorUID() mail.UID {
	if len(m.rows) == 0 || m.selected >= len(m.rows) {
		return ""
	}
	return m.rows[m.selected].msg.UID
}

// snapToUID positions the cursor on the row whose UID matches uid.
// Falls back to clamp at len(rows)-1 when not found.
func (m *MessageList) snapToUID(uid mail.UID) {
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
func (m *MessageList) IsNearBottom(k int) bool {
	return len(m.rows) > 0 && m.selected >= len(m.rows)-k
}

// AppendMessages adds extra to the message list, re-runs the
// group→sort→flatten pipeline, and restores the cursor by UID.
// Used for lazy-loading the next window of a large folder. Safe
// against duplicate UIDs (rebuild dedups).
func (m *MessageList) AppendMessages(extra []mail.MessageInfo) {
	uid := m.cursorUID()
	m.source = append(m.source, extra...)
	m.now = time.Now()
	m.rebuild()
	m.snapToUID(uid)
}

// MarkSeen flips FlagSeen on the local copy of the message with the
// given UID. Used for optimistic display when the viewer opens an
// unread message — the backend MarkRead Cmd runs in parallel.
// Order and visibility don't change, so no rebuild is needed.
func (m *MessageList) MarkSeen(uid mail.UID) {
	for i := range m.source {
		if m.source[i].UID == uid {
			m.source[i].Flags |= mail.FlagSeen
		}
	}
	for i := range m.rows {
		if m.rows[i].msg.UID == uid {
			m.rows[i].msg.Flags |= mail.FlagSeen
		}
	}
}

// moveBy shifts the cursor by delta visible rows, walking past any
// hidden rows in the requested direction. Empty list is a no-op.
func (m *MessageList) moveBy(delta int) {
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
func (m *MessageList) MoveDown() { m.moveBy(1) }

// MoveUp retreats the cursor by one visible row.
func (m *MessageList) MoveUp() { m.moveBy(-1) }

// MoveCursor shifts the cursor by delta visible rows (negative for up,
// positive for down) and returns the resulting UID and whether the
// cursor actually moved. Boundaries are inert — at first/last visible
// row, calling with the corresponding direction returns ("", false).
// Hidden (folded) rows are skipped.
func (m *MessageList) MoveCursor(delta int) (mail.UID, bool) {
	before := m.selected
	m.moveBy(delta)
	if m.selected == before {
		return "", false
	}
	return m.cursorUID(), true
}

// MoveToTop jumps the cursor to the first visible row.
func (m *MessageList) MoveToTop() {
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
func (m *MessageList) MoveToBottom() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.rows[i].hidden {
			m.selected = i
			m.clampOffset()
			return
		}
	}
}

// HalfPageDown moves the cursor down by half the visible height.
func (m *MessageList) HalfPageDown() { m.moveBy(max(1, m.height/2)) }

// HalfPageUp moves the cursor up by half the visible height.
func (m *MessageList) HalfPageUp() { m.moveBy(-max(1, m.height/2)) }

// PageDown moves the cursor down by one full visible page.
func (m *MessageList) PageDown() { m.moveBy(max(1, m.height)) }

// PageUp moves the cursor up by one full visible page.
func (m *MessageList) PageUp() { m.moveBy(-max(1, m.height)) }

// clampOffset adjusts the viewport so the cursor stays visible.
func (m *MessageList) clampOffset() {
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
func (m MessageList) View() string {
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
func (m MessageList) renderRow(idx int, bgStyle lipgloss.Style) string {
	row := m.rows[idx]
	msg := row.msg
	isSelected := idx == m.selected
	isUnread := msg.Flags&mail.FlagSeen == 0

	// Cursor cell (1 col): ▐ when selected, blank otherwise.
	var cursor string
	if isSelected {
		cursor = applyBg(m.styles.MsgListCursor, bgStyle).Render(mlCursorGlyph)
	} else {
		cursor = bgStyle.Render(" ")
	}

	flag := m.renderFlagCell(msg, isUnread, bgStyle)

	// Sender / subject foreground depends on read state.
	senderStyle := m.styles.MsgListReadSender
	subjectStyle := m.styles.MsgListReadSubject
	if isUnread {
		senderStyle = m.styles.MsgListUnreadSender
		subjectStyle = m.styles.MsgListUnreadSubject
	}

	senderText := padRight(truncateCells(msg.From, mlSenderWidth), mlSenderWidth)
	sender := applyBg(senderStyle, bgStyle).Render(senderText)

	dateText := padLeft(truncateCells(row.dateText, mlDateWidth), mlDateWidth)
	date := applyBg(m.styles.MsgListDate, bgStyle).Render(dateText)

	// Subject column: prefix (in MsgListThreadPrefix style) followed by
	// the subject text (in the read/unread style), with the subject
	// truncated to fit whatever space remains after the prefix.
	//
	// mlFixedWidth budgets mlFlagWidth (2) cells for the flag. When the
	// flag cell holds a Nerd Font SPUA-A glyph, lipgloss.Width undercounts
	// it by (spuaCellWidth-1), so the row builder would allocate extra cells
	// to subject — making the assembled row wider than m.width.
	// Subtract spuaCount(flag)*(spuaCellWidth-1) from the subject budget so
	// that displayCells(assembled row) == m.width regardless of flag content.
	flagAdjust := 0
	if spuaCellWidth > 1 {
		flagAdjust = spuaCount(flag) * (spuaCellWidth - 1)
	}
	subjectWidth := max(1, m.width-mlFixedWidth-flagAdjust)
	prefixCells := runewidth.StringWidth(row.prefix)
	subjectCells := max(0, subjectWidth-prefixCells)

	prefixStyled := applyBg(m.styles.MsgListThreadPrefix, bgStyle).Render(row.prefix)
	subjectText := padRight(truncateCells(msg.Subject, subjectCells), subjectCells)
	subjectStyled := applyBg(subjectStyle, bgStyle).Render(subjectText)
	subject := prefixStyled + subjectStyled

	line := cursor +
		bgStyle.Render("  ") +
		flag +
		bgStyle.Render("  ") +
		sender +
		bgStyle.Render("  ") +
		subject +
		bgStyle.Render("  ") +
		date +
		bgStyle.Render(" ")

	return fillRowToWidth(line, m.width, bgStyle)
}

// renderFlagCell renders the flag column. Priority: flagged > answered >
// unread > none. Read state wins over flag state for color — only the
// unread+flagged case escalates to the warning accent. Read rows always
// use the dim icon style so the glyph dims with the rest of the row.
//
// The rendered output is always exactly mlFlagWidth display cells. In
// simple-icon mode, narrow glyphs (1 cell) are padded with one trailing
// space so flagged and unflagged rows keep the sender column aligned.
func (m MessageList) renderFlagCell(msg mail.MessageInfo, isUnread bool, bgStyle lipgloss.Style) string {
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
	rendered := applyBg(iconStyle, bgStyle).Render(glyph)
	// Pad with background spaces until the cell is exactly mlFlagWidth display
	// cells wide. In fancy mode the SPUA-A glyph already occupies 2 cells
	// (spuaCellWidth == 2), so displayCells == mlFlagWidth and the loop is a
	// no-op. In simple mode the narrow glyph is 1 cell, so one space is added.
	for displayCells(rendered) < mlFlagWidth {
		rendered += bgStyle.Render(" ")
	}
	return rendered
}

// renderBlankLine returns a blank line at panel width with the base
// message-list background.
func (m MessageList) renderBlankLine() string {
	return m.styles.MsgListBg.Width(m.width).Render("")
}

// renderEmpty renders the centered placeholder. Wording depends on
// why the list is empty: "No messages" when the source has no
// messages at all, "No matches" when a filter is active and matched
// nothing.
func (m MessageList) renderEmpty() string {
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
func (m MessageList) VisualMode() bool { return m.visualMode }

// EnterVisual enters visual-select mode. The marked set is unchanged.
func (m *MessageList) EnterVisual() { m.visualMode = true }

// ExitVisual leaves visual-select mode and clears the marked set.
func (m *MessageList) ExitVisual() {
	m.visualMode = false
	m.marked = map[mail.UID]struct{}{}
}

// ToggleMark flips membership of uid in the marked set.
func (m *MessageList) ToggleMark(uid mail.UID) {
	if _, ok := m.marked[uid]; ok {
		delete(m.marked, uid)
		return
	}
	m.marked[uid] = struct{}{}
}

// Marked returns the marked UIDs in source order. Returns nil when none
// are marked.
func (m MessageList) Marked() []mail.UID {
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
func (m MessageList) ActionTargets() []mail.UID {
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
func (m MessageList) threadUIDs(root mail.UID) []mail.UID {
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

// uidSet builds a membership set from a slice of UIDs for O(1) lookup.
func uidSet(uids []mail.UID) map[mail.UID]struct{} {
	s := make(map[mail.UID]struct{}, len(uids))
	for _, u := range uids {
		s[u] = struct{}{}
	}
	return s
}

// ApplyDelete removes uids from m.source and rebuilds rows. Cursor
// holds at the index of the first display-row that was removed,
// clamped to len(rows)-1; if the resulting list is empty, selected
// becomes 0.
func (m *MessageList) ApplyDelete(uids []mail.UID) {
	if len(uids) == 0 {
		return
	}
	rm := uidSet(uids)

	firstIdx := -1
	for i, r := range m.rows {
		if _, ok := rm[r.msg.UID]; ok {
			firstIdx = i
			break
		}
	}

	kept := m.source[:0:0]
	for _, msg := range m.source {
		if _, ok := rm[msg.UID]; !ok {
			kept = append(kept, msg)
		}
	}
	m.source = kept
	m.rebuild()

	switch {
	case len(m.rows) == 0:
		m.selected = 0
	case firstIdx < 0:
		if m.selected >= len(m.rows) {
			m.selected = len(m.rows) - 1
		}
	default:
		if firstIdx >= len(m.rows) {
			firstIdx = len(m.rows) - 1
		}
		m.selected = firstIdx
	}
}

// ApplyInsert re-inserts msgs into m.source preserving their original
// positions. positions[i] is the source-index where msgs[i] originally
// lived; positions must be sorted ascending. Used for inverse roll-back.
//
// The merge works in original-index space: the total original length is
// len(m.source)+len(msgs). For each original index 0..total-1, either
// the re-inserted item lives there (from positions) or the next surviving
// source element does. This correctly handles any combination of deleted
// and surviving items.
func (m *MessageList) ApplyInsert(msgs []mail.MessageInfo, positions []int) {
	if len(msgs) == 0 {
		return
	}
	total := len(m.source) + len(msgs)
	out := make([]mail.MessageInfo, 0, total)
	// Build a set of re-insert positions for fast lookup.
	posSet := make(map[int]int, len(positions)) // original-idx → msgs index
	for i, p := range positions {
		posSet[p] = i
	}
	si := 0 // index into current m.source (survivors)
	for orig := 0; orig < total; orig++ {
		if mi, ok := posSet[orig]; ok {
			out = append(out, msgs[mi])
		} else {
			out = append(out, m.source[si])
			si++
		}
	}
	m.source = out
	m.rebuild()
}

// ApplyFlag flips a flag on every msg in m.source whose UID is in uids.
func (m *MessageList) ApplyFlag(uids []mail.UID, flag mail.Flag, set bool) {
	if len(uids) == 0 {
		return
	}
	in := uidSet(uids)
	for i := range m.source {
		if _, ok := in[m.source[i].UID]; !ok {
			continue
		}
		if set {
			m.source[i].Flags |= flag
		} else {
			m.source[i].Flags &^= flag
		}
	}
	m.rebuild()
}

// ApplySeen marks read (set=true) or unread (set=false).
func (m *MessageList) ApplySeen(uids []mail.UID, seen bool) {
	m.ApplyFlag(uids, mail.FlagSeen, seen)
}

// SnapshotSource returns the messages whose UIDs are in uids, paired
// with their source-indexes. positions is sorted ascending. Used to
// build the inverse Cmd before an ApplyDelete.
func (m MessageList) SnapshotSource(uids []mail.UID) (msgs []mail.MessageInfo, positions []int) {
	in := uidSet(uids)
	for i, msg := range m.source {
		if _, ok := in[msg.UID]; ok {
			msgs = append(msgs, msg)
			positions = append(positions, i)
		}
	}
	return msgs, positions
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
