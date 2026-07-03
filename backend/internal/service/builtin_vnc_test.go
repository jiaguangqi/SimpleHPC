package service

import (
	"strings"
	"testing"
)

func TestBuiltinNoVNCScriptOwnsRuntimeLifecycle(t *testing.T) {
	required := []string{
		`[ "$(id -un)" = "$SIMPLEHPC_SUBMIT_USER" ]`,
		`DISPLAY_NUM=$((100 + SLURM_JOB_ID % 400))`,
		`VNC_PORT=$((5900 + DISPLAY_NUM))`,
		`WS_PORT=$((20000 + SLURM_JOB_ID % 20000))`,
		`trap cleanup EXIT TERM INT`,
		`websockify --web=/opt/noVNC`,
		`"$SIMPLEHPC_CALLBACK_URL"`,
		`\"protocol\":\"vnc\"`,
	}
	for _, value := range required {
		if !strings.Contains(builtinNoVNCScript, value) {
			t.Fatalf("builtin noVNC script missing %q", value)
		}
	}
}
