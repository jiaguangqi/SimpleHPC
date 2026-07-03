//go:build !linux

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func secureOpenWithinRoot(root, path string, flags int, mode os.FileMode) (*os.File, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("path is outside configured storage roots")
	}
	current := root
	for _, component := range strings.Split(relative, string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) && flags&os.O_CREATE != 0 && current == path {
				break
			}
			return nil, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("secure open rejected symbolic link")
		}
	}
	return os.OpenFile(path, flags, mode)
}
