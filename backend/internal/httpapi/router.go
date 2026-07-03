package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/config"
	slurmintegration "simplehpc/backend/internal/integrations/slurm"
	"simplehpc/backend/internal/service"
)

type API struct {
	cfg      config.Config
	services *service.Services
}

type healthItem struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func NewRouter(cfg config.Config, services *service.Services) http.Handler {
	api := &API{cfg: cfg, services: services}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(api.auditWriteRequests())

	router.GET("/api/health", api.health)

	v1 := router.Group("/api/v1")
	v1.Use(api.rbacAccessControl())
	v1.Use(api.legacySafetyBoundary())
	{
		v1.POST("/auth/login", api.login)
		v1.POST("/auth/logout", api.logout)
		v1.GET("/auth/me", api.authMe)
		v1.POST("/auth/password-reset/request", api.requestPasswordReset)
		v1.POST("/auth/password-reset/confirm", api.confirmPasswordReset)
		v1.GET("/overview", api.overview)
		v1.GET("/dashboard", api.dashboard)
		v1.GET("/dashboard/queue-job-trends", api.dashboardQueueJobTrends)
		v1.GET("/audit/logs", api.listAuditLogs)
		v1.GET("/logs/auth-events", api.listAuthEvents)
		v1.GET("/logs/system", api.listSystemLogs)
		v1.GET("/config/platform", api.getPlatformConfig)
		v1.GET("/config/platform/public", api.getPublicPlatformConfig)
		v1.PUT("/config/platform", api.savePlatformConfig)
		v1.POST("/config/platform/assets/:kind", api.uploadPlatformAsset)
		v1.GET("/monitoring/alerts", api.listMonitoringAlerts)
		v1.POST("/monitoring/alerts/:id/acknowledge", api.acknowledgeMonitoringAlert)
		v1.POST("/monitoring/refresh", api.refreshMonitoringAlerts)
		v1.GET("/ldap/users", api.ldapUsers)
		v1.POST("/ldap/bootstrap", api.bootstrapLDAP)
		v1.POST("/account/sync-ldap", api.syncLDAPAccounts)
		v1.GET("/account/users", api.accountUsers)
		v1.POST("/account/users", api.createAccountUser)
		v1.PUT("/account/users/:username", api.updateAccountUser)
		v1.POST("/account/users/:username/freeze", api.freezeAccount)
		v1.POST("/account/users/:username/unfreeze", api.unfreezeAccount)
		v1.POST("/account/users/:username/reset-password", api.resetAccountPassword)
		v1.DELETE("/account/users/:username", api.deleteAccount)
		v1.GET("/account/teams", api.accountTeams)
		v1.POST("/account/teams", api.createAccountTeam)
		v1.POST("/account/teams/create-with-leader", api.createAccountTeamWithLeader)
		v1.PUT("/account/teams/:name", api.updateAccountTeam)
		v1.POST("/account/teams/:name/freeze", api.freezeAccountTeam)
		v1.POST("/account/teams/:name/unfreeze", api.unfreezeAccountTeam)
		v1.DELETE("/account/teams/:name", api.deleteAccountTeam)
		v1.GET("/account/teams/:name/members", api.accountTeamMembers)
		v1.GET("/account/units", api.accountUnits)
		v1.POST("/account/units", api.createAccountUnit)
		v1.PUT("/account/units/:code", api.updateAccountUnit)
		v1.DELETE("/account/units/:code", api.deleteAccountUnit)
		v1.GET("/account/admins", api.accountAdmins)
		v1.POST("/account/admins", api.createAccountAdmin)
		v1.PUT("/account/admins/:username", api.updateAccountAdmin)
		v1.POST("/account/admins/:username/reset-password", api.resetAdminPassword)
		v1.DELETE("/account/admins/:username", api.deleteAccountAdmin)
		v1.GET("/account/roles", api.accountRoles)
		v1.POST("/account/roles", api.createAccountRole)
		v1.PUT("/account/roles/:code", api.updateAccountRole)
		v1.GET("/rbac/permissions", api.listRBACPermissions)
		v1.GET("/rbac/menus", api.listRBACMenus)
		v1.GET("/rbac/matrix", api.getRBACMatrix)
		v1.GET("/rbac/shadow/stats", api.getRBACShadowStats)
		v1.GET("/rbac/roles", api.listRBACRoles)
		v1.POST("/rbac/roles", api.createRBACRole)
		v1.GET("/rbac/roles/:code", api.getRBACRole)
		v1.PUT("/rbac/roles/:code", api.updateRBACRole)
		v1.DELETE("/rbac/roles/:code", api.deleteRBACRole)
		v1.POST("/rbac/roles/:code/copy", api.copyRBACRole)
		v1.PUT("/rbac/roles/:code/status", api.setRBACRoleStatus)
		v1.PUT("/rbac/roles/:code/permissions", api.replaceRBACPermissions)
		v1.PUT("/rbac/roles/:code/data-scopes", api.replaceRBACDataScopes)
		v1.PUT("/rbac/roles/:code/file-policies", api.replaceRBACFilePolicies)
		v1.PUT("/rbac/roles/:code/users", api.replaceRBACBindings)
		v1.GET("/slurm/nodes", api.slurmNodes)
		v1.GET("/slurm/partitions", api.slurmPartitions)
		v1.GET("/slurm/partition-configs", api.slurmPartitionConfigs)
		v1.POST("/slurm/partition-configs", api.createSlurmPartition)
		v1.PUT("/slurm/partition-configs/:name", api.updateSlurmPartition)
		v1.DELETE("/slurm/partition-configs/:name", api.deleteSlurmPartition)
		v1.GET("/slurm/partition-description", api.getSlurmPartitionDescription)
		v1.PUT("/slurm/partition-description", api.saveSlurmPartitionDescription)
		v1.GET("/slurm/queue-status", api.slurmQueueStatus)
		v1.GET("/slurm/qos", api.slurmQOS)
		v1.POST("/slurm/qos", api.createSlurmQOS)
		v1.PUT("/slurm/qos/:name", api.updateSlurmQOS)
		v1.DELETE("/slurm/qos/:name", api.deleteSlurmQOS)
		v1.PUT("/slurm/qos/:name/assignments", api.assignSlurmQOS)
		v1.GET("/slurm/jobs", api.slurmJobs)
		v1.GET("/slurm/jobs/history", api.slurmHistory)
		v1.GET("/slurm/jobs/:id", api.slurmJobDetail)
		v1.GET("/slurm/jobs/:id/output", api.slurmJobOutput)
		v1.POST("/slurm/jobs/:id/cancel", api.cancelSlurmJob)
		v1.POST("/slurm/jobs/:id/suspend", api.suspendSlurmJob)
		v1.POST("/slurm/jobs/:id/resume", api.resumeSlurmJob)
		v1.GET("/config/slurm", api.getSlurmConfig)
		v1.PUT("/config/slurm", api.saveSlurmConfig)
		v1.POST("/config/slurm/test", api.testSlurmConfig)
		v1.GET("/config/ldap", api.getLDAPConfig)
		v1.PUT("/config/ldap", api.saveLDAPConfig)
		v1.POST("/config/ldap/test", api.testLDAPConfig)
		v1.GET("/config/notify", api.getNotifyConfig)
		v1.PUT("/config/notify", api.saveNotifyConfig)
		v1.POST("/config/notify/email/test", api.testEmailConfig)
		v1.POST("/config/notify/feishu/test", api.testFeishuWebhook)
		v1.POST("/inspection/run", api.runInspection)
		v1.POST("/inspection/runs", api.runInspection)
		v1.GET("/inspection/runs", api.listInspectionRuns)
		v1.GET("/inspection/runs/:id", api.getInspectionRun)
		v1.GET("/inspection/runs/:id/report", api.downloadInspectionReport)
		v1.GET("/inspection/runs/:id/log", api.viewInspectionLog)
		v1.GET("/inspection/runs/:id/log/download", api.downloadInspectionLog)
		v1.POST("/inspection/runs/:id/notify", api.notifyInspectionReport)
		v1.GET("/inspection/config", api.getInspectionConfig)
		v1.PUT("/inspection/config", api.saveInspectionConfig)
		v1.GET("/storage/roots", api.storageRoots)
		v1.POST("/storage/roots/refresh", api.refreshStorageRoots)
		v1.GET("/storage/acls", api.listStorageACLs)
		v1.POST("/storage/acls", api.createStorageACL)
		v1.DELETE("/storage/acls/:id", api.deleteStorageACL)
		v1.PUT("/storage/roots", api.saveStorageRoots)
		v1.GET("/storage/list", api.storageList)
		v1.POST("/storage/directory", api.storageCreateDirectory)
		v1.POST("/storage/upload", api.storageUpload)
		v1.POST("/storage/copy", api.storageCopy)
		v1.POST("/storage/move", api.storageMove)
		v1.POST("/storage/rename", api.storageRename)
		v1.POST("/storage/delete", api.storageDelete)
		v1.GET("/storage/download", api.storageDownload)
		v1.POST("/storage/archive", api.storageArchive)
		v1.GET("/job-templates", api.listJobTemplates)
		v1.POST("/job-templates", api.createJobTemplate)
		v1.POST("/job-templates/import", api.importTemplate)
		v1.GET("/job-templates/:id", api.getJobTemplate)
		v1.PUT("/job-templates/:id", api.updateJobTemplate)
		v1.DELETE("/job-templates/:id", api.deleteJobTemplate)
		v1.POST("/job-templates/:id/publish", api.setJobTemplateStatus("published"))
		v1.POST("/job-templates/:id/unpublish", api.setJobTemplateStatus("draft"))
		v1.PUT("/job-templates/:id/grants", api.setTemplateGrants)
		v1.POST("/job-templates/:id/access-requests", api.requestTemplateAccess)
		v1.POST("/job-templates/:id/preview", api.previewTemplate)
		v1.POST("/job-templates/:id/submit", api.submitTemplate)
		v1.GET("/job-templates/:id/export", api.exportTemplate)
		v1.GET("/job-template-access-requests", api.listTemplateRequests)
		v1.POST("/job-template-access-requests/:requestId/approve", api.reviewTemplateRequest(true))
		v1.POST("/job-template-access-requests/:requestId/reject", api.reviewTemplateRequest(false))
		v1.GET("/job-template-runs", api.listTemplateRuns)
		v1.POST("/job-template-runs/:token/register", api.registerTemplateEndpoint)
		v1.Any("/job-template-gateway/:token/*path", api.templateGateway)
	}

	router.Match([]string{http.MethodGet, http.MethodHead}, "/", htmlFileHandler(filepath.Join(cfg.FrontendDir, "index.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/index.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "index.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/users.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "users.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/resources.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "resources.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/data.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "data.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/jobs.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "jobs.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/monitoring.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "monitoring.html")))
	router.Match([]string{http.MethodGet, http.MethodHead}, "/settings.html", htmlFileHandler(filepath.Join(cfg.FrontendDir, "settings.html")))
	router.Static("/css", filepath.Join(cfg.FrontendDir, "css"))
	router.Static("/js", filepath.Join(cfg.FrontendDir, "js"))
	router.Static("/assets", filepath.Join(cfg.FrontendDir, "assets"))
	router.Static("/uploads", filepath.Join(cfg.FrontendDir, "uploads"))
	router.NoRoute(htmlPageFallback(cfg.FrontendDir))

	return router
}

func htmlPageFallback(frontendDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.String(http.StatusNotFound, "not found")
			return
		}
		page := strings.TrimPrefix(c.Request.URL.Path, "/")
		if strings.Contains(page, "/") || !strings.HasSuffix(page, ".html") {
			c.String(http.StatusNotFound, "not found")
			return
		}
		htmlFileHandler(filepath.Join(frontendDir, page))(c)
	}
}

func htmlFileHandler(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := os.ReadFile(path)
		if err != nil {
			c.String(http.StatusNotFound, "file not found")
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if c.Request.Method == http.MethodHead {
			c.Header("Content-Length", stringLength(data))
			c.Status(http.StatusOK)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	}
}

func stringLength(data []byte) string {
	return strconv.Itoa(len(data))
}

func (api *API) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	result := map[string]healthItem{}
	result["postgres"] = check(func() error { return api.services.CheckPostgres(ctx) })
	result["redis"] = check(func() error { return api.services.CheckRedis(ctx) })
	result["ldap"] = check(func() error { return api.services.LDAP.Ping() })
	result["slurm"] = check(func() error { return api.services.Slurm.Ping(ctx) })

	status := http.StatusOK
	for _, item := range result {
		if item.Status != "ok" {
			status = http.StatusServiceUnavailable
			break
		}
	}
	c.JSON(status, gin.H{"status": statusText(status), "services": result})
}

func (api *API) overview(c *gin.Context) {
	ctx := c.Request.Context()
	nodes, nodeErr := api.services.Slurm.Nodes(ctx)
	jobs, jobErr := api.services.Slurm.Jobs(ctx)

	nodeStates := map[string]int{}
	totalCPUs := 0
	for _, node := range nodes {
		nodeStates[node.State]++
		totalCPUs += atoi(node.CPUs)
	}
	runningJobs := 0
	pendingJobs := 0
	for _, job := range jobs {
		switch job.State {
		case "RUNNING", "R":
			runningJobs++
		case "PENDING", "PD":
			pendingJobs++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": gin.H{
			"count":  len(nodes),
			"states": nodeStates,
			"cpus":   totalCPUs,
			"error":  errorString(nodeErr),
		},
		"jobs": gin.H{
			"running": runningJobs,
			"pending": pendingJobs,
			"total":   len(jobs),
			"error":   errorString(jobErr),
		},
	})
}

func (api *API) dashboard(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	authz, exists := permissionContext(c)
	if !exists {
		var err error
		authz, err = api.services.ResolvePermissionContext(c.Request.Context(), user)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "无法解析当前账号的数据范围"})
			return
		}
	}
	snapshot, err := api.services.DashboardSnapshot(c.Request.Context(), c.Query("range"), dashboardJobQuery(authz))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (api *API) dashboardQueueJobTrends(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	authz, exists := permissionContext(c)
	if !exists {
		var err error
		authz, err = api.services.ResolvePermissionContext(c.Request.Context(), user)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "无法解析当前账号的数据范围"})
			return
		}
	}
	trends, err := api.services.QueueJobTrends(c.Request.Context(), service.QueueJobTrendRequest{
		Queue: c.Query("queue"),
		Range: c.Query("range"),
		Query: dashboardJobQuery(authz),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, trends)
}

func dashboardJobQuery(authz service.PermissionContext) service.JobQuery {
	return service.ScopeJobQueryByPermission(authz, service.JobQuery{})
}

func (api *API) logout(c *gin.Context) {
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if token == "" {
		token, _ = c.Cookie("simplehpc_session")
	}
	user, _ := api.services.SessionUser(c.Request.Context(), token)
	if err := api.services.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if user.Username != "" {
		_ = api.services.RecordAuthEvent(c.Request.Context(), service.AuthEvent{
			Username: user.Username, DisplayName: user.DisplayName, AccountType: user.Type, Event: "logout", Result: "success",
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), SessionID: sessionDigest(token), Message: "退出登录",
		})
	}
	c.SetCookie("simplehpc_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (api *API) ldapUsers(c *gin.Context) {
	users, err := api.services.LDAP.ListUsers()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users, "count": len(users)})
}

func (api *API) bootstrapLDAP(c *gin.Context) {
	created, err := api.services.LDAP.EnsureBaseOUs()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": created})
}

func (api *API) syncLDAPAccounts(c *gin.Context) {
	result, err := api.services.SyncLDAPAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "sync": result})
}

func (api *API) accountUsers(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	sync, syncErr := api.services.SyncLDAPAccounts(c.Request.Context())
	users, err := api.services.AccountUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "syncError": errorString(syncErr)})
		return
	}
	if authz, ok := permissionContext(c); ok {
		scoped, scopeErr := api.services.FilterAccountUsers(c.Request.Context(), authz, users)
		if scopeErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": scopeErr.Error()})
			return
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacShadow {
			api.recordRBACScopeShadow(c, user, "data.users.list", len(users), len(scoped))
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
			users = scoped
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     users,
		"count":     len(users),
		"source":    "postgres-platform-users",
		"sync":      sync,
		"syncError": errorString(syncErr),
	})
}
func (api *API) createAccountUser(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.CreateUserInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户数据"})
		return
	}
	item, err := api.services.CreatePlatformUser(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (api *API) updateAccountUser(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.UpdateUserInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户数据"})
		return
	}
	item, err := api.services.UpdatePlatformUser(c.Request.Context(), c.Param("username"), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) freezeAccount(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.FreezeAccount(c.Request.Context(), c.Param("username"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "result": result})
}

func (api *API) unfreezeAccount(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.UnfreezeAccount(c.Request.Context(), c.Param("username"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "result": result})
}

func (api *API) resetAccountPassword(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.ResetAccountPassword(c.Request.Context(), c.Param("username"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "result": result})
}

func (api *API) deleteAccount(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.DeleteAccount(c.Request.Context(), c.Param("username"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "result": result})
}

func (api *API) accountTeams(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	sync, syncErr := api.services.SyncLDAPAccounts(c.Request.Context())
	items, err := api.services.AccountTeams(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "syncError": errorString(syncErr)})
		return
	}
	if authz, ok := permissionContext(c); ok {
		scoped, scopeErr := api.services.FilterAccountTeams(c.Request.Context(), authz, items)
		if scopeErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": scopeErr.Error()})
			return
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacShadow {
			api.recordRBACScopeShadow(c, user, "data.teams.list", len(items), len(scoped))
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
			items = scoped
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-teams", "sync": sync, "syncError": errorString(syncErr)})
}
func (api *API) requireAccountTeamWriter(c *gin.Context) (service.AuthUser, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return service.AuthUser{}, false
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return service.AuthUser{}, false
	}
	return user, true
}
func (api *API) createAccountTeam(c *gin.Context) {
	if _, ok := api.requireAccountTeamWriter(c); !ok {
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "新建用户组必须同时创建组长首用户，请使用 /api/v1/account/teams/create-with-leader",
	})
}
func (api *API) createAccountTeamWithLeader(c *gin.Context) {
	if _, ok := api.requireAccountTeamWriter(c); !ok {
		return
	}
	var input service.CreateTeamWithLeaderInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效团队和组长数据"})
		return
	}
	item, err := api.services.CreatePlatformTeamWithLeader(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (api *API) updateAccountTeam(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.CreateTeamInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效团队数据"})
		return
	}
	item, err := api.services.UpdatePlatformTeam(c.Request.Context(), c.Param("name"), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}
func (api *API) setAccountTeamFrozen(c *gin.Context, frozen bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	if err := api.services.FreezePlatformTeam(c.Request.Context(), c.Param("name"), frozen); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
func (api *API) freezeAccountTeam(c *gin.Context)   { api.setAccountTeamFrozen(c, true) }
func (api *API) unfreezeAccountTeam(c *gin.Context) { api.setAccountTeamFrozen(c, false) }
func (api *API) deleteAccountTeam(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	if err := api.services.DeletePlatformTeam(c.Request.Context(), c.Param("name")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) accountTeamMembers(c *gin.Context) {
	sync, syncErr := api.services.SyncLDAPAccounts(c.Request.Context())
	items, err := api.services.AccountTeamMembers(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "syncError": errorString(syncErr)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-team-members", "sync": sync, "syncError": errorString(syncErr)})
}

func (api *API) accountUnits(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	sync, syncErr := api.services.SyncLDAPAccounts(c.Request.Context())
	items, err := api.services.AccountUnits(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "syncError": errorString(syncErr)})
		return
	}
	if authz, ok := permissionContext(c); ok {
		scoped, scopeErr := api.services.FilterAccountUnits(c.Request.Context(), authz, items)
		if scopeErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": scopeErr.Error()})
			return
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacShadow {
			api.recordRBACScopeShadow(c, user, "data.units.list", len(items), len(scoped))
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
			items = scoped
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-units", "sync": sync, "syncError": errorString(syncErr)})
}
func (api *API) saveAccountUnit(c *gin.Context, current string) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.UnitInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效单位数据"})
		return
	}
	item, err := api.services.SaveUnit(c.Request.Context(), current, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}
func (api *API) createAccountUnit(c *gin.Context) { api.saveAccountUnit(c, "") }
func (api *API) updateAccountUnit(c *gin.Context) { api.saveAccountUnit(c, c.Param("code")) }
func (api *API) deleteAccountUnit(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	if err := api.services.DeleteUnit(c.Request.Context(), c.Param("code")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) accountAdmins(c *gin.Context) {
	items, err := api.services.AdminUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-admin-users"})
}
func (api *API) createAccountAdmin(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.AdminCreate
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效管理员数据"})
		return
	}
	item, err := api.services.CreateAdminUser(c.Request.Context(), input, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (api *API) updateAccountAdmin(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	var payload service.AdminUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := api.services.UpdateAdminUser(c.Request.Context(), c.Param("username"), payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "item": item})
}

func (api *API) resetAdminPassword(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.ResetAdminPassword(
		c.Request.Context(),
		c.Param("username"),
		func(email, password string) error {
			subject := "simpleHPC 管理员密码已重置"
			body := strings.Join([]string{
				"您好，",
				"",
				"您的 simpleHPC 管理员账号密码已重置。",
				"管理员账号：" + c.Param("username"),
				"临时密码：" + password,
				"",
				"请登录后尽快修改密码。请勿向他人泄露此邮件。",
			}, "\n")
			return api.sendSystemEmail(c.Request.Context(), email, subject, body)
		},
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"result":  result,
		"message": "新密码已发送到管理员绑定邮箱",
	})
}

func (api *API) deleteAccountAdmin(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	result, err := api.services.DeleteAdminUser(c.Request.Context(), c.Param("username"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"result":  result,
		"message": "管理员账号已删除",
	})
}

func (api *API) accountRoles(c *gin.Context) {
	items, err := api.services.Roles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-roles"})
}
func (api *API) saveAccountRole(c *gin.Context, current string) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var input service.RoleInput
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效角色数据"})
		return
	}
	item, err := api.services.SaveRole(c.Request.Context(), current, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}
func (api *API) createAccountRole(c *gin.Context) { api.saveAccountRole(c, "") }
func (api *API) updateAccountRole(c *gin.Context) { api.saveAccountRole(c, c.Param("code")) }

func (api *API) slurmNodes(c *gin.Context) {
	nodes, err := api.services.Slurm.Nodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": nodes, "count": len(nodes)})
}

func (api *API) slurmPartitions(c *gin.Context) {
	partitions, err := api.services.Slurm.Partitions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	totalCPUs := 0
	totalNodes := 0
	totalGPUs := 0
	for _, item := range partitions {
		totalNodes += countNodeExpression(item.NodeList)
		totalCPUs += atoi(item.CPUsPerNode) * countNodeExpression(item.NodeList)
		totalGPUs += gresCount(item.GRES) * countNodeExpression(item.NodeList)
	}
	c.JSON(http.StatusOK, gin.H{
		"items": partitions,
		"count": len(partitions),
		"summary": gin.H{
			"partitions": len(partitions),
			"nodes":      totalNodes,
			"cpus":       totalCPUs,
			"gpus":       totalGPUs,
		},
	})
}

func (api *API) slurmPartitionConfigs(c *gin.Context) {
	items, err := api.services.Slurm.PartitionConfigs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "slurm-effective-config"})
}

func (api *API) createSlurmPartition(c *gin.Context) {
	var item slurmintegration.PartitionConfig
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := api.services.Slurm.SavePartition(c.Request.Context(), "", item)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "publish": result})
}

func (api *API) updateSlurmPartition(c *gin.Context) {
	var item slurmintegration.PartitionConfig
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := api.services.Slurm.SavePartition(c.Request.Context(), c.Param("name"), item)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "publish": result})
}

func (api *API) deleteSlurmPartition(c *gin.Context) {
	result, err := api.services.Slurm.DeletePartition(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "publish": result})
}

func (api *API) getSlurmPartitionDescription(c *gin.Context) {
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "slurm_partition_description")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"description": value["description"]})
}

func (api *API) saveSlurmPartitionDescription(c *gin.Context) {
	var payload struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "slurm_partition_description", map[string]any{"description": payload.Description}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) slurmQueueStatus(c *gin.Context) {
	jobs, jobErr := api.services.Slurm.Jobs(c.Request.Context())
	partitions, partitionErr := api.services.Slurm.Partitions(c.Request.Context())
	nodes, nodeErr := api.services.Slurm.Nodes(c.Request.Context())
	if jobErr != nil && partitionErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": jobErr.Error()})
		return
	}

	type queueAgg struct {
		Partition    string         `json:"partition"`
		Status       string         `json:"status"`
		ResourceType string         `json:"resourceType"`
		RunningJobs  int            `json:"runningJobs"`
		PendingJobs  int            `json:"pendingJobs"`
		RunningCores int            `json:"runningCores"`
		PendingCores int            `json:"pendingCores"`
		RunningGPUs  int            `json:"runningGpus"`
		PendingGPUs  int            `json:"pendingGpus"`
		TotalCores   int            `json:"totalCores"`
		TotalGPUs    int            `json:"totalGpus"`
		TotalNodes   int            `json:"totalNodes"`
		NodeStates   map[string]int `json:"nodeStates"`
		NodeStatus   string         `json:"nodeStatus"`
	}
	queues := map[string]*queueAgg{}
	for _, p := range partitions {
		status := "UP"
		if strings.ToLower(p.Availability) != "up" {
			status = strings.ToUpper(p.Availability)
		}
		nodeCount := countNodeExpression(p.NodeList)
		gpusPerNode := gresCount(p.GRES)
		resourceType := "CPU"
		if gpusPerNode > 0 {
			resourceType = "GPU"
		}
		queues[p.Name] = &queueAgg{
			Partition:    p.Name,
			Status:       status,
			ResourceType: resourceType,
			TotalCores:   atoi(p.CPUsPerNode) * nodeCount,
			TotalGPUs:    gpusPerNode * nodeCount,
			TotalNodes:   nodeCount,
			NodeStates:   partitionNodeStates(p.Name, nodes),
			NodeStatus:   summarizePartitionNodes(p.Name, nodes),
		}
	}
	for _, job := range jobs {
		name := strings.TrimSuffix(job.Partition, "*")
		if name == "" {
			name = "未分区"
		}
		if _, ok := queues[name]; !ok {
			queues[name] = &queueAgg{
				Partition:  name,
				Status:     "UNKNOWN",
				NodeStates: partitionNodeStates(name, nodes),
				NodeStatus: summarizePartitionNodes(name, nodes),
			}
		}
		state := strings.ToUpper(job.State)
		if state == "RUNNING" || state == "R" {
			queues[name].RunningJobs++
			queues[name].RunningCores += atoi(job.CPUs)
			queues[name].RunningGPUs += atoi(job.GPUs)
		}
		if state == "PENDING" || state == "PD" {
			queues[name].PendingJobs++
			queues[name].PendingCores += atoi(job.CPUs)
			queues[name].PendingGPUs += atoi(job.GPUs)
		}
	}
	items := make([]queueAgg, 0, len(queues))
	summary := gin.H{
		"runningJobs": 0, "pendingJobs": 0,
		"runningCores": 0, "pendingCores": 0,
		"runningGpus": 0, "pendingGpus": 0,
		"totalCores": 0, "totalGpus": 0, "totalNodes": 0,
	}
	for _, item := range queues {
		items = append(items, *item)
		summary["runningJobs"] = summary["runningJobs"].(int) + item.RunningJobs
		summary["pendingJobs"] = summary["pendingJobs"].(int) + item.PendingJobs
		summary["runningCores"] = summary["runningCores"].(int) + item.RunningCores
		summary["pendingCores"] = summary["pendingCores"].(int) + item.PendingCores
		summary["runningGpus"] = summary["runningGpus"].(int) + item.RunningGPUs
		summary["pendingGpus"] = summary["pendingGpus"].(int) + item.PendingGPUs
		summary["totalCores"] = summary["totalCores"].(int) + item.TotalCores
		summary["totalGpus"] = summary["totalGpus"].(int) + item.TotalGPUs
		summary["totalNodes"] = summary["totalNodes"].(int) + item.TotalNodes
	}
	c.JSON(http.StatusOK, gin.H{
		"items":   items,
		"count":   len(items),
		"summary": summary,
		"errors": gin.H{
			"jobs":       errorString(jobErr),
			"partitions": errorString(partitionErr),
			"nodes":      errorString(nodeErr),
		},
	})
}

func (api *API) slurmQOS(c *gin.Context) {
	qos, qosErr := api.services.Slurm.QOS(c.Request.Context())
	accounts, accountErr := api.services.Slurm.Accounts(c.Request.Context())
	associations, associationErr := api.services.Slurm.Associations(c.Request.Context())
	if qosErr != nil && accountErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": qosErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"qos":          qos,
		"accounts":     accounts,
		"associations": associations,
		"errors": gin.H{
			"qos":          errorString(qosErr),
			"accounts":     errorString(accountErr),
			"associations": errorString(associationErr),
		},
	})
}

func (api *API) createSlurmQOS(c *gin.Context) {
	var item slurmintegration.QOS
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.services.Slurm.SaveQOS(c.Request.Context(), "", item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}

func (api *API) updateSlurmQOS(c *gin.Context) {
	var item slurmintegration.QOS
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.services.Slurm.SaveQOS(c.Request.Context(), c.Param("name"), item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}

func (api *API) deleteSlurmQOS(c *gin.Context) {
	if err := api.services.Slurm.DeleteQOS(c.Request.Context(), c.Param("name")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}

func (api *API) assignSlurmQOS(c *gin.Context) {
	var payload slurmintegration.QOSAssignmentRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := api.services.Slurm.AssignQOS(c.Request.Context(), c.Param("name"), payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true, "assignment": result})
}

func (api *API) slurmJobs(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	baseQuery := service.JobQuery{
		Page:      queryInt(c, "page", 1),
		PageSize:  queryInt(c, "pageSize", 15),
		Status:    c.Query("status"),
		Keyword:   c.Query("keyword"),
		Username:  c.Query("user"),
		Group:     c.Query("group"),
		Partition: c.Query("partition"),
	}
	query := api.scopeJobQuery(c, user, baseQuery)
	if api.services.DB != nil && c.Query("live") != "1" {
		if c.Query("refresh") == "1" {
			if err := api.services.SyncRecentSlurmJobs(c.Request.Context()); err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
		}
		page, err := api.services.QuerySlurmJobs(c.Request.Context(), query)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, page)
		return
	}
	jobs, err := api.services.Slurm.Jobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	filtered := jobs[:0]
	for _, job := range jobs {
		if query.Username != "" && job.User != query.Username {
			continue
		}
		if query.Partition != "" && job.Partition != query.Partition {
			continue
		}
		filtered = append(filtered, job)
	}
	jobs = filtered
	source := "slurm-live"
	c.JSON(http.StatusOK, gin.H{
		"items": jobs, "total": len(jobs), "page": 1, "pageSize": len(jobs),
		"totalPages": 1, "source": source,
	})
}

func (api *API) slurmJobDetail(c *gin.Context) {
	_, detail, ok := api.authorizedSlurmJob(c)
	if !ok {
		return
	}
	script, _ := api.services.TemplateRunScript(c.Request.Context(), detail.ID)
	c.JSON(http.StatusOK, gin.H{"detail": detail, "script": script})
}

func (api *API) slurmJobOutput(c *gin.Context) {
	if _, _, ok := api.authorizedSlurmJob(c); !ok {
		return
	}
	output, err := api.services.Slurm.JobOutput(c.Request.Context(), c.Param("id"), c.Query("stream"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, output)
}

func (api *API) authorizedSlurmJob(c *gin.Context) (service.AuthUser, slurmintegration.JobDetail, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return user, slurmintegration.JobDetail{}, false
	}
	detail, err := api.services.Slurm.JobDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return user, detail, false
	}
	legacyAllowed := user.Type == "admin" || detail.User == user.Username
	if authz, exists := permissionContext(c); exists {
		identity := service.ResourceIdentity{Owner: detail.User}
		if api.services.DB != nil {
			if resolved, err := api.services.UserResourceIdentity(c.Request.Context(), detail.User); err == nil {
				identity = resolved
			}
		}
		rbacAllowed := authz.Allows("jobs", identity)
		if normalizeRBACMode(api.cfg.RBACMode) == rbacShadow && !legacyAllowed && rbacAllowed {
			c.Set(downstreamPolicyDeniedKey, true)
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce && !rbacAllowed {
			c.Set(downstreamPolicyDeniedKey, true)
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该作业"})
			return user, detail, false
		}
	}
	if normalizeRBACMode(api.cfg.RBACMode) != rbacEnforce && !legacyAllowed {
		c.Set(downstreamPolicyDeniedKey, true)
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问其他用户的作业"})
		return user, detail, false
	}
	return user, detail, true
}

func (api *API) scopeJobQuery(c *gin.Context, user service.AuthUser, query service.JobQuery) service.JobQuery {
	legacy := service.ScopeJobQuery(user, query)
	authz, ok := permissionContext(c)
	if !ok {
		return legacy
	}
	rbacScoped := service.ScopeJobQueryByPermission(authz, query)
	mode := normalizeRBACMode(api.cfg.RBACMode)
	if mode == rbacEnforce {
		return rbacScoped
	}
	if mode == rbacShadow && !jobScopeEqual(legacy, rbacScoped) {
		api.recordRBACShadow(c, user, "data.jobs.list", !legacy.DenyAll, !rbacScoped.DenyAll, "job_list_scope_mismatch")
	}
	return legacy
}

func jobScopeEqual(a, b service.JobQuery) bool {
	return a.Username == b.Username && a.Group == b.Group && a.DenyAll == b.DenyAll &&
		strings.Join(a.UnitIDs, ",") == strings.Join(b.UnitIDs, ",") &&
		strings.Join(a.TeamIDs, ",") == strings.Join(b.TeamIDs, ",")
}

func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil {
		return fallback
	}
	return value
}

func (api *API) cancelSlurmJob(c *gin.Context) {
	api.applySlurmJobAction(c, "cancel", "CANCELLED", api.services.Slurm.CancelJob)
}

func (api *API) suspendSlurmJob(c *gin.Context) {
	api.applySlurmJobAction(c, "suspend", "SUSPENDED", api.services.Slurm.SuspendJob)
}

func (api *API) resumeSlurmJob(c *gin.Context) {
	api.applySlurmJobAction(c, "resume", "RUNNING", api.services.Slurm.ResumeJob)
}

func (api *API) applySlurmJobAction(
	c *gin.Context,
	action string,
	state string,
	run func(context.Context, string) error,
) {
	jobID := c.Param("id")
	if _, _, ok := api.authorizedSlurmJob(c); !ok {
		return
	}
	if err := run(c.Request.Context(), jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "action": action, "jobId": jobID})
		return
	}
	if err := api.services.MarkSlurmJobState(c.Request.Context(), jobID, state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "action": action, "jobId": jobID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true, "action": action, "jobId": jobID, "state": state})
}

func (api *API) getSlurmConfig(c *gin.Context) {
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "slurm")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if value["controllerHost"] == nil {
		value["controllerHost"] = primaryIPv4()
	}
	if value["dbdHost"] == nil {
		value["dbdHost"] = value["controllerHost"]
	}
	if value["clusterName"] == nil {
		value["clusterName"] = api.cfg.SlurmDefaultAccount
	}
	if value["binDir"] == nil {
		value["binDir"] = api.services.Slurm.BinDir
	}
	c.JSON(http.StatusOK, gin.H{"config": value})
}

func (api *API) saveSlurmConfig(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload = compactConfig(payload)
	if binDir, ok := payload["binDir"].(string); ok && strings.TrimSpace(binDir) != "" {
		api.services.Slurm.BinDir = strings.TrimSpace(binDir)
	}
	if clusterName, ok := payload["clusterName"].(string); ok && strings.TrimSpace(clusterName) != "" {
		api.services.Slurm.DefaultAccount = strings.TrimSpace(clusterName)
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "slurm", payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) testSlurmConfig(c *gin.Context) {
	var payload map[string]any
	_ = c.ShouldBindJSON(&payload)
	if binDir, ok := payload["binDir"].(string); ok && strings.TrimSpace(binDir) != "" {
		api.services.Slurm.BinDir = strings.TrimSpace(binDir)
	}
	if err := api.services.Slurm.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) getLDAPConfig(c *gin.Context) {
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "ldap")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if value["url"] == nil {
		value["url"] = api.services.LDAP.URL
	}
	if value["baseDN"] == nil {
		value["baseDN"] = api.services.LDAP.BaseDN
	}
	if value["bindDN"] == nil {
		value["bindDN"] = api.services.LDAP.AdminDN
	}
	value["bindPassword"] = ""
	c.JSON(http.StatusOK, gin.H{"config": value})
}

func (api *API) saveLDAPConfig(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload = compactConfig(payload)
	if value, ok := payload["url"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.URL = strings.TrimSpace(value)
	}
	if value, ok := payload["baseDN"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.BaseDN = strings.TrimSpace(value)
	}
	if value, ok := payload["bindDN"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.AdminDN = strings.TrimSpace(value)
	}
	if value, ok := payload["bindPassword"].(string); ok && strings.TrimSpace(value) != "" && !strings.Contains(value, "*") {
		api.services.LDAP.AdminPassword = value
	} else {
		delete(payload, "bindPassword")
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "ldap", payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) testLDAPConfig(c *gin.Context) {
	var payload map[string]any
	_ = c.ShouldBindJSON(&payload)
	backupURL := api.services.LDAP.URL
	backupBase := api.services.LDAP.BaseDN
	backupDN := api.services.LDAP.AdminDN
	backupPassword := api.services.LDAP.AdminPassword
	if value, ok := payload["url"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.URL = strings.TrimSpace(value)
	}
	if value, ok := payload["baseDN"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.BaseDN = strings.TrimSpace(value)
	}
	if value, ok := payload["bindDN"].(string); ok && strings.TrimSpace(value) != "" {
		api.services.LDAP.AdminDN = strings.TrimSpace(value)
	}
	if value, ok := payload["bindPassword"].(string); ok && strings.TrimSpace(value) != "" && !strings.Contains(value, "*") {
		api.services.LDAP.AdminPassword = value
	}
	err := api.services.LDAP.Ping()
	api.services.LDAP.URL = backupURL
	api.services.LDAP.BaseDN = backupBase
	api.services.LDAP.AdminDN = backupDN
	api.services.LDAP.AdminPassword = backupPassword
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) getNotifyConfig(c *gin.Context) {
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "notify")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if value["email"] == nil {
		value["email"] = gin.H{
			"account": "",
			"pop3":    "pop.163.com",
			"smtp":    "smtp.163.com",
			"imap":    "imap.163.com",
			"port":    "465 SSL",
		}
	}
	if email, ok := value["email"].(map[string]any); ok {
		email["password"] = ""
	}
	c.JSON(http.StatusOK, gin.H{"config": value})
}

func (api *API) saveNotifyConfig(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload = compactConfig(payload)
	if saved, ok, err := api.services.GetSystemConfig(c.Request.Context(), "notify"); err == nil && ok {
		payload = mergeNotifyConfig(saved, payload)
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "notify", payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) testEmailConfig(c *gin.Context) {
	var payload map[string]any
	_ = c.ShouldBindJSON(&payload)
	payload = compactConfig(payload)
	if saved, ok, err := api.services.GetSystemConfig(c.Request.Context(), "notify"); err == nil && ok {
		payload = mergeNotifyConfig(saved, payload)
	}
	email, ok := payload["email"].(map[string]any)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱配置为空"})
		return
	}
	cfg := emailConfigFromMap(email)
	if cfg.Account == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱账号为空"})
		return
	}
	if cfg.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱密码/授权码为空，请重新输入后保存或测试"})
		return
	}
	if cfg.SMTP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP 服务器为空"})
		return
	}
	if err := sendTestEmail(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "notify", payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) testFeishuWebhook(c *gin.Context) {
	var payload map[string]any
	_ = c.ShouldBindJSON(&payload)
	webhook, _ := payload["webhook"].(string)
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		if saved, _, err := api.services.GetSystemConfig(c.Request.Context(), "notify"); err == nil {
			webhook, _ = saved["webhook"].(string)
		}
	}
	if webhook == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "飞书 Webhook 为空"})
		return
	}
	message := fmt.Sprintf("simpleHPC 飞书 Webhook 测试消息\n时间：%s", time.Now().Format("2006-01-02 15:04:05"))
	if err := sendFeishuText(c.Request.Context(), webhook, message); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) slurmHistory(c *gin.Context) {
	jobs, err := api.services.Slurm.History(c.Request.Context(), c.Query("since"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": jobs, "count": len(jobs)})
}

func (api *API) runInspection(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	run, err := api.services.ExecuteInspection(ctx, user.Username)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}

func (api *API) listInspectionRuns(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	items, err := api.services.InspectionRuns(c.Request.Context(), queryInt(c, "limit", 100))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-inspection-runs"})
}

func (api *API) inspectionRunID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效巡检编号"})
		return 0, false
	}
	return id, true
}

func (api *API) getInspectionRun(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	id, ok := api.inspectionRunID(c)
	if !ok {
		return
	}
	item, err := api.services.InspectionRunByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) downloadInspectionReport(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	id, ok := api.inspectionRunID(c)
	if !ok {
		return
	}
	item, err := api.services.InspectionRunByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(item.ReportHTML))
}

func (api *API) inspectionLog(c *gin.Context, download bool) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	id, ok := api.inspectionRunID(c)
	if !ok {
		return
	}
	item, err := api.services.InspectionRunByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if download {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="inspection-%s.log"`, item.RunID))
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(item.DetailLog))
}

func (api *API) viewInspectionLog(c *gin.Context)     { api.inspectionLog(c, false) }
func (api *API) downloadInspectionLog(c *gin.Context) { api.inspectionLog(c, true) }

func (api *API) notifyInspectionReport(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	id, ok := api.inspectionRunID(c)
	if !ok {
		return
	}
	item, err := api.services.InspectionRunByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	config, _, err := api.services.GetSystemConfig(c.Request.Context(), "notify")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	webhook, _ := config["webhook"].(string)
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统设置中尚未配置飞书机器人 Webhook"})
		return
	}
	reportURL := fmt.Sprintf("%s/api/v1/inspection/runs/%d/report", strings.TrimRight(api.cfg.PublicURL, "/"), item.ID)
	payload := service.BuildInspectionFeishuPost(item, reportURL)
	if err := sendFeishuPayload(c.Request.Context(), webhook, payload); err != nil {
		_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{Actor: user.Username, ActorType: user.Type, Action: "inspection.notify", TargetType: "inspection_run", Target: item.RunID, Result: "failed", Detail: map[string]any{"error": err.Error()}, IPAddress: c.ClientIP()})
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{Actor: user.Username, ActorType: user.Type, Action: "inspection.notify", TargetType: "inspection_run", Target: item.RunID, Result: "success", Detail: map[string]any{"channel": "feishu", "reportUrl": reportURL}, IPAddress: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "巡检报告已发送到飞书机器人", "reportUrl": reportURL})
}

func (api *API) getInspectionConfig(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "inspection")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if value["schedule"] == nil {
		value["schedule"] = "06:30"
	}
	if value["retentionDays"] == nil {
		value["retentionDays"] = 30
	}
	c.JSON(http.StatusOK, gin.H{"config": value})
}

func (api *API) saveInspectionConfig(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var payload struct {
		Schedule      string `json:"schedule"`
		RetentionDays int    `json:"retentionDays"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效巡检配置"})
		return
	}
	if payload.RetentionDays < 1 || payload.RetentionDays > 3650 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "报告保留天数必须为 1-3650"})
		return
	}
	if _, err := time.Parse("15:04", payload.Schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "自动巡检时间格式必须为 HH:MM"})
		return
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "inspection", map[string]any{"schedule": payload.Schedule, "retentionDays": payload.RetentionDays}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) storageRoots(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	items := api.services.Storage.ListRoots()
	value, _, _ := api.services.GetSystemConfig(c.Request.Context(), "storage")
	if configs, ok := value["items"].([]any); ok {
		for index := range items {
			for _, raw := range configs {
				config, _ := raw.(map[string]any)
				if fmt.Sprint(config["path"]) != items[index].Path {
					continue
				}
				if text := strings.TrimSpace(fmt.Sprint(config["type"])); text != "" && text != "<nil>" {
					items[index].Type = text
				}
				if text := strings.TrimSpace(fmt.Sprint(config["name"])); text != "" && text != "<nil>" {
					items[index].Name = text
				}
				if text := strings.TrimSpace(fmt.Sprint(config["fsType"])); text != "" && text != "<nil>" {
					items[index].FSType = text
				}
				if text := strings.TrimSpace(fmt.Sprint(config["purpose"])); text != "" && text != "<nil>" {
					items[index].Purpose = text
				}
				if value, ok := config["warningThreshold"].(float64); ok {
					items[index].WarningThreshold = int(value)
				}
			}
		}
	}
	if user.Type == "admin" && user.Role == "config_admin" {
		if authz, err := api.services.ResolvePermissionContext(c.Request.Context(), user); err == nil &&
			len(authz.FilePolicies) == 0 {
			c.JSON(http.StatusOK, gin.H{"items": items})
			return
		}
	}
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *API) refreshStorageRoots(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	items := api.services.Storage.ListRoots()
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "refreshedAt": time.Now().Format(time.RFC3339)})
}

func (api *API) saveStorageRoots(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	var payload struct {
		Roots []json.RawMessage `json:"roots"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage root payload"})
		return
	}
	roots := make([]string, 0, len(payload.Roots))
	configs := make([]any, 0, len(payload.Roots))
	seen := map[string]bool{}
	for _, raw := range payload.Roots {
		var pathValue string
		config := map[string]any{}
		if json.Unmarshal(raw, &pathValue) != nil {
			if err := json.Unmarshal(raw, &config); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage root"})
				return
			}
			pathValue = fmt.Sprint(config["path"])
		}
		clean := filepath.Clean(strings.TrimSpace(pathValue))
		if clean == "." || !filepath.IsAbs(clean) || seen[clean] {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("storage path is not an accessible directory: %s", clean)})
			return
		}
		seen[clean] = true
		roots = append(roots, clean)
		if len(config) == 0 {
			config = map[string]any{"path": clean}
		}
		config["path"] = clean
		configs = append(configs, config)
	}
	if len(roots) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one storage root is required"})
		return
	}
	values := make([]any, len(roots))
	for index, value := range roots {
		values[index] = value
	}
	if err := api.services.SetSystemConfig(c.Request.Context(), "storage", map[string]any{"roots": values, "items": configs}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	api.services.Storage.Roots = roots
	c.JSON(http.StatusOK, gin.H{"items": api.services.Storage.ListRoots(), "count": len(roots)})
}

func (api *API) storageList(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	showHidden, _ := strconv.ParseBool(c.DefaultQuery("showHidden", "false"))
	path := c.Query("path")
	if !api.requireStorageAccess(c, user, "list", service.AccessView, path) {
		return
	}
	if showHidden {
		authz, _ := storageAuthorization(c)
		if !authz.AllowsHidden(path) {
			api.recordStorageDenied(c, user, "show_hidden", "当前文件策略不允许显示隐藏文件")
			c.JSON(http.StatusForbidden, gin.H{"error": "当前文件策略不允许显示隐藏文件"})
			return
		}
	}
	entries, err := client.List(path, showHidden)
	if err != nil {
		api.storageError(c, user, "list", err)
		return
	}
	meta, err := client.PathMetadata(path)
	if err != nil {
		api.storageError(c, user, "list", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": entries, "count": len(entries),
		"effectivePath": meta.EffectivePath, "initialPath": meta.InitialPath,
		"canGoParent": meta.CanGoParent, "parentPath": meta.ParentPath,
	})
}

func (api *API) storageCreateDirectory(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid directory payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "create_directory", service.AccessManage, payload.Parent) {
		return
	}
	if err := client.CreateDirectory(payload.Parent, payload.Name); err != nil {
		api.storageError(c, user, "create_directory", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (api *API) storageUpload(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<30)
	parent := c.PostForm("path")
	if !api.requireStorageAccess(c, user, "upload", service.AccessManage, parent) {
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	input, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer input.Close()
	if err := client.WriteFile(parent, filepath.Base(header.Filename), input); err != nil {
		api.storageError(c, user, "upload", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "uploaded", "name": filepath.Base(header.Filename), "size": header.Size})
}

type storagePathOperation struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

func (api *API) storageCopy(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload storagePathOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid copy payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "copy_source", service.AccessView, payload.Paths...) ||
		!api.requireStorageAccess(c, user, "copy_destination", service.AccessManage, payload.Destination) {
		return
	}
	if err := client.Copy(payload.Paths, payload.Destination); err != nil {
		api.storageError(c, user, "copy", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "copied"})
}

func (api *API) storageMove(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload storagePathOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid move payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "move_source", service.AccessManage, payload.Paths...) ||
		!api.requireStorageAccess(c, user, "move_destination", service.AccessManage, payload.Destination) {
		return
	}
	if err := client.Move(payload.Paths, payload.Destination); err != nil {
		api.storageError(c, user, "move", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "moved"})
}

func (api *API) storageRename(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rename payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "rename", service.AccessManage, payload.Path) {
		return
	}
	if err := client.Rename(payload.Path, payload.Name); err != nil {
		api.storageError(c, user, "rename", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "renamed"})
}

func (api *API) storageDelete(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload storagePathOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid delete payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "delete", service.AccessManage, payload.Paths...) {
		return
	}
	if err := client.Delete(payload.Paths); err != nil {
		api.storageError(c, user, "delete", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (api *API) storageDownload(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	path := c.Query("path")
	if !api.requireStorageAccess(c, user, "download", service.AccessView, path) {
		return
	}
	file, info, err := client.Open(path)
	if err != nil {
		api.storageError(c, user, "download", err)
		return
	}
	defer file.Close()
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "directories must be downloaded as an archive"})
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, file)
}

func (api *API) storageArchive(c *gin.Context) {
	client, _, user, ok := api.scopedStorage(c)
	if !ok {
		return
	}
	var payload storagePathOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid archive payload"})
		return
	}
	if !api.requireStorageAccess(c, user, "archive", service.AccessView, payload.Paths...) {
		return
	}
	if err := client.ValidateArchive(payload.Paths); err != nil {
		api.storageError(c, user, "archive", err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "simplehpc-selected.zip"}))
	c.Header("Content-Type", "application/zip")
	c.Status(http.StatusOK)
	if err := client.Archive(payload.Paths, c.Writer); err != nil {
		_ = c.Error(err)
	}
}

func compactConfig(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				out[key] = trimmed
			}
		case map[string]any:
			out[key] = compactConfig(typed)
		default:
			if value != nil {
				out[key] = value
			}
		}
	}
	return out
}

func mergeNotifyConfig(saved, incoming map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range saved {
		out[key] = value
	}
	for key, value := range incoming {
		if key != "email" {
			out[key] = value
			continue
		}
		savedEmail, _ := saved["email"].(map[string]any)
		incomingEmail, _ := value.(map[string]any)
		mergedEmail := map[string]any{}
		for emailKey, emailValue := range savedEmail {
			mergedEmail[emailKey] = emailValue
		}
		for emailKey, emailValue := range incomingEmail {
			if emailKey == "password" {
				if password, _ := emailValue.(string); strings.TrimSpace(password) == "" {
					continue
				}
			}
			mergedEmail[emailKey] = emailValue
		}
		out["email"] = mergedEmail
	}
	return out
}

type emailConfig struct {
	Account  string
	Password string
	SMTP     string
	Port     string
}

func emailConfigFromMap(value map[string]any) emailConfig {
	cfg := emailConfig{}
	cfg.Account, _ = value["account"].(string)
	cfg.Password, _ = value["password"].(string)
	cfg.SMTP, _ = value["smtp"].(string)
	cfg.Port, _ = value["port"].(string)
	cfg.Account = strings.TrimSpace(cfg.Account)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.SMTP = strings.TrimSpace(cfg.SMTP)
	cfg.Port = strings.TrimSpace(cfg.Port)
	return cfg
}

func sendTestEmail(ctx context.Context, cfg emailConfig) error {
	subject := "simpleHPC 邮件配置测试"
	body := "这是一封 simpleHPC 系统邮箱测试邮件。\n\n如果你收到这封邮件，说明 SMTP 发信配置可用。\n时间：" + time.Now().Format("2006-01-02 15:04:05")
	return sendEmail(ctx, cfg, cfg.Account, subject, body)
}

func (api *API) sendSystemEmail(ctx context.Context, recipient, subject, body string) error {
	value, ok, err := api.services.GetSystemConfig(ctx, "notify")
	if err != nil {
		return fmt.Errorf("读取系统邮箱配置失败：%w", err)
	}
	if !ok {
		return fmt.Errorf("系统邮箱尚未配置")
	}
	email, ok := value["email"].(map[string]any)
	if !ok {
		return fmt.Errorf("系统邮箱配置无效")
	}
	cfg := emailConfigFromMap(email)
	if cfg.Account == "" || cfg.Password == "" || cfg.SMTP == "" {
		return fmt.Errorf("系统邮箱账号、授权码或 SMTP 配置不完整")
	}
	return sendEmail(ctx, cfg, recipient, subject, body)
}

func sendEmail(ctx context.Context, cfg emailConfig, recipient, subjectText, body string) error {
	port, useSSL, useStartTLS := parseSMTPPort(cfg.Port)
	if port == "" {
		port = "465"
		useSSL = true
	}
	addr := net.JoinHostPort(cfg.SMTP, port)
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return fmt.Errorf("收件人邮箱为空")
	}
	subject := mime.QEncoding.Encode("UTF-8", subjectText)
	message := strings.Join([]string{
		"From: " + cfg.Account,
		"To: " + recipient,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	done := make(chan error, 1)
	go func() {
		auth := smtp.PlainAuth("", cfg.Account, cfg.Password, cfg.SMTP)
		if useSSL {
			done <- sendMailTLS(addr, cfg.SMTP, auth, cfg.Account, []string{recipient}, []byte(message))
			return
		}
		if useStartTLS {
			done <- sendMailStartTLS(addr, cfg.SMTP, auth, cfg.Account, []string{recipient}, []byte(message))
			return
		}
		done <- smtp.SendMail(addr, auth, cfg.Account, []string{recipient}, []byte(message))
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("SMTP 邮件发送失败：%w", err)
		}
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("SMTP 测试邮件发送超时")
	}
}

func parseSMTPPort(value string) (port string, useSSL bool, useStartTLS bool) {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(value)))
	if len(fields) == 0 {
		return "", false, false
	}
	port = fields[0]
	for _, field := range fields[1:] {
		if field == "SSL" || field == "TLS" {
			useSSL = true
		}
		if field == "STARTTLS" {
			useStartTLS = true
		}
	}
	if port == "465" {
		useSSL = true
	}
	if port == "587" {
		useStartTLS = true
	}
	return port, useSSL, useStartTLS
}

func sendMailTLS(addr, serverName string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, serverName)
	if err != nil {
		return err
	}
	defer client.Quit()
	return smtpSendWithClient(client, auth, from, to, msg)
}

func sendMailStartTLS(addr, serverName string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Quit()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	return smtpSendWithClient(client, auth, from, to, msg)
}

func smtpSendWithClient(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func primaryIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	fallback := "127.0.0.1"
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		text := ip.String()
		if strings.HasPrefix(text, "10.") || strings.HasPrefix(text, "172.") || strings.HasPrefix(text, "192.168.") {
			return text
		}
		fallback = text
	}
	return fallback
}

func countNodeExpression(value string) int {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "(null)") {
		return 0
	}
	if !strings.Contains(value, "[") {
		return len(strings.Split(value, ","))
	}
	start := strings.Index(value, "[")
	end := strings.Index(value[start:], "]")
	if start < 0 || end < 0 {
		return 1
	}
	inside := value[start+1 : start+end]
	count := 0
	for _, part := range strings.Split(inside, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, _ := strconv.Atoi(strings.TrimLeft(bounds[0], "0"))
			hi, _ := strconv.Atoi(strings.TrimLeft(bounds[1], "0"))
			if hi >= lo {
				count += hi - lo + 1
				continue
			}
		}
		if part != "" {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func gresCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "(null)") {
		return 0
	}
	total := 0
	for _, token := range strings.Split(value, ",") {
		parts := strings.Split(token, ":")
		if len(parts) == 0 || !strings.Contains(strings.ToLower(token), "gpu") {
			continue
		}
		last := parts[len(parts)-1]
		if n, err := strconv.Atoi(last); err == nil {
			total += n
		}
	}
	return total
}

func summarizePartitionNodes(partition string, nodes []slurmintegration.Node) string {
	counts := partitionNodeStates(partition, nodes)
	if len(counts) == 0 {
		return "数据未获取"
	}
	parts := []string{}
	for _, key := range []string{"idle", "alloc", "mix", "drain", "down"} {
		if counts[key] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[key], key))
		}
	}
	for key, count := range counts {
		if key != "idle" && key != "alloc" && key != "mix" && key != "drain" && key != "down" {
			parts = append(parts, fmt.Sprintf("%d %s", count, key))
		}
	}
	return strings.Join(parts, " / ")
}

func partitionNodeStates(partition string, nodes []slurmintegration.Node) map[string]int {
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, node := range nodes {
		if partition != "" && !strings.Contains(node.Partition, partition) {
			continue
		}
		if seen[node.Name] {
			continue
		}
		seen[node.Name] = true
		state := strings.ToLower(node.State)
		if strings.Contains(state, "idle") {
			counts["idle"]++
		} else if strings.Contains(state, "mix") {
			counts["mix"]++
		} else if strings.Contains(state, "drain") {
			counts["drain"]++
		} else if strings.Contains(state, "down") || strings.Contains(state, "fail") {
			counts["down"]++
		} else if strings.Contains(state, "alloc") {
			counts["alloc"]++
		} else if state != "" {
			counts[strings.TrimRight(state, "*-+~#!")]++
		}
	}
	return counts
}

func sendFeishuText(ctx context.Context, webhook, text string) error {
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]any{"text": text},
	}
	return sendFeishuPayload(ctx, webhook, payload)
}

func sendFeishuPayload(ctx context.Context, webhook string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("飞书返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if len(data) > 0 && json.Unmarshal(data, &result) == nil && result.Code != 0 {
		if result.Msg == "" {
			result.Msg = strings.TrimSpace(string(data))
		}
		return fmt.Errorf("飞书返回失败 code=%d msg=%s", result.Code, result.Msg)
	}
	return nil
}

func check(fn func() error) healthItem {
	if err := fn(); err != nil {
		return healthItem{Status: "error", Error: err.Error()}
	}
	return healthItem{Status: "ok"}
}

func statusText(status int) string {
	if status >= 200 && status < 300 {
		return "ok"
	}
	return "degraded"
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func atoi(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
