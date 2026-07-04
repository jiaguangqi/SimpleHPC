package httpapi

import "testing"

func TestNormalizeLDAPConfigRequiresPrimaryURL(t *testing.T) {
	if _, err := normalizeLDAPConfig(nil, map[string]any{"baseDN": "dc=simplehpc,dc=local"}); err == nil {
		t.Fatal("expected primary ldap url validation error")
	}
}

func TestNormalizeLDAPConfigAcceptsStandbyURL(t *testing.T) {
	normalized, err := normalizeLDAPConfig(nil, map[string]any{
		"url":       "ldap://10.10.38.152:389",
		"backupUrl": "ldaps://ldap-backup.simplehpc.local:636",
		"baseDN":    "dc=simplehpc,dc=local",
	})
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if normalized["url"] != "ldap://10.10.38.152:389" {
		t.Fatalf("unexpected primary url: %#v", normalized["url"])
	}
	if normalized["backupUrl"] != "ldaps://ldap-backup.simplehpc.local:636" {
		t.Fatalf("unexpected backup url: %#v", normalized["backupUrl"])
	}
}

func TestNormalizeLDAPConfigPreservesSavedPassword(t *testing.T) {
	saved := map[string]any{
		"url":          "ldap://10.10.38.152:389",
		"bindPassword": "secret",
	}
	incoming := map[string]any{
		"url":       "ldap://10.10.38.152:389",
		"backupUrl": "ldap://10.10.38.153:389",
	}
	normalized, err := normalizeLDAPConfig(saved, incoming)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if normalized["bindPassword"] != "secret" {
		t.Fatalf("expected password to be preserved, got %#v", normalized["bindPassword"])
	}
}

func TestNormalizeLDAPConfigRejectsInvalidBackupURL(t *testing.T) {
	if _, err := normalizeLDAPConfig(nil, map[string]any{
		"url":       "ldap://10.10.38.152:389",
		"backupUrl": "http://10.10.38.153:389",
	}); err == nil {
		t.Fatal("expected invalid backup url validation error")
	}
}

func TestSanitizeLDAPConfigForResponseHidesPassword(t *testing.T) {
	sanitized := sanitizeLDAPConfigForResponse(map[string]any{
		"url":          "ldap://10.10.38.152:389",
		"backupUrl":    "ldap://10.10.38.153:389",
		"bindPassword": "secret",
	})
	if sanitized["bindPassword"] != "" {
		t.Fatalf("bindPassword should be empty, got %#v", sanitized["bindPassword"])
	}
	if sanitized["passwordConfigured"] != true {
		t.Fatalf("expected passwordConfigured flag, got %#v", sanitized["passwordConfigured"])
	}
	if sanitized["backupUrl"] != "ldap://10.10.38.153:389" {
		t.Fatalf("expected backup url to be preserved, got %#v", sanitized["backupUrl"])
	}
}
