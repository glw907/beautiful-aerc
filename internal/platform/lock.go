// Package platform holds poplar's OS-facing seams that answer to no
// single engine: the instance lock guarding a store against a second
// concurrent poplar (ADR-0015) today, and the opener, clipboard, and
// notification integrations later passes add.
package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofrs/flock"

	"github.com/glw907/poplar/internal/uerr"
)

// InstanceLock is poplar's exclusive claim on one store file, taken
// through gofrs/flock's LOCK_EX|LOCK_NB (ADR-0015). The kernel
// releases the underlying flock on any process death, SIGKILL
// included, so InstanceLock needs no stale-lock heuristic: Release is
// for a clean shutdown only.
type InstanceLock struct {
	fl *flock.Flock
}

// AcquireInstanceLock takes an exclusive, non-blocking lock on a file
// beside dbPath and records this process's pid inside it. It returns a
// uerr.Error under ClassInstanceLocked, naming the holder's pid, when
// another poplar process already holds the lock.
func AcquireInstanceLock(dbPath string) (*InstanceLock, error) {
	lockPath := dbPath + ".lock"
	fl := flock.New(lockPath)

	ok, err := fl.TryLock()
	if err != nil {
		return nil, uerr.New("platform.lock", nil, uerr.ClassStoreLocal, fmt.Errorf("lock %s: %w", lockPath, err))
	}
	if !ok {
		return nil, uerr.New("platform.lock", nil, uerr.ClassInstanceLocked,
			fmt.Errorf("poplar is already running (pid %d); stop it before starting another instance", holderPID(lockPath)))
	}

	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil { //nolint:gosec // G703: lockPath is dbPath+".lock", built from the store path this process already trusts, never user input
		_ = fl.Unlock()
		return nil, uerr.New("platform.lock", nil, uerr.ClassStoreLocal, fmt.Errorf("write pid to %s: %w", lockPath, err))
	}
	return &InstanceLock{fl: fl}, nil
}

// Release drops the lock, letting a future instance acquire it.
func (l *InstanceLock) Release() error {
	return l.fl.Unlock()
}

// holderPID reads the pid the current lock holder recorded at path,
// returning 0 if the file is empty or unreadable. The pid is advisory
// display data (ADR-0015), so a caller degrades to an unnamed pid
// rather than failing the refusal it is already reporting.
func holderPID(path string) int {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is dbPath+".lock", built from the store path this process already trusts, never user input
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}
