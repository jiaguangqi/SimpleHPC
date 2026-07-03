package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) registerTemplateEndpoint(c *gin.Context) {
	var body struct {
		Node     string `json:"node"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "端点数据格式错误"})
		return
	}
	run, err := api.services.RegisterTemplateEndpoint(c.Request.Context(), c.Param("token"), body.Node, body.Port, body.Protocol)
	if err != nil {
		status := http.StatusBadRequest
		if err == sql.ErrNoRows {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (api *API) templateGateway(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	run, err := api.services.TemplateRunByToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交互作业不存在"})
		return
	}
	if run.Username != user.Username && !service.IsTemplateManager(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该交互作业"})
		return
	}
	if run.Status != "ready" || run.TargetNode == "" || run.TargetPort == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "交互服务尚未就绪"})
		return
	}
	target, _ := url.Parse("http://" + run.TargetNode + ":" + strconv.Itoa(run.TargetPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/" + strings.TrimPrefix(c.Param("path"), "/")
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(w, "交互服务连接失败: "+proxyErr.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
