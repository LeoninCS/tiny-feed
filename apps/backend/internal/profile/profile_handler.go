package profile

// 个人主页聚合接口：从 account、video、social 三个 repo 拼出
// "用户主页"所需的所有信息（账号基础信息 + 视频数 + 累计获赞 + 粉丝数 + 关注数）。
// 之所以独立成包：避免 account 依赖 video/social 形成 import cycle。

import (
	"net/http"

	"tiny-feed/internal/account"
	"tiny-feed/internal/apierror"
	"tiny-feed/internal/social"
	"tiny-feed/internal/video"

	"github.com/gin-gonic/gin"
)

// ProfileHandler 拼装个人主页数据。
type ProfileHandler struct {
	accountRepo *account.AccountRepository
	videoRepo   *video.VideoRepository
	socialRepo  *social.SocialRepository
}

// NewProfileHandler 构造 profile 处理器。
func NewProfileHandler(accountRepo *account.AccountRepository, videoRepo *video.VideoRepository, socialRepo *social.SocialRepository) *ProfileHandler {
	return &ProfileHandler{accountRepo: accountRepo, videoRepo: videoRepo, socialRepo: socialRepo}
}

// GetProfile 处理 POST /account/getProfile（公开）。
// 请求体：{account_id}。
// 流程：查账号基础信息 → 查视频数 → 查累计获赞 → 查粉丝数 → 查关注数 → 一起返回。
// 任何一步的统计查询失败都用 0 兜底，不影响主体信息返回。
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	var req GetProfileRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := apierror.RequireID(req.AccountID, "account_id"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 主信息：账号基础资料。
	acc, err := h.accountRepo.FindByID(c.Request.Context(), req.AccountID)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	// 统计信息：每项失败都用 0 兜底，确保 profile 主信息总能展示。
	videoCount, _ := h.videoRepo.CountByAuthor(c.Request.Context(), req.AccountID)
	totalLikes, _ := h.videoRepo.TotalLikesByAuthor(c.Request.Context(), req.AccountID)
	followerCount, _ := h.socialRepo.CountFollowers(c.Request.Context(), req.AccountID)
	vloggerCount, _ := h.socialRepo.CountVloggers(c.Request.Context(), req.AccountID)
	c.JSON(http.StatusOK, GetProfileResponse{
		Account: account.FindByIDResponse{
			ID:        acc.ID,
			Username:  acc.Username,
			AvatarURL: acc.AvatarURL,
			Bio:       acc.Bio,
		},
		VideoCount:    videoCount,
		TotalLikes:    totalLikes,
		FollowerCount: followerCount,
		VloggerCount:  vloggerCount,
	})
}
