package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Client struct {
	Roots []string
}

type Root struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	Path             string   `json:"path"`
	EffectivePath    string   `json:"effectivePath,omitempty"`
	FSType           string   `json:"fsType"`
	Purpose          string   `json:"purpose"`
	WarningThreshold int      `json:"warningThreshold"`
	TotalBytes       *uint64  `json:"totalBytes,omitempty"`
	UsedBytes        *uint64  `json:"usedBytes,omitempty"`
	AvailableBytes   *uint64  `json:"availableBytes,omitempty"`
	UsagePercent     *float64 `json:"usagePercent,omitempty"`
	UsageError       string   `json:"usageError,omitempty"`
}

type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	Owner   string    `json:"owner"`
	Group   string    `json:"group"`
	ModTime time.Time `json:"modTime"`
}

type PathInfo struct {
	EffectivePath string `json:"effectivePath"`
	InitialPath   string `json:"initialPath"`
	CanGoParent   bool   `json:"canGoParent"`
	ParentPath    string `json:"parentPath,omitempty"`
}

func New(roots []string) *Client {
	return &Client{Roots: roots}
}

func (c *Client) ListRoots() []Root {
	roots := make([]Root, 0, len(c.Roots))
	for _, root := range c.Roots {
		item := describeRoot(root)
		item.attachUsage()
		roots = append(roots, item)
	}
	return roots
}

func (r *Root) attachUsage() {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(r.Path, &stat); err != nil {
		r.UsageError = err.Error()
		return
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	available := stat.Bavail * blockSize
	free := stat.Bfree * blockSize
	used := total - free
	r.TotalBytes = &total
	r.UsedBytes = &used
	r.AvailableBytes = &available
	if total > 0 {
		usage := float64(used) / float64(total) * 100
		r.UsagePercent = &usage
	}
}

func describeRoot(path string) Root {
	clean := filepath.Clean(path)
	root := Root{Path: clean, Name: clean, FSType: "POSIX", Purpose: "集群存储目录", WarningThreshold: 85}
	switch {
	case strings.Contains(clean, "home"):
		root.Type = "用户主目录"
		root.Name = "用户主目录"
		root.FSType = "NFS"
		root.Purpose = "用户家目录和团队默认组目录"
	case strings.Contains(clean, "share") || strings.Contains(clean, "project"):
		root.Type = "集群共享目录"
		root.Name = "集群共享目录"
		root.FSType = "GPFS"
		root.Purpose = "团队共享数据和项目目录"
	case strings.Contains(clean, "recycle") || strings.Contains(clean, "trash"):
		root.Type = "系统回收站"
		root.Name = "系统回收站"
		root.FSType = "NFS"
		root.Purpose = "删除账号家目录回收与保留"
	case strings.Contains(clean, "scratch") || strings.Contains(clean, "tmp"):
		root.Type = "临时计算目录"
		root.Name = "临时计算目录"
		root.FSType = "本地/并行存储"
		root.Purpose = "作业临时输入输出和高速缓存"
	default:
		root.Type = "存储目录"
	}
	return root
}

func (c *Client) List(path string, showHidden bool) ([]Entry, error) {
	clean, err := c.safePath(path)
	if err != nil {
		return nil, err
	}
	root, err := c.rootForPath(clean)
	if err != nil {
		return nil, err
	}
	directory, err := secureOpenWithinRoot(root, clean, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	items, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		if !showHidden && strings.HasPrefix(item.Name(), ".") {
			continue
		}
		info, err := item.Info()
		if err != nil {
			return nil, err
		}
		typ := "file"
		if info.IsDir() {
			typ = "directory"
		}
		entries = append(entries, Entry{
			Name: item.Name(), Path: filepath.Join(clean, item.Name()), Type: typ,
			Size: info.Size(), Mode: info.Mode().Perm().String(), Owner: fileOwner(info),
			Group: fileGroup(info), ModTime: info.ModTime(),
		})
	}
	return entries, nil
}

func (c *Client) CreateDirectory(parent, name string) error {
	target, err := c.safeNewPath(parent, name)
	if err != nil {
		return err
	}
	return os.Mkdir(target, 0o750)
}

func (c *Client) WriteFile(parent, name string, input io.Reader) error {
	target, err := c.safeNewPath(parent, name)
	if err != nil {
		return err
	}
	root, err := c.rootForPath(filepath.Dir(target))
	if err != nil {
		return err
	}
	file, err := secureOpenWithinRoot(root, target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, input); err != nil {
		_ = file.Close()
		_ = os.Remove(target)
		return err
	}
	return file.Close()
}

func (c *Client) Delete(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths selected")
	}
	if err := c.ValidatePaths(paths); err != nil {
		return err
	}
	for _, path := range paths {
		clean, err := c.safePath(path)
		if err != nil {
			return err
		}
		if c.isRoot(clean) {
			return fmt.Errorf("configured storage root cannot be deleted")
		}
		if err := c.removeAllSafe(clean); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Copy(paths []string, destination string) error {
	if err := c.ValidatePaths(paths); err != nil {
		return err
	}
	dest, err := c.destination(destination)
	if err != nil {
		return err
	}
	for _, source := range paths {
		clean, err := c.safePath(source)
		if err != nil {
			return err
		}
		if pathWithin(dest, clean) {
			return fmt.Errorf("destination cannot be inside source")
		}
		target := filepath.Join(dest, filepath.Base(clean))
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("destination already contains %s", filepath.Base(clean))
		}
		if err := c.copyPathSafe(clean, target); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Move(paths []string, destination string) error {
	if err := c.ValidatePaths(paths); err != nil {
		return err
	}
	dest, err := c.destination(destination)
	if err != nil {
		return err
	}
	for _, source := range paths {
		clean, err := c.safePath(source)
		if err != nil {
			return err
		}
		if c.isRoot(clean) {
			return fmt.Errorf("configured storage root cannot be moved")
		}
		if err := c.validateNoSymlinkTree(clean); err != nil {
			return err
		}
		if pathWithin(dest, clean) {
			return fmt.Errorf("destination cannot be inside source")
		}
		target := filepath.Join(dest, filepath.Base(clean))
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("destination already contains %s", filepath.Base(clean))
		}
		if err := os.Rename(clean, target); err != nil {
			if err := c.copyPathSafe(clean, target); err != nil {
				return err
			}
			if err := c.removeAllSafe(clean); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) Rename(path, name string) error {
	source, err := c.safePath(path)
	if err != nil {
		return err
	}
	if c.isRoot(source) {
		return fmt.Errorf("configured storage root cannot be renamed")
	}
	target, err := c.safeNewPath(filepath.Dir(source), name)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists")
	}
	return os.Rename(source, target)
}

func (c *Client) Archive(paths []string, output io.Writer) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths selected")
	}
	if err := c.ValidateArchive(paths); err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	for _, path := range paths {
		clean, err := c.safePath(path)
		if err != nil {
			_ = writer.Close()
			return err
		}
		root, err := c.rootForPath(clean)
		if err != nil {
			_ = writer.Close()
			return err
		}
		base := filepath.Base(clean)
		if err := c.archivePathSafe(writer, root, clean, base); err != nil {
			_ = writer.Close()
			return err
		}
	}
	return writer.Close()
}

func (c *Client) ValidateArchive(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths selected")
	}
	for _, path := range paths {
		clean, err := c.safePath(path)
		if err != nil {
			return err
		}
		root, err := c.rootForPath(clean)
		if err != nil {
			return err
		}
		if err := c.validateArchivePathSafe(root, clean, filepath.Base(clean)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) PathMetadata(path string) (PathInfo, error) {
	resolved, err := c.safePath(path)
	if err != nil {
		return PathInfo{}, err
	}
	for _, root := range c.Roots {
		resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
		if err != nil {
			continue
		}
		if resolved == resolvedRoot || strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			parent := ""
			if resolved != resolvedRoot {
				parent = filepath.Dir(resolved)
				if !pathWithin(parent, resolvedRoot) {
					parent = resolvedRoot
				}
			}
			return PathInfo{
				EffectivePath: resolved,
				InitialPath:   resolvedRoot,
				CanGoParent:   resolved != resolvedRoot,
				ParentPath:    parent,
			}, nil
		}
	}
	return PathInfo{}, fmt.Errorf("path is outside configured storage roots")
}

func (c *Client) ValidatePaths(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths selected")
	}
	for _, path := range paths {
		if _, err := c.safePath(path); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Open(path string) (*os.File, os.FileInfo, error) {
	clean, err := c.safePath(path)
	if err != nil {
		return nil, nil, err
	}
	root, err := c.rootForPath(clean)
	if err != nil {
		return nil, nil, err
	}
	file, err := secureOpenWithinRoot(root, clean, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func (c *Client) destination(path string) (string, error) {
	clean, err := c.safePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("destination must be a directory")
	}
	return clean, nil
}

func (c *Client) safeNewPath(parent, name string) (string, error) {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid file name")
	}
	cleanParent, err := c.safePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(cleanParent, name), nil
}

func (c *Client) safePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	clean := filepath.Clean(path)
	for _, root := range c.Roots {
		rootClean := filepath.Clean(root)
		resolvedRoot, err := filepath.EvalSymlinks(rootClean)
		if err != nil {
			return "", fmt.Errorf("storage root is not accessible: %w", err)
		}
		if clean != rootClean && !strings.HasPrefix(clean, rootClean+string(os.PathSeparator)) &&
			clean != resolvedRoot && !strings.HasPrefix(clean, resolvedRoot+string(os.PathSeparator)) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(clean)
		if err != nil {
			return "", err
		}
		if resolved == resolvedRoot || strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			return resolved, nil
		}
		return "", fmt.Errorf("path escapes configured storage root through a symbolic link")
	}
	return "", fmt.Errorf("path is outside configured storage roots")
}

func (c *Client) rootForPath(path string) (string, error) {
	clean := filepath.Clean(path)
	for _, root := range c.Roots {
		resolved, err := filepath.EvalSymlinks(filepath.Clean(root))
		if err != nil {
			continue
		}
		if pathWithin(clean, resolved) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("path is outside configured storage roots")
}

func (c *Client) isRoot(path string) bool {
	for _, root := range c.Roots {
		resolved, err := filepath.EvalSymlinks(filepath.Clean(root))
		if err == nil && resolved == path {
			return true
		}
	}
	return false
}

func (c *Client) removeAllSafe(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links cannot be deleted through file manager")
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	root, err := c.rootForPath(path)
	if err != nil {
		return err
	}
	directory, err := secureOpenWithinRoot(root, path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	items, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links cannot be deleted through file manager")
		}
		if err := c.removeAllSafe(filepath.Join(path, item.Name())); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

func (c *Client) validateNoSymlinkTree(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not allowed in this operation")
	}
	if !info.IsDir() {
		return nil
	}
	root, err := c.rootForPath(path)
	if err != nil {
		return err
	}
	directory, err := secureOpenWithinRoot(root, path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	items, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed in this operation")
		}
		if err := c.validateNoSymlinkTree(filepath.Join(path, item.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) copyPathSafe(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links cannot be copied")
	}
	if info.IsDir() {
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		root, err := c.rootForPath(source)
		if err != nil {
			return err
		}
		directory, err := secureOpenWithinRoot(root, source, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		items, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, item := range items {
			if item.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic links cannot be copied")
			}
			if err := c.copyPathSafe(filepath.Join(source, item.Name()), filepath.Join(target, item.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	sourceRoot, err := c.rootForPath(source)
	if err != nil {
		return err
	}
	targetRoot, err := c.rootForPath(filepath.Dir(target))
	if err != nil {
		return err
	}
	sourceFile, err := secureOpenWithinRoot(sourceRoot, source, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	targetFile, err := secureOpenWithinRoot(targetRoot, target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *Client) archivePathSafe(writer *zip.Writer, root, current, name string) error {
	info, err := os.Lstat(current)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links cannot be archived")
	}
	if !safeArchiveName(name) {
		return fmt.Errorf("unsafe archive member name")
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	if info.IsDir() {
		header.Name += "/"
		if _, err := writer.CreateHeader(header); err != nil {
			return err
		}
		directory, err := secureOpenWithinRoot(root, current, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		items, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, item := range items {
			if item.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic links cannot be archived")
			}
			if err := c.archivePathSafe(writer, root, filepath.Join(current, item.Name()), filepath.Join(name, item.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := secureOpenWithinRoot(root, current, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(entry, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *Client) validateArchivePathSafe(root, current, name string) error {
	info, err := os.Lstat(current)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links cannot be archived")
	}
	if !safeArchiveName(name) {
		return fmt.Errorf("unsafe archive member name")
	}
	if !info.IsDir() {
		return nil
	}
	directory, err := secureOpenWithinRoot(root, current, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	items, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links cannot be archived")
		}
		if err := c.validateArchivePathSafe(root, filepath.Join(current, item.Name()), filepath.Join(name, item.Name())); err != nil {
			return err
		}
	}
	return nil
}

func pathWithin(path, root string) bool {
	cleanPath, cleanRoot := filepath.Clean(path), filepath.Clean(root)
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator))
}

func safeArchiveName(name string) bool {
	clean := filepath.ToSlash(filepath.Clean(name))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") &&
		!strings.HasPrefix(clean, "/") && !strings.Contains(clean, "/../")
}

func fileOwner(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(uint64(stat.Uid), 10)
	}
	return ""
}

func fileGroup(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(uint64(stat.Gid), 10)
	}
	return ""
}
