package jwt

// JWT 中间件：以"回调式"设计避免与业务包 import 循环。
// 它不直接依赖 account 包，而是接收一个 TokenChecker 回调，
// 由调用方（router）告诉它"怎么判断 token 是否仍然有效"。
// 这样本包可以被任何业务包安全地 import 进来。

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"tiny-feed/internal/auth"

	"github.com/gin-gonic/gin"
)

// TokenChecker 是 token 有效性的判定函数。
// 入参：ctx + JWT 中解出的 accountID。
// 返回：账号当前生效的 token（一般是数据库里 account.token 字段的最新值）。
//   - 任何错误或空 token 都视为"账号不存在或 token 已失效"；
//   - 调用方会把返回的 token 与请求头里的 token 比较，不一致则拒绝。
type TokenChecker func(ctx context.Context, accountID uint) (storedToken string, err error)

// JWTAuth 强制鉴权中间件。
// 流程：
//  1. 解析 Authorization: Bearer xxx；
//  2. 解析 JWT claims；
//  3. 调 checker 拿到该账号当前 token；
//  4. 如果 checker 出错或两者不一致，拒绝请求；
//  5. 把 accountID 和 username 放进 gin context，供后续 handler 使用。
func JWTAuth(checker TokenChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		// 必须是 "Bearer <token>" 格式。
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		tokenString := parts[1]
		// 解析 JWT（含签名校验、过期校验）。
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		// checker 为 nil 视为配置错误。
		if checker == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "token checker not configured"})
			return
		}
		// 通过回调查出账号最新 token 并比对。
		stored, err := checker(c.Request.Context(), claims.AccountID)
		if err != nil || stored == "" || stored != tokenString {
			// 拿不到 / 不一致都视为已失效（覆盖改密、登出、改名等场景）。
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}
		// 鉴权通过，把 accountID 和 username 放进 context。
		c.Set("accountID", claims.AccountID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// SoftJWTAuth 是 JWTAuth 的"软"版本：没有 Authorization 头时直接放行（caller=匿名）。
// 有了合法 token 时同样校验一致性。适用于 feed 这种"匿名也能看、登录后体验更好"的接口。
func SoftJWTAuth(checker TokenChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		// 没带 token 视为匿名访问，直接放行。
		if authHeader == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		tokenString := parts[1]
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		// checker 缺失时直接放行（视作匿名），避免因为配置问题把请求全部 500。
		if checker == nil {
			c.Next()
			return
		}
		stored, err := checker(c.Request.Context(), claims.AccountID)
		if err != nil || stored == "" || stored != tokenString {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}
		c.Set("accountID", claims.AccountID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetAccountID 从 gin context 取当前请求的 accountID。
// 取不到说明没经过 JWTAuth / SoftJWTAuth 鉴权，调用方应根据场景返回 401。
func GetAccountID(c *gin.Context) (uint, error) {
	uidValue, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}
	accountID, ok := uidValue.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}
	return accountID, nil
}

// GetUsername 从 gin context 取当前请求的 username。
func GetUsername(c *gin.Context) (string, error) {
	val, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found")
	}
	username, ok := val.(string)
	if !ok {
		return "", errors.New("username has invalid type")
	}
	return username, nil
}
