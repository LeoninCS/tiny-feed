package feed

// Feed 服务层：负责"信息流"相关的查询与拼装。
// 没有 Redis 缓存层，所有请求都直接查 MySQL + 在内存里拼装成前端需要的形态。
// 主要职责：
//  1. 调 FeedRepository 拉原始视频列表；
//  2. 调 LikeRepository 批量取"当前用户对这些视频的点赞状态"；
//  3. 调 AccountRepository 拼装每条视频的作者信息（去重后批量查）；
//  4. 把分页游标（next_time / next_offset）算好返回给前端。

import (
	"context"
	"errors"
	"time"

	"tiny-feed/internal/account"
	"tiny-feed/internal/apierror"
	"tiny-feed/internal/video"
)

// FeedService 是 feed 业务的服务端。
type FeedService struct {
	repo        *FeedRepository
	likeRepo    *video.LikeRepository
	accountRepo *account.AccountRepository
}

// NewFeedService 构造 feed 服务。
func NewFeedService(repo *FeedRepository, likeRepo *video.LikeRepository, accountRepo *account.AccountRepository) *FeedService {
	return &FeedService{repo: repo, likeRepo: likeRepo, accountRepo: accountRepo}
}

// ListLatest 按时间倒序拉取最新视频。
// 分页用 (limit, latest_time) 二元组：latest_time 是上一页最后一条的 create_time 毫秒。
func (s *FeedService) ListLatest(ctx context.Context, accountID uint, req *ListLatestRequest) (*ListLatestResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	// 第一次请求 latest_time=0，把上限设为"当前时间+1s"，确保不漏当前正在发布的那一条。
	var latestBefore time.Time
	if req.LatestTime > 0 {
		latestBefore = time.UnixMilli(req.LatestTime)
	} else {
		latestBefore = time.Now().Add(1 * time.Second)
	}
	videos, err := s.repo.ListLatest(ctx, limit, latestBefore)
	if err != nil {
		return nil, err
	}
	items, nextTime, hasMore, err := s.toFeedItems(ctx, accountID, videos, limit, latestBefore)
	if err != nil {
		return nil, err
	}
	return &ListLatestResponse{
		VideoList: items,
		NextTime:  nextTime,
		HasMore:   hasMore,
	}, nil
}

// ListLikesCount 按点赞数倒序拉取视频。
// 分页用 (likes_count, id) 复合游标，保证翻页时不会出现重复或跳行。
func (s *FeedService) ListLikesCount(ctx context.Context, accountID uint, req *ListLikesCountRequest) (*ListLikesCountResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var cursor *LikesCountCursor
	// 第一页时 LikesCountBefore 和 IDBefore 都为空，从头开始。
	if req.LikesCountBefore != nil && req.IDBefore != nil {
		cursor = &LikesCountCursor{
			LikesCount: *req.LikesCountBefore,
			ID:         *req.IDBefore,
		}
	}
	videos, err := s.repo.ListLikesCountWithCursor(ctx, limit, cursor)
	if err != nil {
		return nil, err
	}
	items, _, _, err := s.toFeedItems(ctx, accountID, videos, limit, time.Time{})
	if err != nil {
		return nil, err
	}
	return &ListLikesCountResponse{
		VideoList: items,
		HasMore:   len(videos) == limit,
	}, nil
}

// ListByFollowing 拉取我关注的人的最近视频（受 JWT 保护）。
func (s *FeedService) ListByFollowing(ctx context.Context, accountID uint, req *ListByFollowingRequest) (*ListByFollowingResponse, error) {
	if err := apierror.RequireID(accountID, "account_id"); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var latestBefore time.Time
	if req.LatestTime > 0 {
		latestBefore = time.UnixMilli(req.LatestTime)
	} else {
		latestBefore = time.Now().Add(1 * time.Second)
	}
	// repo 内部会先查关注列表，再按时间倒序查这些 vlogger 的视频。
	videos, err := s.repo.ListByFollowing(ctx, limit, accountID, latestBefore)
	if err != nil {
		return nil, err
	}
	items, nextTime, hasMore, err := s.toFeedItems(ctx, accountID, videos, limit, latestBefore)
	if err != nil {
		return nil, err
	}
	return &ListByFollowingResponse{
		VideoList: items,
		NextTime:  nextTime,
		HasMore:   hasMore,
	}, nil
}

// ListByPopularity 按热度（popularity、create_time、id 复合）排序的 feed。
// 热度 feed 比较复杂：使用两段式分页——
//  - 第一页：按 (popularity desc, create_time desc, id desc) 取；
//  - 后续页：基于上一页最后一条的 (popularity, create_time, id) 继续翻。
func (s *FeedService) ListByPopularity(ctx context.Context, accountID uint, req *ListByPopularityRequest) (*ListByPopularityResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var popBefore int64
	var timeBefore time.Time
	var idBefore uint
	if req.Offset == 0 {
		// 第一页：as_of 是客户端指定的"截止时间"（分钟级 unix time），
		// 缺省则用当前时间，确保立刻返回最新热度。
		if req.AsOf > 0 {
			timeBefore = time.Unix(req.AsOf, 0)
		} else {
			timeBefore = time.Now()
		}
	} else {
		// 后续页：用上一页返回的游标。
		popBefore = req.LatestPopularity
		timeBefore = req.LatestBefore
		if req.LatestIDBefore != nil {
			idBefore = *req.LatestIDBefore
		}
	}
	videos, err := s.repo.ListByPopularity(ctx, limit, popBefore, timeBefore, idBefore)
	if err != nil {
		return nil, err
	}
	items, _, _, err := s.toFeedItems(ctx, accountID, videos, limit, time.Time{})
	if err != nil {
		return nil, err
	}
	resp := &ListByPopularityResponse{
		VideoList: items,
		AsOf:      timeBefore.Unix(),
		HasMore:   len(videos) == limit,
	}
	if resp.HasMore {
		// 把当前页最后一条的 (popularity, create_time, id) 作为下一页的游标。
		resp.NextOffset = req.Offset + limit
		if len(videos) > 0 {
			last := videos[len(videos)-1]
			lp := last.Popularity
			lb := last.CreateTime
			lid := last.ID
			resp.NextLatestPopularity = &lp
			resp.NextLatestBefore = &lb
			resp.NextLatestIDBefore = &lid
		}
	}
	return resp, nil
}

// ListByTag 按标签查视频（最多 limit 条，按时间倒序）。
func (s *FeedService) ListByTag(ctx context.Context, accountID uint, tag string, limit int) (*ListByTagResponse, error) {
	if tag == "" {
		return nil, errors.New("tag is required")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	videos, err := s.repo.ListByTag(ctx, tag, limit)
	if err != nil {
		return nil, err
	}
	items, _, _, err := s.toFeedItems(ctx, accountID, videos, limit, time.Time{})
	if err != nil {
		return nil, err
	}
	return &ListByTagResponse{VideoList: items}, nil
}

// toFeedItems 把 Video 列表转成前端用的 FeedVideoItem 列表。
// 同时按需填充 is_liked（如果传了 accountID）和 author 信息。
// 这块是 feed 性能的关键——用批量查询 + 内存 map 拼接，避免 N+1。
func (s *FeedService) toFeedItems(ctx context.Context, accountID uint, videos []*video.Video, limit int, latestBefore time.Time) ([]FeedVideoItem, int64, bool, error) {
	// 1) 基础字段拷贝。
	items := make([]FeedVideoItem, 0, len(videos))
	ids := make([]uint, 0, len(videos))
	authorIDs := make(map[uint]struct{}, len(videos))
	for _, v := range videos {
		items = append(items, FeedVideoItem{
			ID:          v.ID,
			Title:       v.Title,
			Description: v.Description,
			PlayURL:     v.PlayURL,
			CoverURL:    v.CoverURL,
			// 序列化为毫秒时间戳，JS 端 new Date(...) 直接可用。
			CreateTime: v.CreateTime.UnixMilli(),
			LikesCount: v.LikesCount,
		})
		ids = append(ids, v.ID)
		authorIDs[v.AuthorID] = struct{}{}
	}

	// 2) 批量取当前用户对这些视频的点赞状态。
	if accountID != 0 && len(ids) > 0 {
		likedSet, err := s.likeRepo.BatchGetLiked(ctx, ids, accountID)
		if err == nil {
			for i := range items {
				items[i].IsLiked = likedSet[items[i].ID]
			}
		}
		// 取点赞状态失败不致命，继续返回（is_liked 全为 false）。
	}

	// 3) 批量取作者信息（用 map 去重，避免一条视频查一次 author）。
	authorCache := make(map[uint]*account.Account, len(authorIDs))
	for aid := range authorIDs {
		acc, err := s.accountRepo.FindByID(ctx, aid)
		if err == nil && acc != nil {
			authorCache[aid] = acc
		}
	}
	for i, v := range videos {
		if acc, ok := authorCache[v.AuthorID]; ok && acc != nil {
			items[i].Author = FeedAuthor{
				ID:        acc.ID,
				Username:  acc.Username,
				AvatarURL: acc.AvatarURL,
			}
		}
	}

	// 4) 计算下一页游标。
	var nextTime int64
	hasMore := len(videos) == limit
	if hasMore && len(videos) > 0 && !latestBefore.IsZero() {
		nextTime = videos[len(videos)-1].CreateTime.UnixMilli()
	}
	return items, nextTime, hasMore, nil
}
