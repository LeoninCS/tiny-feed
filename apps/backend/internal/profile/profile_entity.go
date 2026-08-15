package profile

import "tiny-feed/internal/account"

// GetProfileRequest 主页查询请求体。
type GetProfileRequest struct {
	AccountID uint `json:"account_id"`
}

// GetProfileResponse 主页聚合返回。
// Account 直接复用 account.FindByIDResponse，不再单独定义重复的 ProfileAccount。
type GetProfileResponse struct {
	Account       account.FindByIDResponse `json:"account"`
	VideoCount    int64                    `json:"video_count"`
	TotalLikes    int64                    `json:"total_likes"`
	FollowerCount int64                    `json:"follower_count"`
	VloggerCount  int64                    `json:"vlogger_count"`
}
