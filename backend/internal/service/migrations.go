package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	migrationfiles "simplehpc/backend/migrations"
)

const migrationAdvisoryLock int64 = 7364677801

type Migration struct {
	Version  int64
	Name     string
	Body     []byte
	Checksum string
}

func migrationChecksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func parseMigrationFilename(filename string) (int64, string, error) {
	if !strings.HasSuffix(filename, ".sql") || strings.HasSuffix(filename, ".down.sql") {
		return 0, "", fmt.Errorf("invalid migration filename %q", filename)
	}
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid migration filename %q", filename)
	}
	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("invalid migration version in %q", filename)
	}
	return version, parts[1], nil
}

func selectPendingMigrations(available []Migration, applied map[int64]string) ([]Migration, error) {
	sorted := append([]Migration(nil), available...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })
	pending := make([]Migration, 0, len(sorted))
	seen := map[int64]bool{}
	for _, migration := range sorted {
		if seen[migration.Version] {
			return nil, fmt.Errorf("duplicate migration version %d", migration.Version)
		}
		seen[migration.Version] = true
		if checksum, ok := applied[migration.Version]; ok {
			if checksum != migration.Checksum {
				return nil, fmt.Errorf("migration %d checksum drift", migration.Version)
			}
			continue
		}
		pending = append(pending, migration)
	}
	return pending, nil
}

func runMigrations(ctx context.Context, db *sql.DB, available []Migration) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLock); err != nil {
		return err
	}
	defer db.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLock)

	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return err
	}
	applied := map[int64]string{}
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		applied[version] = checksum
	}
	if err := rows.Close(); err != nil {
		return err
	}
	pending, err := selectPendingMigrations(available, applied)
	if err != nil {
		return err
	}
	for _, migration := range pending {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(migration.Body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %03d_%s: %w", migration.Version, migration.Name, err)
		}
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version,name,checksum) VALUES($1,$2,$3)`,
			migration.Version, migration.Name, migration.Checksum); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func embeddedMigrations() ([]Migration, error) {
	names, err := migrationfiles.ForwardFiles()
	if err != nil {
		return nil, err
	}
	result := make([]Migration, 0, len(names))
	for _, filename := range names {
		version, name, err := parseMigrationFilename(filename)
		if err != nil {
			return nil, err
		}
		body, err := migrationfiles.Files.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		result = append(result, Migration{
			Version: version, Name: name, Body: body, Checksum: migrationChecksum(body),
		})
	}
	return result, nil
}
