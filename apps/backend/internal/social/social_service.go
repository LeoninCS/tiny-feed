package social

// 关注服务层：负责"用户—用户"的关注关系。
// 数据模型：social 表存 (follower_id, vlogger_id) 关系对。
//  - follower_id 关注人；
//  - vlogger_id  被关注的人（视频创作者 / "博主"）。
// 接口语义：
//  - Follow    关注（幂等）；
//  - Unfollow  取关（幂等）；
//  - GetAllFollowers 列出某 vlogger 的所有粉丝；
//  - GetAllVloggers  列出某 follower 关注的所有 vlogger；
//  - GetCounts       同时返回粉丝数和关注数。

import (
	"context"
	"errors"

	"tiny-feed/internal/account"
	"tiny-feed/internal/apierror"
)

// SocialService 关注业务的服务端。
type SocialService struct {
	repo        *SocialRepository
	accountRepo *account.AccountRepository
}

// NewSocialService 构造关注服务。
func NewSocialService(repo *SocialRepository, accountRepo *account.AccountRepository) *SocialService {
	return &SocialService{repo: repo, accountRepo: accountRepo}
}

// Follow 关注一个用户。
// 业务校验：不能关注自己；vlogger 必须存在。
// 写入由 SocialRepository 处理，它依赖 (follower_id, vlogger_id) 唯一索引保证幂等。
func (s *SocialService) Follow(ctx context.Context, followerID, vloggerID uint) error {
	if err := apierror.RequireID(followerID, "follower_id"); err != nil {
		return err
	}
	if err := apierror.RequireID(vloggerID, "vlogger_id"); err != nil {
		return err
	}
	// 自关注没意义。
	if followerID == vloggerID {
		return errors.New("cannot follow yourself")
	}
	// 确认被关注人存在（避免关注一个不存在的 ID 导致表里出现"幽灵关系"）。
	if _, err := s.accountRepo.FindByID(ctx, vloggerID); err != nil {
		return errors.New("vlogger not found")
	}
	return s.repo.Follow(ctx, &Social{
		FollowerID: followerID,
		VloggerID:  vloggerID,
	})
}

// Unfollow 取消关注。
// 没有 (follower_id, vlogger_id) 关系时静默成功（幂等）。
func (s *SocialService) Unfollow(ctx context.Context, followerID, vloggerID uint) error {
	if err := apierror.RequireID(followerID, "follower_id"); err != nil {
		return err
	}
	if err := apierror.RequireID(vloggerID, "vlogger_id"); err != nil {
		return err
	}
	return s.repo.Unfollow(ctx, &Social{
		FollowerID: followerID,
		VloggerID:  vloggerID,
	})
}

// GetAllFollowers 列出 vloggerID 的全部粉丝及粉丝总数。
func (s *SocialService) GetAllFollowers(ctx context.Context, vloggerID uint) (*GetAllFollowersResponse, error) {
	if err := apierror.RequireID(vloggerID, "vlogger_id"); err != nil {
		return nil, err
	}
	followers, err := s.repo.GetAllFollowers(ctx, vloggerID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountFollowers(ctx, vloggerID)
	if err != nil {
		return nil, err
	}
	return &GetAllFollowersResponse{Followers: followers, FollowerCount: count}, nil
}

// GetAllVloggers 列出 followerID 关注的所有 vlogger 及关注总数。
func (s *SocialService) GetAllVloggers(ctx context.Context, followerID uint) (*GetAllVloggersResponse, error) {
	if err := apierror.RequireID(followerID, "follower_id"); err != nil {
		return nil, err
	}
	vloggers, err := s.repo.GetAllVloggers(ctx, followerID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountVloggers(ctx, followerID)
	if err != nil {
		return nil, err
	}
	return &GetAllVloggersResponse{Vloggers: vloggers, VloggerCount: count}, nil
}

// GetCounts 同时返回粉丝数和关注数（个人主页右上角用）。
func (s *SocialService) GetCounts(ctx context.Context, accountID uint) (*SocialCounts, error) {
	if err := apierror.RequireID(accountID, "account_id"); err != nil {
		return nil, err
	}
	followerCount, err := s.repo.CountFollowers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	vloggerCount, err := s.repo.CountVloggers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &SocialCounts{
		FollowerCount: followerCount,
		VloggerCount:  vloggerCount,
	}, nil
}
