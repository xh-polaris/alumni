package chapter

import (
	"context"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const CollectionName = "chapter"

type IMongoMapper interface {
	EnsureDefaults(ctx context.Context) error
	List(ctx context.Context, includeDisabled bool) ([]*Chapter, error)
	FindByID(ctx context.Context, id string) (*Chapter, error)
	FindByCode(ctx context.Context, code string) (*Chapter, error)
	Update(ctx context.Context, item *Chapter) error
	EnsureIndexes(ctx context.Context) error
}

type MongoMapper struct{ conn *monc.Model }

func NewMongoMapper(cfg *config.Config) *MongoMapper {
	return &MongoMapper{conn: monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, CollectionName, cfg.Cache)}
}

// EnsureIndexes 保证分会代码唯一，避免重复初始化出多个同名分会。
func (m *MongoMapper) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// FindByCode 按分会代码查询，代码是固定且唯一的业务标识。
func (m *MongoMapper) FindByCode(ctx context.Context, code string) (*Chapter, error) {
	var item Chapter
	if err := m.conn.FindOneNoCache(ctx, &item, bson.M{"code": code}); err != nil {
		return nil, consts.ErrNotFound
	}
	return &item, nil
}

func (m *MongoMapper) EnsureDefaults(ctx context.Context) error {
	defaults := []Chapter{
		{Code: CodeNingbo, Name: "宁波分会"},
		{Code: CodeShanghai, Name: "上海分会"},
		{Code: CodeHangzhou, Name: "杭州分会"},
		{Code: CodeBeijing, Name: "北京分会"},
	}
	for _, item := range defaults {
		count, err := m.conn.CountDocuments(ctx, bson.M{"code": item.Code})
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		now := time.Now()
		item.ID = primitive.NewObjectID()
		item.Status = 0
		item.CreateTime, item.UpdateTime = now, now
		if _, err = m.conn.InsertOneNoCache(ctx, &item); err != nil {
			return err
		}
	}
	return nil
}

func (m *MongoMapper) List(ctx context.Context, includeDisabled bool) ([]*Chapter, error) {
	filter := bson.M{}
	if !includeDisabled {
		filter["status"] = int64(0)
	}
	items := make([]*Chapter, 0, 4)
	err := m.conn.Find(ctx, &items, filter, &options.FindOptions{Sort: bson.D{{Key: "create_time", Value: 1}}})
	return items, err
}

func (m *MongoMapper) FindByID(ctx context.Context, id string) (*Chapter, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var item Chapter
	if err = m.conn.FindOneNoCache(ctx, &item, bson.M{"_id": oid}); err != nil {
		return nil, consts.ErrNotFound
	}
	return &item, nil
}

func (m *MongoMapper) Update(ctx context.Context, item *Chapter) error {
	item.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, item.ID, bson.M{"$set": item})
	return err
}
