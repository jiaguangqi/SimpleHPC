package service

import (
	"os"
	"path/filepath"
	"testing"

	storageintegration "simplehpc/backend/internal/integrations/storage"
)

func TestUserStorageRootsMapEveryConfiguredRoot(t *testing.T) {
	roots, err := UserStorageRoots([]string{"/data/home", "/data/share", "/data/recycle", "/data/scratch"}, "user001")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"/data/home/user001", "/data/share/user001", "/data/recycle/user001", "/data/scratch/user001"}
	for index := range expected {
		if roots[index] != expected[index] {
			t.Fatalf("root %d = %q, want %q", index, roots[index], expected[index])
		}
	}
}

func TestUserStorageRootsRejectUnsafeUsername(t *testing.T) {
	for _, username := range []string{"../user002", "user/002", "", "."} {
		if _, err := UserStorageRoots([]string{"/data/home"}, username); err == nil {
			t.Fatalf("expected username %q to be rejected", username)
		}
	}
}

func TestEnsureUserStorageDirectoriesCreatesPrivateDirectory(t *testing.T) {
	base := t.TempDir()
	roots, err := UserStorageRoots([]string{base}, "user001")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureUserStorageDirectories(roots, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(base, "user001"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", info.Mode().Perm())
	}
}

func TestScopedStorageRejectsRootOtherUserAndTraversal(t *testing.T) {
	base := t.TempDir()
	own := filepath.Join(base, "user001")
	other := filepath.Join(base, "user002")
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	client := storageintegration.New([]string{own})
	for _, path := range []string{base, other, filepath.Join(own, "..", "user002")} {
		if _, err := client.List(path, false); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	if _, err := client.List(own, false); err != nil {
		t.Fatalf("own directory must be allowed: %v", err)
	}
}

func TestExpandFilePoliciesKeepsRolesIndependentFromDataScopes(t *testing.T) {
	roots := []string{"/data/home", "/data/share", "/data/recycle", "/data/scratch"}
	tests := []struct {
		name     string
		username string
		units    []string
		teams    []string
		policies []FilePolicyGrant
		want     int
	}{
		{
			name: "ordinary user", username: "user001",
			policies: []FilePolicyGrant{
				{StorageRoot: "/data/home", SubjectScope: "self", Access: AccessManage},
				{StorageRoot: "/data/share", SubjectScope: "self", Access: AccessManage},
				{StorageRoot: "/data/recycle", SubjectScope: "self", Access: AccessManage},
				{StorageRoot: "/data/scratch", SubjectScope: "self", Access: AccessManage},
			}, want: 4,
		},
		{name: "config admin without policy", username: "config", want: 0},
		{
			name: "team admin explicit shared policy", username: "teamadmin", teams: []string{"21"},
			policies: []FilePolicyGrant{{StorageRoot: "/data/share", SubjectScope: "team_shared", Access: AccessManage}},
			want:     1,
		},
		{
			name: "unit admin explicit shared policy", username: "unitadmin", units: []string{"11"},
			policies: []FilePolicyGrant{{StorageRoot: "/data/share", SubjectScope: "unit_shared", Access: AccessManage}},
			want:     1,
		},
		{
			name: "cluster global", username: "cluster",
			policies: []FilePolicyGrant{{StorageRoot: "/data/home", SubjectScope: "global", Access: AccessManage}},
			want:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandFilePolicies(roots, tt.username, tt.units, tt.teams, tt.policies, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Fatalf("resolved roots = %#v, want %d", got, tt.want)
			}
		})
	}
}

func TestExpandFilePoliciesRejectsUnconfiguredRoot(t *testing.T) {
	_, err := ExpandFilePolicies(
		[]string{"/data/home"}, "user001", nil, nil,
		[]FilePolicyGrant{{StorageRoot: "/etc", SubjectScope: "global", Access: AccessManage}}, nil,
	)
	if err == nil {
		t.Fatal("unconfigured storage root policy was accepted")
	}
}

func TestStorageAuthorizationSeparatesReadAndManage(t *testing.T) {
	authz := StorageAuthorization{Roots: []ResolvedFileRoot{
		{Path: "/data/share/team", Access: AccessView},
		{Path: "/data/home/user001", Access: AccessManage, AllowHidden: true},
	}}
	if !authz.Allows("/data/share/team/a.txt", AccessView) {
		t.Fatal("read root denied")
	}
	if authz.Allows("/data/share/team/a.txt", AccessManage) {
		t.Fatal("read-only root allowed mutation")
	}
	if !authz.Allows("/data/home/user001/a.txt", AccessManage) {
		t.Fatal("manage root denied mutation")
	}
	if authz.Allows("/data/home/user002/a.txt", AccessView) {
		t.Fatal("other user root allowed")
	}
	if !authz.AllowsHidden("/data/home/user001") || authz.AllowsHidden("/data/share/team") {
		t.Fatal("hidden-file policy mismatch")
	}
}
