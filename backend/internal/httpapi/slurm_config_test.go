package httpapi

import "testing"

func TestNormalizeSlurmConfigRequiresPrimaryController(t *testing.T) {
	if _, err := normalizeSlurmConfig(nil, map[string]any{"clusterName": "simplehpc"}); err == nil {
		t.Fatal("expected primary controller validation error")
	}
}

func TestNormalizeSlurmConfigPreservesSavedMySQLPassword(t *testing.T) {
	saved := map[string]any{
		"controllerHost": "10.10.38.152",
		"mysql": map[string]any{
			"host":          "10.10.38.152",
			"port":          "3306",
			"adminUser":     "root",
			"adminPassword": "secret",
		},
	}
	incoming := map[string]any{
		"controllerHost": "10.10.38.152",
		"mysql": map[string]any{
			"host":      "10.10.38.153",
			"port":      "3307",
			"adminUser": "slurmadmin",
		},
	}
	normalized, err := normalizeSlurmConfig(saved, incoming)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	mysql, ok := normalized["mysql"].(map[string]any)
	if !ok {
		t.Fatalf("expected mysql config, got %#v", normalized["mysql"])
	}
	if mysql["host"] != "10.10.38.153" || mysql["port"] != "3307" || mysql["adminUser"] != "slurmadmin" {
		t.Fatalf("unexpected mysql config: %#v", mysql)
	}
	if mysql["adminPassword"] != "secret" {
		t.Fatalf("expected password to be preserved, got %#v", mysql["adminPassword"])
	}
}

func TestNormalizeSlurmConfigRejectsInvalidMySQLPort(t *testing.T) {
	_, err := normalizeSlurmConfig(nil, map[string]any{
		"controllerHost": "10.10.38.152",
		"mysql": map[string]any{
			"host": "10.10.38.152",
			"port": "70000",
		},
	})
	if err == nil {
		t.Fatal("expected mysql port validation error")
	}
}

func TestSanitizeSlurmConfigForResponseHidesMySQLPassword(t *testing.T) {
	sanitized := sanitizeSlurmConfigForResponse(map[string]any{
		"controllerHost": "10.10.38.152",
		"mysql": map[string]any{
			"host":          "10.10.38.152",
			"port":          "3306",
			"adminUser":     "root",
			"adminPassword": "secret",
		},
	})
	mysql, ok := sanitized["mysql"].(map[string]any)
	if !ok {
		t.Fatalf("expected mysql config, got %#v", sanitized["mysql"])
	}
	if _, ok := mysql["adminPassword"]; ok {
		t.Fatalf("password should not be returned: %#v", mysql)
	}
	if mysql["passwordConfigured"] != true {
		t.Fatalf("expected passwordConfigured flag, got %#v", mysql)
	}
}
