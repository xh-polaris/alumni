package birthday

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Log struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     string             `bson:"user_id" json:"userId"`
	Year       int                `bson:"year" json:"year"`
	Status     string             `bson:"status" json:"status"`
	Attempts   int                `bson:"attempts" json:"attempts"`
	ProviderID string             `bson:"provider_id" json:"providerId"`
	LastError  string             `bson:"last_error" json:"lastError"`
	CreateTime time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time" json:"updateTime"`
}
