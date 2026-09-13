package main

import (
	"sort"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// 本文件是接口层测试：确保契约里承诺的每一条路由都真实注册。
// 它不依赖 MongoDB、Redis 或短信中台，因此可以在 CI 中稳定运行；
// 处理逻辑本身由 biz/application/service 下的单元测试覆盖。

// expectedRoutes 是 docs/superpowers/specs/2026-06-15-chapter-refactor-api-contract.md
// 中列出的全部路由，以及必须继续保留的旧接口。
var expectedRoutes = []string{
	// 公共与用户端
	"GET /ping",
	"GET /chapters",
	"GET /articles",
	"GET /articles/:id",
	"POST /user/register",
	"GET /user/profile",
	"PATCH /user/profile",
	"PUT /user/educations",
	"PUT /user/employments",
	"GET /activities",
	"GET /activities/:id",

	// 图片上传与静态托管
	"POST /upload",
	"GET /files/*filepath",

	// 旧接口：必须保持兼容
	"POST /user/sign_up",
	"POST /user/sign_in",
	"GET /user/info",
	"POST /user/update_info",
	"POST /user/update_edu",
	"POST /user/update_employment",
	"POST /user/exchange_wx_phone",
	"POST /activity/create",
	"POST /activity/update",
	"POST /activity/get",
	"POST /activity/get_many",
	"POST /activity/register",
	"POST /activity/check_in",
	"POST /activity/get_register",
	"POST /sts/apply",
	"POST /sts/send_verify_code",

	// 管理端会话
	"GET /admin/session",

	// 管理端人员与认证
	"GET /admin/users",
	"GET /admin/users/:id",
	"PATCH /admin/users/:id",
	"PATCH /admin/users/:id/role",
	"PATCH /admin/users/:id/status",
	"DELETE /admin/users/:id",
	"POST /admin/users/:id/restore",

	// 管理端管理员配置
	"GET /admin/admins",
	"POST /admin/admins/assign",

	// 管理端分会
	"GET /admin/chapters",
	"PATCH /admin/chapters/:id/contact",

	// 管理端校友名册
	"GET /admin/roster",
	"POST /admin/roster/import",

	// 管理端活动（统一迁移到 /admin/activities/*）
	"GET /admin/activities",
	"POST /admin/activities",
	"GET /admin/activities/:id",
	"PATCH /admin/activities/:id",
	"DELETE /admin/activities/:id",
	"POST /admin/activities/:id/restore",

	// 管理端报名
	"GET /admin/registrations",
	"POST /admin/registrations",
	"PATCH /admin/registrations/:id",
	"DELETE /admin/registrations/:id",
	"POST /admin/registrations/:id/check-in",
	"POST /admin/registrations/:id/cancel-check-in",

	// 管理端资讯
	"GET /admin/articles",
	"POST /admin/articles",
	"GET /admin/articles/:id",
	"PATCH /admin/articles/:id",
	"DELETE /admin/articles/:id",
	"POST /admin/articles/:id/restore",
	"POST /admin/articles/:id/publish",
	"POST /admin/articles/:id/offline",
}

func TestContractRoutesAreRegistered(t *testing.T) {
	h := server.New()
	register(h)

	registered := make(map[string]bool)
	for _, route := range h.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	missing := make([]string, 0)
	for _, expected := range expectedRoutes {
		if !registered[expected] {
			missing = append(missing, expected)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("以下契约路由未注册:\n  %v", missing)
	}
}

// 管理端不得再新增直接调用旧 /activity/create、/activity/update 的入口；
// 这两个旧接口只为兼容老客户端保留。
func TestAdminActivityWritesGoThroughAdminRoutes(t *testing.T) {
	h := server.New()
	register(h)

	registered := make(map[string]bool)
	for _, route := range h.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, required := range []string{
		"POST /admin/activities",
		"PATCH /admin/activities/:id",
		"DELETE /admin/activities/:id",
	} {
		if !registered[required] {
			t.Fatalf("管理端活动写入必须走 %s", required)
		}
	}
	// 旧接口仍然存在，仅作为兼容层。
	for _, legacy := range []string{"POST /activity/create", "POST /activity/update"} {
		if !registered[legacy] {
			t.Fatalf("兼容用的旧接口 %s 不应被删除", legacy)
		}
	}
}
