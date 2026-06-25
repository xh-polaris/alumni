package article

import (
	"context"
	"errors"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	appconsts "github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/provider"
)

type response struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func ListArticles(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().ArticleService.ListPublicArticles(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 10),
	)
	write(c, resp, err)
}

func GetArticle(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().ArticleService.GetPublicArticle(ctx, c.Param("id"))
	write(c, resp, err)
}

func write(c *app.RequestContext, data any, err error) {
	if err == nil {
		c.JSON(hertz.StatusOK, data)
		return
	}
	switch {
	case errors.Is(err, service.ErrAdminNotFound), err == appconsts.ErrNotFound, err == appconsts.ErrInvalidObjectId:
		fail(c, hertz.StatusNotFound, "资源不存在")
	default:
		fail(c, hertz.StatusInternalServerError, "服务异常")
	}
}

func fail(c *app.RequestContext, status int, message string) {
	c.JSON(status, response{
		Code:    int64(status * 100),
		Message: message,
		Data:    nil,
	})
}

func queryInt(c *app.RequestContext, key string, fallback int64) int64 {
	value, err := strconv.ParseInt(c.Query(key), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
