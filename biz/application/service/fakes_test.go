package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/application/dto/basic"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	activitymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	articlemodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	birthdaymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/birthday"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	registermodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/sms"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 本文件提供内存版 mapper 与短信发送器，用于在不依赖 MongoDB / Redis /
// 短信中台的前提下覆盖分会权限、认证与生日任务等业务规则。
// 每个 fake 都记录最近一次收到的查询条件，便于断言服务端确实下达了限制。

type fakeUserMapper struct {
	items []*user.User
	// lastFilter 保存最近一次 FindMany 的条件，用于断言分会与身份筛选。
	lastFilter bson.M
}

var _ user.IMongoMapper = (*fakeUserMapper)(nil)

func (f *fakeUserMapper) Insert(_ context.Context, item *user.User) error {
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeUserMapper) Update(_ context.Context, item *user.User) error {
	for index, existing := range f.items {
		if existing.ID == item.ID {
			f.items[index] = item
			return nil
		}
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeUserMapper) FindOne(_ context.Context, id string) (*user.User, error) {
	for _, item := range f.items {
		if item.ID.Hex() == id {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeUserMapper) FindOneByPhone(_ context.Context, phone string) (*user.User, error) {
	for _, item := range f.items {
		if item.Phone == phone {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeUserMapper) FindMany(_ context.Context, filter bson.M, _, _ int64) ([]*user.User, int64, error) {
	f.lastFilter = filter
	result := make([]*user.User, 0, len(f.items))
	for _, item := range f.items {
		if matchesUserFilter(filter, item) {
			result = append(result, item)
		}
	}
	return result, int64(len(result)), nil
}

func (f *fakeUserMapper) FindBirthdayCandidates(_ context.Context) ([]*user.User, error) {
	result := make([]*user.User, 0, len(f.items))
	for _, item := range f.items {
		if matchesUserFilter(user.BirthdayCandidatesFilter(), item) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeUserMapper) SoftDeleteByID(_ context.Context, id primitive.ObjectID) error {
	for _, item := range f.items {
		if item.ID == id {
			item.Status = 1
			item.DeleteTime = time.Now()
		}
	}
	return nil
}

func (f *fakeUserMapper) EnsureIndexes(context.Context) error { return nil }

// matchesUserFilter 实现服务端真正依赖的那部分筛选语义：
// 等值字段、$ne、$exists / 零值、$or、$and 与 $nin。
// 它刻意保持很小，只覆盖测试断言需要的组合。
func matchesUserFilter(filter bson.M, item *user.User) bool {
	document := map[string]any{
		"chapter_id":          item.ChapterID,
		"member_role":         item.MemberRole,
		"admin_role":          item.AdminRole,
		"admin_chapter_id":    item.AdminChapterID,
		"verification_method": item.VerificationMethod,
		"graduation_year":     item.GraduationYear,
		"name":                item.Name,
		"phone":               item.Phone,
		"status":              item.Status,
	}
	// delete_time 只在真正被软删除时存在，这样 $exists 才能表达“未删除”。
	if !item.DeleteTime.IsZero() {
		document["delete_time"] = item.DeleteTime
	}
	return matchesDocument(filter, document)
}

func matchesDocument(filter bson.M, document map[string]any) bool {
	for key, condition := range filter {
		switch key {
		case "$or":
			branches, _ := condition.([]bson.M)
			matched := false
			for _, branch := range branches {
				if matchesDocument(branch, document) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case "$and":
			branches, _ := condition.([]bson.M)
			for _, branch := range branches {
				if !matchesDocument(branch, document) {
					return false
				}
			}
		default:
			actual, present := document[key]
			if !matchesField(actual, present, condition) {
				return false
			}
		}
	}
	return true
}

func matchesField(actual any, present bool, condition any) bool {
	switch expected := condition.(type) {
	case bson.M:
		for operator, operand := range expected {
			switch operator {
			case "$exists":
				want, _ := operand.(bool)
				if want != present {
					return false
				}
			case "$ne":
				if want, ok := operand.(time.Time); ok {
					if current, isTime := actual.(time.Time); isTime && current.Equal(want) {
						return false
					}
				} else if actual == operand {
					return false
				}
			case "$nin":
				values, _ := operand.([]string)
				if current, ok := actual.(string); ok {
					for _, value := range values {
						if current == value {
							return false
						}
					}
				}
			case "$regex", "$options":
				// 关键字模糊搜索不是本组测试的关注点，一律视为匹配。
				continue
			default:
				return false
			}
		}
		return true
	default:
		if !present {
			return false
		}
		return actual == condition
	}
}

type fakeChapterMapper struct {
	items []*chapter.Chapter
}

var _ chapter.IMongoMapper = (*fakeChapterMapper)(nil)

func (f *fakeChapterMapper) EnsureDefaults(context.Context) error { return nil }
func (f *fakeChapterMapper) EnsureIndexes(context.Context) error  { return nil }

func (f *fakeChapterMapper) List(_ context.Context, includeDisabled bool) ([]*chapter.Chapter, error) {
	result := make([]*chapter.Chapter, 0, len(f.items))
	for _, item := range f.items {
		if includeDisabled || item.Status == 0 {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeChapterMapper) FindByID(_ context.Context, id string) (*chapter.Chapter, error) {
	for _, item := range f.items {
		if item.ID.Hex() == id {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeChapterMapper) FindByCode(_ context.Context, code string) (*chapter.Chapter, error) {
	for _, item := range f.items {
		if item.Code == code {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeChapterMapper) Update(_ context.Context, item *chapter.Chapter) error {
	for index, existing := range f.items {
		if existing.ID == item.ID {
			f.items[index] = item
			return nil
		}
	}
	return consts.ErrNotFound
}

type fakeRosterMapper struct {
	items   []roster.Entry
	ensured bool
}

var _ roster.IMongoMapper = (*fakeRosterMapper)(nil)

func (f *fakeRosterMapper) EnsureIndexes(context.Context) error {
	f.ensured = true
	return nil
}

func (f *fakeRosterMapper) Match(_ context.Context, name string, graduationYear int64, birthDate string) (bool, error) {
	matches := 0
	for _, item := range f.items {
		if item.NormalizedName == name && item.GraduationYear == graduationYear && item.BirthDate == birthDate {
			matches++
		}
	}
	return matches == 1, nil
}

func (f *fakeRosterMapper) Upsert(_ context.Context, item roster.Entry) (bool, error) {
	for index, existing := range f.items {
		if existing.NormalizedName == item.NormalizedName &&
			existing.GraduationYear == item.GraduationYear &&
			existing.BirthDate == item.BirthDate {
			f.items[index].Name = item.Name
			return false, nil
		}
	}
	item.ID = primitive.NewObjectID()
	f.items = append(f.items, item)
	return true, nil
}

func (f *fakeRosterMapper) List(_ context.Context, _, _ int64) ([]*roster.Entry, int64, error) {
	result := make([]*roster.Entry, 0, len(f.items))
	for index := range f.items {
		result = append(result, &f.items[index])
	}
	return result, int64(len(result)), nil
}

type fakeBirthdayMapper struct {
	logs []*birthdaymodel.Log
}

var _ birthdaymodel.IMongoMapper = (*fakeBirthdayMapper)(nil)

func (f *fakeBirthdayMapper) EnsureIndexes(context.Context) error { return nil }

func (f *fakeBirthdayMapper) Find(_ context.Context, userID string, year int) (*birthdaymodel.Log, error) {
	for _, item := range f.logs {
		if item.UserID == userID && item.Year == year {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeBirthdayMapper) Save(_ context.Context, item *birthdaymodel.Log) error {
	for index, existing := range f.logs {
		if existing.UserID == item.UserID && existing.Year == item.Year {
			f.logs[index] = item
			return nil
		}
	}
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
	}
	f.logs = append(f.logs, item)
	return nil
}

type fakeActivityMapper struct {
	items []*activitymodel.Activity
}

var _ activitymodel.IMongoMapper = (*fakeActivityMapper)(nil)

func (f *fakeActivityMapper) EnsureIndexes(context.Context) error { return nil }

func (f *fakeActivityMapper) Insert(_ context.Context, item *activitymodel.Activity) error {
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeActivityMapper) Update(_ context.Context, item *activitymodel.Activity) error {
	for index, existing := range f.items {
		if existing.ID == item.ID {
			f.items[index] = item
			return nil
		}
	}
	return consts.ErrNotFound
}

func (f *fakeActivityMapper) FindById(_ context.Context, id string) (*activitymodel.Activity, error) {
	for _, item := range f.items {
		if item.ID.Hex() == id {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeActivityMapper) FindMany(_ context.Context, _ *basic.PaginationOptions) ([]*activitymodel.Activity, int64, error) {
	return f.items, int64(len(f.items)), nil
}

func (f *fakeActivityMapper) FindManyByFilter(_ context.Context, filter bson.M, _, _ int64) ([]*activitymodel.Activity, int64, error) {
	result := make([]*activitymodel.Activity, 0, len(f.items))
	for _, item := range f.items {
		if chapterMatches(filter, item.ChapterID) && statusMatches(filter, item.Status) {
			result = append(result, item)
		}
	}
	return result, int64(len(result)), nil
}

func (f *fakeActivityMapper) DeleteById(context.Context, string) error { return nil }

type fakeArticleMapper struct {
	items []*articlemodel.Article
}

var _ articlemodel.IMongoMapper = (*fakeArticleMapper)(nil)

func (f *fakeArticleMapper) EnsureIndexes(context.Context) error { return nil }

func (f *fakeArticleMapper) Insert(_ context.Context, item *articlemodel.Article) error {
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeArticleMapper) Update(_ context.Context, item *articlemodel.Article) error {
	for index, existing := range f.items {
		if existing.ID == item.ID {
			f.items[index] = item
			return nil
		}
	}
	return consts.ErrNotFound
}

func (f *fakeArticleMapper) FindByID(_ context.Context, id string) (*articlemodel.Article, error) {
	for _, item := range f.items {
		if item.ID.Hex() == id {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeArticleMapper) FindMany(_ context.Context, filter bson.M, _, _ int64) ([]*articlemodel.Article, int64, error) {
	result := make([]*articlemodel.Article, 0, len(f.items))
	for _, item := range f.items {
		if chapterMatches(filter, item.ChapterID) {
			result = append(result, item)
		}
	}
	return result, int64(len(result)), nil
}

type fakeRegisterMapper struct {
	items []*registermodel.Register
}

var _ registermodel.IMongoMapper = (*fakeRegisterMapper)(nil)

func (f *fakeRegisterMapper) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRegisterMapper) Insert(_ context.Context, item *registermodel.Register) error {
	if item.Id.IsZero() {
		item.Id = primitive.NewObjectID()
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeRegisterMapper) Update(_ context.Context, item *registermodel.Register) error {
	for index, existing := range f.items {
		if existing.Id == item.Id {
			f.items[index] = item
			return nil
		}
	}
	return consts.ErrNotFound
}

func (f *fakeRegisterMapper) FindByID(_ context.Context, id string) (*registermodel.Register, error) {
	for _, item := range f.items {
		if item.Id.Hex() == id {
			return item, nil
		}
	}
	return nil, consts.ErrNotFound
}

func (f *fakeRegisterMapper) CheckIn(context.Context, string, string, string) error { return nil }

func (f *fakeRegisterMapper) FindMany(context.Context, string, *basic.PaginationOptions) ([]*registermodel.Register, int64, error) {
	return f.items, int64(len(f.items)), nil
}

func (f *fakeRegisterMapper) FindManyByFilter(_ context.Context, filter bson.M, _, _ int64) ([]*registermodel.Register, int64, error) {
	result := make([]*registermodel.Register, 0, len(f.items))
	for _, item := range f.items {
		if chapterMatches(filter, item.ChapterID) && activityMatches(filter, item.ActivityId) {
			result = append(result, item)
		}
	}
	return result, int64(len(result)), nil
}

func (f *fakeRegisterMapper) Count(_ context.Context, activityID string) (int64, error) {
	count := int64(0)
	for _, item := range f.items {
		if item.ActivityId == activityID {
			count++
		}
	}
	return count, nil
}

func (f *fakeRegisterMapper) FindAll(context.Context, string) ([]*registermodel.Register, int64, error) {
	return f.items, int64(len(f.items)), nil
}

func (f *fakeRegisterMapper) FindByAidAndUid(context.Context, string, string) ([]*registermodel.Register, int64, error) {
	return f.items, int64(len(f.items)), nil
}

// chapterMatches 只实现本测试需要的 chapter_id 语义。
func chapterMatches(filter bson.M, chapterID string) bool {
	if nested, ok := filter["$and"].([]bson.M); ok {
		for _, branch := range nested {
			if !chapterMatches(branch, chapterID) {
				return false
			}
		}
	}
	condition, ok := filter["chapter_id"]
	if !ok {
		return true
	}
	expected, ok := condition.(string)
	return ok && expected == chapterID
}

func statusMatches(filter bson.M, status int64) bool {
	if nested, ok := filter["$and"].([]bson.M); ok {
		for _, branch := range nested {
			if !statusMatches(branch, status) {
				return false
			}
		}
	}
	condition, ok := filter["status"]
	if !ok {
		return true
	}
	expected, ok := condition.(int64)
	return ok && expected == status
}

func activityMatches(filter bson.M, activityID string) bool {
	if nested, ok := filter["$and"].([]bson.M); ok {
		for _, branch := range nested {
			if !activityMatches(branch, activityID) {
				return false
			}
		}
	}
	condition, ok := filter["activity_id"]
	if !ok {
		return true
	}
	expected, ok := condition.(string)
	return ok && expected == activityID
}

// fakeSender 记录所有发送请求，用于断言幂等与重试行为。
type fakeSender struct {
	sent    []sms.Message
	failFor int
}

var _ sms.Sender = (*fakeSender)(nil)

func (f *fakeSender) Send(_ context.Context, message sms.Message) (sms.Result, error) {
	f.sent = append(f.sent, message)
	if f.failFor > 0 {
		f.failFor--
		return sms.Result{}, errFakeSend
	}
	return sms.Result{MessageID: "msg-" + message.Phone, Status: sms.StatusSent}, nil
}

type fakeSendError struct{}

func (fakeSendError) Error() string { return "短信中台发送失败" }

var errFakeSend = fakeSendError{}

// newTestAdminService 组装一个完全内存化的管理服务。
func newTestAdminService(users []*user.User, chapters []*chapter.Chapter, activities []*activitymodel.Activity) (*AdminService, *fakeUserMapper, *fakeRosterMapper) {
	userMapper := &fakeUserMapper{items: users}
	chapterMapper := &fakeChapterMapper{items: chapters}
	activityMapper := &fakeActivityMapper{items: activities}
	service := &AdminService{
		UserMapper:     userMapper,
		ChapterMapper:  chapterMapper,
		ActivityMapper: activityMapper,
		RegisterMapper: &fakeRegisterMapper{},
		ArticleMapper:  &fakeArticleMapper{},
		RosterMapper:   &fakeRosterMapper{},
	}
	return service, userMapper, service.RosterMapper.(*fakeRosterMapper)
}

// sortedHexes 便于对结果做顺序无关断言。
func sortedHexes(items []*user.User) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.ID.Hex())
	}
	sort.Strings(values)
	return values
}

// chapterFixture 构造分会的固定测试数据。
func chapterFixture(codes ...string) []*chapter.Chapter {
	items := make([]*chapter.Chapter, 0, len(codes))
	for _, code := range codes {
		items = append(items, &chapter.Chapter{
			ID:     primitive.NewObjectID(),
			Code:   code,
			Name:   strings.ToUpper(code[:1]) + code[1:] + "分会",
			Status: 0,
		})
	}
	return items
}
