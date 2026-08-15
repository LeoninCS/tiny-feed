package video

// 视频 HTTP 处理器。

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"tiny-feed/internal/apierror"
	jwtmw "tiny-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// VideoHandler 把 HTTP 请求转给 VideoService。
type VideoHandler struct {
	service *VideoService
}

// NewVideoHandler 构造视频处理器。
func NewVideoHandler(service *VideoService) *VideoHandler {
	return &VideoHandler{service: service}
}

// Publish 处理 POST /video/publish（受 JWT 保护）。
// 从 JWT 取出 accountID 和 username，避免前端伪造。
func (h *VideoHandler) Publish(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username, err := jwtmw.GetUsername(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req PublishVideoRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	v, err := h.service.Publish(c.Request.Context(), accountID, username, &req)
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, v)
}

// ListByAuthorID 处理 POST /video/listByAuthorID（公开）。
// 请求体：{author_id}。
func (h *VideoHandler) ListByAuthorID(c *gin.Context) {
	var req ListByAuthorIDRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	videos, err := h.service.ListByAuthorID(c.Request.Context(), int64(req.AuthorID))
	if err != nil {
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"videos": videos})
}

// GetDetail 处理 POST /video/getDetail（公开）。
// 请求体：{id}。
func (h *VideoHandler) GetDetail(c *gin.Context) {
	var req GetDetailRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	v, err := h.service.GetDetail(c.Request.Context(), req.ID)
	if err != nil {
		// ErrVideoNotFound 已被 apierror.ClassifyHTTPStatus 识别为 404，
		// 这里只需要兜底 map。
		if errors.Is(err, ErrVideoNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, v)
}

// Delete 处理 POST /video/delete（受 JWT 保护）。
// 鉴权失败返回 403，视频不存在返回 404，其他错误由 apierror 兜底映射。
func (h *VideoHandler) Delete(c *gin.Context) {
	accountID, err := jwtmw.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req DeleteVideoRequest
	if !apierror.BindJSON(c, &req) {
		return
	}
	if err := h.service.Delete(c.Request.Context(), accountID, req.ID); err != nil {
		// 业务级 sentinel 单独处理：404 / 403
		// 其它错误交给 apierror 通用映射。
		switch {
		case errors.Is(err, ErrVideoNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrVideoForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// uploadsDir 是上传文件落盘的根目录，由 /static 反代对外提供。
const uploadsDir = "./.run/uploads"

// allowedFileKind 决定保存目录与返回 URL 的前缀。
type allowedFileKind int

const (
	kindVideo allowedFileKind = iota
	kindCover
)

func (k allowedFileKind) subdir() string {
	if k == kindCover {
		return "covers"
	}
	return "videos"
}

func (k allowedFileKind) maxSize() int64 {
	if k == kindCover {
		return 10 << 20 // 10 MB
	}
	return 300 << 20 // 300 MB
}

func (k allowedFileKind) acceptPrefix() []string {
	if k == kindCover {
		return []string{"image/jpeg", "image/png", "image/webp"}
	}
	return []string{"video/mp4", "video/"}
}

// saveUploadedFile 把 multipart 文件保存到 uploadsDir/<subdir>/<filename>。
// 返回值是前端可直接拼到 /static/... 的 URL 路径。
func saveUploadedFile(c *gin.Context, kind allowedFileKind) (string, error) {
	header, err := c.FormFile("file")
	if err != nil {
		return "", fmt.Errorf("missing form file: %w", err)
	}
	if header.Size > kind.maxSize() {
		return "", fmt.Errorf("file too large (max %d bytes)", kind.maxSize())
	}
	// 按扩展名 / content-type 双重过滤，避免上传任意类型。
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !hasAcceptedExt(ext, kind) {
		return "", fmt.Errorf("unsupported file extension %q", ext)
	}
	if len(header.Header.Get("Content-Type")) > 0 {
		ct := header.Header.Get("Content-Type")
		if !hasAcceptedContentType(ct, kind) {
			return "", fmt.Errorf("unsupported content type %q", ct)
		}
	}

	dir := filepath.Join(uploadsDir, kind.subdir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	dst := filepath.Join(dir, filepath.Base(header.Filename))
	// 防目录穿越：filepath.Base 之后文件只能落在 dir 内。
	if !strings.HasPrefix(filepath.Clean(dst), filepath.Clean(dir)+string(os.PathSeparator)) &&
		filepath.Clean(dst) != filepath.Clean(dir) {
		return "", fmt.Errorf("invalid filename")
	}

	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded file: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("save uploaded file: %w", err)
	}
	// 返回 URL 路径，前端用 `${API_BASE}/static/<subdir>/<filename>` 访问。
	return fmt.Sprintf("/static/%s/%s", kind.subdir(), filepath.Base(header.Filename)), nil
}

func hasAcceptedExt(ext string, kind allowedFileKind) bool {
	switch kind {
	case kindCover:
		return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp"
	default:
		return ext == ".mp4" || ext == ".mov" || ext == ".webm" || ext == ".mkv"
	}
}

func hasAcceptedContentType(ct string, kind allowedFileKind) bool {
	for _, p := range kind.acceptPrefix() {
		if strings.HasPrefix(ct, p) {
			return true
		}
	}
	return false
}

// UploadVideo 处理 POST /video/uploadVideo（受 JWT 保护）。
// 接收 multipart file 字段，存到 .run/uploads/videos/，返回 URL。
func (h *VideoHandler) UploadVideo(c *gin.Context) {
	url, err := saveUploadedFile(c, kindVideo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// UploadCover 处理 POST /video/uploadCover（受 JWT 保护）。
// 接收 multipart file 字段，存到 .run/uploads/covers/，返回 URL。
func (h *VideoHandler) UploadCover(c *gin.Context) {
	url, err := saveUploadedFile(c, kindCover)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
