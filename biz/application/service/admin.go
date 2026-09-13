package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	appconsts "github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util/log"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	defaultPage     = int64(1)
	defaultPageSize = int64(20)
	maxPageSize     = int64(100)
)

var (
	ErrAdminUnauthorized = errors.New("登录已失效")
	ErrAdminForbidden    = errors.New("当前账号无管理权限")
	ErrAdminBadRequest   = errors.New("请求参数错误")
	ErrAdminNotFound     = errors.New("资源不存在")
)

// BadRequestError 是可以直接展示给用户的参数错误。
// 它同时匹配 ErrAdminBadRequest，因此控制层无需额外分支即可返回 400 与具体原因。
type BadRequestError struct {
	message string
}

func (e *BadRequestError) Error() string { return e.message }

// Is 让 errors.Is(err, ErrAdminBadRequest) 成立，同时保留可读的错误文案。
func (e *BadRequestError) Is(target error) bool { return target == ErrAdminBadRequest }

// badRequest 构造一条面向用户的参数错误。
func badRequest(message string) error { return &BadRequestError{message: message} }

type AdminService struct {
	UserMapper     user.IMongoMapper
	RegisterMapper register.IMongoMapper
	ArticleMapper  article.IMongoMapper
	ActivityMapper activity.IMongoMapper
	ChapterMapper  chapter.IMongoMapper
	RosterMapper   roster.IMongoMapper
}

var AdminServiceSet = wire.NewSet(
	wire.Struct(new(AdminService), "*"),
)

// nonNil 保证切片字段序列化成 [] 而不是 null。
//
// 数据库里没有该字段时 Go 的零值是 nil，直接返回会让 JSON 变成 null，
// 前端一律按数组处理（.map / .length）就会崩。契约里这些字段都是数组，
// 所以出口统一兜一层。
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// chapterResolver 在一次请求内缓存分会 ID → 名称的映射，
// 避免列表接口对每一行都单独查询一次分会集合。
type chapterResolver struct {
	ctx    context.Context
	mapper chapter.IMongoMapper
	names  map[string]string
}

func (s *AdminService) newChapterResolver(ctx context.Context) *chapterResolver {
	return &chapterResolver{ctx: ctx, mapper: s.ChapterMapper, names: map[string]string{}}
}

func (r *chapterResolver) name(id string) string {
	if id == "" || r.mapper == nil {
		return ""
	}
	if name, ok := r.names[id]; ok {
		return name
	}
	name := ""
	if ch, err := r.mapper.FindByID(r.ctx, id); err == nil {
		name = ch.Name
	}
	r.names[id] = name
	return name
}

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"pageSize"`
}

type AdminSession struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Avatar           string `json:"avatar"`
	Phone            string `json:"phone"`
	Role             string `json:"role"`
	AdminRole        string `json:"adminRole"`
	AdminChapterID   string `json:"adminChapterId"`
	AdminChapterName string `json:"adminChapterName"`
}

type AdminUser struct {
	ID                 string            `json:"id"`
	Avatar             string            `json:"avatar"`
	Name               string            `json:"name"`
	Gender             int64             `json:"gender"`
	Birthday           int64             `json:"birthday"`
	Phone              string            `json:"phone"`
	WxID               string            `json:"wxId"`
	Hometown           string            `json:"hometown"`
	HomeEducations     []user.Education  `json:"homeEducations"`
	ShanghaiEducations []user.Education  `json:"shanghaiEducations"`
	Employments        []user.Employment `json:"employments"`
	Role               string            `json:"role"`
	ChapterID          string            `json:"chapterId"`
	ChapterName        string            `json:"chapterName"`
	GraduationYear     int64             `json:"graduationYear"`
	MemberRole         string            `json:"memberRole"`
	AdminRole          string            `json:"adminRole"`
	AdminChapterID     string            `json:"adminChapterId"`
	VerificationMethod string            `json:"verificationMethod"`
	Educations         []user.Education  `json:"educations"`
	Status             int64             `json:"status"`
	Deleted            bool              `json:"deleted"`
	CreateTime         int64             `json:"createTime"`
}

type AdminUserUpdate struct {
	Avatar             *string           `json:"avatar"`
	Name               *string           `json:"name"`
	Gender             *int64            `json:"gender"`
	Birthday           *int64            `json:"birthday"`
	Phone              *string           `json:"phone"`
	WxID               *string           `json:"wxId"`
	Hometown           *string           `json:"hometown"`
	HomeEducations     []user.Education  `json:"homeEducations"`
	ShanghaiEducations []user.Education  `json:"shanghaiEducations"`
	Employments        []user.Employment `json:"employments"`
	ChapterID          *string           `json:"chapterId"`
	GraduationYear     *int64            `json:"graduationYear"`
	Educations         []user.Education  `json:"educations"`
}

// AdminUserQuery 是人员列表的筛选条件。
// 分会管理员的 ChapterID 由服务端强制覆盖为本分会，前端参数被忽略。
type AdminUserQuery struct {
	Keyword            string
	ChapterID          string
	MemberRole         string
	VerificationMethod string
	GraduationYear     int64
	Status             string
}

// AdminRegistrationQuery 是报名列表的筛选条件。
// ActivityID 与 ChapterID 都为空时，分会管理员查看本分会全部报名，超级管理员查看全部。
type AdminRegistrationQuery struct {
	ActivityID string
	ChapterID  string
	Keyword    string
	CheckIn    string
}

type AdminRegistration struct {
	ID           string `json:"id"`
	ActivityID   string `json:"activityId"`
	ActivityName string `json:"activityName"`
	ChapterID    string `json:"chapterId"`
	ChapterName  string `json:"chapterName"`
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	CheckIn      bool   `json:"checkIn"`
	CheckInTime  *int64 `json:"checkInTime"`
	Deleted      bool   `json:"deleted"`
	CreateTime   int64  `json:"createTime"`
}

type AdminRegistrationInput struct {
	ActivityID string `json:"activityId"`
	UserID     string `json:"userId"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
}

type AdminRegistrationPage struct {
	Items    []AdminRegistration `json:"items"`
	Total    int64               `json:"total"`
	Page     int64               `json:"page"`
	PageSize int64               `json:"pageSize"`
	Checked  int64               `json:"checked"`
}

type AdminArticle struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Cover         string `json:"cover"`
	WechatURL     string `json:"wechatUrl"`
	Source        string `json:"source"`
	Author        string `json:"author"`
	PublishTime   *int64 `json:"publishTime"`
	SortOrder     int64  `json:"sortOrder"`
	PublishStatus string `json:"publishStatus"`
	Deleted       bool   `json:"deleted"`
	CreateTime    int64  `json:"createTime"`
	ChapterID     string `json:"chapterId"`
	ChapterName   string `json:"chapterName"`
}

type AdminArticleInput struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Cover       string `json:"cover"`
	WechatURL   string `json:"wechatUrl"`
	Source      string `json:"source"`
	Author      string `json:"author"`
	PublishTime *int64 `json:"publishTime"`
	SortOrder   int64  `json:"sortOrder"`
	ChapterID   string `json:"chapterId"`
}

func (s *AdminService) GetSession(ctx context.Context) (*AdminSession, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, ErrAdminUnauthorized
	}

	if adaptor.IsDevModeRequest(ctx) && userMeta.GetUserId() == appconsts.DevMockUserID {
		return &AdminSession{
			ID:        appconsts.DevMockUserID,
			Name:      "演示管理员",
			Avatar:    "",
			Phone:     "13800000000",
			Role:      "admin",
			AdminRole: user.AdminSuper,
		}, nil
	}

	item, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, ErrAdminUnauthorized
	}
	adminRole := effectiveAdminRole(item)
	if adminRole == user.AdminNone || item.Status != 0 || !item.DeleteTime.IsZero() {
		return nil, ErrAdminForbidden
	}
	chapterName := ""
	if item.AdminChapterID != "" {
		if ch, findErr := s.ChapterMapper.FindByID(ctx, item.AdminChapterID); findErr == nil {
			chapterName = ch.Name
		}
	}

	return &AdminSession{
		ID:               item.ID.Hex(),
		Name:             item.Name,
		Avatar:           item.Avatar,
		Phone:            item.Phone,
		Role:             "admin",
		AdminRole:        adminRole,
		AdminChapterID:   item.AdminChapterID,
		AdminChapterName: chapterName,
	}, nil
}

func (s *AdminService) requireScope(ctx context.Context, chapterID string) (*AdminSession, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole == user.AdminChapter && (chapterID == "" || chapterID != session.AdminChapterID) {
		// 越权拒绝需要可观测：监控“越权拒绝数”即可发现异常调用或前端缺陷。
		log.CtxError(ctx, "metric=admin_scope_rejected admin=%s adminChapter=%s targetChapter=%s", session.ID, session.AdminChapterID, chapterID)
		return nil, ErrAdminForbidden
	}
	return session, nil
}

func (s *AdminService) ListUsers(ctx context.Context, page, pageSize int64, query AdminUserQuery) (*PageResult[AdminUser], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{}
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case session.AdminRole == user.AdminChapter:
		// 分会管理员的分会范围由服务端决定，忽略前端传入的 chapterId。
		filter["chapter_id"] = session.AdminChapterID
	case strings.TrimSpace(query.ChapterID) != "":
		filter["chapter_id"] = strings.TrimSpace(query.ChapterID)
	}
	andFilters := make([]bson.M, 0)
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		andFilters = append(andFilters, bson.M{"$or": []bson.M{
			{"name": bson.M{"$regex": pattern, "$options": "i"}},
			{"phone": bson.M{"$regex": pattern, "$options": "i"}},
		}})
	}
	if memberRole := normalizeMemberRole(query.MemberRole); memberRole != "" {
		filter["member_role"] = memberRole
	}
	if method := strings.TrimSpace(query.VerificationMethod); method != "" {
		filter["verification_method"] = method
	}
	if query.GraduationYear > 0 {
		filter["graduation_year"] = query.GraduationYear
	}
	switch query.Status {
	case "deleted":
		filter["delete_time"] = bson.M{"$exists": true, "$ne": time.Time{}}
	case "0", "1":
		filter["status"] = parseStatus(query.Status)
		if query.Status != "1" {
			andFilters = append(andFilters, bson.M{"$or": []bson.M{{"delete_time": bson.M{"$exists": false}}, {"delete_time": time.Time{}}}})
		}
	default:
		andFilters = append(andFilters,
			bson.M{"$or": []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}}},
			bson.M{"$or": []bson.M{{"delete_time": bson.M{"$exists": false}}, {"delete_time": time.Time{}}}},
		)
	}
	if len(andFilters) > 0 {
		filter["$and"] = andFilters
	}
	data, total, err := s.UserMapper.FindMany(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	resolver := s.newChapterResolver(ctx)
	items := make([]AdminUser, 0, len(data))
	for _, item := range data {
		items = append(items, s.mapAdminUser(item, resolver.name(item.ChapterID)))
	}
	return &PageResult[AdminUser]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// normalizeMemberRole 把对外暴露的身份值规整为库内取值，兼容旧的 role 语义。
func normalizeMemberRole(role string) string {
	switch strings.TrimSpace(role) {
	case user.MemberAlumni, user.MemberGuest, user.MemberPending:
		return strings.TrimSpace(role)
	case "user":
		return user.MemberPending
	case "admin":
		return user.MemberAlumni
	default:
		return ""
	}
}

func (s *AdminService) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return nil, err
	}
	result := s.mapAdminUserOne(ctx, item)
	return &result, nil
}

func (s *AdminService) UpdateUser(ctx context.Context, id string, input AdminUserUpdate) (*AdminUser, error) {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return nil, err
	}
	if input.Avatar != nil {
		item.Avatar = *input.Avatar
	}
	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Gender != nil {
		item.Gender = *input.Gender
	}
	if input.Birthday != nil {
		item.Birthday = unixToTime(*input.Birthday)
	}
	if input.Phone != nil {
		item.Phone = *input.Phone
	}
	if input.WxID != nil {
		item.WxId = *input.WxID
	}
	if input.Hometown != nil {
		item.Hometown = *input.Hometown
	}
	if input.HomeEducations != nil {
		item.HomeEducations = input.HomeEducations
	}
	if input.ShanghaiEducations != nil {
		item.ShanghaiEducations = input.ShanghaiEducations
	}
	if input.Employments != nil {
		item.Employments = input.Employments
	}
	if input.Educations != nil {
		item.Educations = input.Educations
	}
	if input.GraduationYear != nil {
		item.GraduationYear = *input.GraduationYear
	}
	if input.ChapterID != nil {
		if _, err = s.requireScope(ctx, *input.ChapterID); err != nil {
			return nil, err
		}
		item.ChapterID = *input.ChapterID
	}
	if err = s.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminUserOne(ctx, item)
	return &result, nil
}

func (s *AdminService) SetUserRole(ctx context.Context, id, role string) error {
	if role == "user" {
		role = user.MemberPending
	}
	if role != user.MemberPending && role != user.MemberAlumni && role != user.MemberGuest {
		return ErrAdminBadRequest
	}
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.MemberRole = role
	item.Role = role
	if role == user.MemberAlumni || role == user.MemberGuest {
		item.VerificationMethod = user.VerificationManual
		item.VerifiedAt = time.Now()
		if session, sessionErr := s.GetSession(ctx); sessionErr == nil {
			item.VerifiedBy = session.ID
		}
	} else {
		item.VerificationMethod = user.VerificationNone
		item.VerifiedAt = time.Time{}
		item.VerifiedBy = ""
	}
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) SetUserStatus(ctx context.Context, id string, status int64) error {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Status = status
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) DeleteUser(ctx context.Context, id string) error {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Status = 1
	item.DeleteTime = time.Now()
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) RestoreUser(ctx context.Context, id string) error {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Status = 0
	item.DeleteTime = time.Time{}
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) ListRegistrations(ctx context.Context, page, pageSize int64, query AdminRegistrationQuery) (*AdminRegistrationPage, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePage(page, pageSize)
	scope := bson.M{"$or": []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}}}
	if activityID := strings.TrimSpace(query.ActivityID); activityID != "" {
		// 报名数据的分会归属取自活动，禁止用其他分会的活动 ID 绕过权限。
		act, findErr := s.ActivityMapper.FindById(ctx, activityID)
		if findErr != nil {
			return nil, findErr
		}
		if _, scopeErr := s.requireScope(ctx, act.ChapterID); scopeErr != nil {
			return nil, scopeErr
		}
		scope["activity_id"] = activityID
	} else if session.AdminRole == user.AdminChapter {
		scope["chapter_id"] = session.AdminChapterID
	} else if chapterID := strings.TrimSpace(query.ChapterID); chapterID != "" {
		scope["chapter_id"] = chapterID
	}
	filter := bson.M{"$and": []bson.M{scope}}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		filter["$and"] = append(filter["$and"].([]bson.M), bson.M{"$or": []bson.M{
			{"name": bson.M{"$regex": pattern, "$options": "i"}},
			{"phone": bson.M{"$regex": pattern, "$options": "i"}},
		}})
	}
	if query.CheckIn == "true" || query.CheckIn == "false" {
		filter["check_in"] = query.CheckIn == "true"
	}
	data, total, err := s.RegisterMapper.FindManyByFilter(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	all, _, err := s.RegisterMapper.FindManyByFilter(ctx, scope, 0, 100000)
	if err != nil {
		return nil, err
	}
	checked := int64(0)
	for _, item := range all {
		if item.CheckIn {
			checked++
		}
	}
	resolver := s.newChapterResolver(ctx)
	activityNames := map[string]string{}
	items := make([]AdminRegistration, 0, len(data))
	for _, item := range data {
		name, ok := activityNames[item.ActivityId]
		if !ok {
			if act, findErr := s.ActivityMapper.FindById(ctx, item.ActivityId); findErr == nil {
				name = act.Name
			}
			activityNames[item.ActivityId] = name
		}
		items = append(items, mapAdminRegistration(item, name, resolver.name(item.ChapterID)))
	}
	return &AdminRegistrationPage{Items: items, Total: total, Page: page, PageSize: pageSize, Checked: checked}, nil
}

func (s *AdminService) CreateRegistration(ctx context.Context, input AdminRegistrationInput) (*AdminRegistration, error) {
	act, err := s.ActivityMapper.FindById(ctx, input.ActivityID)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, act.ChapterID); err != nil {
		return nil, err
	}
	phone := normalizePhone(input.Phone)
	now := time.Now()
	item := &register.Register{
		ActivityId: input.ActivityID,
		ChapterID:  act.ChapterID,
		UserId:     input.UserID,
		Name:       strings.TrimSpace(input.Name),
		Phone:      phone,
		CheckIn:    false,
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.RegisterMapper.Insert(ctx, item); err != nil {
		return nil, err
	}
	result := mapAdminRegistration(item, act.Name, s.chapterName(ctx, act.ChapterID))
	return &result, nil
}

func (s *AdminService) UpdateRegistration(ctx context.Context, id string, input AdminRegistrationInput) (*AdminRegistration, error) {
	item, err := s.RegisterMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	act, err := s.ActivityMapper.FindById(ctx, item.ActivityId)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, act.ChapterID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.UserID) != "" {
		item.UserId = input.UserID
	}
	if strings.TrimSpace(input.Name) != "" {
		item.Name = strings.TrimSpace(input.Name)
	}
	item.Phone = normalizePhone(input.Phone)
	if err = s.RegisterMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := mapAdminRegistration(item, act.Name, s.chapterName(ctx, act.ChapterID))
	return &result, nil
}

func (s *AdminService) DeleteRegistration(ctx context.Context, id string) error {
	item, err := s.RegisterMapper.FindByID(ctx, id)
	if err != nil {
		return err
	}
	act, err := s.ActivityMapper.FindById(ctx, item.ActivityId)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, act.ChapterID); err != nil {
		return err
	}
	item.Status = 1
	item.DeleteTime = time.Now()
	return s.RegisterMapper.Update(ctx, item)
}

func (s *AdminService) SetRegistrationCheckIn(ctx context.Context, id string, checked bool) error {
	item, err := s.RegisterMapper.FindByID(ctx, id)
	if err != nil {
		return err
	}
	act, err := s.ActivityMapper.FindById(ctx, item.ActivityId)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, act.ChapterID); err != nil {
		return err
	}
	item.CheckIn = checked
	if checked {
		item.CheckInTime = time.Now()
	} else {
		item.CheckInTime = time.Time{}
	}
	return s.RegisterMapper.Update(ctx, item)
}

func (s *AdminService) ListArticles(ctx context.Context, page, pageSize int64, keyword, status, requestedChapter string) (*PageResult[AdminArticle], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{}
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case session.AdminRole == user.AdminChapter:
		filter["chapter_id"] = session.AdminChapterID
	case strings.TrimSpace(requestedChapter) != "":
		filter["chapter_id"] = strings.TrimSpace(requestedChapter)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": pattern, "$options": "i"}},
			{"source": bson.M{"$regex": pattern, "$options": "i"}},
		}
	}
	if status == "deleted" {
		filter["deleted"] = true
	} else {
		filter["deleted"] = bson.M{"$ne": true}
		if status != "" {
			filter["publish_status"] = status
		}
	}
	data, total, err := s.ArticleMapper.FindMany(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	resolver := s.newChapterResolver(ctx)
	items := make([]AdminArticle, 0, len(data))
	for _, item := range data {
		items = append(items, s.mapAdminArticle(item, resolver.name(item.ChapterID)))
	}
	return &PageResult[AdminArticle]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *AdminService) GetArticle(ctx context.Context, id string) (*AdminArticle, error) {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return nil, err
	}
	result := s.mapAdminArticleOne(ctx, item)
	return &result, nil
}

func (s *AdminService) CreateArticle(ctx context.Context, input AdminArticleInput) (*AdminArticle, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole == user.AdminChapter {
		input.ChapterID = session.AdminChapterID
	}
	if input.ChapterID == "" {
		return nil, ErrAdminBadRequest
	}
	if _, err = s.requireScope(ctx, input.ChapterID); err != nil {
		return nil, err
	}
	item := &article.Article{
		Title:         strings.TrimSpace(input.Title),
		Summary:       strings.TrimSpace(input.Summary),
		Cover:         strings.TrimSpace(input.Cover),
		WechatURL:     strings.TrimSpace(input.WechatURL),
		Source:        strings.TrimSpace(input.Source),
		Author:        strings.TrimSpace(input.Author),
		PublishTime:   pointerUnixToTime(input.PublishTime),
		SortOrder:     input.SortOrder,
		PublishStatus: article.StatusDraft,
		Deleted:       false,
		ChapterID:     input.ChapterID,
		CreatedBy:     session.ID,
		UpdatedBy:     session.ID,
	}
	if err := s.ArticleMapper.Insert(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminArticleOne(ctx, item)
	return &result, nil
}

func (s *AdminService) UpdateArticle(ctx context.Context, id string, input AdminArticleInput) (*AdminArticle, error) {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return nil, err
	}
	item.Title = strings.TrimSpace(input.Title)
	item.Summary = strings.TrimSpace(input.Summary)
	item.Cover = strings.TrimSpace(input.Cover)
	item.WechatURL = strings.TrimSpace(input.WechatURL)
	item.Source = strings.TrimSpace(input.Source)
	item.Author = strings.TrimSpace(input.Author)
	item.PublishTime = pointerUnixToTime(input.PublishTime)
	item.SortOrder = input.SortOrder
	if input.ChapterID != "" {
		if _, err = s.requireScope(ctx, input.ChapterID); err != nil {
			return nil, err
		}
		item.ChapterID = input.ChapterID
	}
	if session, sessionErr := s.GetSession(ctx); sessionErr == nil {
		item.UpdatedBy = session.ID
	}
	if err = s.ArticleMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminArticleOne(ctx, item)
	return &result, nil
}

func (s *AdminService) DeleteArticle(ctx context.Context, id string) error {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Deleted = true
	item.DeleteTime = time.Now()
	return s.ArticleMapper.Update(ctx, item)
}

func (s *AdminService) RestoreArticle(ctx context.Context, id string) error {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Deleted = false
	item.PublishStatus = article.StatusOffline
	item.DeleteTime = time.Time{}
	return s.ArticleMapper.Update(ctx, item)
}

func (s *AdminService) SetArticleStatus(ctx context.Context, id, status string) error {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	if status != article.StatusPublished && status != article.StatusOffline {
		return ErrAdminBadRequest
	}
	item.PublishStatus = status
	if status == article.StatusPublished && item.PublishTime.IsZero() {
		item.PublishTime = time.Now()
	}
	return s.ArticleMapper.Update(ctx, item)
}

func normalizePage(page, pageSize int64) (int64, int64) {
	if page < 1 {
		page = defaultPage
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func offset(page, pageSize int64) int64 {
	return (page - 1) * pageSize
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "-1"
	}
	return phone
}

func parseStatus(status string) int64 {
	if status == "1" {
		return 1
	}
	return 0
}

func unixToTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func pointerUnixToTime(value *int64) time.Time {
	if value == nil || *value <= 0 {
		return time.Time{}
	}
	return time.Unix(*value, 0)
}

func timeToUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

func nullableTimeToUnix(value time.Time) *int64 {
	if value.IsZero() {
		return nil
	}
	ts := value.Unix()
	return &ts
}

func (s *AdminService) mapAdminUserOne(ctx context.Context, item *user.User) AdminUser {
	return s.mapAdminUser(item, s.chapterName(ctx, item.ChapterID))
}

// mapAdminUser 是纯映射，分会名称由调用方解析后传入。
func (s *AdminService) mapAdminUser(item *user.User, chapterName string) AdminUser {
	memberRole := effectiveMemberRole(item)
	educations := item.Educations
	if len(educations) == 0 {
		educations = append(append([]user.Education{}, item.HomeEducations...), item.ShanghaiEducations...)
	}
	return AdminUser{
		ID:                 item.ID.Hex(),
		Avatar:             item.Avatar,
		Name:               item.Name,
		Gender:             item.Gender,
		Birthday:           timeToUnix(item.Birthday),
		Phone:              item.Phone,
		WxID:               item.WxId,
		Hometown:           item.Hometown,
		HomeEducations:     nonNil(item.HomeEducations),
		ShanghaiEducations: nonNil(item.ShanghaiEducations),
		Employments:        nonNil(item.Employments),
		Role:               memberRole,
		ChapterID:          item.ChapterID,
		ChapterName:        chapterName,
		GraduationYear:     item.GraduationYear,
		MemberRole:         memberRole,
		AdminRole:          effectiveAdminRole(item),
		AdminChapterID:     item.AdminChapterID,
		VerificationMethod: item.VerificationMethod,
		Educations:         nonNil(educations),
		Status:             item.Status,
		Deleted:            !item.DeleteTime.IsZero(),
		CreateTime:         timeToUnix(item.CreateTime),
	}
}

func mapAdminRegistration(item *register.Register, activityName, chapterName string) AdminRegistration {
	return AdminRegistration{
		ID:           item.Id.Hex(),
		ActivityID:   item.ActivityId,
		ActivityName: activityName,
		ChapterID:    item.ChapterID,
		ChapterName:  chapterName,
		UserID:       item.UserId,
		Name:         item.Name,
		Phone:        item.Phone,
		CheckIn:      item.CheckIn,
		CheckInTime:  nullableTimeToUnix(item.CheckInTime),
		Deleted:      item.Status == 1 || !item.DeleteTime.IsZero(),
		CreateTime:   timeToUnix(item.CreateTime),
	}
}

// chapterName 解析单个分会名称，用于单条查询。
func (s *AdminService) chapterName(ctx context.Context, id string) string {
	if id == "" || s.ChapterMapper == nil {
		return ""
	}
	if ch, err := s.ChapterMapper.FindByID(ctx, id); err == nil {
		return ch.Name
	}
	return ""
}

func (s *AdminService) mapAdminArticleOne(ctx context.Context, item *article.Article) AdminArticle {
	return s.mapAdminArticle(item, s.chapterName(ctx, item.ChapterID))
}

// mapAdminArticle 是纯映射，分会名称由调用方解析后传入。
func (s *AdminService) mapAdminArticle(item *article.Article, chapterName string) AdminArticle {
	status := item.PublishStatus
	if status == "" {
		status = article.StatusDraft
	}
	return AdminArticle{
		ID:            item.ID.Hex(),
		Title:         item.Title,
		Summary:       item.Summary,
		Cover:         item.Cover,
		WechatURL:     item.WechatURL,
		Source:        item.Source,
		Author:        item.Author,
		PublishTime:   nullableTimeToUnix(item.PublishTime),
		SortOrder:     item.SortOrder,
		PublishStatus: status,
		Deleted:       item.Deleted,
		CreateTime:    timeToUnix(item.CreateTime),
		ChapterID:     item.ChapterID,
		ChapterName:   chapterName,
	}
}
