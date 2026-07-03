//go:build linux

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func secureOpenWithinRoot(root, path string, flags int, mode os.FileMode) (*os.File, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("path is outside configured storage roots")
	}
	rootFD, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootFD)

	how := &unix.OpenHow{
		Flags: uint64(flags | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Mode:  uint64(mode.Perm()),
		Resolve: unix.RESOLVE_BENEATH |
			unix.RESOLVE_NO_MAGICLINKS |
			unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(rootFD, relative, how)
	if err != nil {
		if errors.Is(err, unix.ENOSYS) {
			return secureOpenatFallback(rootFD, relative, path, flags, mode)
		}
		return nil, fmt.Errorf("secure open rejected path: %w", err)
	}
	return os.NewFile(uintptr(fd), path), nil
}

func secureOpenatFallback(rootFD int, relative, displayPath string, flags int, mode os.FileMode) (*os.File, error) {
	components := []string{}
	for _, component := range strings.Split(filepath.Clean(relative), string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		if component == ".." {
			return nil, fmt.Errorf("path is outside configured storage roots")
		}
		components = append(components, component)
	}

	currentFD := rootFD
	closeCurrent := false
	defer func() {
		if closeCurrent {
			_ = unix.Close(currentFD)
		}
	}()

	for _, component := range components[:max(0, len(components)-1)] {
		nextFD, err := unix.Openat(currentFD, component, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return nil, fmt.Errorf("secure open rejected path component: %w", err)
		}
		if closeCurrent {
			_ = unix.Close(currentFD)
		}
		currentFD = nextFD
		closeCurrent = true
	}

	final := "."
	if len(components) > 0 {
		final = components[len(components)-1]
	}
	fd, err := unix.Openat(currentFD, final, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return nil, fmt.Errorf("secure open rejected path: %w", err)
	}
	return os.NewFile(uintptr(fd), displayPath), nil
}
