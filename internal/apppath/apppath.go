package apppath

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirNextToExecutable returns the directory that contains the running binary.
func DirNextToExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}
	return filepath.Dir(executable), nil
}
