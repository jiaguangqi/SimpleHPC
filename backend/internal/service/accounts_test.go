package service

import (
	"strings"
	"testing"
)

func TestRandomPasswordMeetsAdminPolicy(t *testing.T) {
	password, err := randomPassword(18)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != 18 {
		t.Fatalf("password length = %d, want 18", len(password))
	}
	if !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatal("password must contain a lowercase letter")
	}
	if !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatal("password must contain an uppercase letter")
	}
	if !strings.ContainsAny(password, "0123456789") {
		t.Fatal("password must contain a digit")
	}
}

func TestValidateAdminDeletionProtectsSystemAccess(t *testing.T) {
	tests := []struct {
		name                string
		username            string
		status              string
		roleName            string
		activeCount         int
		activeClusterAdmins int
		wantError           bool
	}{
		{name: "system admin", username: "admin", status: "active", roleName: "cluster_admin", activeCount: 2, activeClusterAdmins: 2, wantError: true},
		{name: "last active admin", username: "testadmin", status: "active", roleName: "cluster_admin", activeCount: 1, activeClusterAdmins: 1, wantError: true},
		{name: "last cluster admin among other admins", username: "cluster1", status: "active", roleName: "cluster_admin", activeCount: 3, activeClusterAdmins: 1, wantError: true},
		{name: "another cluster admin", username: "cluster2", status: "active", roleName: "cluster_admin", activeCount: 3, activeClusterAdmins: 2, wantError: false},
		{name: "ordinary active admin", username: "config1", status: "active", roleName: "config_admin", activeCount: 2, activeClusterAdmins: 1, wantError: false},
		{name: "frozen admin", username: "auditadmin", status: "frozen", roleName: "config_admin", activeCount: 1, activeClusterAdmins: 1, wantError: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminDeletion(tt.username, tt.status, tt.roleName, tt.activeCount, tt.activeClusterAdmins)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateAdminDeletion() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCreateUserRejectsMissingRequiredFields(t *testing.T) {
	err := validateCreateUser(CreateUserInput{Username: "user051"})
	if err == nil || !strings.Contains(err.Error(), "姓名") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCreateUserRejectsInvalidLinuxNames(t *testing.T) {
	base := CreateUserInput{
		DisplayName: "User 001",
		Email:       "user001@example.edu.cn",
		Unit:        "unit-a",
		Team:        "team-a",
	}
	tests := []string{"User001", "user 001", " user001", "user001 ", "user.001", "1user", "root"}
	for _, username := range tests {
		t.Run(username, func(t *testing.T) {
			input := base
			input.Username = username
			err := validateCreateUser(input)
			if err == nil {
				t.Fatalf("validateCreateUser(%q) expected error", username)
			}
		})
	}
}

func TestValidateCreateUserRejectsInvalidHomeDirectory(t *testing.T) {
	base := CreateUserInput{
		Username:    "user001",
		DisplayName: "User 001",
		Email:       "user001@example.edu.cn",
		Unit:        "unit-a",
		Team:        "team-a",
	}
	tests := []string{" /data/home/user001", "/data/home/user001 ", "/data/home//user001", "/data/home/user001/../user002"}
	for _, home := range tests {
		t.Run(home, func(t *testing.T) {
			input := base
			input.HomeDirectory = home
			err := validateCreateUser(input)
			if err == nil {
				t.Fatalf("validateCreateUser(home=%q) expected error", home)
			}
		})
	}
}

func TestValidateCreateTeamRequiresUniqueNamesAndUnit(t *testing.T) {
	err := validateCreateTeam(CreateTeamInput{Name: "AI Lab", GroupName: "ai_lab"})
	if err == nil || !strings.Contains(err.Error(), "单位") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCreateTeamRejectsInvalidLDAPGroupNames(t *testing.T) {
	tests := []string{"高鸿祥组", "AI Lab", " team_a", "team_a ", "team.group", "2team", "root"}
	for _, groupName := range tests {
		t.Run(groupName, func(t *testing.T) {
			err := validateCreateTeam(CreateTeamInput{Name: "AI Lab", GroupName: groupName, Unit: "unit-a"})
			if err == nil {
				t.Fatalf("validateCreateTeam(%q) expected error", groupName)
			}
		})
	}
}

func TestValidateCreateTeamRejectsInvalidLeaderUsername(t *testing.T) {
	tests := []string{" leader001", "leader001 ", "Leader001", "leader.001", " "}
	for _, leaderUsername := range tests {
		t.Run(leaderUsername, func(t *testing.T) {
			err := validateCreateTeam(CreateTeamInput{Name: "AI Lab", GroupName: "ai_lab", Unit: "unit-a", LeaderUsername: leaderUsername})
			if err == nil {
				t.Fatalf("validateCreateTeam(leader=%q) expected error", leaderUsername)
			}
		})
	}
}

func TestValidateCreateTeamWithLeaderRequiresLeaderAsFirstUser(t *testing.T) {
	input := CreateTeamWithLeaderInput{
		Team:   CreateTeamInput{Name: "AI Lab", GroupName: "ai_lab", Unit: "unit-a", LeaderUsername: "leader001"},
		Leader: CreateUserInput{Username: "other001", DisplayName: "Leader", Email: "leader@example.edu.cn", Unit: "unit-a", Team: "ai_lab"},
	}
	_, err := validateCreateTeamWithLeader(input)
	if err == nil || !strings.Contains(err.Error(), "首个用户") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCreateTeamWithLeaderRejectsWhitespaceGroupName(t *testing.T) {
	input := CreateTeamWithLeaderInput{
		Team:   CreateTeamInput{Name: "ai_lab", GroupName: " ", Unit: "unit-a"},
		Leader: CreateUserInput{Username: "leader001", DisplayName: "Leader", Email: "leader@example.edu.cn"},
	}
	_, err := validateCreateTeamWithLeader(input)
	if err == nil {
		t.Fatal("validateCreateTeamWithLeader() expected error")
	}
}

func TestValidateCreateTeamWithLeaderDefaultsGroupAndLeader(t *testing.T) {
	input := CreateTeamWithLeaderInput{
		Team:   CreateTeamInput{Name: "ai_lab", Unit: "unit-a"},
		Leader: CreateUserInput{Username: "leader001", DisplayName: "Leader", Email: "leader@example.edu.cn"},
	}
	normalized, err := validateCreateTeamWithLeader(input)
	if err != nil {
		t.Fatalf("validateCreateTeamWithLeader() error = %v", err)
	}
	if normalized.Team.GroupName != "ai_lab" {
		t.Fatalf("group name = %q, want team name", normalized.Team.GroupName)
	}
	if normalized.Team.LeaderUsername != "leader001" {
		t.Fatalf("leader username = %q", normalized.Team.LeaderUsername)
	}
	if normalized.Leader.Team != "ai_lab" || normalized.Leader.Unit != "unit-a" {
		t.Fatalf("leader scope = team %q unit %q", normalized.Leader.Team, normalized.Leader.Unit)
	}
}

func TestValidateUnitCodeRejectsInvalidValues(t *testing.T) {
	tests := []string{"UnitA", "unit a", " unit_a", "unit_a ", "unit.a", "1unit", "root"}
	for _, code := range tests {
		t.Run(code, func(t *testing.T) {
			err := validateUnitCode(code)
			if err == nil {
				t.Fatalf("validateUnitCode(%q) expected error", code)
			}
		})
	}
}

func TestValidateHomeDirectoryRejectsTraversal(t *testing.T) {
	tests := []string{
		"data/home/user001",
		" /data/home/user001",
		"/data/home/user001 ",
		"/",
		"/data/home/user001/../user002",
		"/data/home//user001",
	}
	for _, home := range tests {
		t.Run(home, func(t *testing.T) {
			err := validateHomeDirectoryForUser(home, "user001")
			if err == nil {
				t.Fatalf("validateHomeDirectoryForUser(%q) expected error", home)
			}
		})
	}
}
