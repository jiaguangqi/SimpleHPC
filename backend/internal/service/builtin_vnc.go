package service

import (
	"context"
	"encoding/json"
)

const builtinNoVNCVersion = 4

const builtinNoVNCScript = `command -v websockify >/dev/null || { echo "websockify 未安装" >&2; exit 127; }
[ -f /opt/noVNC/vnc.html ] || { echo "noVNC 静态文件不存在: /opt/noVNC" >&2; exit 127; }
[ "$(id -un)" = "$SIMPLEHPC_SUBMIT_USER" ] || { echo "VNC 作业用户与提交用户不一致" >&2; exit 126; }

VNC_BIN="$(command -v vncserver || true)"
VNC_PASSWD_BIN="$(command -v vncpasswd || true)"
[ -x "$VNC_BIN" ] && [ -x "$VNC_PASSWD_BIN" ] || { echo "TigerVNC 服务端组件不完整" >&2; exit 127; }

DISPLAY_NUM=$((100 + SLURM_JOB_ID % 400))
VNC_PORT=$((5900 + DISPLAY_NUM))
WS_PORT=$((20000 + SLURM_JOB_ID % 20000))
VNC_PASSWORD="${SIMPLEHPC_ACCESS_TOKEN:0:8}"
WS_PID=""
VNC_PID=""
RUNTIME_DIR="/tmp/simplehpc-vnc-${UID}-${SLURM_JOB_ID}"

mkdir -p "$HOME/.vnc"
mkdir -p "$RUNTIME_DIR"
chmod 700 "$RUNTIME_DIR"
export XDG_RUNTIME_DIR="$RUNTIME_DIR"
export XDG_SESSION_TYPE=x11
printf "%s\n" "$VNC_PASSWORD" | "$VNC_PASSWD_BIN" -f > "$HOME/.vnc/passwd"
chmod 600 "$HOME/.vnc/passwd"

if [ "$DESKTOP" != "kde" ]; then
  echo "当前集群的 GNOME 不支持 LDAP 用户 VNC 会话，自动使用 KDE" >&2
fi
SESSION_COMMAND="startkde"
printf '#!/bin/sh\nunset SESSION_MANAGER\nunset DBUS_SESSION_BUS_ADDRESS\nexec %s\n' "$SESSION_COMMAND" > "$HOME/.vnc/xstartup"
chmod 700 "$HOME/.vnc/xstartup"

cleanup() {
  if [ -n "$WS_PID" ]; then kill "$WS_PID" >/dev/null 2>&1 || true; fi
  "$VNC_BIN" -kill ":$DISPLAY_NUM" >/dev/null 2>&1 || true
  if [ -n "$VNC_PID" ]; then kill "$VNC_PID" >/dev/null 2>&1 || true; fi
  rm -rf "$RUNTIME_DIR"
}
trap cleanup EXIT TERM INT

"$VNC_BIN" ":$DISPLAY_NUM" -geometry "$RESOLUTION" -depth 24 -SecurityTypes VncAuth -fg -autokill &
VNC_PID=$!
for attempt in $(seq 1 30); do
  if (echo >/dev/tcp/127.0.0.1/"$VNC_PORT") >/dev/null 2>&1; then break; fi
  sleep 1
done
(echo >/dev/tcp/127.0.0.1/"$VNC_PORT") >/dev/null 2>&1 || { echo "VNC 端口未就绪" >&2; exit 1; }

websockify --web=/opt/noVNC "$WS_PORT" "127.0.0.1:$VNC_PORT" &
WS_PID=$!
for attempt in $(seq 1 30); do
  if (echo >/dev/tcp/127.0.0.1/"$WS_PORT") >/dev/null 2>&1; then break; fi
  sleep 1
done
kill -0 "$WS_PID"

curl -fsS -X POST -H "Content-Type: application/json" \
  -d "{\"node\":\"$(hostname -f)\",\"port\":$WS_PORT,\"protocol\":\"vnc\"}" \
  "$SIMPLEHPC_CALLBACK_URL" >/dev/null
wait "$WS_PID"`

func (s *Services) syncBuiltinNoVNCTemplate(ctx context.Context) error {
	schema, _ := json.Marshal([]TemplateField{
		{
			ID: "desktop", Type: "select", Label: "桌面环境", Variable: "DESKTOP", Required: true, Default: "kde",
			Options: []TemplateOption{{Label: "KDE Plasma", Value: "kde"}},
		},
		{
			ID: "resolution", Type: "select", Label: "分辨率", Variable: "RESOLUTION", Required: true, Default: "1920x1080",
			Options: []TemplateOption{{Label: "1920 x 1080", Value: "1920x1080"}, {Label: "2560 x 1440", Value: "2560x1440"}},
		},
	})
	runtime, _ := json.Marshal(TemplateRuntime{
		Desktop: "kde", VNCBackend: "tigervnc", Resolution: "1920x1080", Protocol: "vnc",
		BuiltinVersion: builtinNoVNCVersion,
	})
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
INSERT INTO job_templates(
  name,description,category,kind,status,form_schema,script_template,runtime_config,created_by,updated_by
) VALUES(
  'noVNC Linux 桌面','通过 Slurm 在计算节点启动当前用户的 Linux 桌面','交互式桌面','novnc','published',
  $1,$2,$3,'system','system'
)
ON CONFLICT(name) DO UPDATE SET
  description=EXCLUDED.description,
  category=EXCLUDED.category,
  kind=EXCLUDED.kind,
  status='published',
  form_schema=EXCLUDED.form_schema,
  script_template=EXCLUDED.script_template,
  runtime_config=EXCLUDED.runtime_config,
  version=job_templates.version+1,
  updated_at=now()
WHERE job_templates.updated_by='system'
  AND COALESCE((job_templates.runtime_config->>'builtinVersion')::integer,0) < $4`,
		schema, builtinNoVNCScript, runtime, builtinNoVNCVersion); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO job_template_grants(template_id,target_type,target_id,granted_by)
SELECT id,'all','*','system'
FROM job_templates
WHERE name='noVNC Linux 桌面'
ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	return tx.Commit()
}
