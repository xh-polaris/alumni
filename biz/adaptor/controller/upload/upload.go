package upload

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/provider"
)

// 图片上传与静态托管。
//
// 返回体与其它公共接口一致（裸数据，不套 code/message/data），
// 因此前端不需要走 unwrap，直接用返回的 url 写入业务字段即可。

type errorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// Save 处理 POST /upload（multipart/form-data，字段名 file，可选 scope）。
func Save(ctx context.Context, c *app.RequestContext) {
	// 上传同样需要登录态：既能拿到用户身份，也避免匿名写入磁盘。
	if adaptor.ExtractUserMeta(ctx).GetUserId() == "" {
		fail(c, hertz.StatusUnauthorized, "请先登录")
		return
	}

	uploadService := provider.Get().UploadService
	header, err := c.FormFile("file")
	if err != nil {
		fail(c, hertz.StatusBadRequest, "请选择要上传的图片")
		return
	}
	data, err := readLimited(header, uploadService.MaxBytes())
	if err != nil {
		if errors.Is(err, errTooLarge) {
			fail(c, hertz.StatusBadRequest, "图片过大，请压缩后重试")
			return
		}
		fail(c, hertz.StatusBadRequest, "无法读取上传文件")
		return
	}

	result, err := uploadService.Save(ctx, c.PostForm("scope"), data)
	if err != nil {
		write(c, nil, err)
		return
	}
	c.JSON(hertz.StatusOK, result)
}

// Serve 处理 GET /files/*filepath，对外托管已上传的图片。
func Serve(ctx context.Context, c *app.RequestContext) {
	key := c.Param("filepath")
	target, ok := provider.Get().UploadService.Resolve(key)
	if !ok {
		fail(c, hertz.StatusNotFound, "资源不存在")
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		fail(c, hertz.StatusNotFound, "资源不存在")
		return
	}
	// 上传内容不可变（文件名带 uuid），可以放心长缓存。
	c.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	c.File(target)
}

var errTooLarge = errors.New("upload: file too large")

// readLimited 读取上传内容，超出上限立即报错，避免把大文件整个读进内存。
func readLimited(header *multipart.FileHeader, maxBytes int64) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// 多读 1 字节用于判断是否超限
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errTooLarge
	}
	return data, nil
}

func write(c *app.RequestContext, data any, err error) {
	if err == nil {
		c.JSON(hertz.StatusOK, data)
		return
	}
	switch {
	case errors.Is(err, service.ErrAdminBadRequest):
		fail(c, hertz.StatusBadRequest, err.Error())
	case errors.Is(err, consts.ErrNotAuthentication):
		fail(c, hertz.StatusUnauthorized, "请先登录")
	default:
		fail(c, hertz.StatusInternalServerError, "服务异常")
	}
}

func fail(c *app.RequestContext, status int, message string) {
	c.JSON(status, errorResponse{Code: int64(status * 100), Message: strings.TrimSpace(message)})
}
