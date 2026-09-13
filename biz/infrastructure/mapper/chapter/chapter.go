package chapter

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CodeNingbo   = "ningbo"
	CodeShanghai = "shanghai"
	CodeHangzhou = "hangzhou"
	CodeBeijing  = "beijing"
)

type Contact struct {
	Name        string `bson:"name" json:"name"`
	Wechat      string `bson:"wechat" json:"wechat"`
	Phone       string `bson:"phone" json:"phone"`
	Description string `bson:"description" json:"description"`
	QRCodeURL   string `bson:"qr_code_url" json:"qrCodeUrl"`
}

type Chapter struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code       string             `bson:"code" json:"code"`
	Name       string             `bson:"name" json:"name"`
	Contact    Contact            `bson:"contact" json:"contact"`
	Status     int64              `bson:"status" json:"status"`
	CreateTime time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time,omitempty" json:"updateTime"`
}
