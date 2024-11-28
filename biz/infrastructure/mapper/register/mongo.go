package register

import (
	"context"
	"github.com/xh-polaris/alumni-core_api/biz/application/dto/basic"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	util "github.com/xh-polaris/alumni-core_api/biz/infrastructure/util/page"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

const (
	prefixKeyCacheKey = "cache:register"
	CollectionName    = "register"
)

type IMongoMapper interface {
	Insert(ctx context.Context, r *Register) error
	CheckIn(ctx context.Context, activityId string, phone string, name string) error
	FindMany(ctx context.Context, activityId string, p *basic.PaginationOptions) (registers []*Register, total int64, err error)
	Count(ctx context.Context, activityId string) (count int64, err error)
	FindAll(ctx context.Context, activityId string) (registers []*Register, total int64, err error)
}

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{
		conn: conn,
	}
}

func (m *MongoMapper) Insert(ctx context.Context, r *Register) error {
	if r.Id.IsZero() {
		r.Id = primitive.NewObjectID()
		r.CreateTime = time.Now()
	}
	ket := prefixKeyCacheKey + r.Id.Hex()
	_, err := m.conn.InsertOne(ctx, ket, r)
	return err
}

func (m *MongoMapper) CheckIn(ctx context.Context, activityId string, phone string, name string) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{
		consts.ActivityId: activityId,
		consts.Phone: bson.M{
			"$in": []string{phone, "-1"},
		},
		consts.Name: name,
	}, bson.M{
		"$set": bson.M{
			consts.CheckIn:    true,
			consts.UpdateTime: time.Now(),
		},
	})
	return err
}

func (m *MongoMapper) FindMany(ctx context.Context, activityId string, p *basic.PaginationOptions) (registers []*Register, total int64, err error) {
	skip, limit := util.ParsePageOpt(p)
	registers = make([]*Register, 0, limit)
	err = m.conn.Find(ctx, &registers,
		bson.M{
			consts.ActivityId: activityId,
		}, &options.FindOptions{
			Skip:  &skip,
			Limit: &limit,
			Sort:  bson.M{consts.CreateTime: -1},
		})
	if err != nil {
		return nil, 0, err
	}

	total, err = m.conn.CountDocuments(ctx, bson.M{
		consts.ActivityId: activityId,
	})
	if err != nil {
		return nil, 0, err
	}
	return registers, total, nil
}

func (m *MongoMapper) FindAll(ctx context.Context, activityId string) (registers []*Register, total int64, err error) {
	registers = make([]*Register, 0)
	err = m.conn.Find(ctx, &registers,
		bson.M{
			consts.ActivityId: activityId,
		}, &options.FindOptions{
			Sort: bson.M{consts.CreateTime: -1},
		})
	if err != nil {
		return nil, 0, err
	}

	total, err = m.conn.CountDocuments(ctx, bson.M{
		consts.ActivityId: activityId,
	})
	if err != nil {
		return nil, 0, err
	}
	return registers, total, nil
}

func (m *MongoMapper) Count(ctx context.Context, activityId string) (count int64, err error) {
	count, err = m.conn.CountDocuments(ctx, bson.M{
		consts.ActivityId: activityId,
	})
	return count, err
}
