package httpapi

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) authMe(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	authz, err := api.services.ResolvePermissionContext(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "权限数据暂不可用"})
		return
	}
	profile, profileErr := api.services.CurrentUserProfile(c.Request.Context(), user)
	if profileErr != nil {
		profile = service.AuthProfile{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			AccountType: user.Type,
			Role:        user.Role,
		}
	}
	menus := service.BuildNavigation(authz.AccountType, authz.Permissions, service.DefaultMenuCatalog())
	c.JSON(http.StatusOK, gin.H{
		"user": user, "profile": profile, "roles": authz.RoleCodes, "permissions": permissionKeys(authz.Permissions),
		"dataScopes": authz.DataScopes, "accessLevels": authz.AccessLevels,
		"menus": menus, "flatMenu": authz.AccountType != "admin", "version": authz.Version,
	})
}

func permissionKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Type     string `json:"type" binding:"required"`
}

type resetRequest struct {
	Username string `json:"username" binding:"required"`
	Type     string `json:"type" binding:"required"`
}

type resetConfirmRequest struct {
	RequestID string `json:"requestId" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

func (api *API) login(c *gin.Context) {
	var payload loginRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请输入账号和密码"})
		return
	}
	user, token, err := api.services.Authenticate(
		c.Request.Context(), payload.Type, payload.Username, payload.Password,
	)
	if err != nil {
		_ = api.services.RecordAuthEvent(c.Request.Context(), service.AuthEvent{
			Username: payload.Username, AccountType: payload.Type, Event: "login", Result: "failed",
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), Message: "认证失败",
		})
		status := http.StatusUnauthorized
		message := "账号或密码错误"
		if errors.Is(err, service.ErrAccountFrozen) {
			status = http.StatusLocked
			if strings.EqualFold(payload.Type, "ldap") {
				message = "账号已冻结，请联系组长、组长助理或集群管理员"
			} else {
				message = "管理员账号已冻结，请联系集群管理员"
			}
		}
		c.JSON(status, gin.H{"status": "error", "message": message})
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("simplehpc_session", token, 12*60*60, "/", "", false, true)
	_ = api.services.RecordAuthEvent(c.Request.Context(), service.AuthEvent{
		Username: user.Username, DisplayName: user.DisplayName, AccountType: user.Type, Event: "login", Result: "success",
		IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), SessionID: sessionDigest(token), Message: "登录成功",
	})
	c.JSON(http.StatusOK, gin.H{"status": "ok", "token": token, "user": user})
}

func sessionDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:8])
}

func (api *API) requestPasswordReset(c *gin.Context) {
	var payload resetRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请输入账号"})
		return
	}
	requestID, err := api.services.RequestPasswordReset(
		c.Request.Context(),
		payload.Type,
		payload.Username,
		func(email, code string) error {
			subject := "simpleHPC 密码重置验证码"
			body := strings.Join([]string{
				"您好，",
				"",
				"您正在重置 simpleHPC 账号密码。",
				"验证码：" + code,
				"验证码 10 分钟内有效，最多允许输入 5 次。",
				"",
				"如非本人操作，请忽略本邮件。",
			}, "\n")
			return api.sendSystemEmail(c.Request.Context(), email, subject, body)
		},
	)
	if err != nil {
		if errors.Is(err, service.ErrResetTooFrequent) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": "验证码发送失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"requestId": requestID,
		"message":   "如果账号存在且已绑定邮箱，验证码将发送至该邮箱",
	})
}

func (api *API) confirmPasswordReset(c *gin.Context) {
	var payload resetConfirmRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请输入邮箱验证码"})
		return
	}
	result, err := api.services.ConfirmPasswordReset(
		c.Request.Context(),
		payload.RequestID,
		payload.Code,
		func(email, password string) error {
			subject := "simpleHPC 密码重置成功"
			body := strings.Join([]string{
				"您好，",
				"",
				"您的 simpleHPC 账号密码已重置。",
				"新密码：" + password,
				"",
				"请使用新密码登录并妥善保管。请勿向他人泄露此邮件。",
			}, "\n")
			return api.sendSystemEmail(c.Request.Context(), email, subject, body)
		},
	)
	if err != nil {
		if errors.Is(err, service.ErrResetCodeInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": "密码重置失败，请重新获取验证码"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"result":  result,
		"message": "密码重置成功，请至邮箱查看新密码",
	})
}
