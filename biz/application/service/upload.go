package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
)

// 图片上传的本地存储实现。
//
// 线上推荐客户端直传对象存储：`POST /sts/apply` 会返回带签名的 PUT URL，
// 客户端直接上传后把最终 URL 存进业务字段。本地联调没有中台 STS 服务，
// 因此这里提供「存本机磁盘 + /files 静态托管」的兜底，接口形状保持一致：
// 调用方拿到 url 之后照常写入 avatar / cover / qrCodeUrl 等字段。

const (
	defaultUploadDir      = "./output/upload"
	defaultUploadMaxBytes = 5 << 20 // 5MB
	uploadURLPrefix       = "/files/"
)

// 只接受图片，且扩展名由**嗅探出的内容类型**决定，不信任客户端文件名，
// 这样既避免路径穿越，也避免 .jpg 里塞别的东西。
var allowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type UploadService struct {
	Config *config.Config
}

var UploadServiceSet = wire.NewSet(wire.Struct(new(UploadService), "*"))

// UploadResult 是上传成功后的结果，url 可直接用于业务字段。
type UploadResult struct {
	URL         string `json:"url"`
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

func (s *UploadService) uploadConfig() (dir, baseURL string, maxBytes int64) {
	dir = defaultUploadDir
	maxBytes = defaultUploadMaxBytes
	if s.Config != nil {
		if value := strings.TrimSpace(s.Config.Upload.Dir); value != "" {
			dir = value
		}
		baseURL = strings.TrimRight(strings.TrimSpace(s.Config.Upload.PublicBaseURL), "/")
		if s.Config.Upload.MaxBytes > 0 {
			maxBytes = s.Config.Upload.MaxBytes
		}
	}
	return dir, baseURL, maxBytes
}

// MaxBytes 暴露上限，供控制层在读取请求体时限制大小。
func (s *UploadService) MaxBytes() int64 {
	_, _, maxBytes := s.uploadConfig()
	return maxBytes
}

// Save 校验并落盘一个图片文件。
//
// scope 只用于给存储路径加一层可读前缀（如 avatar / activity），
// 会被严格清洗，不会影响最终目录层级。
func (s *UploadService) Save(ctx context.Context, scope string, data []byte) (*UploadResult, error) {
	dir, baseURL, maxBytes := s.uploadConfig()

	if len(data) == 0 {
		return nil, badRequest("请选择要上传的图片")
	}
	if int64(len(data)) > maxBytes {
		return nil, badRequest(fmt.Sprintf("图片不能超过 %d MB", maxBytes>>20))
	}

	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	contentType := http.DetectContentType(sniff)
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return nil, badRequest("只支持 PNG / JPG / WEBP / GIF 图片")
	}

	key := path.Join(time.Now().Format("200601"), sanitizeScope(scope)+uuid.New().String()+ext)
	target := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return nil, err
	}

	return &UploadResult{
		URL:         baseURL + uploadURLPrefix + key,
		Key:         key,
		Size:        int64(len(data)),
		ContentType: contentType,
	}, nil
}

// Resolve 把 /files 下的相对 key 解析为磁盘绝对路径。
//
// 显式拒绝含 `..` 的路径（而不是先清洗再放行），并对最终绝对路径再做一次
// 前缀校验，两道防线共同保证不会读到存储根目录之外的文件。
func (s *UploadService) Resolve(key string) (string, bool) {
	dir, _, _ := s.uploadConfig()
	trimmed := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if trimmed == "" {
		return "", false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", false
		}
	}
	cleaned := path.Clean("/" + trimmed)
	if cleaned == "/" || strings.Contains(cleaned, "..") {
		return "", false
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(cleaned)))
	if err != nil {
		return "", false
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", false
	}
	return target, true
}

// sanitizeScope 只保留字母数字与短横线，并限制长度，避免用户输入进入路径。
func sanitizeScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	var builder strings.Builder
	for _, r := range scope {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
		if builder.Len() >= 24 {
			break
		}
	}
	if builder.Len() == 0 {
		return ""
	}
	return builder.String() + "/"
}
