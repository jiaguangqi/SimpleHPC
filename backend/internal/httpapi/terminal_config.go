package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) getTerminalConfig(c *gin.Context) {
	config, err := api.loadTerminalConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (api *API) saveTerminalConfig(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" || (user.Role != service.ClusterAdminRole && user.Role != "config_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅集群管理员或配置管理员可以修改登录节点配置"})
		return
	}
	var payload terminalConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效登录节点配置"})
		return
	}
	normalized, err := validateTerminalConfigPayload(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	nodes := make([]any, 0, len(normalized.Nodes))
	for _, node := range normalized.Nodes {
		nodes = append(nodes, map[string]any{
			"hostname": node.Hostname,
			"address":  node.Address,
			"enabled":  node.Enabled,
		})
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "terminal", map[string]any{
		"strategy": normalized.Strategy,
		"nodes":    nodes,
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor:      user.Username,
		ActorType:  user.Type,
		Action:     "config.terminal.update",
		TargetType: "config",
		Target:     "terminal",
		Result:     "success",
		Detail:     map[string]any{"nodes": len(nodes), "strategy": normalized.Strategy},
		IPAddress:  c.ClientIP(),
	})
	c.JSON(http.StatusOK, gin.H{"config": normalized})
}

func validateTerminalConfigPayload(payload terminalConfigPayload) (terminalConfigPayload, error) {
	strategy := strings.TrimSpace(payload.Strategy)
	if strategy == "" {
		strategy = "round_robin"
	}
	if strategy != "round_robin" && strategy != "least_sessions" {
		return terminalConfigPayload{}, errBadTerminalConfig("分配策略必须为 round_robin 或 least_sessions")
	}
	nodes := make([]terminalLoginNode, 0, len(payload.Nodes))
	seen := map[string]bool{}
	for _, node := range payload.Nodes {
		node.Hostname = strings.TrimSpace(node.Hostname)
		node.Address = strings.TrimSpace(node.Address)
		if node.Hostname == "" && node.Address == "" {
			continue
		}
		if node.Hostname != "" {
			if err := validateTerminalHost(node.Hostname); err != nil {
				return terminalConfigPayload{}, err
			}
		}
		if node.Address != "" {
			if err := validateTerminalHost(node.Address); err != nil {
				return terminalConfigPayload{}, err
			}
		}
		key := node.Hostname + "|" + node.Address
		if seen[key] {
			continue
		}
		seen[key] = true
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return terminalConfigPayload{}, errBadTerminalConfig("至少需要配置一个登录节点")
	}
	return terminalConfigPayload{Strategy: strategy, Nodes: nodes}, nil
}

type errBadTerminalConfig string

func (e errBadTerminalConfig) Error() string { return string(e) }
