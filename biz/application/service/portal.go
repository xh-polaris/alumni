package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	activitymodel "github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/text/unicode/norm"
)

type RegisterProfileInput struct {
	AuthID         string `json:"authId"`
	AuthType       string `json:"authType"`
	VerifyCode     string `json:"verifyCode"`
	Password       string `json:"password"`
	Name           string `json:"name"`
	GraduationYear int64  `json:"graduationYear"`
	BirthDate      string `json:"birthDate"`
	ChapterID      string `json:"chapterId"`
}

type ChapterContact struct {
	ID      string          `json:"id"`
	Code    string          `json:"code"`
	Name    string          `json:"name"`
	Contact chapter.Contact `json:"contact"`
}

type RegisterProfileResult struct {
	ID             string         `json:"id"`
	AccessToken    string         `json:"accessToken"`
	AccessExpire   int64          `json:"accessExpire"`
	MemberRole     string         `json:"memberRole"`
	AutoVerified   bool           `json:"autoVerified"`
	ChapterContact ChapterContact `json:"chapterContact"`
}

type Profile struct {
	ID                 string            `json:"id"`
	Avatar             string            `json:"avatar"`
	Name               string            `json:"name"`
	Gender             int64             `json:"gender"`
	Birthday           int64             `json:"birthday"`
	BirthDate          string            `json:"birthDate"`
	Phone              string            `json:"phone"`
	WxID               string            `json:"wxId"`
	Hometown           string            `json:"hometown"`
	ChapterID          string            `json:"chapterId"`
	ChapterName        string            `json:"chapterName"`
	GraduationYear     int64             `json:"graduationYear"`
	MemberRole         string            `json:"memberRole"`
	VerificationMethod string            `json:"verificationMethod"`
	ProfileComplete    bool              `json:"profileComplete"`
	Educations         []user.Education  `json:"educations"`
	Employments        []user.Employment `json:"employments"`
}

// HasRequiredProfile 判断用户是否已补齐分会与认证所需资料。
// 旧客户端注册的用户缺少这些字段，用户端据此强制补全。
func HasRequiredProfile(item *user.User) bool {
	return item.ChapterID != "" && item.GraduationYear > 0 && item.BirthDate != ""
}

type ProfileUpdate struct {
	Avatar         *string `json:"avatar"`
	Name           *string `json:"name"`
	Gender         *int64  `json:"gender"`
	Phone          *string `json:"phone"`
	WxID           *string `json:"wxId"`
	Hometown       *string `json:"hometown"`
	ChapterID      *string `json:"chapterId"`
	GraduationYear *int64  `json:"graduationYear"`
	BirthDate      *string `json:"birthDate"`
}

// allowedEducationPhases 是新增教育经历允许的学段。
// 旧数据里的中小学记录仍然允许原样提交，只要它与库中已有记录完全一致。
var allowedEducationPhases = map[string]bool{
	"大专": true, "本科": true, "硕士": true, "博士": true, "其他": true,
}

// educationKey 用学段+学校+毕业年份标识一条教育经历。
func educationKey(item user.Education) string {
	return fmt.Sprintf("%s|%s|%d", item.Phase, item.School, item.Year)
}

// validateEducationPhases 只允许新增高等教育阶段，同时放行已有的旧记录。
func validateEducationPhases(existing, incoming []user.Education) error {
	legacy := make(map[string]bool, len(existing))
	for _, item := range existing {
		legacy[educationKey(item)] = true
	}
	for _, item := range incoming {
		if allowedEducationPhases[item.Phase] || legacy[educationKey(item)] {
			continue
		}
		return badRequest("新增教育经历只支持大专、本科、硕士、博士及其他")
	}
	return nil
}

func normalizeRosterName(value string) string {
	return norm.NFC.String(strings.TrimSpace(value))
}

func parseBirthDate(value string) (time.Time, error) {
	if len(value) != len("2006-01-02") {
		return time.Time{}, ErrAdminBadRequest
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.FixedZone("Asia/Shanghai", 8*60*60))
	if err != nil {
		return time.Time{}, ErrAdminBadRequest
	}
	return parsed, nil
}

func effectiveMemberRole(item *user.User) string {
	if item.MemberRole != "" {
		return item.MemberRole
	}
	switch item.Role {
	case "alumni", "admin":
		return user.MemberAlumni
	case "guest":
		return user.MemberGuest
	default:
		return user.MemberPending
	}
}

func effectiveAdminRole(item *user.User) string {
	if item.AdminRole != "" {
		return item.AdminRole
	}
	if item.Role == "admin" {
		return user.AdminSuper
	}
	return user.AdminNone
}

func (u *UserService) MigrateLegacy(ctx context.Context) error {
	items, _, err := u.UserMapper.FindMany(ctx, bson.M{}, 0, 100000)
	if err != nil {
		return err
	}
	for _, item := range items {
		changed := false
		if item.MemberRole == "" {
			item.MemberRole = effectiveMemberRole(item)
			changed = true
		}
		if item.AdminRole == "" {
			item.AdminRole = effectiveAdminRole(item)
			changed = true
		}
		if item.VerificationMethod == "" {
			item.VerificationMethod = user.VerificationNone
			changed = true
		}
		if len(item.Educations) == 0 && (len(item.HomeEducations) > 0 || len(item.ShanghaiEducations) > 0) {
			item.Educations = append([]user.Education{}, item.HomeEducations...)
			for _, education := range item.ShanghaiEducations {
				if education.ProvinceName == "" {
					education.ProvinceCode, education.ProvinceName, education.CityCode, education.CityName = "310000", "上海市", "310100", "上海市"
				}
				item.Educations = append(item.Educations, education)
			}
			changed = true
		}
		if changed {
			if err = u.UserMapper.Update(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *UserService) RegisterProfile(ctx context.Context, input RegisterProfileInput) (*RegisterProfileResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.AuthID = strings.TrimSpace(input.AuthID)
	if input.AuthType != "phone" || input.AuthID == "" || input.Password == "" || input.Name == "" {
		return nil, badRequest("请完整填写手机号、验证码、密码与姓名")
	}
	if input.GraduationYear < 1900 {
		return nil, badRequest("请选择四位毕业年份")
	}
	birth, err := parseBirthDate(input.BirthDate)
	if err != nil {
		return nil, badRequest("出生日期必须为 YYYY-MM-DD")
	}
	ch, err := u.ChapterMapper.FindByID(ctx, input.ChapterID)
	if err != nil || ch.Status != 0 {
		return nil, badRequest("请选择有效的所属分会")
	}

	httpClient := util.NewHttpClient()
	signUpResponse, err := httpClient.SignUp(input.AuthType, input.AuthID, &input.VerifyCode)
	if err != nil {
		return nil, consts.ErrSignUp
	}
	authorization, ok := signUpResponse["accessToken"].(string)
	if !ok || authorization == "" {
		return nil, consts.ErrSignUp
	}
	if _, err = httpClient.SetPassword(authorization, input.Password); err != nil {
		return nil, consts.ErrSignUp
	}
	userID, ok := signUpResponse["userId"].(string)
	if !ok || userID == "" {
		return nil, consts.ErrSignUp
	}
	if err = u.ensureLocalUser(ctx, userID, input.AuthID, input.Name); err != nil {
		return nil, consts.ErrSignUp
	}
	item, err := u.UserMapper.FindOne(ctx, userID)
	if err != nil {
		return nil, err
	}
	matched, err := u.RosterMapper.Match(ctx, normalizeRosterName(input.Name), input.GraduationYear, input.BirthDate)
	if err != nil {
		return nil, err
	}
	item.Name = input.Name
	item.Phone = input.AuthID
	item.Birthday = birth
	item.BirthDate = input.BirthDate
	item.GraduationYear = input.GraduationYear
	item.ChapterID = input.ChapterID
	item.AdminRole = effectiveAdminRole(item)
	item.MemberRole = user.MemberPending
	item.Role = user.MemberPending
	item.VerificationMethod = user.VerificationNone
	item.VerifiedAt = time.Time{}
	if matched {
		item.MemberRole = user.MemberAlumni
		item.Role = user.MemberAlumni
		item.VerificationMethod = user.VerificationRoster
		item.VerifiedAt = time.Now()
	}
	if err = u.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	expire, _ := signUpResponse["accessExpire"].(float64)
	return &RegisterProfileResult{
		ID: userID, AccessToken: authorization, AccessExpire: int64(expire), MemberRole: item.MemberRole, AutoVerified: matched,
		ChapterContact: ChapterContact{ID: ch.ID.Hex(), Code: ch.Code, Name: ch.Name, Contact: ch.Contact},
	}, nil
}

func (u *UserService) GetProfile(ctx context.Context) (*Profile, error) {
	item, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	return u.mapProfile(ctx, item), nil
}

func (u *UserService) mapProfile(ctx context.Context, item *user.User) *Profile {
	educations := item.Educations
	if len(educations) == 0 {
		educations = append(append([]user.Education{}, item.HomeEducations...), item.ShanghaiEducations...)
	}
	chapterName := ""
	if item.ChapterID != "" {
		if ch, err := u.ChapterMapper.FindByID(ctx, item.ChapterID); err == nil {
			chapterName = ch.Name
		}
	}
	birthday := int64(0)
	if !item.Birthday.IsZero() {
		birthday = item.Birthday.Unix()
	}
	return &Profile{
		ID: item.ID.Hex(), Avatar: item.Avatar, Name: item.Name, Gender: item.Gender, Birthday: birthday,
		BirthDate: item.BirthDate, Phone: item.Phone, WxID: item.WxId, Hometown: item.Hometown,
		ChapterID: item.ChapterID, ChapterName: chapterName, GraduationYear: item.GraduationYear,
		MemberRole: effectiveMemberRole(item), VerificationMethod: item.VerificationMethod,
		ProfileComplete: HasRequiredProfile(item),
		Educations:      nonNil(educations), Employments: nonNil(item.Employments),
	}
}

func (u *UserService) UpdateProfile(ctx context.Context, input ProfileUpdate) (*Profile, error) {
	item, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	if input.ChapterID != nil {
		ch, findErr := u.ChapterMapper.FindByID(ctx, *input.ChapterID)
		if findErr != nil || ch.Status != 0 {
			return nil, badRequest("请选择有效的所属分会")
		}
		item.ChapterID = *input.ChapterID
	}
	if input.Avatar != nil {
		item.Avatar = strings.TrimSpace(*input.Avatar)
	}
	if input.Name != nil {
		item.Name = strings.TrimSpace(*input.Name)
	}
	if input.Gender != nil {
		item.Gender = *input.Gender
	}
	if input.Phone != nil {
		item.Phone = strings.TrimSpace(*input.Phone)
	}
	if input.WxID != nil {
		item.WxId = strings.TrimSpace(*input.WxID)
	}
	if input.Hometown != nil {
		item.Hometown = strings.TrimSpace(*input.Hometown)
	}
	if input.GraduationYear != nil {
		if *input.GraduationYear < 1900 {
			return nil, badRequest("请选择四位毕业年份")
		}
		item.GraduationYear = *input.GraduationYear
	}
	if input.BirthDate != nil {
		value := strings.TrimSpace(*input.BirthDate)
		if value != "" {
			birth, parseErr := parseBirthDate(value)
			if parseErr != nil {
				return nil, badRequest("出生日期必须为 YYYY-MM-DD")
			}
			item.BirthDate = value
			item.Birthday = birth
		}
	}
	// 补全资料时重新做一次名册匹配，避免用户因为一次填错而永久停留在待认证。
	if err = u.applyRosterVerification(ctx, item); err != nil {
		return nil, err
	}
	if err = u.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	return u.mapProfile(ctx, item), nil
}

// applyRosterVerification 在用户补齐姓名、毕业年份与出生日期后重新核验名册。
// 只有仍处于待认证的用户才会被自动升级，已认证身份不会被覆盖。
func (u *UserService) applyRosterVerification(ctx context.Context, item *user.User) error {
	if effectiveMemberRole(item) != user.MemberPending {
		return nil
	}
	if item.Name == "" || item.GraduationYear <= 0 || item.BirthDate == "" {
		return nil
	}
	matched, err := u.RosterMapper.Match(ctx, normalizeRosterName(item.Name), item.GraduationYear, item.BirthDate)
	if err != nil || !matched {
		return err
	}
	item.MemberRole = user.MemberAlumni
	item.Role = user.MemberAlumni
	item.VerificationMethod = user.VerificationRoster
	item.VerifiedAt = time.Now()
	return nil
}

func (u *UserService) ReplaceEducations(ctx context.Context, values []user.Education) (*Profile, error) {
	item, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	if err = validateEducationPhases(item.Educations, values); err != nil {
		return nil, err
	}
	item.Educations = values
	if err = u.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	return u.mapProfile(ctx, item), nil
}

func (u *UserService) ReplaceEmployments(ctx context.Context, values []user.Employment) (*Profile, error) {
	item, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	item.Employments = values
	if err = u.UserMapper.Update(ctx, item); err != nil {
		return nil, err
	}
	return u.mapProfile(ctx, item), nil
}

type PublicActivity struct {
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
	RegistrationCount int64  `json:"registrationCount"`
}

func (s *ActivityService) ListPublic(ctx context.Context, chapterID string, page, pageSize int64) (*PageResult[PublicActivity], error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := bson.M{"status": int64(0)}
	if chapterID != "" {
		filter["chapter_id"] = chapterID
	}
	data, total, err := s.ActivityMapper.FindManyByFilter(ctx, filter, offset(page, pageSize), pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]PublicActivity, 0, len(data))
	for _, item := range data {
		// 列表也需要报名人数，用户端据此展示剩余名额。
		items = append(items, s.mapPublicActivity(ctx, item, true))
	}
	return &PageResult[PublicActivity]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *ActivityService) GetPublic(ctx context.Context, id string) (*PublicActivity, error) {
	item, err := s.ActivityMapper.FindById(ctx, id)
	if err != nil || item.Status != 0 {
		return nil, consts.ErrNotFound
	}
	result := s.mapPublicActivity(ctx, item, true)
	return &result, nil
}

func (s *ActivityService) mapPublicActivity(ctx context.Context, item *activitymodel.Activity, includeCount bool) PublicActivity {
	chapterName := ""
	if item.ChapterID != "" {
		if ch, err := s.ChapterMapper.FindByID(ctx, item.ChapterID); err == nil {
			chapterName = ch.Name
		}
	}
	count := int64(0)
	if includeCount {
		count, _ = s.RegisterMapper.Count(ctx, item.ID.Hex())
	}
	return PublicActivity{
		ID: item.ID.Hex(), Cover: item.Cover, Name: item.Name, Location: item.Location,
		ExactLocation: item.ExactLocation, Sponsor: item.Sponsor, ChapterID: item.ChapterID,
		ChapterName: chapterName, Start: item.Start, RegisterStart: item.RegisterStart.Unix(),
		RegisterEnd: item.RegisterEnd.Unix(), Description: item.Description, Contact: item.Contact,
		Limit: item.Limit, Status: item.Status, RegistrationCount: count,
	}
}
