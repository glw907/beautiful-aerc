package messagelist

import (
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestWalkThread_PrefixParity(t *testing.T) {
	// Build a tree:
	//   root
	//   ├── a
	//   │   └── a1
	//   └── b
	root := &threadNode{msg: mail.MessageInfo{UID: "root"}}
	a := &threadNode{msg: mail.MessageInfo{UID: "a"}}
	a1 := &threadNode{msg: mail.MessageInfo{UID: "a1"}}
	b := &threadNode{msg: mail.MessageInfo{UID: "b"}}
	a.children = []*threadNode{a1}
	root.children = []*threadNode{a, b}

	type seen struct {
		uid    mail.UID
		depth  uint8
		prefix string
	}
	var got []seen
	for node, step := range walkThread(root) {
		got = append(got, seen{
			uid:    node.msg.UID,
			depth:  step.depth,
			prefix: buildPrefix(step.ancestorLastFlags, step.isLast),
		})
	}

	want := []seen{
		{uid: "a", depth: 1, prefix: "├─ "},
		{uid: "a1", depth: 2, prefix: "│  └─ "},
		{uid: "b", depth: 1, prefix: "└─ "},
	}
	if len(got) != len(want) {
		t.Fatalf("walk yielded %d nodes, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestWalkThread_EarlyStop(t *testing.T) {
	root := &threadNode{msg: mail.MessageInfo{UID: "root"}}
	a := &threadNode{msg: mail.MessageInfo{UID: "a"}}
	b := &threadNode{msg: mail.MessageInfo{UID: "b"}}
	root.children = []*threadNode{a, b}

	count := 0
	for range walkThread(root) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break consumed %d nodes, want 1", count)
	}
}
