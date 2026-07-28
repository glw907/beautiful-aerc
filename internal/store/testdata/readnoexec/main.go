// Package readnoexec is TestReadHandleHasNoExec's compile-failure
// fixture: it attempts a write through store.ReadPool, the read-only
// handle type, which has no Exec method. go build must fail here; a
// successful build is the regression.
package readnoexec

import (
	"context"

	"github.com/glw907/poplar/internal/store"
)

func attemptWrite(p *store.ReadPool) {
	p.Exec(context.Background(), "DELETE FROM message")
}
