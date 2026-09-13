package admin

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
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
		if _, err := provider.Get().AdminService.GetSession(ctx); err != nil {
			status := hertz.StatusInternalServerError
			switch {
			case errors.Is(err, service.ErrAdminUnauthorized):
				status = hertz.StatusUnauthorized
			case errors.Is(err, service.ErrAdminForbidden):
				status = hertz.StatusForbidden
			}
			fail(c, status, err.Error())
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func GetSession(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.GetSession(ctx)
	write(c, resp, err)
}

func ListUsers(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListUsers(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 20),
		service.AdminUserQuery{
			Keyword:   c.Query("keyword"),
			ChapterID: c.Query("chapterId"),
			// role 为旧参数名，保留兼容；memberRole 为新参数名。
			MemberRole:         firstNonEmpty(c.Query("memberRole"), c.Query("role")),
			VerificationMethod: c.Query("verificationMethod"),
			GraduationYear:     queryInt(c, "graduationYear", 0),
			Status:             c.Query("status"),
		},
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

func ListActivities(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListActivities(ctx, queryInt(c, "page", 1), queryInt(c, "pageSize", 20), c.Query("keyword"), c.Query("status"), c.Query("chapterId"))
	write(c, resp, err)
}

func GetActivity(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.GetActivity(ctx, c.Param("id"))
	write(c, resp, err)
}
func CreateActivity(ctx context.Context, c *app.RequestContext) {
	var req service.AdminActivityInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.CreateActivity(ctx, req)
	write(c, resp, err)
}
func UpdateActivity(ctx context.Context, c *app.RequestContext) {
	var req service.AdminActivityInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.UpdateActivity(ctx, c.Param("id"), req)
	write(c, resp, err)
}
func DeleteActivity(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.DeleteActivity(ctx, c.Param("id")))
}
func RestoreActivity(ctx context.Context, c *app.RequestContext) {
	write(c, nil, provider.Get().AdminService.RestoreActivity(ctx, c.Param("id")))
}

func ListChapters(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListChapters(ctx)
	write(c, resp, err)
}
func UpdateChapterContact(ctx context.Context, c *app.RequestContext) {
	var req chapter.Contact
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.UpdateChapterContact(ctx, c.Param("id"), req)
	write(c, resp, err)
}

func AssignAdmin(ctx context.Context, c *app.RequestContext) {
	var req service.AdminAssignmentInput
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, hertz.StatusBadRequest, err.Error())
		return
	}
	resp, err := provider.Get().AdminService.AssignAdmin(ctx, req)
	write(c, resp, err)
}

func ListAdmins(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListAdmins(ctx)
	write(c, resp, err)
}

func ListRoster(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListRoster(ctx, queryInt(c, "page", 1), queryInt(c, "pageSize", 20))
	write(c, resp, err)
}
func ImportRoster(ctx context.Context, c *app.RequestContext) {
	header, err := c.FormFile("file")
	if err != nil {
		fail(c, hertz.StatusBadRequest, "请选择 CSV 或 XLSX 文件")
		return
	}
	file, err := header.Open()
	if err != nil {
		fail(c, hertz.StatusBadRequest, "无法读取文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, consts.MaxRosterImportBytes))
	if err != nil {
		fail(c, hertz.StatusBadRequest, "无法读取文件")
		return
	}
	resp, err := provider.Get().AdminService.ImportRoster(ctx, header.Filename, data)
	write(c, resp, err)
}

func ListRegistrations(ctx context.Context, c *app.RequestContext) {
	resp, err := provider.Get().AdminService.ListRegistrations(
		ctx,
		queryInt(c, "page", 1),
		queryInt(c, "pageSize", 20),
		service.AdminRegistrationQuery{
			ActivityID: c.Query("activityId"),
			ChapterID:  c.Query("chapterId"),
			Keyword:    c.Query("keyword"),
			CheckIn:    c.Query("checkIn"),
		},
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
		c.Query("chapterId"),
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
	case errors.Is(err, service.ErrAdminForbidden):
		fail(c, hertz.StatusForbidden, err.Error())
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

// firstNonEmpty 返回第一个非空字符串，用于同时兼容新旧查询参数。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func queryInt(c *app.RequestContext, key string, fallback int64) int64 {
	value, err := strconv.ParseInt(c.Query(key), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
