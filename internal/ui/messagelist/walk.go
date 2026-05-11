package messagelist

import "iter"

// walkStep carries the per-node prefix inputs walkThread yields
// alongside each *threadNode. ancestorLastFlags is the trail of
// "is-last-sibling" flags from root to this node's parent;
// isLast is the flag for this node itself.
type walkStep struct {
	depth             uint8
	ancestorLastFlags []bool
	isLast            bool
}

// walkThread visits every non-root descendant of root in
// depth-first root-then-children order. It yields the node along
// with the inputs buildPrefix needs to render the box-drawing
// prefix for that row.
//
// A break in the range loop stops the walk before the current
// node's children are visited.
func walkThread(root *threadNode) iter.Seq2[*threadNode, walkStep] {
	return func(yield func(*threadNode, walkStep) bool) {
		var rec func(n *threadNode, ancestors []bool) bool
		rec = func(n *threadNode, ancestors []bool) bool {
			for i, c := range n.children {
				isLast := i == len(n.children)-1
				step := walkStep{
					depth:             uint8(len(ancestors) + 1),
					ancestorLastFlags: ancestors,
					isLast:            isLast,
				}
				if !yield(c, step) {
					return false
				}
				if !rec(c, append(ancestors, isLast)) {
					return false
				}
			}
			return true
		}
		rec(root, nil)
	}
}
