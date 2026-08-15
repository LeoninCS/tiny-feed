package apierror

// apierror 把散落在各个 handler 里的"错误→HTTP 状态码"和"参数绑定"
// 两类重复模式收拢到一处。目标：让 service 用 sentinel error，
// handler 写 c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": ...}) 一行
// 就能拿到正确状态码，不再到处 if errors.Is(...) { c.JSON(404) } ... else { c.JSON(500) }。

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 通用 sentinel error。service 可以用 errors.Is 判断并交给 ClassifyHTTPStatus 映射。
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrValidation   = errors.New("validation error")
)

// ClassifyHTTPStatus 把任意 error 映射到 HTTP 状态码。
// 命中条件：
//   - nil                          → 200
//   - gorm.ErrRecordNotFound        → 404
//   - gorm.ErrDuplicatedKey         → 409（资源冲突，比如 username 撞名）
//   - ErrUnauthorized              → 401
//   - ErrForbidden                 → 403
//   - ErrNotFound / ErrValidation  → 404 / 400
//   - 其它（含业务自定义 sentinel）→ 500
//
// 注意：各业务包自己定义的 sentinel error（video.ErrVideoNotFound 等）
// 不会被这里识别——它们要么继续在 handler 里用 errors.Is 显式处理，
// 要么业务方包装成上面这几个通用 sentinel 之一。
func ClassifyHTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// BindJSON 替代 handler 里重复的 ShouldBindJSON + 400 返回。
// 绑定失败时已经写好响应并返回 false，调用方只需：
//
//	if !apierror.BindJSON(c, &req) { return }
func BindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

// RequireID 校验 ID 类参数非 0，常用于"X is required" 校验。
// id 为 0 时返回带字段名的错误，调用方直接返回即可：
//
//	if err := apierror.RequireID(req.VideoID, "video_id"); err != nil { return err }
func RequireID(id uint, name string) error {
	if id == 0 {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
