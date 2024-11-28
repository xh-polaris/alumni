package service

import (
	"context"
	"github.com/google/wire"
	"github.com/jinzhu/copier"
	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/alumni/core_api"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type IUserService interface {
	SignUp(ctx context.Context, req *core_api.SignUpReq) (*core_api.SignUpResp, error)
	SignIn(ctx context.Context, req *core_api.SignInReq) (*core_api.SignInResp, error)
	UpdateUserInfo(ctx context.Context, req *core_api.UpdateUserInfoReq) (resp *core_api.Response, err error)
	UpdateEducation(ctx context.Context, req *core_api.UpdateEducationReq) (resp *core_api.Response, err error)
	GetUserInfo(ctx context.Context, req *core_api.GetUserInfoReq) (resp *core_api.GetUserInfoResp, err error)
}
type UserService struct {
	UserMapper *user.MongoMapper
}

var UserServiceSet = wire.NewSet(
	wire.Struct(new(UserService), "*"),
	wire.Bind(new(IUserService), new(*UserService)),
)

func (u *UserService) SignUp(ctx context.Context, req *core_api.SignUpReq) (*core_api.SignUpResp, error) {
	// 在中台注册账户
	httpClient := util.NewHttpClient()
	signUpResponse, err := httpClient.SignUp(req.AuthType, req.AuthId, &req.VerifyCode)
	if err != nil {
		return nil, consts.ErrSignUp
	}

	// 在中台设置密码
	authorization := signUpResponse["accessToken"].(string)
	_, err = httpClient.SetPassword(authorization, req.Password)
	if err != nil {
		return nil, consts.ErrSignUp
	}

	// 初始化用户
	userId := signUpResponse["userId"].(string)
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	aUser := user.User{
		ID:         oid,
		Phone:      req.AuthId,
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}

	// 向数据库中插入数据
	err = u.UserMapper.Insert(ctx, &aUser)
	if err != nil {
		return nil, consts.ErrSignUp
	}

	// 返回响应
	return &core_api.SignUpResp{
		Id:           userId,
		AccessToken:  authorization,
		AccessExpire: int64(signUpResponse["accessExpire"].(float64)),
	}, nil
}

func (u *UserService) SignIn(ctx context.Context, req *core_api.SignInReq) (*core_api.SignInResp, error) {
	// 通过中台登录
	httpClient := util.NewHttpClient()
	signInResponse, err := httpClient.SignIn(req.AuthType, req.AuthId, req.VerifyCode, req.Password)
	if err != nil {
		return nil, consts.ErrSignIn
	}

	return &core_api.SignInResp{
		Id:           signInResponse["userId"].(string),
		AccessToken:  signInResponse["accessToken"].(string),
		AccessExpire: int64(signInResponse["accessExpire"].(float64)),
	}, nil
}

func (u *UserService) UpdateUserInfo(ctx context.Context, req *core_api.UpdateUserInfoReq) (resp *core_api.Response, err error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	aUser, err := u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return resp, consts.ErrNotFound
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
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	aUser, err := u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
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
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	aUser, err := u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
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
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	aUser, err := u.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
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
