package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"simplehpc/backend/internal/service"
)

type websshSession struct {
	ID            string            `json:"sessionId"`
	OwnerUsername string            `json:"ownerUsername"`
	Username      string            `json:"username"`
	AccountType   string            `json:"accountType"`
	Node          terminalLoginNode `json:"-"`
	NodeName      string            `json:"node"`
	NodeHost      string            `json:"nodeHost"`
	Status        string            `json:"status"`
	InitialPath   string            `json:"initialPath"`
	CurrentPath   string            `json:"currentPath"`
	Cols          uint16            `json:"cols"`
	Rows          uint16            `json:"rows"`
	CreatedAt     time.Time         `json:"createdAt"`
	LastActive    time.Time         `json:"lastActiveAt"`
	ClosedAt      *time.Time        `json:"closedAt,omitempty"`
	CloseReason   string            `json:"closeReason,omitempty"`

	mu   sync.Mutex
	ptmx *os.File
	cmd  *exec.Cmd
	seq  int64
}

var websshSessions sync.Map

func (api *API) websshNodes(c *gin.Context) {
	config, err := api.loadTerminalConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "读取登录节点配置失败：" + err.Error()})
		return
	}
	nodes := api.selectableTerminalNodes(config)
	if len(nodes) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"nodes":      []gin.H{},
			"strategy":   config.Strategy,
			"configured": false,
			"message":    "未配置可用登录节点，请到系统设置页面中设置登录节点信息才可以使用该功能",
		})
		return
	}
	response := make([]gin.H, 0, len(nodes))
	for index, node := range nodes {
		name := node.DisplayName()
		status := "online"
		latency := 0
		if api.services.Redis != nil {
			count, _ := api.services.Redis.Get(c.Request.Context(), terminalNodeSessionKey(node)).Int64()
			latency = int(count)
		}
		label := ""
		if index == 0 {
			label = "推荐"
		}
		response = append(response, gin.H{
			"name": name, "host": node.SSHTarget(), "hostname": node.Hostname, "address": node.Address,
			"status": status, "label": label, "latencyMs": latency, "description": "登录节点",
		})
	}
	c.JSON(http.StatusOK, gin.H{"nodes": response, "strategy": config.Strategy})
}

func (api *API) websshFilesTree(c *gin.Context) {
	if strings.TrimSpace(c.Query("path")) == "" && strings.TrimSpace(c.Query("root")) == "" {
		items := api.services.Storage.ListRoots()
		_, effectiveRoots, _, ok := api.scopedStorage(c)
		if !ok {
			return
		}
		baseItems := items
		items = nil
		for _, effective := range effectiveRoots {
			for _, base := range baseItems {
				basePath := filepath.Clean(base.Path)
				effectivePath := filepath.Clean(effective)
				if effectivePath == basePath || strings.HasPrefix(effectivePath, basePath+string(os.PathSeparator)) {
					base.EffectivePath = effective
					items = append(items, base)
					break
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"roots": items, "items": items, "count": len(items)})
		return
	}
	api.websshFilesList(c)
}

func (api *API) websshFilesList(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	showHidden, _ := strconv.ParseBool(c.DefaultQuery("showHidden", "false"))
	path := c.Query("path")
	if path == "" {
		path = c.Query("root")
	}
	if !api.requireStorageAccess(c, user, "webssh.files.list", service.AccessView, path) {
		return
	}
	if showHidden {
		authz, _ := storageAuthorization(c)
		if !authz.AllowsHidden(path) {
			api.recordStorageDenied(c, user, "webssh.files.show_hidden", "当前文件策略不允许显示隐藏文件")
			c.JSON(http.StatusForbidden, gin.H{"error": "当前文件策略不允许显示隐藏文件"})
			return
		}
	}
	entries, err := client.List(path, showHidden)
	if err != nil {
		api.storageError(c, user, "webssh.files.list", err)
		return
	}
	meta, err := client.PathMetadata(path)
	if err != nil {
		api.storageError(c, user, "webssh.files.list", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"entries": entries, "items": entries, "count": len(entries),
		"effectivePath": meta.EffectivePath, "initialPath": meta.InitialPath,
		"canGoParent": meta.CanGoParent, "parentPath": meta.ParentPath,
	})
}

func (api *API) websshFilesUpload(c *gin.Context) {
	api.storageUpload(c)
}

func (api *API) websshFilesDownload(c *gin.Context) {
	api.storageDownload(c)
}

func (api *API) websshFilesMkdir(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload struct {
		Parent string `json:"parent"`
		Path   string `json:"path"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mkdir payload"})
		return
	}
	parent := strings.TrimSpace(payload.Parent)
	if parent == "" {
		parent = strings.TrimSpace(payload.Path)
	}
	if !api.requireStorageAccess(c, user, "webssh.files.mkdir", service.AccessManage, parent) {
		return
	}
	if err := client.CreateDirectory(parent, payload.Name); err != nil {
		api.storageError(c, user, "webssh.files.mkdir", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (api *API) websshFilesDelete(c *gin.Context) {
	api.storageDelete(c)
}

func (api *API) websshFilesRename(c *gin.Context) {
	api.storageRename(c)
}

func (api *API) websshFilesCopy(c *gin.Context) {
	api.storageCopy(c)
}

func (api *API) websshFilesMove(c *gin.Context) {
	api.storageMove(c)
}

func (api *API) websshFilesArchive(c *gin.Context) {
	api.storageArchive(c)
}

type websshSessionCreateRequest struct {
	Node        string `json:"node"`
	Shell       string `json:"shell"`
	InitialPath string `json:"initialPath"`
	Cols        uint16 `json:"cols"`
	Rows        uint16 `json:"rows"`
}

func (api *API) websshCreateSession(c *gin.Context) {
	authUser, ok := api.currentUser(c)
	if !ok {
		return
	}
	authz, _ := permissionContext(c)
	linuxUser := resolveTerminalLinuxUsername(authUser, authz)
	if err := validateLinuxUsername(linuxUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := user.Lookup(linuxUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前平台账号未映射 Linux/LDAP 用户，无法打开 WebSSH 终端"})
		return
	}
	var payload websshSessionCreateRequest
	if err := c.ShouldBindJSON(&payload); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webssh session payload"})
		return
	}
	config, err := api.loadTerminalConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "读取登录节点配置失败：" + err.Error()})
		return
	}
	node, err := api.selectTerminalLoginNode(c.Request.Context(), config, payload.Node)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client, roots, storageUser, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	initialPath := strings.TrimSpace(payload.InitialPath)
	if initialPath == "" && len(roots) > 0 {
		initialPath = roots[0]
	}
	if !api.requireStorageAccess(c, storageUser, "webssh.session.initial_path", service.AccessView, initialPath) {
		return
	}
	meta, err := client.PathMetadata(initialPath)
	if err != nil {
		api.storageError(c, storageUser, "webssh.session.initial_path", err)
		return
	}
	cols := payload.Cols
	if cols < 10 {
		cols = 120
	}
	rows := payload.Rows
	if rows < 5 {
		rows = 36
	}
	session := &websshSession{
		ID:            newWebSSHSessionID(),
		OwnerUsername: authUser.Username,
		Username:      linuxUser,
		AccountType:   authUser.Type,
		Node:          node,
		NodeName:      node.DisplayName(),
		NodeHost:      node.SSHTarget(),
		Status:        "connecting",
		InitialPath:   meta.EffectivePath,
		CurrentPath:   meta.EffectivePath,
		Cols:          cols,
		Rows:          rows,
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
	}
	websshSessions.Store(session.ID, session)
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.create", session.ID, "created", map[string]any{
		"node": session.NodeName, "initialPath": session.InitialPath,
		"linuxUser": session.Username, "ownerUsername": session.OwnerUsername,
	})
	c.JSON(http.StatusCreated, gin.H{
		"sessionId": session.ID, "node": session.NodeName, "username": session.Username,
		"ownerUsername": session.OwnerUsername,
		"status":        session.Status, "initialPath": session.InitialPath,
		"wsUrl": "/api/v1/webssh/sessions/" + session.ID + "/ws",
	})
}

func (api *API) websshListSessions(c *gin.Context) {
	authUser, ok := api.currentUser(c)
	if !ok {
		return
	}
	items := []websshSession{}
	websshSessions.Range(func(_, value any) bool {
		session, ok := value.(*websshSession)
		if !ok || sessionOwnerUsername(session) != authUser.Username {
			return true
		}
		session.mu.Lock()
		item := *session
		item.ptmx = nil
		item.cmd = nil
		session.mu.Unlock()
		items = append(items, item)
		return true
	})
	c.JSON(http.StatusOK, gin.H{"items": items, "sessions": items, "count": len(items)})
}

func (api *API) websshResizeSession(c *gin.Context) {
	session, authUser, ok := api.websshOwnedSession(c)
	if !ok {
		return
	}
	var payload struct {
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resize payload"})
		return
	}
	if payload.Cols < 10 || payload.Rows < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid terminal size"})
		return
	}
	session.mu.Lock()
	session.Cols = payload.Cols
	session.Rows = payload.Rows
	session.LastActive = time.Now()
	if session.ptmx != nil {
		_ = pty.Setsize(session.ptmx, &pty.Winsize{Cols: payload.Cols, Rows: payload.Rows})
	}
	session.mu.Unlock()
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.resize", session.ID, "resized", map[string]any{"cols": payload.Cols, "rows": payload.Rows})
	c.JSON(http.StatusOK, gin.H{"status": "resized"})
}

func (api *API) websshReconnectSession(c *gin.Context) {
	session, authUser, ok := api.websshOwnedSession(c)
	if !ok {
		return
	}
	api.closeWebSSHSession(session, "reconnect_requested")
	session.mu.Lock()
	session.Status = "connecting"
	session.ClosedAt = nil
	session.CloseReason = ""
	session.LastActive = time.Now()
	status := session.Status
	session.mu.Unlock()
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.reconnect", session.ID, "requested", nil)
	c.JSON(http.StatusOK, gin.H{"status": status, "wsUrl": "/api/v1/webssh/sessions/" + session.ID + "/ws"})
}

func (api *API) websshDeleteSession(c *gin.Context) {
	session, authUser, ok := api.websshOwnedSession(c)
	if !ok {
		return
	}
	api.closeWebSSHSession(session, "closed_by_user")
	websshSessions.Delete(session.ID)
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.delete", session.ID, "closed", nil)
	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}

func (api *API) websshSessionWS(c *gin.Context) {
	session, authUser, ok := api.websshOwnedSession(c)
	if !ok {
		return
	}
	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	session.mu.Lock()
	if session.Status == "connected" {
		session.mu.Unlock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n会话已连接，请勿重复连接同一 session。\r\n"))
		return
	}
	session.seq++
	seq := session.seq
	session.Status = "connecting"
	session.LastActive = time.Now()
	session.mu.Unlock()

	ctx := c.Request.Context()
	api.incrementTerminalNodeSessions(ctx, session.Node)
	defer api.decrementTerminalNodeSessions(context.Background(), session.Node)

	cmd := terminalCommand(ctx, session.Username, session.Node)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: session.Cols, Rows: session.Rows})
	if err != nil {
		session.mu.Lock()
		if session.seq == seq {
			session.Status = "disconnected"
			session.CloseReason = err.Error()
		}
		session.mu.Unlock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n终端启动失败："+err.Error()+"\r\n"))
		_ = api.recordWebSSHAudit(c, authUser, "webssh.session.ws", session.ID, "failed", map[string]any{"error": err.Error()})
		return
	}
	defer ptmx.Close()
	session.mu.Lock()
	if session.seq != seq {
		session.mu.Unlock()
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		return
	}
	session.ptmx = ptmx
	session.cmd = cmd
	session.Status = "connected"
	session.LastActive = time.Now()
	session.mu.Unlock()
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.ws", session.ID, "connected", map[string]any{"node": session.NodeName})
	_, _ = ptmx.Write([]byte(fmt.Sprintf("cd %s 2>/dev/null || true\n", shellQuote(session.InitialPath))))
	_, _ = ptmx.Write([]byte(fmt.Sprintf("echo '[simpleHPC] connected to %s as %s'\n", session.NodeName, session.Username)))

	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buffer)
			if n > 0 {
				session.mu.Lock()
				session.LastActive = time.Now()
				session.mu.Unlock()
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

	_ = ptmx.Close()
	select {
	case <-done:
	case <-time.After(800 * time.Millisecond):
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	session.mu.Lock()
	if session.seq == seq {
		now := time.Now()
		session.Status = "disconnected"
		session.ClosedAt = &now
		session.CloseReason = "websocket_closed"
		session.ptmx = nil
		session.cmd = nil
	}
	session.mu.Unlock()
	_ = api.recordWebSSHAudit(c, authUser, "webssh.session.ws", session.ID, "disconnected", nil)
}

func (api *API) websshOwnedSession(c *gin.Context) (*websshSession, service.AuthUser, bool) {
	authUser, ok := api.currentUser(c)
	if !ok {
		return nil, service.AuthUser{}, false
	}
	raw, exists := websshSessions.Load(c.Param("id"))
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "WebSSH 会话不存在或已过期"})
		return nil, authUser, false
	}
	session, ok := raw.(*websshSession)
	if !ok || sessionOwnerUsername(session) != authUser.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该 WebSSH 会话"})
		return nil, authUser, false
	}
	return session, authUser, true
}

func sessionOwnerUsername(session *websshSession) string {
	if session == nil {
		return ""
	}
	if strings.TrimSpace(session.OwnerUsername) != "" {
		return strings.TrimSpace(session.OwnerUsername)
	}
	return strings.TrimSpace(session.Username)
}

func (api *API) closeWebSSHSession(session *websshSession, reason string) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.seq++
	if session.ptmx != nil {
		_ = session.ptmx.Close()
		session.ptmx = nil
	}
	if session.cmd != nil && session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
		_ = session.cmd.Wait()
		session.cmd = nil
	}
	now := time.Now()
	session.Status = "closed"
	session.ClosedAt = &now
	session.CloseReason = reason
}

func newWebSSHSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b[:])
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (api *API) recordWebSSHAudit(c *gin.Context, actor service.AuthUser, action, target, result string, detail map[string]any) error {
	if api == nil || api.services == nil || api.services.DB == nil {
		return nil
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor: actor.Username, ActorType: actor.Type,
		Action: action, TargetType: "webssh_session", Target: target, Result: result,
		Detail: detail, IPAddress: c.ClientIP(),
	})
}
