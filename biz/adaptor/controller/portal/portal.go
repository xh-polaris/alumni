package portal

import (
	"context"
	"errors"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/provider"
)

type errorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

func Register(ctx context.Context, c *app.RequestContext) {
	var input service.RegisterProfileInput
	if err := c.BindAndValidate(&input); err != nil {
		fail(c, hertz.StatusBadRequest, "请完整填写注册信息")
		return
	}
	result, err := provider.Get().UserService.RegisterProfile(ctx, input)
	write(c, result, err)
}

func ListChapters(ctx context.Context, c *app.RequestContext) {
	items, err := provider.Get().ChapterMapper.List(ctx, false)
	write(c, items, err)
}

func GetProfile(ctx context.Context, c *app.RequestContext) {
	result, err := provider.Get().UserService.GetProfile(ctx)
	write(c, result, err)
}

func UpdateProfile(ctx context.Context, c *app.RequestContext) {
	var input service.ProfileUpdate
	if err := c.BindAndValidate(&input); err != nil {
		fail(c, hertz.StatusBadRequest, "资料格式不正确")
		return
	}
	result, err := provider.Get().UserService.UpdateProfile(ctx, input)
	write(c, result, err)
}

func ReplaceEducations(ctx context.Context, c *app.RequestContext) {
	var input struct {
		Educations []user.Education `json:"educations"`
	}
	if err := c.BindAndValidate(&input); err != nil {
		fail(c, hertz.StatusBadRequest, "教育经历格式不正确")
		return
	}
	result, err := provider.Get().UserService.ReplaceEducations(ctx, input.Educations)
	write(c, result, err)
}

func ReplaceEmployments(ctx context.Context, c *app.RequestContext) {
	var input struct {
		Employments []user.Employment `json:"employments"`
	}
	if err := c.BindAndValidate(&input); err != nil {
		fail(c, hertz.StatusBadRequest, "工作经历格式不正确")
		return
	}
	result, err := provider.Get().UserService.ReplaceEmployments(ctx, input.Employments)
	write(c, result, err)
}

func ListActivities(ctx context.Context, c *app.RequestContext) {
	result, err := provider.Get().ActivityService.ListPublic(ctx, c.Query("chapterId"), queryInt(c, "page", 1), queryInt(c, "pageSize", 10))
	write(c, result, err)
}

func GetActivity(ctx context.Context, c *app.RequestContext) {
	result, err := provider.Get().ActivityService.GetPublic(ctx, c.Param("id"))
	write(c, result, err)
}

func write(c *app.RequestContext, data any, err error) {
	if err == nil {
		c.JSON(hertz.StatusOK, data)
		return
	}
	switch {
	case errors.Is(err, service.ErrAdminBadRequest):
		fail(c, hertz.StatusBadRequest, err.Error())
	case err == consts.ErrNotAuthentication:
		fail(c, hertz.StatusUnauthorized, "请先登录")
	case err == consts.ErrForbidden, errors.Is(err, service.ErrAdminForbidden):
		fail(c, hertz.StatusForbidden, "当前身份无权执行此操作")
	case err == consts.ErrNotFound || err == consts.ErrInvalidObjectId:
		fail(c, hertz.StatusNotFound, "资源不存在")
	default:
		if errno, ok := err.(*consts.Errno); ok {
			c.JSON(hertz.StatusOK, map[string]any{"code": errno.GRPCStatus().Code(), "msg": errno.Error()})
			return
		}
		fail(c, hertz.StatusInternalServerError, "服务异常")
	}
}

func fail(c *app.RequestContext, status int, message string) {
	c.JSON(status, errorResponse{Code: int64(status * 100), Message: message})
}
func queryInt(c *app.RequestContext, key string, fallback int64) int64 {
	value, err := strconv.ParseInt(c.Query(key), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
