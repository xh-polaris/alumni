package user

import (
	"context"
	"errors"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

const (
	prefixUserCacheKey = "cache:user"
	CollectionName     = "user"
)

type IMongoMapper interface {
	Insert(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	FindOne(ctx context.Context, id string) (*User, error)
	FindOneByPhone(ctx context.Context, phone string) (*User, error)
	FindMany(ctx context.Context, filter bson.M, skip, limit int64) ([]*User, int64, error)
	FindBirthdayCandidates(ctx context.Context) ([]*User, error)
	SoftDeleteByID(ctx context.Context, id primitive.ObjectID) error
	EnsureIndexes(ctx context.Context) error
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

func (m *MongoMapper) Insert(ctx context.Context, user *User) error {
	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
		user.CreateTime = time.Now()
		user.UpdateTime = user.CreateTime
	}
	if user.Role == "" {
		user.Role = "user"
	}
	if user.MemberRole == "" {
		switch user.Role {
		case "alumni", "admin":
			user.MemberRole = MemberAlumni
		case "guest":
			user.MemberRole = MemberGuest
		default:
			user.MemberRole = MemberPending
		}
	}
	if user.AdminRole == "" {
		if user.Role == "admin" {
			user.AdminRole = AdminSuper
		} else {
			user.AdminRole = AdminNone
		}
	}
	if user.VerificationMethod == "" {
		user.VerificationMethod = VerificationNone
	}
	_, err := m.conn.InsertOneNoCache(ctx, user)
	return err
}

// BirthdayCandidatesFilter 是生日短信任务的筛选条件：
// 仅已认证校友、状态正常、未删除、并且留有可用手机号。
// 嘉宾、待认证、停用与已删除用户一律排除。
// 抽成独立函数是为了让这条业务规则可以在不连接 MongoDB 的情况下被测试。
func BirthdayCandidatesFilter() bson.M {
	return bson.M{
		"member_role": MemberAlumni,
		"phone":       bson.M{"$nin": []string{"", "-1"}},
		"$and": []bson.M{
			{"$or": []bson.M{{"status": int64(0)}, {"status": bson.M{"$exists": false}}}},
			{"$or": []bson.M{
				{"delete_time": bson.M{"$exists": false}},
				{"delete_time": time.Time{}},
			}},
		},
	}
}

// FindBirthdayCandidates 按 BirthdayCandidatesFilter 返回候选用户。
func (m *MongoMapper) FindBirthdayCandidates(ctx context.Context) ([]*User, error) {
	items := make([]*User, 0)
	err := m.conn.Find(ctx, &items, BirthdayCandidatesFilter(), &options.FindOptions{})
	return items, err
}

// EnsureIndexes 建立分会、身份与认证字段的查询索引。
func (m *MongoMapper) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "chapter_id", Value: 1}}},
		{Keys: bson.D{{Key: "member_role", Value: 1}}},
		{Keys: bson.D{{Key: "admin_role", Value: 1}}},
		{Keys: bson.D{{Key: "phone", Value: 1}}},
	})
	return err
}

func (m *MongoMapper) Update(ctx context.Context, user *User) error {
	user.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, user.ID, bson.M{"$set": user})
	return err
}

func (m *MongoMapper) FindOne(ctx context.Context, id string) (*User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var u User
	err = m.conn.FindOneNoCache(ctx, &u, bson.M{
		consts.ID: oid,
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &u, nil

}

func (m *MongoMapper) FindOneByPhone(ctx context.Context, phone string) (*User, error) {
	var u User
	err := m.conn.FindOneNoCache(ctx, &u, bson.M{
		consts.Phone: phone,
	})
	switch {
	case err == nil:
		return &u, nil
	case errors.Is(err, monc.ErrNotFound):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

func (m *MongoMapper) SoftDeleteByID(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, id, bson.M{"$set": bson.M{
		"status":      int64(1),
		"delete_time": now,
		"update_time": now,
	}})
	return err
}

func (m *MongoMapper) FindMany(ctx context.Context, filter bson.M, skip, limit int64) ([]*User, int64, error) {
	users := make([]*User, 0, limit)
	err := m.conn.Find(ctx, &users, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &limit,
		Sort:  bson.M{consts.CreateTime: -1},
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
