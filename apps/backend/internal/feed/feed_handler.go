package feed

// Feed HTTP 处理器：把不同类型的 feed 查询转给 FeedService。
// accountID 通过 JWT 中间件或可选认证拿到，未登录时为 0（不填 is_liked）。

import (
	"net/http"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// FeedHandler 把 HTTP 请求转给 FeedService。
type FeedHandler struct {
	service *FeedService
}

// NewFeedHandler 构造 feed 处理器。
func NewFeedHandler(service *FeedService) *FeedHandler {
	return &FeedHandler{service: service}
}

// currentAccountID 试着从 context 拿 accountID，拿不到返回 0。
// 用于公开 feed 接口的"可选登录"场景：登录了就填 is_liked，未登录就全 false。
func (h *FeedHandler) currentAccountID(c *gin.Context) uint {
	id, err := jwtmw.GetAccountID(c)
	if err != nil {
		return 0
	}
	return id
}

// ListLatest 处理 POST /feed/listLatest（公开，可选登录）。
// 请求体：{limit, latest_time}。
func (h *FeedHandler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.ListLatest(c.Request.Context(), h.currentAccountID(c), &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListLikesCount 处理 POST /feed/listLikesCount（公开，可选登录）。
// 请求体：{limit, likes_count_before, id_before}。
func (h *FeedHandler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.ListLikesCount(c.Request.Context(), h.currentAccountID(c), &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListByFollowing 处理 POST /feed/listByFollowing（受 JWT 保护）。
// 必须登录才能拉"我关注的人"的视频。
func (h *FeedHandler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.ListByFollowing(c.Request.Context(), h.currentAccountID(c), &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListByPopularity 处理 POST /feed/listByPopularity（受 JWT 保护）。
// 请求体：{limit, as_of, offset, latest_popularity, latest_before, latest_id_before}。
// 首次请求 offset=0；后续请求把上次的 next_* 字段带回来。
func (h *FeedHandler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.ListByPopularity(c.Request.Context(), h.currentAccountID(c), &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListByTag 处理 POST /feed/listByTag（公开，可选登录）。
// 请求体：{tag, limit}。
func (h *FeedHandler) ListByTag(c *gin.Context) {
	var req ListByTagRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	resp, err := h.service.ListByTag(c.Request.Context(), h.currentAccountID(c), req.Tag, req.Limit)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
