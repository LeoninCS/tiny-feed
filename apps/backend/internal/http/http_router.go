package http

// 路由层：把所有业务 handler 集中起来，注册到 gin 上。
// 整个项目只有一个 SetRouter 函数，串起所有路由：
//   /healthz                     健康检查
//   /static/*                    静态文件（上传的视频/封面）
//   /account/register,/login...  账号相关
//   /video/publish,...           视频相关
//   /like/like,...               点赞
//   /comment/publish,...         评论
//   /social/follow,...           关注
//   /feed/listLatest,...         feed
// 受 JWT 保护的接口放在 group.Use(jwtmw.JWTAuth(checker)) 之后。

import (
	"context"
	"errors"

	"tiny-feed/internal/account"
	"tiny-feed/internal/feed"
	jwtmw "tiny-feed/internal/middleware/jwt"
	"tiny-feed/internal/profile"
	"tiny-feed/internal/social"
	"tiny-feed/internal/video"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetRouter 是路由总入口。db 是已经连接好的 gorm.DB。
// 内部完成：
//  1. 创建所有 repo；
//  2. 构造一个 token 校验回调（jwt 中间件用）；
//  3. 构造所有 service 和 handler；
//  4. 把路由按 group 注册到 gin engine；
//  5. 划分公开路由组与受 JWT 保护的路由组。
func SetRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	// gin 内置 TrustedProxies 警告关闭（开发环境 IP 来源不固定）。
	if err := r.SetTrustedProxies(nil); err != nil {
		_ = err
	}
	// 健康检查：负载均衡/探针用。
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	// 静态资源目录：前端播放视频/封面图时通过 /static/xxx 访问。
	r.Static("/static", "./.run/uploads")

	// ---------- 1. 构造所有 repo（数据层） ----------
	accountRepo := account.NewAccountRepository(db)
	videoRepo := video.NewVideoRepository(db)
	likeRepo := video.NewLikeRepository(db)
	commentRepo := video.NewCommentRepository(db)
	socialRepo := social.NewSocialRepository(db)
	feedRepo := feed.NewFeedRepository(db)

	// ---------- 2. JWT token 校验回调 ----------
	// jwt 中间件不直接依赖 account 包，而是接收一个回调函数：
	//   根据 accountID 查出账号当前存的 token，如果和请求里的 token 不一致就算失效。
	// 这样既打破了 import cycle，又把"如何判定 token 失效"的策略收敛在一处。
	checker := func(ctx context.Context, accountID uint) (string, error) {
		acc, err := accountRepo.FindByID(ctx, accountID)
		if err != nil {
			return "", err
		}
		if acc == nil {
			return "", errors.New("account not found")
		}
		return acc.Token, nil
	}

	// ---------- 3. account 模块 ----------
	accountService := account.NewAccountService(accountRepo)
	accountHandler := account.NewAccountHandler(accountService)
	// profile 单独成包，用于跨表聚合用户主页信息。
	profileHandler := profile.NewProfileHandler(accountRepo, videoRepo, socialRepo)
	accountGroup := r.Group("/account")
	// 公开：注册/登录/查资料/改密/refresh。
	{
		accountGroup.POST("/register", accountHandler.CreateAccount)
		accountGroup.POST("/login", accountHandler.Login)
		accountGroup.POST("/changePassword", accountHandler.ChangePassword)
		accountGroup.POST("/findByID", accountHandler.FindByID)
		accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		accountGroup.POST("/getProfile", profileHandler.GetProfile)
		accountGroup.POST("/refresh", accountHandler.Refresh)
	}
	// 受保护：登出/改名/更新资料。
	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedAccountGroup.POST("/logout", accountHandler.Logout)
		protectedAccountGroup.POST("/rename", accountHandler.Rename)
		protectedAccountGroup.POST("/updateProfile", accountHandler.UpdateProfile)
	}

	// ---------- 4. video 模块 ----------
	videoService := video.NewVideoService(videoRepo, accountRepo)
	likeService := video.NewLikeService(likeRepo, videoRepo)
	commentService := video.NewCommentService(commentRepo, videoRepo)
	videoHandler := video.NewVideoHandler(videoService)
	likeHandler := video.NewLikeHandler(likeService)
	commentHandler := video.NewCommentHandler(commentService)
	// 公开：列出某作者视频、查视频详情。
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		videoGroup.POST("/getDetail", videoHandler.GetDetail)
	}
	// 受保护：发布视频、删除视频、上传视频和封面。
	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedVideoGroup.POST("/publish", videoHandler.Publish)
		protectedVideoGroup.POST("/delete", videoHandler.Delete)
		protectedVideoGroup.POST("/uploadVideo", videoHandler.UploadVideo)
		protectedVideoGroup.POST("/uploadCover", videoHandler.UploadCover)
	}

	// ---------- 5. like 模块（全部受保护） ----------
	likeGroup := r.Group("/like")
	protectedLikeGroup := likeGroup.Group("")
	protectedLikeGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedLikeGroup.POST("/like", likeHandler.Like)
		protectedLikeGroup.POST("/unlike", likeHandler.Unlike)
		protectedLikeGroup.POST("/isLiked", likeHandler.IsLiked)
		protectedLikeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
	}

	// ---------- 6. comment 模块 ----------
	// 公开：列出评论。
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", commentHandler.GetAll)
	}
	// 受保护：发/删评论。
	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedCommentGroup.POST("/publish", commentHandler.Publish)
		protectedCommentGroup.POST("/delete", commentHandler.Delete)
	}

	// ---------- 7. social 模块 ----------
	socialService := social.NewSocialService(socialRepo, accountRepo)
	socialHandler := social.NewSocialHandler(socialService)
	// 公开：粉丝列表、关注列表。
	socialGroup := r.Group("/social")
	{
		socialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
		socialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)
	}
	// 受保护：关注/取关/查计数。
	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedSocialGroup.POST("/follow", socialHandler.Follow)
		protectedSocialGroup.POST("/unfollow", socialHandler.Unfollow)
		protectedSocialGroup.POST("/getCounts", socialHandler.GetCounts)
	}

	// ---------- 8. feed 模块 ----------
	feedService := feed.NewFeedService(feedRepo, likeRepo, accountRepo)
	feedHandler := feed.NewFeedHandler(feedService)
	// 公开：latest、likes count、tag（可选登录，未登录时 is_liked 全部为 false）。
	feedGroup := r.Group("/feed")
	{
		feedGroup.POST("/listLatest", feedHandler.ListLatest)
		feedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
		feedGroup.POST("/listByTag", feedHandler.ListByTag)
	}
	// 受保护：following、popularity。
	protectedFeedGroup := feedGroup.Group("")
	protectedFeedGroup.Use(jwtmw.JWTAuth(checker))
	{
		protectedFeedGroup.POST("/listByFollowing", feedHandler.ListByFollowing)
		protectedFeedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
	}
	return r
}
