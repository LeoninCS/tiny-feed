package video

// 点赞服务层：
//  - Like    点赞（同时把 video.likes_count +1）；
//  - Unlike  取消点赞（同时把 video.likes_count -1，且不能减为负数）；
//  - IsLiked 查询当前用户是否已点赞某视频；
//  - ListMyLikedVideos 列出我点过赞的全部视频；
//  - BatchGetLiked 批量查询当前用户对一组视频的点赞状态（feed 列表用）。
// 同一 video_id+account_id 通过 (video_id, account_id) 唯一索引去重。
// likes_count 字段本身通过 SQL UpdateColumn 原子增减，所以点赞/取消点赞
// 并发安全；这里只关心主表写入成功，计数失败 log 出来不阻断主流程。

import (
	"context"
	"log"

	"tiny-feed/internal/apierror"

	"gorm.io/gorm"
)

// LikeService 点赞业务的服务端。
type LikeService struct {
	likeRepo  *LikeRepository
	videoRepo *VideoRepository
}

// NewLikeService 构造点赞服务。
func NewLikeService(likeRepo *LikeRepository, videoRepo *VideoRepository) *LikeService {
	return &LikeService{likeRepo: likeRepo, videoRepo: videoRepo}
}

// Like 点赞。
// 流程：校验 video_id → 确认视频存在 → 写入 like 表（已存在则忽略） → 若是新插入则 likes_count +1。
// 这样即使前端重复点击也不会产生脏数据。
func (s *LikeService) Like(ctx context.Context, accountID, videoID uint) error {
	if err := apierror.RequireID(videoID, "video_id"); err != nil {
		return err
	}
	exists, err := s.videoRepo.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrVideoNotFound
	}
	// LikeIgnoreDuplicate 借助唯一索引 (video_id, account_id) 实现幂等。
	created, err := s.likeRepo.LikeIgnoreDuplicate(ctx, &Like{
		VideoID:   videoID,
		AccountID: accountID,
	})
	if err != nil {
		return err
	}
	// 只有真正新插入时才 +1，避免重复点击导致计数错乱。
	// ChangeLikesCount 本身用 UpdateColumn 原子操作，失败仅意味着统计字段
	// 没及时追上（异步对账/定时修复可补），主流程不应因此报错。
	if created {
		if err := s.videoRepo.ChangeLikesCount(ctx, videoID, 1); err != nil {
			log.Printf("点赞成功但 likes_count 自增失败：videoID=%d err=%v", videoID, err)
		}
	}
	return nil
}

// Unlike 取消点赞。
// 流程：调用 repo 的 DeleteByVideoAndAccount（利用 unique 索引的删除） → 若真的删了行则 likes_count -1。
// ChangeLikesCount 在 repo 里用 GREATEST(.., 0) 兜底，天然不会减成负数。
func (s *LikeService) Unlike(ctx context.Context, accountID, videoID uint) error {
	if err := apierror.RequireID(videoID, "video_id"); err != nil {
		return err
	}
	deleted, err := s.likeRepo.DeleteByVideoAndAccount(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if deleted {
		if err := s.videoRepo.ChangeLikesCount(ctx, videoID, -1); err != nil {
			log.Printf("取消点赞成功但 likes_count 自减失败：videoID=%d err=%v", videoID, err)
		}
	}
	return nil
}

// IsLiked 查询当前用户是否对某视频点过赞。
func (s *LikeService) IsLiked(ctx context.Context, accountID, videoID uint) (bool, error) {
	if err := apierror.RequireID(videoID, "video_id"); err != nil {
		return false, err
	}
	return s.likeRepo.IsLiked(ctx, videoID, accountID)
}

// ListMyLikedVideos 列出我点过赞的全部视频。
// 主要用于"我点过赞"页面，按点赞时间倒序。
func (s *LikeService) ListMyLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	if err := apierror.RequireID(accountID, "account_id"); err != nil {
		return nil, err
	}
	return s.likeRepo.ListLikedVideos(ctx, accountID)
}

// BatchGetLiked 批量查询当前用户对一组视频的点赞状态，返回 {videoID: bool}。
// 给 feed 列表用，避免 N 次单查。
func (s *LikeService) BatchGetLiked(ctx context.Context, accountID uint, videoIDs []uint) (map[uint]bool, error) {
	if len(videoIDs) == 0 {
		return map[uint]bool{}, nil
	}
	result, err := s.likeRepo.BatchGetLiked(ctx, videoIDs, accountID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if result == nil {
		return map[uint]bool{}, nil
	}
	return result, nil
}
