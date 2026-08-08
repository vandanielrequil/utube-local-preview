package applog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu         sync.Mutex
	logFile    *os.File
	outW       *os.File
	errW       *os.File
	origStdout *os.File
	origStderr *os.File
	copyDone   chan struct{}
	logPath    string
)

// Init creates/truncates <exeName>.log next to the binary and tees all
// stdout/stderr (Go + child tools) into both console and that file.
func Init() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve executable symlink: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	logPath = filepath.Join(filepath.Dir(executable), baseName+".log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", logPath, err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		f.Close()
		return fmt.Errorf("create log pipe: %w", err)
	}

	mu.Lock()
	logFile = f
	outW = w
	errW = w
	origStdout = os.Stdout
	origStderr = os.Stderr
	os.Stdout = w
	os.Stderr = w
	copyDone = make(chan struct{})
	mu.Unlock()

	go func() {
		defer close(copyDone)
		_, _ = io.Copy(io.MultiWriter(origStdout, f), r)
		_ = r.Close()
	}()

	fmt.Printf("log: %s\n", logPath)
	return nil
}

// Close flushes the tee and restores stdout/stderr.
func Close() error {
	mu.Lock()
	w := outW
	f := logFile
	stdout := origStdout
	stderr := origStderr
	done := copyDone
	mu.Unlock()

	if w == nil {
		return nil
	}

	_ = w.Close()
	if done != nil {
		<-done
	}

	if stdout != nil {
		os.Stdout = stdout
	}
	if stderr != nil {
		os.Stderr = stderr
	}

	mu.Lock()
	outW = nil
	errW = nil
	logFile = nil
	origStdout = nil
	origStderr = nil
	copyDone = nil
	mu.Unlock()

	if f != nil {
		return f.Close()
	}
	return nil
}

// Path returns the active log file path (empty before Init).
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}
