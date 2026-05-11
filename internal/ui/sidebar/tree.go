package sidebar

import (
	"slices"
	"strings"
)

// rowMeta describes one visible sidebar row produced by walkCustom and
// consumed by renderRow.
type rowMeta struct {
	entry          folderEntry
	depth          int
	isLast         bool
	ancestorIsLast []bool
	aggUnread      int
	hasChildren    bool
	syntheticPath  string
}

// prefix returns the box-drawing column for this row. Empty for depth 0.
func (r rowMeta) prefix() string {
	if r.depth == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(r.depth * 3)
	for i := range r.depth - 1 {
		if i < len(r.ancestorIsLast) && r.ancestorIsLast[i] {
			b.WriteString("   ")
		} else {
			b.WriteString("│  ")
		}
	}
	if r.isLast {
		b.WriteString("└─ ")
	} else {
		b.WriteString("├─ ")
	}
	return b.String()
}

// expandKey is the map key used to look up expand state for this row.
func (r rowMeta) expandKey() string {
	if r.syntheticPath != "" {
		return r.syntheticPath
	}
	return r.entry.cf.Folder.Name
}

type node struct {
	name     string
	path     string
	entry    *folderEntry
	children map[string]*node
	order    []string
}

func newNode(name, path string) *node {
	return &node{name: name, path: path, children: map[string]*node{}}
}

// walkCustom builds visible rows from a flat list of custom folder entries.
// expanded keys control which intermediate nodes are open; nil means all
// collapsed. maxDepth > 0 caps the tree at that depth.
func walkCustom(custom []folderEntry, expanded map[string]bool, maxDepth int) []rowMeta {
	root := newNode("", "")
	for i := range custom {
		insertPath(root, &custom[i])
	}
	sortChildren(root)

	var out []rowMeta
	for i, childName := range root.order {
		isLast := i == len(root.order)-1
		visit(root.children[childName], 0, isLast, nil, expanded, maxDepth, &out)
	}
	return out
}

func insertPath(root *node, e *folderEntry) {
	segments := strings.Split(e.cf.Folder.Name, "/")
	cur := root
	for i, seg := range segments {
		next, ok := cur.children[seg]
		if !ok {
			path := strings.Join(segments[:i+1], "/")
			next = newNode(seg, path)
			cur.children[seg] = next
			cur.order = append(cur.order, seg)
		}
		cur = next
	}
	cur.entry = e
}

func sortChildren(n *node) {
	slices.Sort(n.order)
	for _, name := range n.order {
		sortChildren(n.children[name])
	}
}

func visit(n *node, depth int, isLast bool, ancestorIsLast []bool, expanded map[string]bool, maxDepth int, out *[]rowMeta) {
	atCap := maxDepth > 0 && depth >= maxDepth
	hasKids := len(n.order) > 0
	isExpanded := !atCap && expanded[n.path]

	var ancCopy []bool
	if len(ancestorIsLast) > 0 {
		ancCopy = append([]bool{}, ancestorIsLast...)
	}
	row := rowMeta{
		entry:          materializeEntry(n),
		depth:          depth,
		isLast:         isLast,
		ancestorIsLast: ancCopy,
		hasChildren:    hasKids,
		syntheticPath:  syntheticPath(n),
	}
	if hasKids && !isExpanded {
		// Collapsed parent: sum descendants only, excluding own Unseen.
		// renderRow adds entry.cf.Folder.Unseen separately, so aggUnread
		// must not double-count it.
		var descUnseen int
		for _, name := range n.order {
			descUnseen += subtreeUnseen(n.children[name])
		}
		row.aggUnread = descUnseen
	}
	*out = append(*out, row)

	if hasKids && isExpanded {
		childAncestors := append(append([]bool{}, ancestorIsLast...), isLast)
		for i, name := range n.order {
			childIsLast := i == len(n.order)-1
			visit(n.children[name], depth+1, childIsLast, childAncestors, expanded, maxDepth, out)
		}
	}
}

func subtreeUnseen(n *node) int {
	total := 0
	if n.entry != nil {
		total += n.entry.cf.Folder.Unseen
	}
	for _, name := range n.order {
		total += subtreeUnseen(n.children[name])
	}
	return total
}

func materializeEntry(n *node) folderEntry {
	if n.entry != nil {
		e := *n.entry
		e.cf.DisplayName = n.name
		return e
	}
	return folderEntry{cf: synthFolder(n.path, n.name)}
}

func syntheticPath(n *node) string {
	if n.entry == nil {
		return n.path
	}
	return ""
}
