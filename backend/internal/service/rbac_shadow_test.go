package service

import (
	"testing"
	"time"
)

func TestSummarizeRBACShadowTracksDifferencesByModule(t *testing.T) {
	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	summary := SummarizeRBACShadow([]RBACShadowComparison{
		{Permission: "api.jobs.list", LegacyAllowed: true, RBACAllowed: true, Matched: true},
		{Permission: "api.jobs.cancel", LegacyAllowed: true, RBACAllowed: false, Matched: false, Reason: "legacy_rbac_mismatch"},
		{Permission: "api.storage.files.list", LegacyAllowed: true, RBACAllowed: false, Matched: false, Reason: "resolver_error"},
	}, since)
	if summary.Total != 3 || summary.Matched != 1 || summary.Mismatched != 2 {
		t.Fatalf("unexpected totals: %#v", summary)
	}
	if summary.ResolverErrors != 1 || summary.MatchRate != 1.0/3.0 {
		t.Fatalf("unexpected quality metrics: %#v", summary)
	}
	if len(summary.ByModule) != 2 || summary.ByModule[0].Key != "jobs" ||
		summary.ByModule[0].Mismatched != 1 {
		t.Fatalf("unexpected module buckets: %#v", summary.ByModule)
	}
	if len(summary.Differences) != 2 || summary.Differences[0].Permission != "api.jobs.cancel" {
		t.Fatalf("unexpected differences: %#v", summary.Differences)
	}
}

func TestShadowSummaryEmptyWindowIsSafe(t *testing.T) {
	summary := SummarizeRBACShadow(nil, time.Now())
	if summary.Total != 0 || summary.MatchRate != 0 || summary.Mismatched != 0 {
		t.Fatalf("unexpected empty summary: %#v", summary)
	}
}

func TestShadowAuthorizedModuleCorpusHasZeroDifferences(t *testing.T) {
	modules := []string{
		"auth", "dashboard", "roles", "users", "teams", "units", "jobs",
		"queue", "nodes", "qos", "partitions", "storage", "templates",
		"inspection", "monitoring", "logs", "config",
	}
	items := make([]RBACShadowComparison, 0, len(modules))
	for _, module := range modules {
		items = append(items, RBACShadowComparison{
			Permission:    "api." + module + ".list",
			LegacyAllowed: true, RBACAllowed: true, Matched: true,
			Reason: "legacy_rbac_match",
		})
	}
	summary := SummarizeRBACShadow(items, time.Now())
	if summary.Total != len(modules) || summary.Mismatched != 0 || summary.MatchRate != 1 {
		t.Fatalf("authorized module corpus differs: %#v", summary)
	}
}
