package video

// 点赞 HTTP 处理器。

import (
	"net/http"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// LikeHandler 把 HTTP 请求转给 LikeService。
type LikeHandler struct {
	service *LikeService
}

// NewLikeHandler 构造点赞处理器。
func NewLikeHandler(service *LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

// Like 处理 POST /like/like（受 JWT 保护）。
func (h *LikeHandler) Like(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req LikeRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Like(c.Request.Context(), accountID, req.VideoID); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Unlike 处理 POST /like/unlike（受 JWT 保护）。
func (h *LikeHandler) Unlike(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req LikeRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Unlike(c.Request.Context(), accountID, req.VideoID); err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// IsLiked 处理 POST /like/isLiked（受 JWT 保护）。
// 返回 {is_liked: bool}。
func (h *LikeHandler) IsLiked(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req LikeRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	liked, err := h.service.IsLiked(c.Request.Context(), accountID, req.VideoID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_liked": liked})
}

// ListMyLikedVideos 处理 POST /like/listMyLikedVideos（受 JWT 保护）。
// 返回 {videos: [...]}
func (h *LikeHandler) ListMyLikedVideos(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	videos, err := h.service.ListMyLikedVideos(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"videos": videos})
}
