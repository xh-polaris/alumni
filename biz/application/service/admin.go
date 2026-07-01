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
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
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

type AdminService struct {
	UserMapper     *user.MongoMapper
	RegisterMapper *register.MongoMapper
	ArticleMapper  *article.MongoMapper
}

var AdminServiceSet = wire.NewSet(
	wire.Struct(new(AdminService), "*"),
)

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"pageSize"`
}

type AdminSession struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
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
}

type AdminRegistration struct {
	ID          string `json:"id"`
	ActivityID  string `json:"activityId"`
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CheckIn     bool   `json:"checkIn"`
	CheckInTime *int64 `json:"checkInTime"`
	Deleted     bool   `json:"deleted"`
	CreateTime  int64  `json:"createTime"`
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
}

func (s *AdminService) GetSession(ctx context.Context) (*AdminSession, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, ErrAdminUnauthorized
	}

	if adaptor.IsDevModeRequest(ctx) && userMeta.GetUserId() == appconsts.DevMockUserID {
		return &AdminSession{
			ID:     appconsts.DevMockUserID,
			Name:   "演示管理员",
			Avatar: "",
			Phone:  "13800000000",
			Role:   "admin",
		}, nil
	}

	item, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, ErrAdminUnauthorized
	}
	if item.Role != "admin" || item.Status != 0 || !item.DeleteTime.IsZero() {
		return nil, ErrAdminForbidden
	}

	return &AdminSession{
		ID:     item.ID.Hex(),
		Name:   item.Name,
		Avatar: item.Avatar,
		Phone:  item.Phone,
		Role:   "admin",
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, page, pageSize int64, keyword, role, status string) (*PageResult[AdminUser], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{}
	andFilters := make([]bson.M, 0)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		andFilters = append(andFilters, bson.M{"$or": []bson.M{
			{"name": bson.M{"$regex": pattern, "$options": "i"}},
			{"phone": bson.M{"$regex": pattern, "$options": "i"}},
		}})
	}
	if role = strings.TrimSpace(role); role != "" {
		filter["role"] = role
	}
	switch status {
	case "deleted":
		filter["delete_time"] = bson.M{"$exists": true, "$ne": time.Time{}}
	case "0", "1":
		filter["status"] = parseStatus(status)
		if status != "1" {
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
	items := make([]AdminUser, 0, len(data))
	for _, item := range data {
		items = append(items, mapAdminUser(item))
	}
	return &PageResult[AdminUser]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *AdminService) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	result := mapAdminUser(item)
	return &result, nil
}

func (s *AdminService) UpdateUser(ctx context.Context, id string, input AdminUserUpdate) (*AdminUser, error) {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
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
	if err = s.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := mapAdminUser(item)
	return &result, nil
}

func (s *AdminService) SetUserRole(ctx context.Context, id, role string) error {
	if !validUserRole(role) {
		return ErrAdminBadRequest
	}
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	item.Role = role
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) SetUserStatus(ctx context.Context, id string, status int64) error {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
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
	item.Status = 1
	item.DeleteTime = time.Now()
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) RestoreUser(ctx context.Context, id string) error {
	item, err := s.UserMapper.FindOne(ctx, id)
	if err != nil {
		return err
	}
	item.Status = 0
	item.DeleteTime = time.Time{}
	return s.UserMapper.Update(ctx, item)
}

func (s *AdminService) ListRegistrations(ctx context.Context, page, pageSize int64, activityID, keyword, checkIn string) (*AdminRegistrationPage, error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{
		"activity_id": activityID,
		"$or":         []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}},
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := regexp.QuoteMeta(keyword)
		filter["$and"] = []bson.M{{
			"$or": []bson.M{
				{"name": bson.M{"$regex": pattern, "$options": "i"}},
				{"phone": bson.M{"$regex": pattern, "$options": "i"}},
			},
		}}
	}
	if checkIn == "true" || checkIn == "false" {
		filter["check_in"] = checkIn == "true"
	}
	data, total, err := s.RegisterMapper.FindManyByFilter(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	all, _, err := s.RegisterMapper.FindManyByFilter(ctx, bson.M{
		"activity_id": activityID,
		"$or":         []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}},
	}, 0, 100000)
	if err != nil {
		return nil, err
	}
	checked := int64(0)
	for _, item := range all {
		if item.CheckIn {
			checked++
		}
	}
	items := make([]AdminRegistration, 0, len(data))
	for _, item := range data {
		items = append(items, mapAdminRegistration(item))
	}
	return &AdminRegistrationPage{Items: items, Total: total, Page: page, PageSize: pageSize, Checked: checked}, nil
}

func (s *AdminService) CreateRegistration(ctx context.Context, input AdminRegistrationInput) (*AdminRegistration, error) {
	phone := normalizePhone(input.Phone)
	now := time.Now()
	item := &register.Register{
		ActivityId: input.ActivityID,
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
	result := mapAdminRegistration(item)
	return &result, nil
}

func (s *AdminService) UpdateRegistration(ctx context.Context, id string, input AdminRegistrationInput) (*AdminRegistration, error) {
	item, err := s.RegisterMapper.FindByID(ctx, id)
	if err != nil {
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
	result := mapAdminRegistration(item)
	return &result, nil
}

func (s *AdminService) DeleteRegistration(ctx context.Context, id string) error {
	item, err := s.RegisterMapper.FindByID(ctx, id)
	if err != nil {
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
	item.CheckIn = checked
	if checked {
		item.CheckInTime = time.Now()
	} else {
		item.CheckInTime = time.Time{}
	}
	return s.RegisterMapper.Update(ctx, item)
}

func (s *AdminService) ListArticles(ctx context.Context, page, pageSize int64, keyword, status string) (*PageResult[AdminArticle], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{}
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
	items := make([]AdminArticle, 0, len(data))
	for _, item := range data {
		items = append(items, mapAdminArticle(item))
	}
	return &PageResult[AdminArticle]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *AdminService) GetArticle(ctx context.Context, id string) (*AdminArticle, error) {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	result := mapAdminArticle(item)
	return &result, nil
}

func (s *AdminService) CreateArticle(ctx context.Context, input AdminArticleInput) (*AdminArticle, error) {
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
	}
	if err := s.ArticleMapper.Insert(ctx, item); err != nil {
		return nil, err
	}
	result := mapAdminArticle(item)
	return &result, nil
}

func (s *AdminService) UpdateArticle(ctx context.Context, id string, input AdminArticleInput) (*AdminArticle, error) {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
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
	if err = s.ArticleMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	result := mapAdminArticle(item)
	return &result, nil
}

func (s *AdminService) DeleteArticle(ctx context.Context, id string) error {
	item, err := s.ArticleMapper.FindByID(ctx, id)
	if err != nil {
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

func validUserRole(role string) bool {
	switch role {
	case "admin", "guest", "alumni", "user":
		return true
	default:
		return false
	}
}

func mapAdminUser(item *user.User) AdminUser {
	role := item.Role
	if role == "" {
		role = "user"
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
		HomeEducations:     item.HomeEducations,
		ShanghaiEducations: item.ShanghaiEducations,
		Employments:        item.Employments,
		Role:               role,
		Status:             item.Status,
		Deleted:            !item.DeleteTime.IsZero(),
		CreateTime:         timeToUnix(item.CreateTime),
	}
}

func mapAdminRegistration(item *register.Register) AdminRegistration {
	return AdminRegistration{
		ID:          item.Id.Hex(),
		ActivityID:  item.ActivityId,
		UserID:      item.UserId,
		Name:        item.Name,
		Phone:       item.Phone,
		CheckIn:     item.CheckIn,
		CheckInTime: nullableTimeToUnix(item.CheckInTime),
		Deleted:     item.Status == 1 || !item.DeleteTime.IsZero(),
		CreateTime:  timeToUnix(item.CreateTime),
	}
}

func mapAdminArticle(item *article.Article) AdminArticle {
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
	}
}
