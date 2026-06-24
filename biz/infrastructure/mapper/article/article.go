package article

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusOffline   = "offline"
)

type Article struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title         string             `bson:"title" json:"title"`
	Summary       string             `bson:"summary" json:"summary"`
	Cover         string             `bson:"cover" json:"cover"`
	WechatURL     string             `bson:"wechat_url" json:"wechatUrl"`
	Source        string             `bson:"source" json:"source"`
	Author        string             `bson:"author" json:"author"`
	PublishTime   time.Time          `bson:"publish_time,omitempty" json:"publishTime"`
	SortOrder     int64              `bson:"sort_order" json:"sortOrder"`
	PublishStatus string             `bson:"publish_status" json:"publishStatus"`
	Deleted       bool               `bson:"deleted" json:"deleted"`
	CreateTime    time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime    time.Time          `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime    time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
