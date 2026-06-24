package article

import (
	"context"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	prefixKeyCacheKey = "cache:article"
	CollectionName    = "article"
)

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{conn: conn}
}

func (m *MongoMapper) Insert(ctx context.Context, a *Article) error {
	now := time.Now()
	if a.ID.IsZero() {
		a.ID = primitive.NewObjectID()
	}
	if a.PublishStatus == "" {
		a.PublishStatus = StatusDraft
	}
	a.CreateTime = now
	a.UpdateTime = now
	key := prefixKeyCacheKey + a.ID.Hex()
	_, err := m.conn.InsertOne(ctx, key, a)
	return err
}

func (m *MongoMapper) Update(ctx context.Context, a *Article) error {
	a.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, a.ID, bson.M{"$set": a})
	return err
}

func (m *MongoMapper) FindByID(ctx context.Context, id string) (*Article, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var a Article
	err = m.conn.FindOneNoCache(ctx, &a, bson.M{consts.ID: oid})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &a, nil
}

func (m *MongoMapper) FindMany(ctx context.Context, filter bson.M, skip, limit int64) ([]*Article, int64, error) {
	articles := make([]*Article, 0, limit)
	err := m.conn.Find(ctx, &articles, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &limit,
		Sort: bson.D{
			{Key: "sort_order", Value: -1},
			{Key: "publish_time", Value: -1},
			{Key: consts.CreateTime, Value: -1},
		},
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return articles, total, nil
}
