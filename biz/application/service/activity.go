package service

import (
	"context"
	"github.com/google/wire"
	"github.com/jinzhu/copier"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/alumni/core_api"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/essay-show/biz/adaptor"
	"strings"
	"time"
)

type IActivityService interface {
	CreateActivity(ctx context.Context, req *core_api.CreateActivityReq) (resp *core_api.Response, err error)
	UpdateActivity(ctx context.Context, req *core_api.UpdateActivityReq) (resp *core_api.Response, err error)
	GetActivities(ctx context.Context, req *core_api.GetActivitiesReq) (resp *core_api.GetActivitiesResp, err error)
	GetActivity(ctx context.Context, req *core_api.GetActivityReq) (resp *core_api.GetActivityResp, err error)
	RegisterActivity(ctx context.Context, req *core_api.RegisterActivityReq) (resp *core_api.Response, err error)
	CheckInActivity(ctx context.Context, req *core_api.CheckInReq) (resp *core_api.Response, err error)
}
type ActivityService struct {
	ActivityMapper *activity.MongoMapper
	RegisterMapper *register.MongoMapper
}

var ActivityServiceSet = wire.NewSet(
	wire.Struct(new(ActivityService), "*"),
	wire.Bind(new(IActivityService), new(*ActivityService)),
)

func (s *ActivityService) CreateActivity(ctx context.Context, req *core_api.CreateActivityReq) (resp *core_api.Response, err error) {
	limit := int64(-1)
	if req.Limit != nil {
		limit = *req.Limit
	}
	now := time.Now()
	a := activity.Activity{
		Cover:         req.Cover,
		Name:          req.Name,
		Location:      req.Location,
		ExactLocation: req.ExactLocation,
		Sponsor:       req.Sponsor,
		Start:         req.Start,
		Description:   req.Description,
		RegisterStart: time.Unix(req.RegisterStart, 0),
		RegisterEnd:   time.Unix(req.RegisterEnd, 0),
		Contact:       req.Contact,
		Limit:         limit,
		Status:        0,
		CreateTime:    now,
		UpdateTime:    now,
	}
	err = s.ActivityMapper.Insert(ctx, &a)
	if err != nil {
		return nil, consts.ErrCreate
	}
	resp = &core_api.Response{
		Code: 0,
		Msg:  "创建成功",
	}
	return resp, nil
}

func (s *ActivityService) UpdateActivity(ctx context.Context, req *core_api.UpdateActivityReq) (resp *core_api.Response, err error) {
	a, err := s.ActivityMapper.FindById(ctx, req.GetId())
	if err != nil {
		return nil, consts.ErrNotFound
	}
	id := req.GetId()
	if req.Cover != nil {
		a.Cover = *req.Cover
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Location != nil {
		a.Location = *req.Location
	}
	if req.ExactLocation != nil {
		a.ExactLocation = *req.ExactLocation
	}
	if req.Sponsor != nil {
		a.Sponsor = *req.Sponsor
	}
	if req.Start != nil {
		a.Start = *req.Start
	}
	if req.RegisterStart != nil {
		a.RegisterStart = time.Unix(*req.RegisterStart, 0)
	}
	if req.RegisterEnd != nil {
		a.RegisterEnd = time.Unix(*req.RegisterEnd, 0)
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.Contact != nil {
		a.Contact = *req.Contact
	}
	if req.Limit != nil {
		a.Limit = *req.Limit
	}
	if req.Status != nil {
		a.Status = *req.Status
	}
	err = s.ActivityMapper.UpdateById(ctx, id, a)
	if err != nil {
		return nil, consts.ErrUpdate
	}
	resp = &core_api.Response{
		Code: 0,
		Msg:  "更新成功",
	}
	return resp, nil
}

func (s *ActivityService) GetActivities(ctx context.Context, req *core_api.GetActivitiesReq) (resp *core_api.GetActivitiesResp, err error) {
	data, total, err := s.ActivityMapper.FindMany(ctx, req.PaginationOptions)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	var activities []*core_api.Activity
	for _, act := range data {
		a := &core_api.Activity{}
		err2 := copier.Copy(a, act)
		if err2 != nil {
			return nil, consts.ErrCopier
		}
		a.Id = act.ID.Hex()
		activities = append(activities, a)
	}

	resp = &core_api.GetActivitiesResp{
		Total:      total,
		Activities: activities,
	}
	return resp, nil
}

func (s *ActivityService) GetActivity(ctx context.Context, req *core_api.GetActivityReq) (resp *core_api.GetActivityResp, err error) {
	act, err := s.ActivityMapper.FindById(ctx, req.GetId())
	if err != nil {
		return nil, consts.ErrNotFound
	}
	var a *core_api.Activity
	err = copier.Copy(a, act)
	if err != nil {
		return nil, consts.ErrCopier
	}
	a.Id = act.ID.Hex()

	resp = &core_api.GetActivityResp{
		Activity: a,
	}
	return resp, nil
}

func (s *ActivityService) RegisterActivity(ctx context.Context, req *core_api.RegisterActivityReq) (resp *core_api.Response, err error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userId := userMeta.GetUserId()
	activityId := req.ActivityId

	failed := make([]string, 0)

	for _, item := range req.Items {
		name := item.Name
		phone := item.Phone
		r := &register.Register{
			ActivityId: activityId,
			UserId:     userId,
			Name:       name,
			Phone:      phone,
			CheckIn:    false,
		}
		err2 := s.RegisterMapper.Insert(ctx, r)
		if err2 != nil {
			failed = append(failed, name)
		}
	}
	resp = &core_api.Response{
		Code: 0,
		Msg:  "报名成功",
	}
	if len(failed) > 0 {
		resp = &core_api.Response{
			Code: 1003,
			Msg:  "以下报名失败:" + strings.Join(failed, ","),
		}
	}
	return resp, nil
}

func (s *ActivityService) CheckInActivity(ctx context.Context, req *core_api.CheckInReq) (resp *core_api.Response, err error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	userId := userMeta.GetUserId()
	activityId := req.ActivityId
	phone := req.Phone

	err = s.RegisterMapper.CheckIn(ctx, activityId, userId, phone)
	if err != nil {
		return nil, consts.ErrCheckIn
	}
	resp = &core_api.Response{
		Code: 0,
		Msg:  "签到成功",
	}
	return resp, nil

}
