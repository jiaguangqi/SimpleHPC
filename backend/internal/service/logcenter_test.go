package service

import (
	"strings"
	"testing"
)

func TestSystemLogSourceWhitelist(t *testing.T) {
	for _, source := range []string{"simplehpc-backend", "slurmctld", "slurmd", "postgres", "redis", "ldap"} {
		if _, ok := systemLogSources[source]; !ok {
			t.Fatalf("expected source %q to be allowed", source)
		}
	}
	if _, ok := systemLogSources["../../etc/passwd"]; ok {
		t.Fatal("arbitrary source must not be allowed")
	}
}

func TestParseJournalLine(t *testing.T) {
	item := parseSystemLogLine("simplehpc-backend", "2026-06-30T17:00:00+08:00 cae simplehpc-backend[123]: request failed")
	if item.Source != "simplehpc-backend" || !strings.Contains(item.Message, "request failed") || item.Level != "error" {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestJournalMetadataHeaderIsNotAPlatformLog(t *testing.T) {
	if !isSystemLogMetadata("-- Logs begin at Tue 2026-06-16, end at Tue 2026-06-30. --") {
		t.Fatal("journal metadata header must be filtered")
	}
}

func TestAuditActionForWriteRequest(t *testing.T) {
	if got := AuditActionForRequest("POST", "/api/v1/inspection/runs/:id/notify"); got != "http.post.inspection.runs.notify" {
		t.Fatalf("unexpected action %q", got)
	}
	if got := AuditActionForRequest("GET", "/api/v1/slurm/jobs"); got != "" {
		t.Fatalf("GET should not be audited, got %q", got)
	}
}

func TestNormalizeAuthEventQueryCapsPageSize(t *testing.T) {
	query := normalizeAuthEventQuery(AuthEventQuery{Page: 0, PageSize: 1000, Event: " LOGIN "})
	if query.Page != 1 || query.PageSize != 100 || query.Event != "login" {
		t.Fatalf("unexpected normalized query: %#v", query)
	}
}
