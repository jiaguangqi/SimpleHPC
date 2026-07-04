package httpapi

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeTerminalConfigDoesNotFallbackToLocalhost(t *testing.T) {
	config := normalizeTerminalConfig(map[string]any{})
	if len(config.Nodes) != 0 {
		t.Fatalf("empty terminal config should not create fallback nodes: %#v", config.Nodes)
	}
}

func TestSelectTerminalLoginNodeRequiresConfiguredNode(t *testing.T) {
	api := &API{}
	_, err := api.selectTerminalLoginNode(context.Background(), terminalConfigPayload{Strategy: "round_robin"}, "")
	if err == nil {
		t.Fatal("expected missing login node error")
	}
	if !strings.Contains(err.Error(), "系统设置") {
		t.Fatalf("missing node error should guide user to system settings, got %q", err.Error())
	}
}

