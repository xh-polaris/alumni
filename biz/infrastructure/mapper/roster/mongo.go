package roster

import (
	"context"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const CollectionName = "alumni_roster"

type IMongoMapper interface {
	Match(ctx context.Context, name string, graduationYear int64, birthDate string) (bool, error)
	Upsert(ctx context.Context, item Entry) (created bool, err error)
	List(ctx context.Context, skip, limit int64) ([]*Entry, int64, error)
	EnsureIndexes(ctx context.Context) error
}

type MongoMapper struct{ conn *monc.Model }

func NewMongoMapper(cfg *config.Config) *MongoMapper {
	return &MongoMapper{conn: monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, CollectionName, cfg.Cache)}
}

// EnsureIndexes 建立 (规范化姓名, 毕业年份, 出生日期) 唯一复合索引。
// 唯一约束是自动认证的安全底线：同一三项组合出现两条记录时必须让导入失败，
// 而不是让注册接口随机命中其中一条。
func (m *MongoMapper) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "normalized_name", Value: 1},
			{Key: "graduation_year", Value: 1},
			{Key: "birth_date", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (m *MongoMapper) Match(ctx context.Context, name string, graduationYear int64, birthDate string) (bool, error) {
	count, err := m.conn.CountDocuments(ctx, bson.M{
		"normalized_name": name,
		"graduation_year": graduationYear,
		"birth_date":      birthDate,
	})
	return count == 1, err
}

func (m *MongoMapper) Upsert(ctx context.Context, item Entry) (created bool, err error) {
	filter := bson.M{
		"normalized_name": item.NormalizedName,
		"graduation_year": item.GraduationYear,
		"birth_date":      item.BirthDate,
	}
	count, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	now := time.Now()
	item.UpdateTime = now
	if count == 0 {
		item.ID = primitive.NewObjectID()
		item.CreateTime = now
		_, err = m.conn.InsertOneNoCache(ctx, &item)
		return true, err
	}
	_, err = m.conn.UpdateOneNoCache(ctx, filter, bson.M{"$set": bson.M{
		"name": item.Name, "update_time": now,
	}})
	return false, err
}

func (m *MongoMapper) List(ctx context.Context, skip, limit int64) ([]*Entry, int64, error) {
	items := make([]*Entry, 0, limit)
	err := m.conn.Find(ctx, &items, bson.M{}, &options.FindOptions{
		Skip: &skip, Limit: &limit, Sort: bson.D{{Key: "graduation_year", Value: -1}, {Key: "name", Value: 1}},
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := m.conn.CountDocuments(ctx, bson.M{})
	return items, total, err
}
