package service

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestMigrationChecksumIsStable(t *testing.T) {
	body := []byte("CREATE TABLE example(id bigint);")
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	if got := migrationChecksum(body); got != want {
		t.Fatalf("checksum = %q, want %q", got, want)
	}
}

func TestMigrationVersionFromFilename(t *testing.T) {
	version, name, err := parseMigrationFilename("002_rbac_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 || name != "rbac_schema" {
		t.Fatalf("got version=%d name=%q", version, name)
	}
	if _, _, err := parseMigrationFilename("rbac.sql"); err == nil {
		t.Fatal("invalid migration filename was accepted")
	}
}

func TestSelectPendingMigrationsRejectsChecksumDrift(t *testing.T) {
	available := []Migration{{Version: 2, Name: "rbac_schema", Checksum: "new"}}
	applied := map[int64]string{2: "old"}
	if _, err := selectPendingMigrations(available, applied); err == nil {
		t.Fatal("checksum drift was accepted")
	}
}

func TestSelectPendingMigrationsOrdersAndSkipsApplied(t *testing.T) {
	available := []Migration{
		{Version: 3, Name: "seed", Checksum: "c"},
		{Version: 1, Name: "init", Checksum: "a"},
		{Version: 2, Name: "schema", Checksum: "b"},
	}
	pending, err := selectPendingMigrations(available, map[int64]string{1: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || pending[0].Version != 2 || pending[1].Version != 3 {
		t.Fatalf("unexpected pending order: %#v", pending)
	}
}

func TestEmbeddedRBACMigrationsAreComplete(t *testing.T) {
	items, err := embeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 16 {
		t.Fatalf("embedded migrations = %d, want 16", len(items))
	}
	for index, version := range []int64{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17} {
		if items[index].Version != version || len(items[index].Body) == 0 || items[index].Checksum == "" {
			t.Fatalf("invalid embedded migration: %#v", items[index])
		}
	}
}
