package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/provider"
)

type response struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func RequireAuth() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if strings.TrimSpace(string(c.GetHeader("Authorization"))) == "" {
			fail(c, hertz.StatusUnauthorized, "登录已失效")
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func GetSession(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.GetSession(ctx, string(c.GetHeader("Authorization")))
	write(c, resp, err)
}

func ListUsers(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListUsers(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 20),
		c.Query("keyword"),
		c.Query("role"),
		c.Query("status"),
	)
	write(c, resp, err)
}

func GetUser(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.GetUser(ctx, c.Param("id"))
	write(c, resp, err)
}

func UpdateUser(ctx context.Context, c *app.RequestContext) {
	var req service.AdminUserUpdate
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.UpdateUser(ctx, c.Param("id"), req)
	write(c, resp, err)
}

func SetUserRole(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Role string `json:"role"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	write(c, nil, provider.Get().AdminService.SetUserRole(ctx, c.Param("id"), req.Role))
}

func SetUserStatus(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Status int64 `json:"status"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	write(c, nil, provider.Get().AdminService.SetUserStatus(ctx, c.Param("id"), req.Status))
}

func DeleteUser(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.DeleteUser(ctx, c.Param("id")))
}

func RestoreUser(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.RestoreUser(ctx, c.Param("id")))
}

func ListRegistrations(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListRegistrations(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 20),
		c.Query("activityId"),
		c.Query("keyword"),
		c.Query("checkIn"),
	)
	write(c, resp, err)
}

func CreateRegistration(ctx context.Context, c *app.RequestContext) {
	var req service.AdminRegistrationInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.CreateRegistration(ctx, req)
	write(c, resp, err)
}

func UpdateRegistration(ctx context.Context, c *app.RequestContext) {
	var req service.AdminRegistrationInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.UpdateRegistration(ctx, c.Param("id"), req)
	write(c, resp, err)
}

func DeleteRegistration(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.DeleteRegistration(ctx, c.Param("id")))
}

func CheckInRegistration(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.SetRegistrationCheckIn(ctx, c.Param("id"), true))
}

func CancelCheckInRegistration(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.SetRegistrationCheckIn(ctx, c.Param("id"), false))
}

func ListArticles(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListArticles(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 20),
		c.Query("keyword"),
		c.Query("status"),
	)
	write(c, resp, err)
}

func GetArticle(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.GetArticle(ctx, c.Param("id"))
	write(c, resp, err)
}

func CreateArticle(ctx context.Context, c *app.RequestContext) {
	var req service.AdminArticleInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.CreateArticle(ctx, req)
	write(c, resp, err)
}

func UpdateArticle(ctx context.Context, c *app.RequestContext) {
	var req service.AdminArticleInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.UpdateArticle(ctx, c.Param("id"), req)
	write(c, resp, err)
}

func DeleteArticle(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.DeleteArticle(ctx, c.Param("id")))
}

func RestoreArticle(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.RestoreArticle(ctx, c.Param("id")))
}

func PublishArticle(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.SetArticleStatus(ctx, c.Param("id"), "published"))
}

func OfflineArticle(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.SetArticleStatus(ctx, c.Param("id"), "offline"))
}

func write(c *app.RequestContext, data any, err error) {
	if err == nil {
		ok(c, data)
		return
	}
	switch {
	case errors.Is(err, service.ErrAdminUnauthorized):
		fail(c, hertz.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrAdminBadRequest):
		fail(c, hertz.StatusBadRequest, err.Error())
	case err == consts.ErrNotFound || err == consts.ErrInvalidObjectId:
		fail(c, hertz.StatusNotFound, "资源不存在")
	default:
		fail(c, hertz.StatusInternalServerError, "服务异常")
	}
}

func ok(c *app.RequestContext, data any) {
	c.JSON(hertz.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
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
