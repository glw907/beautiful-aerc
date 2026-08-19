//go:build perf && !race && linux && (amd64 || arm64)

package main

import (
	"os"
	"syscall"
	"testing"
)

// posixFadvDontNeed is Linux's POSIX_FADV_DONTNEED advice code: tell
// the kernel to evict path's cached pages.
const posixFadvDontNeed = 4

// perfDropPageCache advises the kernel to evict path's cached pages,
// so QA-1's cold run measures a genuine disk read rather than
// replaying the page cache this same process warmed moments earlier
// seeding the store. This needs no root, unlike
// /proc/sys/vm/drop_caches, and stays inside the stdlib-plus-benchstat
// dependency budget: a raw syscall rather than a golang.org/x/sys/unix
// import. amd64 and arm64 pass fadvise64's offset and length as plain
// 64-bit registers; other Linux architectures split them across
// register pairs and fall back to the approximation in
// perf_cachedrop_other_test.go.
func perfDropPageCache(t *testing.T, path string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s for cache drop: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	if _, _, errno := syscall.Syscall6(syscall.SYS_FADVISE64, f.Fd(), 0, 0, posixFadvDontNeed, 0, 0); errno != 0 {
		t.Fatalf("fadvise DONTNEED %s: %v", path, errno)
	}
}
