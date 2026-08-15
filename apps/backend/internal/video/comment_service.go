package video

// 评论服务层：负责视频评论的发布、删除、列表查询。
// 业务规则：评论内容必须非空；只能删除自己的评论。

import (
	"context"
	"errors"
	"strings"

	"tiny-feed/internal/apierror"

	"gorm.io/gorm"
)

// 评论模块的 sentinel errors，集中放在这里便于 handler 统一映射。
var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrCommentForbidden = errors.New("permission denied: not the comment owner")
)

// CommentService 评论业务的服务端。
type CommentService struct {
	commentRepo *CommentRepository
	videoRepo   *VideoRepository
}

// NewCommentService 构造评论服务。
func NewCommentService(commentRepo *CommentRepository, videoRepo *VideoRepository) *CommentService {
	return &CommentService{commentRepo: commentRepo, videoRepo: videoRepo}
}

// Publish 发布一条评论。
// 校验：video_id 必须存在，content 必须非空。
// 写入评论表时把 author_id 和 username 冗余存储，方便列表渲染时不必再 join account 表。
func (s *CommentService) Publish(ctx context.Context, accountID uint, username string, req *PublishCommentRequest) (*Comment, error) {
	if err := apierror.RequireID(req.VideoID, "video_id"); err != nil {
		return nil, err
	}
	// content 去掉首尾空白后再判断，避免出现"全空格评论"。
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errors.New("content is required")
	}
	// 确认视频存在。
	exists, err := s.videoRepo.IsExist(ctx, req.VideoID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrVideoNotFound
	}
	c := &Comment{
		VideoID:  req.VideoID,
		AuthorID: accountID,
		Username: username,
		Content:  content,
	}
	if err := s.commentRepo.CreateComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete 删除评论。
// 权限校验：只能删除自己发的评论（按 author_id 匹配）。
func (s *CommentService) Delete(ctx context.Context, accountID, commentID uint) error {
	if err := apierror.RequireID(commentID, "comment_id"); err != nil {
		return err
	}
	c, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommentNotFound
		}
		return err
	}
	// 鉴权：只有作者本人能删。
	if c.AuthorID != accountID {
		return ErrCommentForbidden
	}
	return s.commentRepo.DeleteComment(ctx, c)
}

// GetAll 列出某视频下的全部评论（按时间正序）。
func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	if err := apierror.RequireID(videoID, "video_id"); err != nil {
		return nil, err
	}
	return s.commentRepo.GetAllComments(ctx, videoID)
}
