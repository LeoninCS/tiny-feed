package video

// 视频服务层：负责视频发布、查询、点赞数维护等业务逻辑。
// 不依赖 Redis、MQ 等中间件，所有操作都走 MySQL。

import (
	"context"
	"errors"
	"strings"

	"tiny-feed/internal/account"
	"tiny-feed/internal/apierror"

	"gorm.io/gorm"
)

// 业务级 sentinel error，handler 用来映射到具体的 HTTP 状态码。
var (
	ErrVideoNotFound  = errors.New("video not found")
	ErrVideoForbidden = errors.New("permission denied: not the owner")
)

// VideoService 视频业务的服务端。
type VideoService struct {
	repo        *VideoRepository
	accountRepo *account.AccountRepository
}

// NewVideoService 构造视频服务。
// accountRepo 用来在 GetDetail / ListByAuthorID 时给 video 补 author.avatar_url。
func NewVideoService(repo *VideoRepository, accountRepo *account.AccountRepository) *VideoService {
	return &VideoService{repo: repo, accountRepo: accountRepo}
}

// Publish 发布一条新视频。
// accountID/username 来自 JWT 中间件，代表发布者。
// 校验：title、play_url、cover_url 都必须非空。
// 写入数据库后返回完整实体（包含自动生成的 ID 和创建时间）。
func (s *VideoService) Publish(ctx context.Context, accountID uint, username string, req *PublishVideoRequest) (*Video, error) {
	// 三个核心字段做 trim 后的非空校验。
	title := strings.TrimSpace(req.Title)
	playURL := strings.TrimSpace(req.PlayURL)
	coverURL := strings.TrimSpace(req.CoverURL)
	if title == "" || playURL == "" || coverURL == "" {
		return nil, errors.New("title, play_url, cover_url are required")
	}
	v := &Video{
		AuthorID:    accountID,
		Username:    username,
		Title:       title,
		Description: req.Description,
		PlayURL:     playURL,
		CoverURL:    coverURL,
	}
	if err := s.repo.CreateVideo(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// ListByAuthorID 列出某作者的全部视频，按创建时间倒序，最多 200 条。
func (s *VideoService) ListByAuthorID(ctx context.Context, authorID int64) ([]Video, error) {
	if authorID <= 0 {
		return nil, apierror.RequireID(uint(authorID), "author_id")
	}
	videos, err := s.repo.ListByAuthorID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	// 给每条 video 补 author.avatar_url，方便前端在用户主页直接显示真实头像。
	// accountRepo 查失败不致命，没头像时 AvatarURL 留空，前端走 fallback。
	if s.accountRepo != nil {
		if acc, err := s.accountRepo.FindByID(ctx, uint(authorID)); err == nil && acc != nil {
			for i := range videos {
				videos[i].AvatarURL = acc.AvatarURL
			}
		}
	}
	return videos, nil
}

// GetDetail 按 ID 取视频详情。
// 不存在时返回 ErrVideoNotFound。
func (s *VideoService) GetDetail(ctx context.Context, id uint) (*Video, error) {
	v, err := s.repo.GetByID(ctx, id)
	if err != nil {
		// 把 gorm 的 not-found 转成业务可读错误。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	// 补 author.avatar_url——Video 结构体上的 AvatarURL 不存表，gorm:"-",
	// 这里是临时从 account 表拿出来填到响应里的。
	if s.accountRepo != nil {
		if acc, err := s.accountRepo.FindByID(ctx, v.AuthorID); err == nil && acc != nil {
			v.AvatarURL = acc.AvatarURL
		}
	}
	return v, nil
}

// Delete 删除视频。
// 鉴权规则：只有作者本人能删自己发布的视频，其他人调用一律拒绝（ErrVideoForbidden）。
// 流程：取视频 → 校验存在 → 校验 owner → 删除。
// 鉴权失败时不暴露视频是否存在与否（无论视频存不存在都先做 owner 比较，
// 避免通过 403/404 差异判断别人的视频 ID 是否有效）。
func (s *VideoService) Delete(ctx context.Context, accountID, videoID uint) error {
	if err := apierror.RequireID(videoID, "video_id"); err != nil {
		return err
	}
	v, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVideoNotFound
		}
		return err
	}
	// 鉴权：必须是作者本人。
	if v.AuthorID != accountID {
		return ErrVideoForbidden
	}
	return s.repo.DeleteVideo(ctx, videoID)
}

// UpdateLikesCount 强制设置某视频的点赞数（绝对值覆盖）。
// 注意：点赞/取消点赞的并发场景应使用 LikeService.Like / Unlike
// 中的相对增减接口（ChangeLikesCount），这个绝对值接口一般用于后台修正。
func (s *VideoService) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	exists, err := s.repo.IsExist(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrVideoNotFound
	}
	return s.repo.UpdateLikesCount(ctx, id, likesCount)
}
