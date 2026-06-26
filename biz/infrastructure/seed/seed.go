package seed

import (
	"context"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const devMockUserPhone = "DEMO_USER"

func EnsureDevData(cfg *config.Config) {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, user.CollectionName, cfg.Cache)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	count, err := conn.CountDocuments(ctx, bson.M{"phone": devMockUserPhone})
	if err != nil || count > 0 {
		return
	}

	now := time.Now()
	newUser := &user.User{
		ID:         primitive.NewObjectID(),
		Name:       "演示用户",
		Phone:      devMockUserPhone,
		Role:       "admin",
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}

	_, _ = conn.InsertOneNoCache(ctx, newUser)
}
