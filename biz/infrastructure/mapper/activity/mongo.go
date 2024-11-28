package activity

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
	prefixKeyCacheKey = "cache:activity"
	CollectionName    = "activity"
)

type IMongoMapper interface {
	Insert(ctx context.Context, a *Activity) error
	Update(ctx context.Context, a *Activity) error
	FindById(ctx context.Context, id string) (*Activity, error)
	FindMany(ctx context.Context, p *basic.PaginationOptions) (activities []*Activity, total int64, err error)
	DeleteById(ctx context.Context, id string) error
}

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{conn: conn}
}

func (m *MongoMapper) Insert(ctx context.Context, a *Activity) error {
	if a.ID.IsZero() {
		a.ID = primitive.NewObjectID()
		a.CreateTime = time.Now()
	}
	key := prefixKeyCacheKey + a.ID.Hex()
	_, err := m.conn.InsertOne(ctx, key, a)
	return err
}

func (m *MongoMapper) Update(ctx context.Context, a *Activity) error {
	a.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, a.ID, bson.M{"$set": a})
	return err
}

func (m *MongoMapper) FindById(ctx context.Context, id string) (*Activity, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var a Activity
	err = m.conn.FindOneNoCache(ctx, &a, bson.M{consts.ID: oid})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &a, nil
}

func (m *MongoMapper) FindMany(ctx context.Context, p *basic.PaginationOptions) (activities []*Activity, total int64, err error) {
	skip, limit := util.ParsePageOpt(p)
	activities = make([]*Activity, 0, limit)
	err = m.conn.Find(ctx, &activities,
		bson.M{
			consts.Status: consts.EffectStatus,
		}, &options.FindOptions{
			Skip:  &skip,
			Limit: &limit,
			Sort:  bson.M{consts.CreateTime: -1},
		})
	if err != nil {
		return nil, 0, err
	}

	total, err = m.conn.CountDocuments(ctx, bson.M{
		consts.Status: consts.EffectStatus,
	})
	if err != nil {
		return nil, 0, err
	}
	return activities, total, nil
}

func (m *MongoMapper) DeleteById(ctx context.Context, id string) error {
	key := prefixKeyCacheKey + id
	now := time.Now()
	_, err := m.conn.UpdateByID(ctx, key, id, bson.M{
		"$set": bson.M{
			consts.Status:     consts.DeleteStatus,
			consts.UpdateTime: now,
			consts.DeleteTime: now,
		},
	})
	return err
}
