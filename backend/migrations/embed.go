package migrations

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

// Files contains forward and rollback SQL so the release binary never depends
// on its working directory.
//
//go:embed *.sql
var Files embed.FS

func ForwardFiles() ([]string, error) {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".sql") &&
			!strings.HasSuffix(name, ".down.sql") && name != "001_init.sql" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}
