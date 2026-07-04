package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"simplehpc/backend/internal/service"
)

var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return parsed.Host == r.Host
	},
}

type terminalResizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

type terminalLoginNode struct {
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
	Enabled  bool   `json:"enabled"`
}

type terminalConfigPayload struct {
	Strategy string              `json:"strategy"`
	Nodes    []terminalLoginNode `json:"nodes"`
}

func (api *API) terminalSession(c *gin.Context) {
	authUser, ok := api.currentUser(c)
	if !ok {
		return
	}
	authz, _ := permissionContext(c)
	defaultUser := resolveTerminalLinuxUsername(authUser, authz)
	targetUser := strings.TrimSpace(c.Query("user"))
	if targetUser == "" {
		targetUser = defaultUser
	}
	if targetUser != defaultUser && !authz.IsClusterAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能打开当前账号对应的 Linux 终端"})
		return
	}
	if err := validateLinuxUsername(targetUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := user.Lookup(targetUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前平台账号未映射 Linux/LDAP 用户，无法打开 WebSSH 终端"})
		return
	}
	config, err := api.loadTerminalConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "读取登录节点配置失败：" + err.Error()})
		return
	}
	node, err := api.selectTerminalLoginNode(c.Request.Context(), config, c.Query("node"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	cols := queryUint16(c, "cols", 120)
	rows := queryUint16(c, "rows", 32)
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	api.incrementTerminalNodeSessions(ctx, node)
	defer api.decrementTerminalNodeSessions(context.Background(), node)

	cmd := terminalCommand(ctx, targetUser, node)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n终端启动失败："+err.Error()+"\r\n"))
		_ = api.recordTerminalAudit(c, authUser, targetUser, "failed", map[string]any{"error": err.Error(), "node": node.Hostname, "address": node.Address})
		return
	}
	defer ptmx.Close()

	_ = api.recordTerminalAudit(c, authUser, targetUser, "started", map[string]any{"cols": cols, "rows": rows, "node": node.Hostname, "address": node.Address})
	_, _ = ptmx.Write([]byte(fmt.Sprintf("echo '[simpleHPC] connected to %s as %s'\n", node.DisplayName(), targetUser)))
	done := make(chan struct{})

	go func() {
		defer close(done)
		buffer := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buffer)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n终端输出读取结束："+err.Error()+"\r\n"))
				}
				return
			}
		}
	}()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if handleTerminalControlMessage(ptmx, payload) {
			continue
		}
		if _, err := ptmx.Write(payload); err != nil {
			break
		}
	}

	cancel()
	_ = ptmx.Close()
	select {
	case <-done:
	case <-time.After(800 * time.Millisecond):
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	_ = api.recordTerminalAudit(c, authUser, targetUser, "closed", map[string]any{"exit": processStateString(cmd), "node": node.Hostname, "address": node.Address})
}

func resolveTerminalLinuxUsername(authUser service.AuthUser, authz service.PermissionContext) string {
	if shouldMapAdminToRoot(authUser, authz) {
		return "root"
	}
	return strings.TrimSpace(authUser.Username)
}

func shouldMapAdminToRoot(authUser service.AuthUser, authz service.PermissionContext) bool {
	if strings.TrimSpace(authUser.Type) != "admin" {
		return false
	}
	if authz.IsClusterAdmin {
		return true
	}
	if roleMapsToRoot(authUser.Role) {
		return true
	}
	for _, role := range authz.RoleCodes {
		if roleMapsToRoot(role) {
			return true
		}
	}
	return false
}

func roleMapsToRoot(role string) bool {
	switch strings.TrimSpace(role) {
	case service.ClusterAdminRole, "config_admin":
		return true
	default:
		return false
	}
}

func terminalCommand(ctx context.Context, username string, node terminalLoginNode) *exec.Cmd {
	if node.IsRemote() && os.Geteuid() == 0 {
		host := node.SSHTarget()
		command := "exec ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o GlobalKnownHostsFile=/dev/null " + host
		cmd := exec.CommandContext(ctx, "su", "-", username, "-c", command)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SHELL=/bin/bash")
		return cmd
	}
	if node.IsRemote() {
		cmd := exec.CommandContext(ctx, "ssh", "-tt", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "GlobalKnownHostsFile=/dev/null", node.SSHTarget())
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		return cmd
	}
	if os.Geteuid() == 0 {
		cmd := exec.CommandContext(ctx, "su", "-", username)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SHELL=/bin/bash")
		return cmd
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.CommandContext(ctx, shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return cmd
}

func (node terminalLoginNode) DisplayName() string {
	if strings.TrimSpace(node.Hostname) != "" {
		return strings.TrimSpace(node.Hostname)
	}
	return strings.TrimSpace(node.Address)
}

func (node terminalLoginNode) SSHTarget() string {
	if strings.TrimSpace(node.Address) != "" {
		return strings.TrimSpace(node.Address)
	}
	return strings.TrimSpace(node.Hostname)
}

func (node terminalLoginNode) IsRemote() bool {
	target := strings.ToLower(strings.TrimSpace(node.SSHTarget()))
	if target == "" || target == "localhost" || target == "127.0.0.1" || target == "::1" {
		return false
	}
	hostname, _ := os.Hostname()
	short := strings.Split(strings.ToLower(hostname), ".")[0]
	targetShort := strings.Split(target, ".")[0]
	return targetShort != short
}

func validateLinuxUsername(value string) error {
	if value == "" || len(value) > 64 {
		return fmt.Errorf("无效 Linux 用户名")
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			continue
		}
		return fmt.Errorf("Linux 用户名包含非法字符")
	}
	if strings.HasPrefix(value, "-") || strings.Contains(value, "..") {
		return fmt.Errorf("Linux 用户名包含非法格式")
	}
	return nil
}

func validateTerminalHost(value string) error {
	if value == "" || len(value) > 255 {
		return fmt.Errorf("登录节点地址不能为空")
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' || r == ':' {
			continue
		}
		return fmt.Errorf("登录节点地址包含非法字符")
	}
	return nil
}

func queryUint16(c *gin.Context, key string, fallback uint16) uint16 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 10 || value > 500 {
		return fallback
	}
	return uint16(value)
}

func handleTerminalControlMessage(ptmx *os.File, payload []byte) bool {
	if len(payload) == 0 || payload[0] != '{' {
		return false
	}
	var message terminalResizeMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return false
	}
	if message.Type != "resize" || message.Cols < 10 || message.Rows < 5 {
		return false
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Cols: message.Cols, Rows: message.Rows})
	return true
}

func (api *API) recordTerminalAudit(c *gin.Context, actor service.AuthUser, target, result string, detail map[string]any) error {
	if api == nil || api.services == nil || api.services.DB == nil {
		return nil
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor:      actor.Username,
		ActorType:  actor.Type,
		Action:     "terminal.session",
		TargetType: "linux_user",
		Target:     target,
		Result:     result,
		Detail:     detail,
		IPAddress:  c.ClientIP(),
	})
}

func processStateString(cmd *exec.Cmd) string {
	if cmd == nil || cmd.ProcessState == nil {
		return "unknown"
	}
	return cmd.ProcessState.String()
}

func (api *API) loadTerminalConfig(ctx context.Context) (terminalConfigPayload, error) {
	value, _, err := api.services.GetSystemConfig(ctx, "terminal")
	if err != nil {
		return terminalConfigPayload{}, err
	}
	config := normalizeTerminalConfig(value)
	return config, nil
}

func normalizeTerminalConfig(value map[string]any) terminalConfigPayload {
	strategy := strings.TrimSpace(fmt.Sprint(value["strategy"]))
	if strategy != "least_sessions" {
		strategy = "round_robin"
	}
	nodes := make([]terminalLoginNode, 0)
	if rawNodes, ok := value["nodes"].([]any); ok {
		for _, raw := range rawNodes {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			node := terminalLoginNode{
				Hostname: strings.TrimSpace(fmt.Sprint(item["hostname"])),
				Address:  strings.TrimSpace(fmt.Sprint(item["address"])),
				Enabled:  true,
			}
			if enabled, ok := item["enabled"].(bool); ok {
				node.Enabled = enabled
			}
			if node.Hostname == "" && node.Address == "" {
				continue
			}
			nodes = append(nodes, node)
		}
	}
	return terminalConfigPayload{Strategy: strategy, Nodes: nodes}
}

func (api *API) selectableTerminalNodes(config terminalConfigPayload) []terminalLoginNode {
	nodes := make([]terminalLoginNode, 0, len(config.Nodes))
	for _, node := range config.Nodes {
		if !node.Enabled || (strings.TrimSpace(node.Hostname) == "" && strings.TrimSpace(node.Address) == "") {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (api *API) selectTerminalLoginNode(ctx context.Context, config terminalConfigPayload, requested string) (terminalLoginNode, error) {
	nodes := api.selectableTerminalNodes(config)
	if len(nodes) == 0 {
		return terminalLoginNode{}, fmt.Errorf("未配置可用登录节点，请到系统设置页面中设置登录节点信息才可以使用该功能")
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if err := validateTerminalHost(requested); err != nil {
			return terminalLoginNode{}, err
		}
		for _, node := range nodes {
			if requested == node.Hostname || requested == node.Address || requested == node.DisplayName() {
				return node, nil
			}
		}
		return terminalLoginNode{}, fmt.Errorf("请求的登录节点不在授权配置中")
	}
	if config.Strategy == "least_sessions" && api.services.Redis != nil {
		bestIndex := 0
		bestCount := int64(math.MaxInt64)
		for index, node := range nodes {
			count, _ := api.services.Redis.Get(ctx, terminalNodeSessionKey(node)).Int64()
			if count < bestCount {
				bestCount = count
				bestIndex = index
			}
		}
		return nodes[bestIndex], nil
	}
	if api.services.Redis != nil {
		next, err := api.services.Redis.Incr(ctx, "simplehpc:terminal:login_node:round_robin").Result()
		if err == nil {
			return nodes[int((next-1)%int64(len(nodes)))], nil
		}
	}
	return nodes[0], nil
}

func terminalNodeSessionKey(node terminalLoginNode) string {
	name := node.DisplayName()
	if name == "" {
		name = "default"
	}
	return "simplehpc:terminal:login_node:sessions:" + name
}

func (api *API) incrementTerminalNodeSessions(ctx context.Context, node terminalLoginNode) {
	if api.services == nil || api.services.Redis == nil {
		return
	}
	key := terminalNodeSessionKey(node)
	pipe := api.services.Redis.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 6*time.Hour)
	_, _ = pipe.Exec(ctx)
}

func (api *API) decrementTerminalNodeSessions(ctx context.Context, node terminalLoginNode) {
	if api.services == nil || api.services.Redis == nil {
		return
	}
	_ = api.services.Redis.Decr(ctx, terminalNodeSessionKey(node)).Err()
}
