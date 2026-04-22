package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

// setupCrashLog tees the process stderr fd to <profile>/crash.log so that any
// panic stack printed by bubbletea's recover (via debug.PrintStack → stderr)
// is captured to disk in addition to being written to the terminal. Returns
// the log path and a restore function that must be called to undo the redirect.
func setupCrashLog() (logPath string, restore func(), err error) {
	profileDir, err := config.ProfileDir()
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return "", nil, err
	}
	logPath = filepath.Join(profileDir, "crash.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", nil, err
	}

	origFd, err := syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		f.Close()
		return "", nil, err
	}
	origStderr := os.NewFile(uintptr(origFd), "origStderr")

	r, w, err := os.Pipe()
	if err != nil {
		origStderr.Close()
		f.Close()
		return "", nil, err
	}

	if err := syscall.Dup2(int(w.Fd()), int(os.Stderr.Fd())); err != nil {
		r.Close()
		w.Close()
		origStderr.Close()
		f.Close()
		return "", nil, err
	}

	fmt.Fprintf(f, "\n=== fylla session %s ===\n", time.Now().Format(time.RFC3339))

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				origStderr.Write(buf[:n])
				f.Write(buf[:n])
			}
			if rerr != nil {
				return
			}
		}
	}()

	restore = func() {
		syscall.Dup2(int(origStderr.Fd()), int(os.Stderr.Fd()))
		w.Close()
		<-done
		r.Close()
		origStderr.Close()
		f.Close()
	}
	return logPath, restore, nil
}
