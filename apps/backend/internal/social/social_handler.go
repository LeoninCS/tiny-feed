package social

// 关注 HTTP 处理器。

import (
	"net/http"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// SocialHandler 把 HTTP 请求转给 SocialService。
type SocialHandler struct {
	service *SocialService
}

// NewSocialHandler 构造关注处理器。
func NewSocialHandler(service *SocialService) *SocialHandler {
	return &SocialHandler{service: service}
}

// Follow 处理 POST /social/follow（受 JWT 保护）。
// follower_id 自动从 JWT 取，前端只需传 vlogger_id。
func (h *SocialHandler) Follow(c *gin.Context) {
	followerID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req FollowRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Follow(c.Request.Context(), followerID, req.VloggerID); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Unfollow 处理 POST /social/unfollow（受 JWT 保护）。
func (h *SocialHandler) Unfollow(c *gin.Context) {
	followerID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req UnfollowRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Unfollow(c.Request.Context(), followerID, req.VloggerID); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetAllFollowers 处理 POST /social/getAllFollowers（公开）。
// 请求体：{vlogger_id}。
func (h *SocialHandler) GetAllFollowers(c *gin.Context) {
	var req GetAllFollowersRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.GetAllFollowers(c.Request.Context(), req.VloggerID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllVloggers 处理 POST /social/getAllVloggers（公开）。
// 请求体：{follower_id}。
func (h *SocialHandler) GetAllVloggers(c *gin.Context) {
	var req GetAllVloggersRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.GetAllVloggers(c.Request.Context(), req.FollowerID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetCounts 处理 POST /social/getCounts（受 JWT 保护）。
// 返回当前登录用户的粉丝数和关注数。
func (h *SocialHandler) GetCounts(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	counts, err := h.service.GetCounts(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, counts)
}
