package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTemplateRejectsUnsafeVariable(t *testing.T) {
	tpl := JobTemplate{
		Name: "unsafe", Kind: "batch", ScriptTemplate: "echo ok",
		FormSchema: []TemplateField{{ID: "name", Type: "text", Label: "名称", Variable: "NAME;rm"}},
	}
	if err := ValidateTemplate(tpl); err == nil {
		t.Fatal("ValidateTemplate accepted an unsafe variable")
	}
}

func TestRenderTemplateShellQuotesUserValues(t *testing.T) {
	tpl := JobTemplate{
		Name: "batch", Kind: "batch", ScriptTemplate: "printf '%s\\n' \"$INPUT_FILE\"",
		FormSchema: []TemplateField{{ID: "input", Type: "text", Label: "输入文件", Variable: "INPUT_FILE", Required: true}},
	}
	script, err := RenderTemplateScript(tpl, map[string]any{
		"input":   "results/a'; touch /tmp/pwned; echo '",
		"jobName": "safe-job", "partition": "debug", "nodes": 1, "cpus": 2, "gpus": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `export INPUT_FILE='results/a'"'"'; touch /tmp/pwned; echo '"'"''`) {
		t.Fatalf("script did not safely quote value:\n%s", script)
	}
	if !strings.Contains(script, "#SBATCH --job-name=safe-job") || !strings.Contains(script, "#SBATCH --partition=debug") {
		t.Fatalf("script missing generated Slurm directives:\n%s", script)
	}
}

func TestRenderTemplateScriptPlacesSlurmDirectivesBeforeFirstCommand(t *testing.T) {
	tpl := JobTemplate{Name: "batch", Kind: "batch", ScriptTemplate: "sleep 1"}
	script, err := RenderTemplateScript(tpl, map[string]any{
		"jobName": "ten-core-job",
		"cpus":    10,
		"workdir": "/data/home/testadmin",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstCommand := strings.Index(script, "set -euo pipefail")
	for _, directive := range []string{
		"#SBATCH --job-name=ten-core-job",
		"#SBATCH --cpus-per-task=10",
		"#SBATCH --chdir=/data/home/testadmin",
		"#SBATCH --output=slurm-%j.out",
		"#SBATCH --error=slurm-%j.err",
	} {
		position := strings.Index(script, directive)
		if position < 0 {
			t.Fatalf("script missing %q:\n%s", directive, script)
		}
		if position > firstCommand {
			t.Fatalf("%q appears after the first shell command, so Slurm will ignore it:\n%s", directive, script)
		}
	}
}

func TestRenderNoVNCTemplateDefaultsToTwentyFourHours(t *testing.T) {
	tpl := JobTemplate{Name: "desktop", Kind: "novnc", ScriptTemplate: "wait"}
	script, err := RenderTemplateScript(tpl, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "#SBATCH --time=24:00:00") {
		t.Fatalf("noVNC script missing default 24-hour walltime:\n%s", script)
	}
}

func TestResolveNoVNCSubmitUserRequiresSameLinuxIdentity(t *testing.T) {
	exists := func(username string) error {
		if username == "user001" {
			return nil
		}
		return errors.New("not found")
	}
	got, err := resolveTemplateSubmitUser(AuthUser{Username: "user001", Type: "ldap"}, "novnc", exists)
	if err != nil || got != "user001" {
		t.Fatalf("resolveTemplateSubmitUser() = %q, %v", got, err)
	}
	if _, err := resolveTemplateSubmitUser(AuthUser{Username: "testadmin", Type: "admin"}, "novnc", exists); err == nil {
		t.Fatal("admin without a same-name Linux identity was allowed to submit noVNC")
	}
	got, err = resolveTemplateSubmitUser(AuthUser{Username: "testadmin", Type: "admin"}, "batch", exists)
	if err != nil || got != "root" {
		t.Fatalf("batch admin submit user = %q, %v", got, err)
	}
}

func TestResolveNoVNCSubmitUserRejectsUnsafeUsernameBeforeLookup(t *testing.T) {
	called := false
	_, err := resolveTemplateSubmitUser(AuthUser{Username: "user001;id", Type: "ldap"}, "novnc", func(username string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("unsafe noVNC submit username was accepted")
	}
	if called {
		t.Fatal("lookup was called for an unsafe username")
	}
}

func TestNoVNCAccessURLIncludesGatewayPassword(t *testing.T) {
	run := TemplateRun{AccessToken: "1234567890abcdef1234567890abcdef", Protocol: "vnc"}
	url := templateRunAccessURL(run)
	for _, expected := range []string{
		"/api/v1/job-template-gateway/1234567890abcdef1234567890abcdef/vnc.html",
		"path=api%2Fv1%2Fjob-template-gateway%2F1234567890abcdef1234567890abcdef%2Fwebsockify",
		"password=12345678",
	} {
		if !strings.Contains(url, expected) {
			t.Fatalf("access URL %q missing %q", url, expected)
		}
	}
}

func TestValidateSubmissionEnforcesNumericBounds(t *testing.T) {
	tpl := JobTemplate{
		Name: "bounded", Kind: "batch", ScriptTemplate: "true",
		FormSchema: []TemplateField{{ID: "gpu", Type: "number", Label: "GPU", Variable: "GPU_COUNT", Min: floatPtr(0), Max: floatPtr(8)}},
	}
	if _, err := RenderTemplateScript(tpl, map[string]any{"gpu": 9}); err == nil {
		t.Fatal("RenderTemplateScript accepted a value above max")
	}
}

func TestRenderTemplateScriptExportsReservedRuntimeValues(t *testing.T) {
	tpl := JobTemplate{Name: "desktop", Kind: "novnc", ScriptTemplate: `curl "$SIMPLEHPC_CALLBACK_URL"`}
	script, err := RenderTemplateScript(tpl, map[string]any{
		"SIMPLEHPC_RUN_TOKEN":    "run-token",
		"SIMPLEHPC_ACCESS_TOKEN": "access-token",
		"SIMPLEHPC_CALLBACK_URL": "http://controller/api/register",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"export SIMPLEHPC_RUN_TOKEN='run-token'",
		"export SIMPLEHPC_ACCESS_TOKEN='access-token'",
		"export SIMPLEHPC_CALLBACK_URL='http://controller/api/register'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("rendered script missing %q:\n%s", expected, script)
		}
	}
}

func TestRenderTemplateScriptUsesComponentSlurmOptions(t *testing.T) {
	tpl := JobTemplate{
		Name: "component slurm", Kind: "batch", ScriptTemplate: "python train.py --input \"$INPUT_FILE\"",
		FormSchema: []TemplateField{
			{ID: "job", Type: "slurm-job-name", Label: "作业名称", Control: "text", Variable: "JOB_NAME", SlurmOption: "--job-name", Required: true},
			{ID: "account", Type: "slurm-account", Label: "项目名称", Control: "project", Variable: "PROJECT_ACCOUNT", SlurmOption: "--account", Required: true},
			{ID: "nodes", Type: "slurm-nodes", Label: "节点数", Control: "number", Variable: "NODE_COUNT", SlurmOption: "--nodes", Min: floatPtr(1), Max: floatPtr(64)},
			{ID: "gpu_per_node", Type: "slurm-gpus-per-node", Label: "每节点 GPU", Control: "number", Variable: "GPU_PER_NODE", SlurmOption: "--gpus-per-node", Min: floatPtr(0), Max: floatPtr(8)},
			{ID: "mail", Type: "slurm-mail-user", Label: "通知邮箱", Control: "text", Variable: "MAIL_USER", SlurmOption: "--mail-user"},
			{ID: "input", Type: "app-file", Label: "输入文件", Control: "file", Variable: "INPUT_FILE", Required: true},
		},
	}
	script, err := RenderTemplateScript(tpl, map[string]any{
		"job":          "ansys-run-01",
		"account":      "demo-cfd",
		"nodes":        2,
		"gpu_per_node": 1,
		"mail":         "user@example.com",
		"input":        "/data/home/user/model.dat",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"#SBATCH --job-name=ansys-run-01",
		"#SBATCH --account=demo-cfd",
		"#SBATCH --nodes=2",
		"#SBATCH --gpus-per-node=1",
		"#SBATCH --mail-user=user@example.com",
		"export PROJECT_ACCOUNT='demo-cfd'",
		"export INPUT_FILE='/data/home/user/model.dat'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script missing %q:\n%s", expected, script)
		}
	}
}

func TestRenderTemplateScriptUsesValidatedProjectAccountFallback(t *testing.T) {
	tpl := JobTemplate{
		Name: "account fallback", Kind: "batch", ScriptTemplate: "true",
		FormSchema: []TemplateField{{ID: "project", Type: "slurm-account", Label: "项目", Control: "project", Variable: "PROJECT_ACCOUNT", SlurmOption: "--account", Required: true}},
	}
	script, err := RenderTemplateScript(tpl, map[string]any{"account": "default-project"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "#SBATCH --account=default-project") || !strings.Contains(script, "export PROJECT_ACCOUNT='default-project'") {
		t.Fatalf("script did not use account fallback:\n%s", script)
	}
}

func TestRenderTemplateScriptRejectsUnsafeDynamicSlurmValue(t *testing.T) {
	tpl := JobTemplate{
		Name: "unsafe slurm", Kind: "batch", ScriptTemplate: "true",
		FormSchema: []TemplateField{{ID: "job", Type: "slurm-job-name", Label: "作业名称", Control: "text", Variable: "JOB_NAME", SlurmOption: "--job-name"}},
	}
	if _, err := RenderTemplateScript(tpl, map[string]any{"job": "good\n#SBATCH --nodes=999"}); err == nil {
		t.Fatal("RenderTemplateScript accepted a newline in a Slurm directive value")
	}
}

func TestValidateTemplateRejectsUnsafeSlurmOption(t *testing.T) {
	tpl := JobTemplate{
		Name: "unsafe option", Kind: "batch", ScriptTemplate: "true",
		FormSchema: []TemplateField{{ID: "custom", Type: "slurm-custom", Label: "参数", Control: "text", Variable: "CUSTOM", SlurmOption: "--comment;rm"}},
	}
	if err := ValidateTemplate(tpl); err == nil {
		t.Fatal("ValidateTemplate accepted an unsafe Slurm option")
	}
}

func TestRenderTemplateScriptUsesSafeDefaultJobNameForChineseTemplate(t *testing.T) {
	item := JobTemplate{
		ID:             42,
		Name:           "中文作业模板",
		Kind:           "batch",
		ScriptTemplate: "true",
	}
	script, err := RenderTemplateScript(item, map[string]any{})
	if err != nil {
		t.Fatalf("RenderTemplateScript() error = %v", err)
	}
	if !strings.Contains(script, "#SBATCH --job-name=simplehpc-template-42") {
		t.Fatalf("script missing safe job name:\n%s", script)
	}
}

func TestDisplayFieldsDoNotRequireOrExportVariables(t *testing.T) {
	item := JobTemplate{
		Name:           "layout",
		Kind:           "batch",
		ScriptTemplate: "true",
		FormSchema: []TemplateField{
			{ID: "intro", Type: "section", Label: "输入参数"},
			{ID: "notice", Type: "hint", Label: "请选择正确的输入文件"},
			{ID: "separator", Type: "divider"},
			{ID: "input", Type: "text", Label: "输入文件", Variable: "INPUT_FILE"},
		},
	}
	script, err := RenderTemplateScript(item, map[string]any{"input": "data.in"})
	if err != nil {
		t.Fatalf("RenderTemplateScript() error = %v", err)
	}
	if strings.Contains(script, "export =") {
		t.Fatalf("display field generated an invalid export:\n%s", script)
	}
	if !strings.Contains(script, "export INPUT_FILE='data.in'") {
		t.Fatalf("input field was not exported:\n%s", script)
	}
}

func TestEnsureTemplateUsableReportsMaintenanceForUnpublishedTemplate(t *testing.T) {
	err := ensureTemplateUsable(JobTemplate{Status: "draft"}, AuthUser{Username: "user001", Type: "ldap"})
	if err == nil || err.Error() != "模板维护中，请稍后！" {
		t.Fatalf("ensureTemplateUsable() error = %v, want maintenance message", err)
	}
}

func TestEnsureTemplateEditableRejectsPublishedTemplate(t *testing.T) {
	err := ensureTemplateEditable("published")
	if err == nil || err.Error() != "请先取消发布后再编辑模板" {
		t.Fatalf("ensureTemplateEditable() error = %v, want unpublish-first message", err)
	}
}

func floatPtr(value float64) *float64 { return &value }
