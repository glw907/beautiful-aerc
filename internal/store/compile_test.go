package store

import (
	"os/exec"
	"strings"
	"testing"
)

// TestReadHandleHasNoExec is a compile-failure fixture:
// testdata/readnoexec attempts a write through ReadPool, the read-only
// handle type, and this test asserts the package does not build. A
// runtime assertion cannot prove this; ReadPool simply has no Exec
// method to call, so only the compiler can (build machine section 3).
func TestReadHandleHasNoExec(t *testing.T) {
	out, err := exec.Command("go", "build", "./testdata/readnoexec").CombinedOutput()
	if err == nil {
		t.Fatalf("testdata/readnoexec built successfully, want a compile failure; output:\n%s", out)
	}
	if !strings.Contains(string(out), "Exec") {
		t.Fatalf("build failed for an unexpected reason, want an undefined Exec method; got:\n%s", out)
	}
}
