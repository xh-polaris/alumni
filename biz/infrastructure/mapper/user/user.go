package user

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type User struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Avatar             string             `bson:"avatar" json:"avatar"`
	Name               string             `bson:"name" json:"name"`
	Gender             int64              `bson:"gender" json:"gender"`
	Birthday           time.Time          `bson:"birthday" json:"birthday"`
	Phone              string             `bson:"phone" json:"phone"`
	WxId               string             `bson:"wx_id" json:"wxId"`
	Hometown           string             `bson:"hometown" json:"hometown"`
	HomeEducations     []Education        `bson:"home_educations" json:"homeEducations"`
	ShanghaiEducations []Education        `bson:"shanghai_educations" json:"shanghaiEducations"`
	Employments        []Employment       `bson:"employments" json:"employments"`
	Status             int64              `bson:"status" json:"status"`
	CreateTime         time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime         time.Time          `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime         time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}

type Education struct {
	Phase  string `bson:"phase" json:"phase"`
	School string `bson:"school" json:"school"`
	Year   int64  `bson:"year" json:"year"`
}

type Employment struct {
	Organization string `bson:"organization" json:"organization"`
	Position     string `bson:"position" json:"position"`
	Industry     string `bson:"industry" json:"industry"`
	Entry        int64  `bson:"entry" json:"entry"`
	Departure    int64  `bson:"departure" json:"departure"`
}
