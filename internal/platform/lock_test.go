package platform

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/uerr"
)

// holderEnvVar names the environment variable TestLockReleasedOnKill
// sets to re-invoke its own test binary as a lock-holding subprocess.
const holderEnvVar = "POPLAR_LOCK_HOLDER_DB_PATH"

// TestMain lets the test binary double as TestLockReleasedOnKill's
// subprocess: re-invoked with holderEnvVar set, it holds the lock and
// blocks instead of running the test suite.
func TestMain(m *testing.M) {
	if dbPath := os.Getenv(holderEnvVar); dbPath != "" {
		holdLockUntilKilled(dbPath)
	}
	os.Exit(m.Run())
}

// holdLockUntilKilled acquires dbPath's instance lock, prints a ready
// line so the parent test knows the lock is held, and blocks forever:
// the parent ends it with SIGKILL.
func holdLockUntilKilled(dbPath string) {
	if _, err := AcquireInstanceLock(dbPath); err != nil {
		fmt.Println("lock failed:", err)
		os.Exit(1)
	}
	fmt.Println("ready")
	select {}
}

// TestSecondInstanceRefused proves a second AcquireInstanceLock call
// against the same store path refuses with a uerr.Error under
// ClassInstanceLocked naming the first call's pid (SY-7, ADR-0015).
func TestSecondInstanceRefused(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	first, err := AcquireInstanceLock(dbPath)
	if err != nil {
		t.Fatalf("AcquireInstanceLock (first): %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = AcquireInstanceLock(dbPath)
	if err == nil {
		t.Fatal("AcquireInstanceLock (second) succeeded, want refusal")
	}

	var uerrErr uerr.Error
	if !errors.As(err, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", err)
	}
	if uerrErr.Class != uerr.ClassInstanceLocked {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassInstanceLocked)
	}

	wantPID := strconv.Itoa(os.Getpid())
	if !strings.Contains(uerrErr.Cause.Error(), wantPID) {
		t.Errorf("cause %q does not name the holder's pid %s", uerrErr.Cause.Error(), wantPID)
	}
}

// TestLockReleasedOnKill SIGKILLs the lock's holder and proves a new
// instance starts right after, with no stale-lock heuristic involved:
// the kernel drops the flock on process death by itself (ADR-0015).
func TestLockReleasedOnKill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	cmd := exec.Command(os.Args[0]) //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own lock-holding subprocess, never external input
	cmd.Env = append(os.Environ(), holderEnvVar+"="+dbPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start holder: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("holder exited before signaling ready: %v", scanner.Err())
	}
	if line := scanner.Text(); line != "ready" {
		t.Fatalf("holder's first line = %q, want %q", line, "ready")
	}

	if _, err := AcquireInstanceLock(dbPath); err == nil {
		t.Fatal("AcquireInstanceLock succeeded while the holder is alive, want refusal")
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("SIGKILL the holder: %v", err)
	}
	_ = cmd.Wait()

	second, err := AcquireInstanceLock(dbPath)
	if err != nil {
		t.Fatalf("AcquireInstanceLock after SIGKILL: %v", err)
	}
	_ = second.Release()
}
