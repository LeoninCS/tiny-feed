package video

// 评论 HTTP 处理器。

import (
	"errors"
	"net/http"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// CommentHandler 把 HTTP 请求转给 CommentService。
type CommentHandler struct {
	service *CommentService
}

// NewCommentHandler 构造评论处理器。
func NewCommentHandler(service *CommentService) *CommentHandler {
	return &CommentHandler{service: service}
}

// Publish 处理 POST /comment/publish（受 JWT 保护）。
// 请求体：{video_id, content}。
func (h *CommentHandler) Publish(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username, err := jwtmw.GetUsername(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req PublishCommentRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	cm, err := h.service.Publish(c.Request.Context(), accountID, username, &req)
	if err != nil {
		// ErrCommentForbidden 不在 apierror 通用列表里，单独映射 403。
		if errors.Is(err, ErrCommentForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cm)
}

// Delete 处理 POST /comment/delete（受 JWT 保护）。
// 请求体：{comment_id}。鉴权：只能删自己的评论。
func (h *CommentHandler) Delete(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req DeleteCommentRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Delete(c.Request.Context(), accountID, req.CommentID); err != nil {
		// 业务级 sentinel 单独处理：404 / 403
		switch {
		case errors.Is(err, ErrCommentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCommentForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetAll 处理 POST /comment/listAll（公开）。
// 请求体：{video_id}。
func (h *CommentHandler) GetAll(c *gin.Context) {
	var req GetAllCommentsRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	comments, err := h.service.GetAll(c.Request.Context(), req.VideoID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments})
}
