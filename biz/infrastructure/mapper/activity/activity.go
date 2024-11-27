package activity

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Activity struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Cover         string             `bson:"cover" json:"cover"`
	Name          string             `bson:"name" json:"name"`
	Location      string             `bson:"location" json:"location"`
	ExactLocation string             `bson:"exact_location" json:"exactLocation"`
	Sponsor       string             `bson:"sponsor" json:"sponsor"`
	Start         int64              `bson:"start" json:"start"`
	Description   string             `bson:"description" json:"description"`
	RegisterStart time.Time          `bson:"register_start" json:"registerStart"`
	RegisterEnd   time.Time          `bson:"register_end" json:"registerEnd"`
	Contact       string             `bson:"contact" json:"contact"`
	Limit         int64              `bson:"limit" json:"limit"`
	Status        int64              `bson:"status" json:"status"`
	CreateTime    time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime    time.Time          `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime    time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
