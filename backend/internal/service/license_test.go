package service

import "testing"

func TestParseFlexLMOutputFeaturesSessionsAndExpiry(t *testing.T) {
	raw := `
Users of ansys:  (Total of 10 licenses issued;  Total of 3 licenses in use)
  "ansys" v2026.0101, vendor: ansys
  license expires: 31-dec-2026
    alice cae01 /dev/pts/0 (v2026.0101) (lic01/1055 1234), start Wed 7/8 10:00
    bob cae02 /dev/pts/1 (v2026.0101) (lic01/1055 5678), start Wed 7/8 10:05

Users of fluent:  (Total of 5 licenses issued;  Total of 0 licenses in use)
  license expires: permanent
`
	parsed := parseFlexLMOutput(raw)
	if len(parsed.Features) != 2 {
		t.Fatalf("features length = %d, want 2", len(parsed.Features))
	}
	if parsed.Features[0].Name != "ansys" || parsed.Features[0].Total != 10 || parsed.Features[0].Used != 3 {
		t.Fatalf("unexpected first feature: %+v", parsed.Features[0])
	}
	if parsed.Features[0].ExpiresAt == nil {
		t.Fatalf("expected ansys expiry to be parsed")
	}
	if parsed.Features[1].ExpiresAt != nil {
		t.Fatalf("expected permanent expiry to stay empty")
	}
	if len(parsed.Sessions) != 2 {
		t.Fatalf("sessions length = %d, want 2", len(parsed.Sessions))
	}
	if parsed.Sessions[0].Username != "alice" || parsed.Sessions[0].HostName != "cae01" {
		t.Fatalf("unexpected first session: %+v", parsed.Sessions[0])
	}
}

func TestLicenseCommandArgsRejectsUnsafeCommand(t *testing.T) {
	_, _, err := licenseCommandArgs(LicenseConfig{CollectCommand: "bash -lc whoami"})
	if err == nil {
		t.Fatalf("expected unsafe command to be rejected")
	}
}
