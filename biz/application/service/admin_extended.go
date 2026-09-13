package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	activitymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"go.mongodb.org/mongo-driver/bson"
)

type AdminActivity struct {
	ID                string `json:"id"`
	Cover             string `json:"cover"`
	Name              string `json:"name"`
	Location          string `json:"location"`
	ExactLocation     string `json:"exactLocation"`
	Sponsor           string `json:"sponsor"`
	ChapterID         string `json:"chapterId"`
	ChapterName       string `json:"chapterName"`
	Start             int64  `json:"start"`
	RegisterStart     int64  `json:"registerStart"`
	RegisterEnd       int64  `json:"registerEnd"`
	Description       string `json:"description"`
	Contact           string `json:"contact"`
	Limit             int64  `json:"limit"`
	Status            int64  `json:"status"`
	Deleted           bool   `json:"deleted"`
	RegistrationCount int64  `json:"registrationCount"`
	CheckInCount      int64  `json:"checkInCount"`
	CreateTime        int64  `json:"createTime"`
}

type AdminActivityInput struct {
	Cover         string `json:"cover"`
	Name          string `json:"name"`
	Location      string `json:"location"`
	ExactLocation string `json:"exactLocation"`
	Sponsor       string `json:"sponsor"`
	ChapterID     string `json:"chapterId"`
	Start         int64  `json:"start"`
	RegisterStart int64  `json:"registerStart"`
	RegisterEnd   int64  `json:"registerEnd"`
	Description   string `json:"description"`
	Contact       string `json:"contact"`
	Limit         int64  `json:"limit"`
}

func (s *AdminService) ListActivities(ctx context.Context, page, pageSize int64, keyword, status, requestedChapter string) (*PageResult[AdminActivity], error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{}
	if session.AdminRole == user.AdminChapter {
		filter["chapter_id"] = session.AdminChapterID
	} else if requestedChapter != "" {
		filter["chapter_id"] = requestedChapter
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		filter["$or"] = []bson.M{{"name": bson.M{"$regex": pattern, "$options": "i"}}, {"sponsor": bson.M{"$regex": pattern, "$options": "i"}}}
	}
	if status == "deleted" {
		filter["status"] = int64(1)
	} else {
		filter["status"] = int64(0)
	}
	data, total, err := s.ActivityMapper.FindManyByFilter(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	resolver := s.newChapterResolver(ctx)
	items := make([]AdminActivity, 0, len(data))
	for _, item := range data {
		items = append(items, s.mapAdminActivity(ctx, item, resolver.name(item.ChapterID)))
	}
	return &PageResult[AdminActivity]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *AdminService) GetActivity(ctx context.Context, id string) (*AdminActivity, error) {
	item, err := s.ActivityMapper.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return nil, err
	}
	result := s.mapAdminActivityOne(ctx, item)
	return &result, nil
}

func (s *AdminService) CreateActivity(ctx context.Context, input AdminActivityInput) (*AdminActivity, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole == user.AdminChapter {
		input.ChapterID = session.AdminChapterID
	}
	if input.ChapterID == "" || strings.TrimSpace(input.Name) == "" || input.RegisterEnd < input.RegisterStart || input.Start < input.RegisterEnd {
		return nil, ErrAdminBadRequest
	}
	if _, err = s.requireScope(ctx, input.ChapterID); err != nil {
		return nil, err
	}
	item := &activitymodel.Activity{Cover: input.Cover, Name: strings.TrimSpace(input.Name), Location: input.Location, ExactLocation: input.ExactLocation, Sponsor: input.Sponsor, ChapterID: input.ChapterID, Start: input.Start, RegisterStart: time.Unix(input.RegisterStart, 0), RegisterEnd: time.Unix(input.RegisterEnd, 0), Description: input.Description, Contact: input.Contact, Limit: input.Limit, Status: 0, CreatedBy: session.ID, UpdatedBy: session.ID}
	if err = s.ActivityMapper.Insert(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminActivityOne(ctx, item)
	return &result, nil
}

func (s *AdminService) UpdateActivity(ctx context.Context, id string, input AdminActivityInput) (*AdminActivity, error) {
	item, err := s.ActivityMapper.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	session, err := s.requireScope(ctx, item.ChapterID)
	if err != nil {
		return nil, err
	}
	if session.AdminRole == user.AdminChapter {
		input.ChapterID = session.AdminChapterID
	}
	if input.ChapterID == "" || input.RegisterEnd < input.RegisterStart || input.Start < input.RegisterEnd {
		return nil, ErrAdminBadRequest
	}
	if _, err = s.requireScope(ctx, input.ChapterID); err != nil {
		return nil, err
	}
	item.Cover, item.Name, item.Location, item.ExactLocation, item.Sponsor = input.Cover, strings.TrimSpace(input.Name), input.Location, input.ExactLocation, input.Sponsor
	item.ChapterID, item.Start, item.RegisterStart, item.RegisterEnd = input.ChapterID, input.Start, time.Unix(input.RegisterStart, 0), time.Unix(input.RegisterEnd, 0)
	item.Description, item.Contact, item.Limit, item.UpdatedBy = input.Description, input.Contact, input.Limit, session.ID
	if err = s.ActivityMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminActivityOne(ctx, item)
	return &result, nil
}

func (s *AdminService) DeleteActivity(ctx context.Context, id string) error {
	item, err := s.ActivityMapper.FindById(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Status, item.DeleteTime = 1, time.Now()
	return s.ActivityMapper.Update(ctx, item)
}

func (s *AdminService) RestoreActivity(ctx context.Context, id string) error {
	item, err := s.ActivityMapper.FindById(ctx, id)
	if err != nil {
		return err
	}
	if _, err = s.requireScope(ctx, item.ChapterID); err != nil {
		return err
	}
	item.Status, item.DeleteTime = 0, time.Time{}
	return s.ActivityMapper.Update(ctx, item)
}

// mapAdminActivityOne 用于单条查询：先解析分会名称，再做映射。
func (s *AdminService) mapAdminActivityOne(ctx context.Context, item *activitymodel.Activity) AdminActivity {
	return s.mapAdminActivity(ctx, item, s.chapterName(ctx, item.ChapterID))
}

// mapAdminActivity 负责活动到管理端 DTO 的映射。
// 分会名称由调用方一次性解析后传入，列表接口因此不会逐行查询分会集合。
func (s *AdminService) mapAdminActivity(ctx context.Context, item *activitymodel.Activity, chapterName string) AdminActivity {
	registrations, _ := s.RegisterMapper.Count(ctx, item.ID.Hex())
	regs, _, _ := s.RegisterMapper.FindManyByFilter(ctx, bson.M{
		"activity_id": item.ID.Hex(),
		"check_in":    true,
		"$or":         []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}},
	}, 0, 100000)
	return AdminActivity{ID: item.ID.Hex(), Cover: item.Cover, Name: item.Name, Location: item.Location, ExactLocation: item.ExactLocation, Sponsor: item.Sponsor, ChapterID: item.ChapterID, ChapterName: chapterName, Start: item.Start, RegisterStart: item.RegisterStart.Unix(), RegisterEnd: item.RegisterEnd.Unix(), Description: item.Description, Contact: item.Contact, Limit: item.Limit, Status: item.Status, Deleted: !item.DeleteTime.IsZero(), RegistrationCount: registrations, CheckInCount: int64(len(regs)), CreateTime: timeToUnix(item.CreateTime)}
}

func (s *AdminService) ListChapters(ctx context.Context) ([]*chapter.Chapter, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.ChapterMapper.List(ctx, true)
	if err != nil {
		return nil, err
	}
	// 分会管理员只能看到并维护本分会，因此不返回其他分会的联络信息。
	if session.AdminRole == user.AdminChapter {
		filtered := make([]*chapter.Chapter, 0, 1)
		for _, item := range items {
			if item.ID.Hex() == session.AdminChapterID {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	}
	return items, nil
}

func (s *AdminService) UpdateChapterContact(ctx context.Context, id string, contact chapter.Contact) (*chapter.Chapter, error) {
	item, err := s.ChapterMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireScope(ctx, id); err != nil {
		return nil, err
	}
	item.Contact = contact
	if err = s.ChapterMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

type AdminAssignmentInput struct {
	UserID         string `json:"userId"`
	AdminRole      string `json:"adminRole"`
	AdminChapterID string `json:"adminChapterId"`
}

func (s *AdminService) AssignAdmin(ctx context.Context, input AdminAssignmentInput) (*AdminUser, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole != user.AdminSuper {
		return nil, ErrAdminForbidden
	}
	if input.AdminRole != user.AdminNone && input.AdminRole != user.AdminChapter {
		return nil, ErrAdminBadRequest
	}
	if input.AdminRole == user.AdminChapter {
		if _, err = s.ChapterMapper.FindByID(ctx, input.AdminChapterID); err != nil {
			return nil, ErrAdminBadRequest
		}
	} else {
		input.AdminChapterID = ""
	}
	item, err := s.UserMapper.FindOne(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	// 已存在的超级管理员只能由运维直接调整，管理端不可降级或覆盖。
	if effectiveAdminRole(item) == user.AdminSuper {
		return nil, ErrAdminForbidden
	}
	item.AdminRole, item.AdminChapterID = input.AdminRole, input.AdminChapterID
	if err = s.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := s.mapAdminUserOne(ctx, item)
	return &result, nil
}

func (s *AdminService) ListRoster(ctx context.Context, page, pageSize int64) (*PageResult[roster.Entry], error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole != user.AdminSuper {
		return nil, ErrAdminForbidden
	}
	page, pageSize = normalizePage(page, pageSize)
	items, total, err := s.RosterMapper.List(ctx, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	values := make([]roster.Entry, 0, len(items))
	for _, item := range items {
		values = append(values, *item)
	}
	return &PageResult[roster.Entry]{Items: values, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListAdmins 返回当前所有的分会管理员，供超级管理员在“管理员配置”中查看与撤销。
// 超级管理员自身不出现在列表中，避免通过该入口变更全局权限。
func (s *AdminService) ListAdmins(ctx context.Context) ([]AdminUser, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole != user.AdminSuper {
		return nil, ErrAdminForbidden
	}
	items, _, err := s.UserMapper.FindMany(ctx, bson.M{
		"admin_role": user.AdminChapter,
		"$or": []bson.M{
			{"delete_time": bson.M{"$exists": false}},
			{"delete_time": time.Time{}},
		},
	}, 0, maxPageSize*10)
	if err != nil {
		return nil, err
	}
	resolver := s.newChapterResolver(ctx)
	result := make([]AdminUser, 0, len(items))
	for _, item := range items {
		result = append(result, s.mapAdminUser(item, resolver.name(item.ChapterID)))
	}
	return result, nil
}
