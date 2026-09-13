package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	activitymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 本文件覆盖方案里最关键的越权场景：分会管理员在读取、修改、
// 以及伪造 ID 时都不能越过自己的分会。

func chapterAdminFixture(t *testing.T) (context.Context, *AdminService, *user.User, *chapter.Chapter, *chapter.Chapter, *user.User) {
	t.Helper()
	chapters := chapterFixture(chapter.CodeShanghai, chapter.CodeNingbo)
	shanghai, ningbo := chapters[0], chapters[1]
	admin := &user.User{
		ID:             primitive.NewObjectID(),
		Name:           "上海管理员",
		MemberRole:     user.MemberAlumni,
		AdminRole:      user.AdminChapter,
		AdminChapterID: shanghai.ID.Hex(),
		ChapterID:      shanghai.ID.Hex(),
		Status:         0,
	}
	other := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "宁波校友",
		MemberRole: user.MemberAlumni,
		ChapterID:  ningbo.ID.Hex(),
		Status:     0,
	}
	service, _, _ := newTestAdminService([]*user.User{admin, other}, chapters, []*activitymodel.Activity{
		{ID: primitive.NewObjectID(), Name: "上海活动", ChapterID: shanghai.ID.Hex(), Status: 0},
		{ID: primitive.NewObjectID(), Name: "宁波活动", ChapterID: ningbo.ID.Hex(), Status: 0},
	})
	return adaptor.WithUserID(context.Background(), admin.ID.Hex()), service, admin, shanghai, ningbo, other
}

func TestChapterAdminCannotCrossChapter(t *testing.T) {
	ctx, service, admin, shanghai, ningbo, other := chapterAdminFixture(t)

	// 列表：即使前端伪造 chapterId，服务端也必须用管理员的实际分会。
	result, err := service.ListUsers(ctx, 1, 20, AdminUserQuery{ChapterID: ningbo.ID.Hex()})
	if err != nil {
		t.Fatalf("ListUsers 返回错误: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != admin.ID.Hex() {
		t.Fatalf("分会管理员应只看到本分会人员，实际得到 %d 条", len(result.Items))
	}

	if _, err = service.GetUser(ctx, other.ID.Hex()); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("读取其他分会用户应被拒绝，实际 %v", err)
	}
	if _, err = service.UpdateUser(ctx, other.ID.Hex(), AdminUserUpdate{}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("修改其他分会用户应被拒绝，实际 %v", err)
	}
	if err = service.SetUserRole(ctx, other.ID.Hex(), user.MemberGuest); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("认证其他分会用户应被拒绝，实际 %v", err)
	}
	if err = service.DeleteUser(ctx, other.ID.Hex()); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("删除其他分会用户应被拒绝，实际 %v", err)
	}

	// 伪造其他分会的活动 ID。
	activities := service.ActivityMapper.(*fakeActivityMapper).items
	if _, err = service.GetActivity(ctx, activities[1].ID.Hex()); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("读取其他分会活动应被拒绝，实际 %v", err)
	}
	if _, err = service.UpdateActivity(ctx, activities[1].ID.Hex(), AdminActivityInput{}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("修改其他分会活动应被拒绝，实际 %v", err)
	}
	if err = service.DeleteActivity(ctx, activities[1].ID.Hex()); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("删除其他分会活动应被拒绝，实际 %v", err)
	}

	// 伪造其他分会的活动 ID 读取报名数据。
	if _, err = service.ListRegistrations(ctx, 1, 20, AdminRegistrationQuery{ActivityID: activities[1].ID.Hex()}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("按其他分会活动读取报名应被拒绝，实际 %v", err)
	}

	// 分会联络信息同样受限。
	if _, err = service.UpdateChapterContact(ctx, ningbo.ID.Hex(), chapter.Contact{Name: "越权"}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("修改其他分会联络人应被拒绝，实际 %v", err)
	}
	// 本分会可以修改。
	if _, err = service.UpdateChapterContact(ctx, shanghai.ID.Hex(), chapter.Contact{Name: "李老师"}); err != nil {
		t.Fatalf("修改本分会联络人失败: %v", err)
	}

	// 只返回本分会。
	chapters, err := service.ListChapters(ctx)
	if err != nil {
		t.Fatalf("ListChapters 返回错误: %v", err)
	}
	if len(chapters) != 1 || chapters[0].ID.Hex() != shanghai.ID.Hex() {
		t.Fatalf("分会管理员应只看到本分会，实际 %d 个", len(chapters))
	}
}

func TestChapterAdminCannotSeeSuperAdminCapabilities(t *testing.T) {
	ctx, service, admin, shanghai, _, _ := chapterAdminFixture(t)

	if _, err := service.ListAdmins(ctx); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("分会管理员不应看到管理员配置，实际 %v", err)
	}
	if _, err := service.ListRoster(ctx, 1, 20); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("分会管理员不应看到校友名册，实际 %v", err)
	}
	if _, err := service.ImportRoster(ctx, "roster.csv", []byte("姓名,毕业年份,出生日期\n张三,2018,2000-05-20\n")); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("分会管理员不应导入名册，实际 %v", err)
	}
	// 分会管理员也不能通过任命接口提权。
	if _, err := service.AssignAdmin(ctx, AdminAssignmentInput{UserID: admin.ID.Hex(), AdminRole: user.AdminChapter, AdminChapterID: shanghai.ID.Hex()}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("分会管理员不应能任命管理员，实际 %v", err)
	}
}

func TestSuperAdminSeesEverythingAndAssignsChapterAdmins(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai, chapter.CodeNingbo)
	super := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "超级管理员",
		MemberRole: user.MemberAlumni,
		AdminRole:  user.AdminSuper,
		Status:     0,
	}
	target := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "宁波校友",
		MemberRole: user.MemberAlumni,
		ChapterID:  chapters[1].ID.Hex(),
		Status:     0,
	}
	service, _, _ := newTestAdminService([]*user.User{super, target}, chapters, nil)
	ctx := adaptor.WithUserID(context.Background(), super.ID.Hex())

	result, err := service.ListUsers(ctx, 1, 20, AdminUserQuery{})
	if err != nil {
		t.Fatalf("ListUsers 返回错误: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("超级管理员应看到全部人员，实际 %d 条", len(result.Items))
	}

	// 超级管理员可以按分会筛选。
	result, err = service.ListUsers(ctx, 1, 20, AdminUserQuery{ChapterID: chapters[1].ID.Hex()})
	if err != nil {
		t.Fatalf("ListUsers 按分会筛选失败: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != target.ID.Hex() {
		t.Fatalf("按分会筛选结果不正确: %+v", result.Items)
	}

	// 任命分会管理员。
	assigned, err := service.AssignAdmin(ctx, AdminAssignmentInput{
		UserID:         target.ID.Hex(),
		AdminRole:      user.AdminChapter,
		AdminChapterID: chapters[1].ID.Hex(),
	})
	if err != nil {
		t.Fatalf("任命分会管理员失败: %v", err)
	}
	if assigned.AdminRole != user.AdminChapter || assigned.AdminChapterID != chapters[1].ID.Hex() {
		t.Fatalf("任命结果不正确: %+v", assigned)
	}

	// 撤销。
	revoked, err := service.AssignAdmin(ctx, AdminAssignmentInput{UserID: target.ID.Hex(), AdminRole: user.AdminNone})
	if err != nil {
		t.Fatalf("撤销分会管理员失败: %v", err)
	}
	if revoked.AdminRole != user.AdminNone || revoked.AdminChapterID != "" {
		t.Fatalf("撤销结果不正确: %+v", revoked)
	}

	// 不能通过接口创建超级管理员。
	if _, err = service.AssignAdmin(ctx, AdminAssignmentInput{UserID: target.ID.Hex(), AdminRole: user.AdminSuper}); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("不应允许任命超级管理员，实际 %v", err)
	}
	// 分会必须存在。
	if _, err = service.AssignAdmin(ctx, AdminAssignmentInput{UserID: target.ID.Hex(), AdminRole: user.AdminChapter, AdminChapterID: primitive.NewObjectID().Hex()}); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("不存在的分会应被拒绝，实际 %v", err)
	}
}

func TestChapterAdminSessionReportsScope(t *testing.T) {
	ctx, service, _, shanghai, _, _ := chapterAdminFixture(t)

	session, err := service.GetSession(ctx)
	if err != nil {
		t.Fatalf("GetSession 返回错误: %v", err)
	}
	if session.AdminRole != user.AdminChapter {
		t.Fatalf("分会管理员 adminRole 应为 %s，实际 %s", user.AdminChapter, session.AdminRole)
	}
	if session.AdminChapterID != shanghai.ID.Hex() || session.AdminChapterName != shanghai.Name {
		t.Fatalf("会话未返回管理分会: %+v", session)
	}
}

func TestNonAdminIsRejected(t *testing.T) {
	member := &user.User{ID: primitive.NewObjectID(), Name: "普通校友", MemberRole: user.MemberAlumni, Status: 0}
	service, _, _ := newTestAdminService([]*user.User{member}, chapterFixture(chapter.CodeShanghai), nil)
	ctx := adaptor.WithUserID(context.Background(), member.ID.Hex())

	if _, err := service.GetSession(ctx); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("非管理员应被拒绝，实际 %v", err)
	}
	if _, err := service.ListUsers(ctx, 1, 20, AdminUserQuery{}); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("非管理员不应读取人员列表，实际 %v", err)
	}
	if _, err := service.GetSession(context.Background()); !errors.Is(err, ErrAdminUnauthorized) {
		t.Fatalf("未登录应返回未认证，实际 %v", err)
	}
}

func TestDisabledAdminIsRejected(t *testing.T) {
	disabled := &user.User{
		ID:             primitive.NewObjectID(),
		AdminRole:      user.AdminChapter,
		AdminChapterID: primitive.NewObjectID().Hex(),
		Status:         1,
	}
	service, _, _ := newTestAdminService([]*user.User{disabled}, chapterFixture(chapter.CodeShanghai), nil)
	if _, err := service.GetSession(adaptor.WithUserID(context.Background(), disabled.ID.Hex())); !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("停用的管理员应被拒绝，实际 %v", err)
	}
}

func TestAdminUserFiltersCoverIdentityAndVerification(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai)
	admin := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	pending := &user.User{ID: primitive.NewObjectID(), Name: "待认证", MemberRole: user.MemberPending, ChapterID: chapters[0].ID.Hex(), GraduationYear: 2018, VerificationMethod: user.VerificationNone, Status: 0}
	manual := &user.User{ID: primitive.NewObjectID(), Name: "人工认证", MemberRole: user.MemberAlumni, ChapterID: chapters[0].ID.Hex(), GraduationYear: 2019, VerificationMethod: user.VerificationManual, Status: 0}
	rosterVerified := &user.User{ID: primitive.NewObjectID(), Name: "名册认证", MemberRole: user.MemberAlumni, ChapterID: chapters[0].ID.Hex(), GraduationYear: 2019, VerificationMethod: user.VerificationRoster, Status: 0}
	service, _, _ := newTestAdminService([]*user.User{admin, pending, manual, rosterVerified}, chapters, nil)
	ctx := adaptor.WithUserID(context.Background(), admin.ID.Hex())

	cases := []struct {
		name  string
		query AdminUserQuery
		want  string
	}{
		{"按身份筛选", AdminUserQuery{MemberRole: user.MemberPending}, pending.ID.Hex()},
		{"按认证方式筛选", AdminUserQuery{VerificationMethod: user.VerificationManual}, manual.ID.Hex()},
		{"按认证方式筛选名册", AdminUserQuery{VerificationMethod: user.VerificationRoster}, rosterVerified.ID.Hex()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := service.ListUsers(ctx, 1, 20, testCase.query)
			if err != nil {
				t.Fatalf("ListUsers 返回错误: %v", err)
			}
			if len(result.Items) != 1 || result.Items[0].ID != testCase.want {
				t.Fatalf("筛选结果不正确: %+v", result.Items)
			}
		})
	}

	// 同时按毕业年份与身份筛选。
	result, err := service.ListUsers(ctx, 1, 20, AdminUserQuery{MemberRole: user.MemberAlumni, GraduationYear: 2019, VerificationMethod: user.VerificationRoster})
	if err != nil {
		t.Fatalf("ListUsers 返回错误: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != rosterVerified.ID.Hex() {
		t.Fatalf("组合筛选结果不正确: %+v", result.Items)
	}

	// 旧角色值兼容。
	result, err = service.ListUsers(ctx, 1, 20, AdminUserQuery{MemberRole: "user"})
	if err != nil {
		t.Fatalf("兼容旧角色值失败: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != pending.ID.Hex() {
		t.Fatalf("旧角色值 user 应映射为待认证: %+v", result.Items)
	}
}

func TestSetUserRoleMarksManualVerification(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai)
	admin := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminChapter, AdminChapterID: chapters[0].ID.Hex(), ChapterID: chapters[0].ID.Hex(), Status: 0}
	target := &user.User{ID: primitive.NewObjectID(), Name: "待认证", MemberRole: user.MemberPending, ChapterID: chapters[0].ID.Hex(), VerificationMethod: user.VerificationNone, Status: 0}
	service, _, _ := newTestAdminService([]*user.User{admin, target}, chapters, nil)
	ctx := adaptor.WithUserID(context.Background(), admin.ID.Hex())

	if err := service.SetUserRole(ctx, target.ID.Hex(), user.MemberAlumni); err != nil {
		t.Fatalf("认证为校友失败: %v", err)
	}
	if target.MemberRole != user.MemberAlumni {
		t.Fatalf("身份未更新: %s", target.MemberRole)
	}
	if target.VerificationMethod != user.VerificationManual {
		t.Fatalf("认证方式应为 manual，实际 %s", target.VerificationMethod)
	}
	if target.VerifiedAt.IsZero() || target.VerifiedBy != admin.ID.Hex() {
		t.Fatalf("认证审计信息缺失: %+v", target)
	}

	if err := service.SetUserRole(ctx, target.ID.Hex(), user.MemberGuest); err != nil {
		t.Fatalf("设为嘉宾失败: %v", err)
	}
	if target.MemberRole != user.MemberGuest || target.VerificationMethod != user.VerificationManual {
		t.Fatalf("设为嘉宾后状态不正确: %+v", target)
	}

	// 非法身份必须被拒绝。
	if err := service.SetUserRole(ctx, target.ID.Hex(), user.AdminSuper); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("非法身份应被拒绝，实际 %v", err)
	}
}

func TestAdminListIgnoresRemovedUsersByDefault(t *testing.T) {
	active := &user.User{ID: primitive.NewObjectID(), Name: "正常", Status: 0}
	removed := &user.User{ID: primitive.NewObjectID(), Name: "已删除", Status: 1, DeleteTime: time.Now()}
	super := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	service, _, _ := newTestAdminService([]*user.User{super, active, removed}, chapterFixture(chapter.CodeShanghai), nil)
	ctx := adaptor.WithUserID(context.Background(), super.ID.Hex())

	result, err := service.ListUsers(ctx, 1, 20, AdminUserQuery{})
	if err != nil {
		t.Fatalf("ListUsers 返回错误: %v", err)
	}
	for _, item := range result.Items {
		if item.ID == removed.ID.Hex() {
			t.Fatalf("默认列表不应包含已删除用户")
		}
	}

	result, err = service.ListUsers(ctx, 1, 20, AdminUserQuery{Status: "deleted"})
	if err != nil {
		t.Fatalf("ListUsers 返回错误: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != removed.ID.Hex() {
		t.Fatalf("已删除筛选结果不正确: %+v", result.Items)
	}
}

func TestUpdateUserPersistsGraduationYearAndChapter(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai, chapter.CodeNingbo)
	admin := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	target := &user.User{ID: primitive.NewObjectID(), Name: "校友", MemberRole: user.MemberAlumni, VerificationMethod: user.VerificationNone, ChapterID: chapters[0].ID.Hex(), Status: 0}
	service, _, _ := newTestAdminService([]*user.User{admin, target}, chapters, nil)
	ctx := adaptor.WithUserID(context.Background(), admin.ID.Hex())

	year := int64(2019)
	ningbo := chapters[1].ID.Hex()
	updated, err := service.UpdateUser(ctx, target.ID.Hex(), AdminUserUpdate{
		GraduationYear: &year,
		ChapterID:      &ningbo,
		Educations: []user.Education{{
			Phase: "本科", School: "同济大学", Year: 2019,
			ProvinceCode: "310000", ProvinceName: "上海市", CityCode: "310104", CityName: "徐汇区",
		}},
	})
	if err != nil {
		t.Fatalf("更新用户失败: %v", err)
	}
	if updated.GraduationYear != 2019 {
		t.Fatalf("毕业年份未保存，实际 %d", updated.GraduationYear)
	}
	if updated.ChapterID != ningbo || updated.ChapterName != chapters[1].Name {
		t.Fatalf("分会归属未更新: %s / %s", updated.ChapterID, updated.ChapterName)
	}
	if len(updated.Educations) != 1 || updated.Educations[0].CityCode != "310104" {
		t.Fatalf("教育经历省市代码未保存: %+v", updated.Educations)
	}
	if updated.MemberRole != user.MemberAlumni || updated.VerificationMethod != user.VerificationNone {
		t.Fatalf("更新资料不应改变身份，实际 %s / %s", updated.MemberRole, updated.VerificationMethod)
	}
}
