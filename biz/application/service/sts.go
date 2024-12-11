package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/adaptor"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/alumni/core_api"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/rpc/platform_sts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util"
	"github.com/xh-polaris/service-idl-gen-go/kitex_gen/platform/sts"
	"net/http"
)

type IStsService interface {
	ApplySignedUrl(ctx context.Context, req *core_api.ApplySignedUrlReq) (*core_api.ApplySignedUrlResp, error)
	SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*core_api.Response, error)
}

type StsService struct {
	PlatformSts platform_sts.IPlatformSts
	UserMapper  *user.MongoMapper
}

var StsServiceSet = wire.NewSet(
	wire.Struct(new(StsService), "*"),
	wire.Bind(new(IStsService), new(*StsService)),
)

func (s *StsService) ApplySignedUrl(ctx context.Context, req *core_api.ApplySignedUrlReq) (*core_api.ApplySignedUrlResp, error) {
	aUser := adaptor.ExtractUserMeta(ctx)
	if aUser.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	resp := new(core_api.ApplySignedUrlResp)
	userId := aUser.GetUserId()
	data, err := s.PlatformSts.GenCosSts(ctx, &sts.GenCosStsReq{Path: "alumni/" + userId + "/*"})
	if err != nil {
		return nil, err
	}
	resp.SessionToken = data.SessionToken
	if req.Prefix != nil {
		*req.Prefix += "/"
	}
	data2, err := s.PlatformSts.GenSignedUrl(ctx, &sts.GenSignedUrlReq{
		SecretId:  data.SecretId,
		SecretKey: data.SecretKey,
		Method:    http.MethodPut,
		Path:      "alumni/" + userId + "/" + req.GetPrefix() + uuid.New().String() + req.GetSuffix(),
	})
	if err != nil {
		return nil, err
	}
	resp.Url = data2.SignedUrl
	return resp, nil
}

func (s *StsService) SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*core_api.Response, error) {
	aUser, err := s.UserMapper.FindOneByPhone(ctx, req.AuthId)
	if req.Type == 1 { // 登录验证码
		// 查找数据库判断手机号是否注册过
		if errors.Is(err, consts.ErrNotFound) || aUser == nil { // 未找到，说明没有注册
			return nil, consts.ErrNotSignUp
		} else if err != nil {
			return nil, consts.ErrSend
		}
	} else {
		if err == nil && aUser != nil {
			return nil, consts.ErrRepeatedSignUp
		} else if err != nil && !errors.Is(err, consts.ErrNotFound) {
			return nil, consts.ErrSignUp
		}
	}

	// 通过中台发送验证码
	httpClient := util.NewHttpClient()
	_, err = httpClient.SendVerifyCode(req.AuthType, req.AuthId)
	if err != nil {
		return nil, consts.ErrSend
	}

	return &core_api.Response{
		Code: 0,
		Msg:  "发送验证码成功，请注意查收",
	}, nil
}
