package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/alumni/core_api"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	activitymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	registermodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/sms"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 本文件覆盖报名身份门控、名册自动认证与导入、生日短信与资料补全。

func activityForRegistration(t *testing.T) (*ActivityService, *activitymodel.Activity) {
	t.Helper()
	now := time.Now()
	activity := &activitymodel.Activity{
		ID:            primitive.NewObjectID(),
		Name:          "夏日交流会",
		ChapterID:     primitive.NewObjectID().Hex(),
		Start:         now.Add(48 * time.Hour).Unix(),
		RegisterStart: now.Add(-time.Hour),
		RegisterEnd:   now.Add(24 * time.Hour),
		Limit:         -1,
		Status:        0,
	}
	service := &ActivityService{
		ActivityMapper: &fakeActivityMapper{items: []*activitymodel.Activity{activity}},
		RegisterMapper: &fakeRegisterMapper{},
		UserMapper:     &fakeUserMapper{},
		ChapterMapper:  &fakeChapterMapper{},
	}
	return service, activity
}

func TestRegisterActivityRequiresVerifiedIdentity(t *testing.T) {
	service, activity := activityForRegistration(t)
	request := &core_api.RegisterActivityReq{
		ActivityId: activity.ID.Hex(),
		Items:      []*core_api.RegisterActivityReq_RegisterItem{{Name: "张三", Phone: "13800000000"}},
	}

	// 未登录。
	if _, err := service.RegisterActivity(context.Background(), request); !errors.Is(err, consts.ErrNotAuthentication) {
		t.Fatalf("未登录报名应被拒绝，实际 %v", err)
	}

	cases := []struct {
		name       string
		memberRole string
		allowed    bool
	}{
		{"待认证用户不能报名", user.MemberPending, false},
		{"校友可以报名", user.MemberAlumni, true},
		{"嘉宾可以报名", user.MemberGuest, true},
		{"身份缺失按待认证处理", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			item := &user.User{
				ID:         primitive.NewObjectID(),
				Name:       "报名人",
				MemberRole: testCase.memberRole,
				Status:     0,
			}
			service.UserMapper = &fakeUserMapper{items: []*user.User{item}}
			ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())
			resp, err := service.RegisterActivity(ctx, request)
			if testCase.allowed {
				if err != nil {
					t.Fatalf("应允许报名，实际 %v", err)
				}
				if resp.Code != 0 {
					t.Fatalf("报名返回码异常: %d", resp.Code)
				}
				return
			}
			if err == nil {
				t.Fatalf("应拒绝报名，实际成功: %+v", resp)
			}
		})
	}
}

func TestRegisterActivityChecksWindowAndStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		mutate   func(*activitymodel.Activity)
		expected error
	}{
		{"活动已下架", func(a *activitymodel.Activity) { a.Status = 1 }, consts.ErrNotFound},
		{"报名未开始", func(a *activitymodel.Activity) { a.RegisterStart = now.Add(time.Hour) }, consts.ErrForbidden},
		{"报名已截止", func(a *activitymodel.Activity) { a.RegisterEnd = now.Add(-time.Hour) }, consts.ErrForbidden},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, activity := activityForRegistration(t)
			testCase.mutate(activity)
			item := &user.User{ID: primitive.NewObjectID(), Name: "校友", MemberRole: user.MemberAlumni, Status: 0}
			service.UserMapper = &fakeUserMapper{items: []*user.User{item}}
			ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())
			_, err := service.RegisterActivity(ctx, &core_api.RegisterActivityReq{
				ActivityId: activity.ID.Hex(),
				Items:      []*core_api.RegisterActivityReq_RegisterItem{{Name: "校友", Phone: "13800000000"}},
			})
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("期望 %v，实际 %v", testCase.expected, err)
			}
		})
	}
}

func TestRegisterActivityRejectsOverCapacity(t *testing.T) {
	service, activity := activityForRegistration(t)
	activity.Limit = 2
	existing := &registermodel.Register{Id: primitive.NewObjectID(), ActivityId: activity.ID.Hex(), Name: "已有", Phone: "13800000001"}
	service.RegisterMapper = &fakeRegisterMapper{items: []*registermodel.Register{existing}}
	item := &user.User{ID: primitive.NewObjectID(), Name: "校友", MemberRole: user.MemberAlumni, Status: 0}
	service.UserMapper = &fakeUserMapper{items: []*user.User{item}}
	ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())

	_, err := service.RegisterActivity(ctx, &core_api.RegisterActivityReq{
		ActivityId: activity.ID.Hex(),
		Items: []*core_api.RegisterActivityReq_RegisterItem{
			{Name: "甲", Phone: "13800000002"},
			{Name: "乙", Phone: "13800000003"},
		},
	})
	if !errors.Is(err, consts.ErrForbidden) {
		t.Fatalf("超额报名应被拒绝，实际 %v", err)
	}
}

func TestRegisterActivityCarriesChapterFromActivity(t *testing.T) {
	service, activity := activityForRegistration(t)
	item := &user.User{ID: primitive.NewObjectID(), Name: "校友", MemberRole: user.MemberAlumni, Status: 0}
	service.UserMapper = &fakeUserMapper{items: []*user.User{item}}
	ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())

	if _, err := service.RegisterActivity(ctx, &core_api.RegisterActivityReq{
		ActivityId: activity.ID.Hex(),
		Items:      []*core_api.RegisterActivityReq_RegisterItem{{Name: "校友", Phone: "13800000000"}},
	}); err != nil {
		t.Fatalf("报名失败: %v", err)
	}
	registers := service.RegisterMapper.(*fakeRegisterMapper).items
	if len(registers) != 1 {
		t.Fatalf("应写入一条报名，实际 %d 条", len(registers))
	}
	if registers[0].ChapterID != activity.ChapterID {
		t.Fatalf("报名的分会应取自活动，实际 %s", registers[0].ChapterID)
	}
}

func rosterFixture() []*chapter.Chapter { return chapterFixture(chapter.CodeShanghai) }

// birthdayTestConfig 只提供生日任务需要的模板配置。
func birthdayTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.BirthdaySms.TemplateCode = "ALUMNI_BIRTHDAY"
	return cfg
}

func TestUpdateProfileRerunsRosterVerification(t *testing.T) {
	chapters := rosterFixture()
	pending := &user.User{
		ID:                 primitive.NewObjectID(),
		Name:               "张三",
		MemberRole:         user.MemberPending,
		VerificationMethod: user.VerificationNone,
		Status:             0,
	}
	userMapper := &fakeUserMapper{items: []*user.User{pending}}
	rosterMapper := &fakeRosterMapper{items: []roster.Entry{{
		Name: "张三", NormalizedName: "张三", GraduationYear: 2018, BirthDate: "2000-05-20",
	}}}
	service := &UserService{
		UserMapper:    userMapper,
		ChapterMapper: &fakeChapterMapper{items: chapters},
		RosterMapper:  rosterMapper,
	}
	ctx := adaptor.WithUserID(context.Background(), pending.ID.Hex())

	chapterID := chapters[0].ID.Hex()
	graduationYear := int64(2018)
	birthDate := "2000-05-20"
	profile, err := service.UpdateProfile(ctx, ProfileUpdate{
		ChapterID:      &chapterID,
		GraduationYear: &graduationYear,
		BirthDate:      &birthDate,
	})
	if err != nil {
		t.Fatalf("补全资料失败: %v", err)
	}
	if profile.MemberRole != user.MemberAlumni {
		t.Fatalf("名册命中后应自动成为校友，实际 %s", profile.MemberRole)
	}
	if profile.VerificationMethod != user.VerificationRoster {
		t.Fatalf("认证方式应为 roster，实际 %s", profile.VerificationMethod)
	}
	if !profile.ProfileComplete {
		t.Fatalf("补全后 profileComplete 应为 true")
	}

	// 不匹配时保持待认证。
	other := &user.User{ID: primitive.NewObjectID(), Name: "李四", MemberRole: user.MemberPending, Status: 0}
	userMapper.items = append(userMapper.items, other)
	ctx = adaptor.WithUserID(context.Background(), other.ID.Hex())
	profile, err = service.UpdateProfile(ctx, ProfileUpdate{
		ChapterID:      &chapterID,
		GraduationYear: &graduationYear,
		BirthDate:      &birthDate,
	})
	if err != nil {
		t.Fatalf("补全资料失败: %v", err)
	}
	if profile.MemberRole != user.MemberPending {
		t.Fatalf("名册未命中应保持待认证，实际 %s", profile.MemberRole)
	}
	if profile.ProfileComplete != true {
		t.Fatalf("资料填齐后 profileComplete 应为 true")
	}
}

func TestUpdateProfileDoesNotDowngradeVerifiedUser(t *testing.T) {
	chapters := rosterFixture()
	alumni := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		MemberRole: user.MemberAlumni,
		Status:     0,
	}
	service := &UserService{
		UserMapper:    &fakeUserMapper{items: []*user.User{alumni}},
		ChapterMapper: &fakeChapterMapper{items: chapters},
		RosterMapper:  &fakeRosterMapper{},
	}
	ctx := adaptor.WithUserID(context.Background(), alumni.ID.Hex())
	graduationYear := int64(2018)
	birthDate := "2000-05-20"
	profile, err := service.UpdateProfile(ctx, ProfileUpdate{GraduationYear: &graduationYear, BirthDate: &birthDate})
	if err != nil {
		t.Fatalf("更新资料失败: %v", err)
	}
	if profile.MemberRole != user.MemberAlumni {
		t.Fatalf("已认证身份不应被覆盖，实际 %s", profile.MemberRole)
	}
}

func TestNegativeGraduationYearRejected(t *testing.T) {
	item := &user.User{ID: primitive.NewObjectID(), Name: "张三", MemberRole: user.MemberPending, Status: 0}
	service := &UserService{
		UserMapper:    &fakeUserMapper{items: []*user.User{item}},
		ChapterMapper: &fakeChapterMapper{items: rosterFixture()},
		RosterMapper:  &fakeRosterMapper{},
	}
	ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())
	bad := int64(1800)
	if _, err := service.UpdateProfile(ctx, ProfileUpdate{GraduationYear: &bad}); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("非法毕业年份应被拒绝，实际 %v", err)
	}
	invalidDate := "2000/05/20"
	if _, err := service.UpdateProfile(ctx, ProfileUpdate{BirthDate: &invalidDate}); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("非法出生日期应被拒绝，实际 %v", err)
	}
}

func TestValidateEducationPhases(t *testing.T) {
	legacy := []user.Education{{Phase: "高中", School: "北仑中学", Year: 2013}}

	cases := []struct {
		name     string
		incoming []user.Education
		ok       bool
	}{
		{"允许的高等教育阶段", []user.Education{{Phase: "本科", School: "同济大学", Year: 2018}}, true},
		{"允许的其他", []user.Education{{Phase: "其他", School: "某学院", Year: 2010}}, true},
		{"新增中小学记录被拒绝", []user.Education{{Phase: "小学", School: "某某小学", Year: 2005}}, false},
		{"新增初中记录被拒绝", []user.Education{{Phase: "初中", School: "某某初中", Year: 2008}}, false},
		{"旧记录原样提交被放行", []user.Education{{Phase: "高中", School: "北仑中学", Year: 2013}}, true},
		{"旧记录改动后被拒绝", []user.Education{{Phase: "高中", School: "另一所中学", Year: 2013}}, false},
		{"混合提交", []user.Education{
			{Phase: "本科", School: "同济大学", Year: 2018},
			{Phase: "高中", School: "北仑中学", Year: 2013},
		}, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateEducationPhases(legacy, testCase.incoming)
			if testCase.ok && err != nil {
				t.Fatalf("应通过校验，实际 %v", err)
			}
			if !testCase.ok && err == nil {
				t.Fatalf("应拒绝，实际通过")
			}
		})
	}
}

func TestImportRosterReportsCounts(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai)
	super := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	service, _, rosterMapper := newTestAdminService([]*user.User{super}, chapters, nil)
	ctx := adaptor.WithUserID(context.Background(), super.ID.Hex())

	csv := "姓名,毕业年份,出生日期\n" +
		"张三,2018,2000-05-20\n" +
		"张三,2018,2000-05-20\n" + // 文件内重复
		"李四,2019,2001-01-01\n" +
		"王五,abc,2000-01-01\n" // 年份非法
	result, err := service.ImportRoster(ctx, "roster.csv", []byte(csv))
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if result.Created != 2 {
		t.Fatalf("新增数应为 2，实际 %d", result.Created)
	}
	if result.Duplicates != 1 {
		t.Fatalf("重复数应为 1，实际 %d", result.Duplicates)
	}
	if result.Failed != 1 {
		t.Fatalf("失败数应为 1，实际 %d", result.Failed)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("应返回 2 条逐行错误，实际 %d", len(result.Errors))
	}
	if result.Errors[0].Row != 3 {
		t.Fatalf("重复行号应为 3，实际 %d", result.Errors[0].Row)
	}
	if result.Errors[1].Row != 5 {
		t.Fatalf("非法行号应为 5，实际 %d", result.Errors[1].Row)
	}
	if len(rosterMapper.items) != 2 {
		t.Fatalf("名册应只保存 2 条，实际 %d", len(rosterMapper.items))
	}

	// 重复导入同一文件不应产生新记录，只更新。
	again, err := service.ImportRoster(ctx, "roster.csv", []byte(csv))
	if err != nil {
		t.Fatalf("二次导入失败: %v", err)
	}
	if again.Created != 0 || again.Updated != 2 {
		t.Fatalf("二次导入应全部更新: %+v", again)
	}
	if len(rosterMapper.items) != 2 {
		t.Fatalf("二次导入后名册数不变，实际 %d", len(rosterMapper.items))
	}
}

func TestImportRosterValidatesHeaderAndExtension(t *testing.T) {
	super := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	service, _, _ := newTestAdminService([]*user.User{super}, chapterFixture(chapter.CodeShanghai), nil)
	ctx := adaptor.WithUserID(context.Background(), super.ID.Hex())

	badHeader := []byte("名字,年份,生日\n张三,2018,2000-05-20\n")
	if _, err := service.ImportRoster(ctx, "roster.csv", badHeader); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("表头错误应被拒绝，实际 %v", err)
	}
	good := []byte("姓名,毕业年份,出生日期\n张三,2018,2000-05-20\n")
	if _, err := service.ImportRoster(ctx, "roster.txt", good); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("不支持的扩展名应被拒绝，实际 %v", err)
	}
	if _, err := service.ImportRoster(ctx, "roster.csv", []byte("姓名,毕业年份,出生日期\n")); err != nil {
		t.Fatalf("只有表头应视为空导入而不是错误: %v", err)
	}
}

func TestImportRosterAcceptsSlashDates(t *testing.T) {
	super := &user.User{ID: primitive.NewObjectID(), AdminRole: user.AdminSuper, Status: 0}
	service, _, rosterMapper := newTestAdminService([]*user.User{super}, chapterFixture(chapter.CodeShanghai), nil)
	ctx := adaptor.WithUserID(context.Background(), super.ID.Hex())

	result, err := service.ImportRoster(ctx, "roster.csv", []byte("姓名,毕业年份,出生日期\n张三,2018,2000/05/20\n"))
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("应新增 1 条，实际 %+v", result)
	}
	if rosterMapper.items[0].BirthDate != "2000-05-20" {
		t.Fatalf("斜杠日期应被规范化，实际 %s", rosterMapper.items[0].BirthDate)
	}
}

func birthdayServiceFixture(t *testing.T, items []*user.User, sender *fakeSender) (*BirthdayService, *fakeBirthdayMapper) {
	t.Helper()
	birthdayMapper := &fakeBirthdayMapper{}
	service := &BirthdayService{
		Config:         birthdayTestConfig(),
		UserMapper:     &fakeUserMapper{items: items},
		BirthdayMapper: birthdayMapper,
		Sender:         sender,
	}
	return service, birthdayMapper
}

func TestBirthdayMatchesHandlesLeapDay(t *testing.T) {
	birth := time.Date(2000, time.February, 29, 0, 0, 0, 0, time.UTC)
	if !birthdayMatches(birth, time.Date(2024, time.February, 29, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("闰年 2 月 29 日应命中")
	}
	if !birthdayMatches(birth, time.Date(2026, time.February, 28, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("非闰年应在 2 月 28 日发送")
	}
	if birthdayMatches(birth, time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("3 月 1 日不应命中")
	}
	if !birthdayMatches(time.Date(1999, time.March, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("普通日期应命中")
	}
}

func TestBirthdayRunSendsOncePerYear(t *testing.T) {
	sender := &fakeSender{}
	alumni := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		Phone:      "13800000000",
		MemberRole: user.MemberAlumni,
		BirthDate:  "2000-05-20",
		Status:     0,
	}
	service, _ := birthdayServiceFixture(t, []*user.User{alumni}, sender)
	date := time.Date(2026, time.May, 20, 9, 0, 0, 0, time.UTC)

	sent, err := service.Run(context.Background(), date)
	if err != nil {
		t.Fatalf("生日任务失败: %v", err)
	}
	if sent != 1 || len(sender.sent) != 1 {
		t.Fatalf("应发送 1 条，实际 sent=%d calls=%d", sent, len(sender.sent))
	}
	if sender.sent[0].IdempotencyKey != "birthday:"+alumni.ID.Hex()+":2026" {
		t.Fatalf("幂等键不正确: %s", sender.sent[0].IdempotencyKey)
	}
	if sender.sent[0].Params["name"] != "张三" || sender.sent[0].Phone != "13800000000" {
		t.Fatalf("模板参数不正确: %+v", sender.sent[0])
	}

	// 同一年度重复执行不应重复发送。
	repeated, err := service.Run(context.Background(), date)
	if err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}
	if repeated != 0 || len(sender.sent) != 1 {
		t.Fatalf("同一年度不应重复发送，实际 sent=%d calls=%d", repeated, len(sender.sent))
	}

	// 跨年度可以再次发送。
	nextYear, err := service.Run(context.Background(), time.Date(2027, time.May, 20, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("次年执行失败: %v", err)
	}
	if nextYear != 1 || len(sender.sent) != 2 {
		t.Fatalf("次年应再次发送，实际 sent=%d calls=%d", nextYear, len(sender.sent))
	}
}

func TestBirthdayRunRetriesAtMostTwice(t *testing.T) {
	sender := &fakeSender{failFor: 10}
	alumni := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		Phone:      "13800000000",
		MemberRole: user.MemberAlumni,
		BirthDate:  "2000-05-20",
		Status:     0,
	}
	service, birthdayMapper := birthdayServiceFixture(t, []*user.User{alumni}, sender)

	sent, err := service.Run(context.Background(), time.Date(2026, time.May, 20, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("生日任务失败: %v", err)
	}
	if sent != 0 {
		t.Fatalf("全部失败时不应计入成功，实际 %d", sent)
	}
	if len(sender.sent) != maxBirthdayAttempts {
		t.Fatalf("最多尝试 %d 次，实际 %d 次", maxBirthdayAttempts, len(sender.sent))
	}
	if len(birthdayMapper.logs) != 1 {
		t.Fatalf("应记录一条失败日志，实际 %d 条", len(birthdayMapper.logs))
	}
	if birthdayMapper.logs[0].Status != "failed" || birthdayMapper.logs[0].Attempts != maxBirthdayAttempts {
		t.Fatalf("失败日志不正确: %+v", birthdayMapper.logs[0])
	}

	// 单日尝试次数用尽后，同一天再次执行不会再发送，也不会被误判为已发送。
	before := len(sender.sent)
	repeated, err := service.Run(context.Background(), time.Date(2026, time.May, 20, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}
	if repeated != 0 || len(sender.sent) != before {
		t.Fatalf("次数用尽后不应继续发送，实际 sent=%d calls=%d", repeated, len(sender.sent)-before)
	}
	if birthdayMapper.logs[0].Status == sms.StatusSent {
		t.Fatalf("未发送成功的记录不能被标记为已发送")
	}

	// 次日重置尝试次数，可以重新发送。
	sender.failFor = 0
	nextDay, err := service.Run(context.Background(), time.Date(2027, time.May, 20, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("次年执行失败: %v", err)
	}
	if nextDay != 1 {
		t.Fatalf("次年应重新发送成功，实际 %d", nextDay)
	}
}

func TestBirthdayRunSkipsNonMatchingDates(t *testing.T) {
	sender := &fakeSender{}
	alumni := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		Phone:      "13800000000",
		MemberRole: user.MemberAlumni,
		BirthDate:  "2000-05-20",
		Status:     0,
	}
	service, _ := birthdayServiceFixture(t, []*user.User{alumni}, sender)
	sent, err := service.Run(context.Background(), time.Date(2026, time.May, 21, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("生日任务失败: %v", err)
	}
	if sent != 0 || len(sender.sent) != 0 {
		t.Fatalf("非生日不应发送，实际 sent=%d calls=%d", sent, len(sender.sent))
	}
}

func TestBirthdayCandidatesFilterExcludesNonAlumniAndDisabled(t *testing.T) {
	filter := user.BirthdayCandidatesFilter()
	if filter["member_role"] != user.MemberAlumni {
		t.Fatalf("候选人必须限定为已认证校友，实际 %v", filter["member_role"])
	}
	excluded := []*user.User{
		{ID: primitive.NewObjectID(), Name: "嘉宾", Phone: "13800000001", MemberRole: user.MemberGuest, BirthDate: "2000-05-20"},
		{ID: primitive.NewObjectID(), Name: "待认证", Phone: "13800000002", MemberRole: user.MemberPending, BirthDate: "2000-05-20"},
		{ID: primitive.NewObjectID(), Name: "停用", Phone: "13800000003", MemberRole: user.MemberAlumni, Status: 1, BirthDate: "2000-05-20"},
		{ID: primitive.NewObjectID(), Name: "已删除", Phone: "13800000004", MemberRole: user.MemberAlumni, Status: 1, DeleteTime: time.Now(), BirthDate: "2000-05-20"},
		{ID: primitive.NewObjectID(), Name: "无手机号", Phone: "-1", MemberRole: user.MemberAlumni, BirthDate: "2000-05-20"},
		{ID: primitive.NewObjectID(), Name: "空手机号", Phone: "", MemberRole: user.MemberAlumni, BirthDate: "2000-05-20"},
	}
	for _, item := range excluded {
		if matchesUserFilter(filter, item) {
			t.Fatalf("用户 %s 不应成为生日短信候选人", item.Name)
		}
	}
	included := &user.User{ID: primitive.NewObjectID(), Name: "校友", Phone: "13800000005", MemberRole: user.MemberAlumni, BirthDate: "2000-05-20", Status: 0}
	if !matchesUserFilter(filter, included) {
		t.Fatalf("正常校友应成为候选人")
	}
}

func TestBirthdaySenderResultIsPersisted(t *testing.T) {
	sender := &fakeSender{}
	alumni := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		Phone:      "13800000000",
		MemberRole: user.MemberAlumni,
		BirthDate:  "2000-05-20",
		Status:     0,
	}
	service, birthdayMapper := birthdayServiceFixture(t, []*user.User{alumni}, sender)
	if _, err := service.Run(context.Background(), time.Date(2026, time.May, 20, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("生日任务失败: %v", err)
	}
	if len(birthdayMapper.logs) != 1 {
		t.Fatalf("应写入发送日志")
	}
	record := birthdayMapper.logs[0]
	if record.Status != sms.StatusSent || record.ProviderID != "msg-13800000000" {
		t.Fatalf("发送回执未正确保存: %+v", record)
	}
}

func TestPublicActivityListFiltersByChapter(t *testing.T) {
	shanghai := primitive.NewObjectID().Hex()
	ningbo := primitive.NewObjectID().Hex()
	service := &ActivityService{
		ActivityMapper: &fakeActivityMapper{items: []*activitymodel.Activity{
			{ID: primitive.NewObjectID(), Name: "上海活动", ChapterID: shanghai, Status: 0},
			{ID: primitive.NewObjectID(), Name: "宁波活动", ChapterID: ningbo, Status: 0},
			{ID: primitive.NewObjectID(), Name: "已下架", ChapterID: shanghai, Status: 1},
		}},
		ChapterMapper:  &fakeChapterMapper{},
		RegisterMapper: &fakeRegisterMapper{},
	}

	all, err := service.ListPublic(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatalf("公开活动列表失败: %v", err)
	}
	if all.Total != 2 {
		t.Fatalf("不传分会应返回全部有效活动，实际 %d", all.Total)
	}

	onlyShanghai, err := service.ListPublic(context.Background(), shanghai, 1, 10)
	if err != nil {
		t.Fatalf("按分会筛选失败: %v", err)
	}
	if onlyShanghai.Total != 1 || onlyShanghai.Items[0].ChapterID != shanghai {
		t.Fatalf("按分会筛选结果不正确: %+v", onlyShanghai.Items)
	}
}

func TestReplaceEducationsKeepsLegacyRecordsOnRoundTrip(t *testing.T) {
	item := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "张三",
		MemberRole: user.MemberAlumni,
		Status:     0,
		Educations: []user.Education{
			{Phase: "小学", School: "某某小学", Year: 2006},
			{Phase: "本科", School: "同济大学", Year: 2018},
		},
	}
	userMapper := &fakeUserMapper{items: []*user.User{item}}
	chapterMapper := &fakeChapterMapper{items: rosterFixture()}
	service := &UserService{
		UserMapper:    userMapper,
		ChapterMapper: chapterMapper,
		RosterMapper:  &fakeRosterMapper{},
	}
	ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())

	// 客户端把旧记录原样回传并新增一条硕士记录：全部保留。
	profile, err := service.ReplaceEducations(ctx, []user.Education{
		{Phase: "小学", School: "某某小学", Year: 2006},
		{Phase: "本科", School: "同济大学", Year: 2018},
		{Phase: "硕士", School: "某大学", Year: 2021},
	})
	if err != nil {
		t.Fatalf("整体替换失败: %v", err)
	}
	if len(profile.Educations) != 3 {
		t.Fatalf("旧记录应被保留，实际 %d 条: %+v", len(profile.Educations), profile.Educations)
	}

	// 用户显式删除旧记录后不再提交：整体替换语义下即完成清理。
	profile, err = service.ReplaceEducations(ctx, []user.Education{
		{Phase: "本科", School: "同济大学", Year: 2018},
		{Phase: "硕士", School: "某大学", Year: 2021},
	})
	if err != nil {
		t.Fatalf("整体替换失败: %v", err)
	}
	if len(profile.Educations) != 2 {
		t.Fatalf("显式删除后应只剩 2 条，实际 %d 条", len(profile.Educations))
	}

	// 凭空新增一条中小学记录必须被拒绝。
	if _, err = service.ReplaceEducations(ctx, []user.Education{
		{Phase: "初中", School: "某某初中", Year: 2009},
	}); !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("新增中小学记录应被拒绝，实际 %v", err)
	}
	if len(userMapper.items[0].Educations) != 2 {
		t.Fatalf("被拒绝的请求不应改动数据，实际 %d 条", len(userMapper.items[0].Educations))
	}
}

func TestBadRequestErrorsAreUserFacingAndStillMatchSentinel(t *testing.T) {
	item := &user.User{ID: primitive.NewObjectID(), Name: "张三", MemberRole: user.MemberPending, Status: 0}
	service := &UserService{
		UserMapper:    &fakeUserMapper{items: []*user.User{item}},
		ChapterMapper: &fakeChapterMapper{items: rosterFixture()},
		RosterMapper:  &fakeRosterMapper{},
	}
	ctx := adaptor.WithUserID(context.Background(), item.ID.Hex())

	bad := int64(1800)
	_, err := service.UpdateProfile(ctx, ProfileUpdate{GraduationYear: &bad})
	if err == nil {
		t.Fatalf("非法毕业年份应被拒绝")
	}
	// 既要能被控制层识别为 400，也要能直接展示给用户。
	if !errors.Is(err, ErrAdminBadRequest) {
		t.Fatalf("错误应匹配 ErrAdminBadRequest，实际 %v", err)
	}
	if err.Error() != "请选择四位毕业年份" {
		t.Fatalf("错误文案应面向用户，实际 %q", err.Error())
	}

	_, err = service.ReplaceEducations(ctx, []user.Education{{Phase: "小学", School: "某某小学", Year: 2005}})
	if !errors.Is(err, ErrAdminBadRequest) || err.Error() != "新增教育经历只支持大专、本科、硕士、博士及其他" {
		t.Fatalf("教育阶段错误文案不正确: %v", err)
	}
}
