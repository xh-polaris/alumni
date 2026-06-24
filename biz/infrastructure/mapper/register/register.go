package register

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Register struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id" `
	ActivityId  string             `bson:"activity_id" json:"activityId" `
	UserId      string             `bson:"user_id" json:"userId" `
	Name        string             `bson:"name" json:"name" `
	Phone       string             `bson:"phone" json:"phone" `
	CheckIn     bool               `bson:"check_in" json:"checkIn" `
	CheckInTime time.Time          `bson:"check_in_time,omitempty" json:"checkInTime"`
	Status      int64              `bson:"status" json:"status"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime" `
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime" `
	DeleteTime  time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
