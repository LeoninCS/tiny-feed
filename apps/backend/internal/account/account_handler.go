package account

// 账号 HTTP 处理器：负责把 HTTP 请求转换成对 AccountService 的调用。
// 每个方法对应 router 里注册的一个路由，负责参数绑定、调用服务、
// 把结果（或错误）按统一格式返回给前端。

import (
	"net/http"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// AccountHandler 把 HTTP 层的请求转给 AccountService。
type AccountHandler struct {
	service *AccountService
}

// NewAccountHandler 构造账号处理器。
func NewAccountHandler(service *AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// CreateAccount 处理 POST /account/register。
// 请求体：{username, password}。
// 成功返回 200 + {id, username}；用户名/密码为空或已存在返回 400。
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	acc, err := h.service.CreateAccount(c.Request.Context(), &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": acc.ID, "username": acc.Username})
}

// Login 处理 POST /account/login。
// 请求体：{username, password}。
// 成功返回 200 + {token, refresh_token, account_id, username}。
// 用户名/密码错误统一返回 401，避免暴露账号是否存在。
func (h *AccountHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ChangePassword 处理 POST /account/changePassword。
// 请求体：{username, old_password, new_password}。
// 注意：该接口在当前 router 中是公开的（不要求登录），
// 因为它本身通过"知道旧密码"来完成身份验证。
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), &req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// FindByID 处理 POST /account/findByID。
// 请求体：{id}。
func (h *AccountHandler) FindByID(c *gin.Context) {
	var req FindByIDRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.FindByID(c.Request.Context(), req.ID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// FindByUsername 处理 POST /account/findByUsername。
// 请求体：{username}。
func (h *AccountHandler) FindByUsername(c *gin.Context) {
	var req FindByUsernameRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Logout 处理 POST /account/logout（受 JWT 保护）。
// 从 context 取出当前登录用户 ID，调 service 把 token 清空。
func (h *AccountHandler) Logout(c *gin.Context) {
	// GetAccountID 失败说明 jwt 中间件没把 accountID 放进 context，
	// 理论上不会被触发（因为已经过了 JWT 中间件），兜底返回 401。
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Logout(c.Request.Context(), accountID); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Rename 处理 POST /account/rename（受 JWT 保护）。
// 请求体：{new_username}。
// 改名会重新签发 token，但前端需要用新 token 替换旧值。
func (h *AccountHandler) Rename(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req RenameRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Rename(c.Request.Context(), accountID, req.NewUsername); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UpdateProfile 处理 POST /account/updateProfile（受 JWT 保护）。
// 请求体：{avatar_url, bio}，只更新非空字段。
func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req UpdateProfileRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.UpdateProfile(c.Request.Context(), accountID, &req); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Refresh 处理 POST /account/refresh。
// 不走 JWT 中间件，而是通过 X-Refresh-Token 请求头里的 refresh token
// 重新签发一对新令牌（access + refresh）。
// 流程：取 header → 查 DB 找到对应账号 → 重新签发 → 写回 DB → 返回新令牌。
func (h *AccountHandler) Refresh(c *gin.Context) {
	// refresh token 通过专用 header 传，避免和 Bearer access token 混在一起。
	refreshToken := c.GetHeader("X-Refresh-Token")
	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token required"})
		return
	}
	// 根据 refresh token 找出账号。
	accountID, username, err := h.service.ResolveRefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		// refresh token 无效或已撤销，统一返回 401。
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// 签发新的一对令牌，旧 refresh token 也会随之失效。
	newToken, newRefresh, err := h.service.IssueTokens(c.Request.Context(), accountID, username)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":         newToken,
		"refresh_token": newRefresh,
		"account_id":    accountID,
		"username":      username,
	})
}
