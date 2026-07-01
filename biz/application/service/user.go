package service

import (
	"context"
	"strings"

	"github.com/google/wire"
	"github.com/jinzhu/copier"
	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/alumni/core_api"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type IUserService interface {
	SignUp(ctx context.Context, req *core_api.SignUpReq) (*core_api.SignUpResp, error)
	SignIn(ctx context.Context, req *core_api.SignInReq) (*core_api.SignInResp, error)
	UpdateUserInfo(ctx context.Context, req *core_api.UpdateUserInfoReq) (resp *core_api.Response, err error)
	UpdateEducation(ctx context.Context, req *core_api.UpdateEducationReq) (resp *core_api.Response, err error)
	GetUserInfo(ctx context.Context, req *core_api.GetUserInfoReq) (resp *core_api.GetUserInfoResp, err error)
	ExchangeWxPhone(ctx context.Context, code string) (*core_api.ExchangeWxPhoneResp, error)
}
type UserService struct {
	UserMapper *user.MongoMapper
}

var UserServiceSet = wire.NewSet(
	wire.Struct(new(UserService), "*"),
	wire.Bind(new(IUserService), new(*UserService)),
)

func (u *UserService) findAuthenticatedUser(ctx context.Context) (*user.User, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	aUser, err := u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err == nil {
		return aUser, nil
	}
	if adaptor.IsDevModeRequest(ctx) && userMeta.GetUserId() == consts.DevMockUserID && err == consts.ErrNotFound {
		if createErr := u.createLocalUser(ctx, consts.DevMockUserID, "13800000000", "开发测试用户"); createErr != nil {
			return nil, createErr
		}
		return u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	}
	return nil, err
}

func (u *UserService) ensureLocalUser(ctx context.Context, platformUserID, phone, name string) error {
	phone = strings.TrimSpace(phone)
	name = strings.TrimSpace(name)

	if existing, err := u.UserMapper.FindOne(ctx, platformUserID); err == nil {
		changed := false
		if phone != "" && existing.Phone != phone {
			existing.Phone = phone
			changed = true
		}
		if name != "" && existing.Name == "" {
			existing.Name = name
			changed = true
		}
		if existing.Role == "" {
			existing.Role = "user"
			changed = true
		}
		if changed {
			return u.UserMapper.Update(ctx, existing)
		}
		return nil
	} else if err != consts.ErrNotFound {
		return err
	}

	existing, err := u.UserMapper.FindOneByPhone(ctx, phone)
	if err == nil {
		return u.linkUserToPlatformID(ctx, existing, platformUserID, name)
	}
	if err != consts.ErrNotFound {
		return err
	}

	return u.createLocalUser(ctx, platformUserID, phone, name)
}

func (u *UserService) createLocalUser(ctx context.Context, platformUserID, phone, name string) error {
	oid, err := primitive.ObjectIDFromHex(platformUserID)
	if err != nil {
		return consts.ErrSignIn
	}
	now := time.Now()
	aUser := user.User{
		ID:         oid,
		Name:       name,
		Phone:      phone,
		Role:       "user",
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}
	return u.UserMapper.Insert(ctx, &aUser)
}

func (u *UserService) linkUserToPlatformID(ctx context.Context, existing *user.User, platformUserID, name string) error {
	oid, err := primitive.ObjectIDFromHex(platformUserID)
	if err != nil {
		return consts.ErrSignIn
	}
	if existing.ID == oid {
		return nil
	}

	migrated := *existing
	migrated.ID = oid
	if strings.TrimSpace(name) != "" && migrated.Name == "" {
		migrated.Name = strings.TrimSpace(name)
	}
	if migrated.Role == "" {
		migrated.Role = "user"
	}
	migrated.Status = 0
	migrated.DeleteTime = time.Time{}
	migrated.UpdateTime = time.Now()
	if err := u.UserMapper.Insert(ctx, &migrated); err != nil {
		return err
	}
	return u.UserMapper.SoftDeleteByID(ctx, existing.ID)
}

func (u *UserService) SignUp(ctx context.Context, req *core_api.SignUpReq) (*core_api.SignUpResp, error) {
	if req.AuthType != "phone" || strings.TrimSpace(req.AuthId) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, consts.ErrSignUp
	}

	httpClient := util.NewHttpClient()
	signUpResponse, err := httpClient.SignUp(req.AuthType, strings.TrimSpace(req.AuthId), &req.VerifyCode)
	if err != nil {
		return nil, consts.ErrSignUp
	}

	authorization := signUpResponse["accessToken"].(string)
	_, err = httpClient.SetPassword(authorization, req.Password)
	if err != nil {
		return nil, consts.ErrSignUp
	}

	userId := signUpResponse["userId"].(string)
	if err := u.ensureLocalUser(ctx, userId, req.AuthId, req.Name); err != nil {
		return nil, consts.ErrSignUp
	}

	return &core_api.SignUpResp{
		Id:           userId,
		AccessToken:  authorization,
		AccessExpire: int64(signUpResponse["accessExpire"].(float64)),
	}, nil
}

func (u *UserService) SignIn(ctx context.Context, req *core_api.SignInReq) (*core_api.SignInResp, error) {
	if adaptor.IsDevModeRequest(ctx) {
		if err := u.ensureLocalUser(ctx, consts.DevMockUserID, "13800000000", "开发测试用户"); err != nil {
			return nil, consts.ErrSignIn
		}
		return &core_api.SignInResp{
			Id:           consts.DevMockUserID,
			AccessToken:  consts.DevMockAccessToken,
			AccessExpire: time.Now().Add(24 * time.Hour).Unix(),
		}, nil
	}

	if req.AuthType != "phone" || strings.TrimSpace(req.AuthId) == "" {
		return nil, consts.ErrSignIn
	}

	httpClient := util.NewHttpClient()
	signInResponse, err := httpClient.SignIn(req.AuthType, strings.TrimSpace(req.AuthId), req.VerifyCode, req.Password)
	if err != nil {
		return nil, consts.ErrSignIn
	}

	userId := signInResponse["userId"].(string)
	if err := u.ensureLocalUser(ctx, userId, req.AuthId, ""); err != nil {
		return nil, consts.ErrSignIn
	}

	return &core_api.SignInResp{
		Id:           userId,
		AccessToken:  signInResponse["accessToken"].(string),
		AccessExpire: int64(signInResponse["accessExpire"].(float64)),
	}, nil
}

func (u *UserService) UpdateUserInfo(ctx context.Context, req *core_api.UpdateUserInfoReq) (resp *core_api.Response, err error) {
	aUser, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Phone != nil {
		aUser.Phone = *req.Phone
		req.GetPhone()
	}
	if req.Avatar != nil {
		aUser.Avatar = *req.Avatar
	}
	if req.Name != nil {
		aUser.Name = *req.Name
	}
	if req.Birthday != nil {
		aUser.Birthday = time.Unix(*req.Birthday, 0)
	}
	if req.Gender != nil {
		aUser.Gender = *req.Gender
	}
	if req.WxId != nil {
		aUser.WxId = *req.WxId
	}
	if req.Hometown != nil {
		aUser.Hometown = *req.Hometown
	}

	err = u.UserMapper.Update(ctx, aUser)
	if err != nil {
		return nil, consts.ErrUpdate
	}
	return &core_api.Response{
		Code: 0,
		Msg:  "更新成功",
	}, nil
}

func (u *UserService) UpdateEducation(ctx context.Context, req *core_api.UpdateEducationReq) (resp *core_api.Response, err error) {
	aUser, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	educations := make([]user.Education, 0)
	for _, edu := range req.Educations {
		var e user.Education
		err2 := copier.Copy(&e, &edu)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		educations = append(educations, e)
	}

	switch req.Type {
	case 0:
		aUser.HomeEducations = educations
	case 1:
		aUser.ShanghaiEducations = educations
	}

	err = u.UserMapper.Update(ctx, aUser)
	if err != nil {
		return nil, consts.ErrUpdate
	}

	return &core_api.Response{
		Code: 0,
		Msg:  "更新成功",
	}, nil
}

func (u *UserService) UpdateEmployment(ctx context.Context, req *core_api.UpdateEmploymentReq) (resp *core_api.Response, err error) {
	aUser, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	employments := make([]user.Employment, 0)
	for _, em := range req.Employments {
		var e user.Employment
		err2 := copier.Copy(&e, &em)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		employments = append(employments, e)
	}

	aUser.Employments = employments
	err = u.UserMapper.Update(ctx, aUser)
	if err != nil {
		return nil, consts.ErrUpdate
	}

	return &core_api.Response{
		Code: 0,
		Msg:  "更新成功",
	}, nil
}

func (u *UserService) GetUserInfo(ctx context.Context, req *core_api.GetUserInfoReq) (resp *core_api.GetUserInfoResp, err error) {
	aUser, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	homeEducations := make([]*core_api.Education, 0)
	for _, edu := range aUser.HomeEducations {
		var e core_api.Education
		err2 := copier.Copy(&e, &edu)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		homeEducations = append(homeEducations, &e)
	}

	shanghaiEducations := make([]*core_api.Education, 0)
	for _, edu := range aUser.ShanghaiEducations {
		var e core_api.Education
		err2 := copier.Copy(&e, &edu)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		shanghaiEducations = append(shanghaiEducations, &e)
	}

	employments := make([]*core_api.Employment, 0)
	for _, em := range aUser.Employments {
		var e core_api.Employment
		err2 := copier.Copy(&e, &em)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		employments = append(employments, &e)
	}

	resp = &core_api.GetUserInfoResp{
		Avatar:             aUser.Avatar,
		Name:               aUser.Name,
		Gender:             aUser.Gender,
		Birthday:           aUser.Birthday.Unix(),
		Phone:              aUser.Phone,
		WxId:               aUser.WxId,
		Hometown:           aUser.Hometown,
		HometownEducations: homeEducations,
		ShanghaiEducations: shanghaiEducations,
		Employments:        employments,
	}
	return resp, nil
}

func (u *UserService) ExchangeWxPhone(ctx context.Context, code string) (*core_api.ExchangeWxPhoneResp, error) {
	_, err := u.findAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	wxClient := util.NewWxClient()
	phoneNumber, err := wxClient.GetPhoneNumber(code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", consts.ErrWxPhoneExchange, err)
	}

	return &core_api.ExchangeWxPhoneResp{
		PhoneNumber: phoneNumber,
	}, nil
}
