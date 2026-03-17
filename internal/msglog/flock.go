package msglog

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// FileLock provides cross-process advisory file locking using flock(2).
type FileLock struct {
	path string   // path to the .lock file
	file *os.File // open file handle for the lock
}

// NewFileLock creates a FileLock for the given path. The actual lock file
// is path + ".lock", so locking "foo.jsonl" uses "foo.jsonl.lock".
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path + ".lock"}
}

// Lock acquires an exclusive advisory lock, blocking until available.
func (fl *FileLock) Lock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o755); err != nil {
		return fmt.Errorf("mkdir for lock %s: %w", fl.path, err)
	}

	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock file %s: %w", fl.path, err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return fmt.Errorf("flock %s: %w", fl.path, err)
	}

	fl.file = f
	return nil
}

// Unlock releases the advisory lock and closes the file handle.
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}
	err := unix.Flock(int(fl.file.Fd()), unix.LOCK_UN)
	closeErr := fl.file.Close()
	fl.file = nil
	if err != nil {
		return fmt.Errorf("unlock %s: %w", fl.path, err)
	}
	return closeErr
}
