package birthday

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

const CollectionName = "birthday_sms_log"

type IMongoMapper interface {
	Find(ctx context.Context, userID string, year int) (*Log, error)
	Save(ctx context.Context, item *Log) error
	EnsureIndexes(ctx context.Context) error
}

type MongoMapper struct{ conn *monc.Model }

func NewMongoMapper(cfg *config.Config) *MongoMapper {
	return &MongoMapper{conn: monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, CollectionName, cfg.Cache)}
}

// EnsureIndexes 建立 (用户 ID, 年份) 唯一索引，保证一年只发送一次。
func (m *MongoMapper) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "year", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Find 返回某个用户某一年度的发送记录；不存在时返回 consts.ErrNotFound。
func (m *MongoMapper) Find(ctx context.Context, userID string, year int) (*Log, error) {
	var item Log
	if err := m.conn.FindOneNoCache(ctx, &item, bson.M{"user_id": userID, "year": year}); err != nil {
		return nil, consts.ErrNotFound
	}
	return &item, nil
}

func (m *MongoMapper) Save(ctx context.Context, item *Log) error {
	now := time.Now()
	item.UpdateTime = now
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
		item.CreateTime = now
		_, err := m.conn.InsertOneNoCache(ctx, item)
		return err
	}
	_, err := m.conn.UpdateByIDNoCache(ctx, item.ID, bson.M{"$set": item})
	return err
}
