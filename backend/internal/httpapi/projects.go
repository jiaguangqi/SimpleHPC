package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func parseProjectID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目 ID 无效"})
		return 0, false
	}
	return id, true
}

func parseProjectChildID(c *gin.Context, name string, label string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": label + " ID 无效"})
		return 0, false
	}
	return id, true
}

func writeProjectError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目数据不存在或无权访问"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (api *API) projectCanManageAll(c *gin.Context, user service.AuthUser) bool {
	if authz, ok := permissionContext(c); ok {
		return authz.IsClusterAdmin ||
			authz.Has("api.projects.create") && strings.Contains(strings.Join(authz.RoleCodes, ","), "unit_admin") ||
			authz.Has("api.projects.create") && strings.Contains(strings.Join(authz.RoleCodes, ","), "team_admin")
	}
	return false
}

func (api *API) requireProjectAccess(c *gin.Context, projectID int64, needManage bool) (service.AuthUser, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return service.AuthUser{}, false
	}
	canAll := api.projectCanManageAll(c, user)
	role, access, err := api.services.ProjectAccess(c.Request.Context(), projectID, user.Username, canAll)
	if err != nil {
		writeProjectError(c, err)
		return service.AuthUser{}, false
	}
	if needManage && access != "manage" {
		c.JSON(http.StatusForbidden, gin.H{"error": "当前账号无权管理该项目"})
		return service.AuthUser{}, false
	}
	c.Set("simplehpc.project.role", role)
	c.Set("simplehpc.project.access", access)
	return user, true
}

func (api *API) listProjects(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	query := service.ProjectQuery{
		Keyword:      c.Query("q"),
		Status:       c.Query("status"),
		Username:     user.Username,
		CanManageAll: api.projectCanManageAll(c, user),
	}
	resp, err := api.services.ListProjects(c.Request.Context(), query)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (api *API) getProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	project, err := api.services.GetProject(c.Request.Context(), id, user.Username, api.projectCanManageAll(c, user))
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (api *API) createProject(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	var input service.ProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if strings.TrimSpace(input.OwnerUsername) == "" {
		input.OwnerUsername = user.Username
	}
	project, err := api.services.SaveProject(c.Request.Context(), 0, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusCreated, project)
}

func (api *API) updateProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	var input service.ProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	project, err := api.services.SaveProject(c.Request.Context(), id, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (api *API) deleteProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.DeleteProject(c.Request.Context(), id, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *API) upsertProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	var input service.ProjectMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	member, err := api.services.SaveProjectMember(c.Request.Context(), id, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, member)
}

func (api *API) deleteProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.DeleteProjectMember(c.Request.Context(), id, c.Param("username"), user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *API) syncProjectSlurm(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.SyncProjectSlurmAccount(c.Request.Context(), id, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	project, err := api.services.GetProject(c.Request.Context(), id, user.Username, api.projectCanManageAll(c, user))
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (api *API) setDefaultProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员账号不能为空"})
		return
	}
	if err := api.services.SetDefaultProjectForUser(c.Request.Context(), id, username, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	project, err := api.services.GetProject(c.Request.Context(), id, user.Username, api.projectCanManageAll(c, user))
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (api *API) upsertProjectTask(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	var input service.ProjectTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	task, err := api.services.SaveProjectTask(c.Request.Context(), id, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func (api *API) deleteProjectTask(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	taskID, ok := parseProjectChildID(c, "taskId", "任务")
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.DeleteProjectTask(c.Request.Context(), id, taskID, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *API) upsertProjectDirectory(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	var input service.ProjectDirectoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	item, err := api.services.SaveProjectDirectory(c.Request.Context(), id, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) deleteProjectDirectory(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	dirID, ok := parseProjectChildID(c, "directoryId", "目录")
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.DeleteProjectDirectory(c.Request.Context(), id, dirID, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *API) upsertProjectJobLink(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	var input service.ProjectJobLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	item, err := api.services.SaveProjectJobLink(c.Request.Context(), id, input, user.Username)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) deleteProjectJobLink(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	linkID, ok := parseProjectChildID(c, "linkId", "作业关联")
	if !ok {
		return
	}
	user, ok := api.requireProjectAccess(c, id, true)
	if !ok {
		return
	}
	if err := api.services.DeleteProjectJobLink(c.Request.Context(), id, linkID, user.Username); err != nil {
		writeProjectError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
