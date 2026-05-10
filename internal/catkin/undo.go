package catkin

import "unicode/utf8"

const undoCap = 50

type snap struct {
	val string
	cur int
}

// undoRing is a bounded linear history of buffer snapshots.
// Index points at the current entry. Everything past it is the
// redo stack and is discarded on the next push.
type undoRing struct {
	snaps []snap
	idx   int
}

func (r *undoRing) seed(s snap) {
	r.snaps = []snap{s}
	r.idx = 0
}

// record pushes s onto the ring, discarding any redo entries.
// Successive intra-word edits coalesce: when the prior top and
// the incoming snapshot both end on a word rune, the top is
// replaced rather than appended.
func (r *undoRing) record(s snap) {
	if len(r.snaps) == 0 {
		r.seed(s)
		return
	}
	r.snaps = r.snaps[:r.idx+1]
	top := r.snaps[r.idx]
	if top.val == s.val {
		r.snaps[r.idx] = s
		return
	}
	if coalesce(top.val, s.val) {
		r.snaps[r.idx] = s
		return
	}
	r.snaps = append(r.snaps, s)
	if len(r.snaps) > undoCap {
		r.snaps = r.snaps[len(r.snaps)-undoCap:]
	}
	r.idx = len(r.snaps) - 1
}

// push is record without coalescing. The caller guarantees the edit
// deserves its own undo step regardless of surrounding text. Pastes
// are the canonical use.
func (r *undoRing) push(s snap) {
	if len(r.snaps) == 0 {
		r.seed(s)
		return
	}
	r.snaps = r.snaps[:r.idx+1]
	top := r.snaps[r.idx]
	if top.val == s.val {
		r.snaps[r.idx] = s
		return
	}
	r.snaps = append(r.snaps, s)
	if len(r.snaps) > undoCap {
		r.snaps = r.snaps[len(r.snaps)-undoCap:]
	}
	r.idx = len(r.snaps) - 1
}

func (r *undoRing) undo() (snap, bool) {
	if r.idx <= 0 {
		return snap{}, false
	}
	r.idx--
	return r.snaps[r.idx], true
}

func (r *undoRing) redo() (snap, bool) {
	if r.idx >= len(r.snaps)-1 {
		return snap{}, false
	}
	r.idx++
	return r.snaps[r.idx], true
}

// coalesce reports whether two snapshots are part of the same
// in-word edit run: both values non-empty and ending on a word
// rune. Whitespace, punctuation, or newline forces a fresh entry.
func coalesce(prev, next string) bool {
	return endsWithWordRune(prev) && endsWithWordRune(next)
}

func endsWithWordRune(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return isWordRune(r)
}
