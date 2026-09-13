package user

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	MemberPending = "pending"
	MemberAlumni  = "alumni"
	MemberGuest   = "guest"

	AdminNone    = "none"
	AdminChapter = "chapter_admin"
	AdminSuper   = "super_admin"

	VerificationNone   = "none"
	VerificationRoster = "roster"
	VerificationManual = "manual"
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
	ChapterID          string             `bson:"chapter_id" json:"chapterId"`
	GraduationYear     int64              `bson:"graduation_year" json:"graduationYear"`
	BirthDate          string             `bson:"birth_date" json:"birthDate"`
	MemberRole         string             `bson:"member_role" json:"memberRole"`
	AdminRole          string             `bson:"admin_role" json:"adminRole"`
	AdminChapterID     string             `bson:"admin_chapter_id" json:"adminChapterId"`
	VerificationMethod string             `bson:"verification_method" json:"verificationMethod"`
	VerifiedAt         time.Time          `bson:"verified_at,omitempty" json:"verifiedAt"`
	VerifiedBy         string             `bson:"verified_by" json:"verifiedBy"`
	Educations         []Education        `bson:"educations" json:"educations"`
	HomeEducations     []Education        `bson:"home_educations" json:"homeEducations"`
	ShanghaiEducations []Education        `bson:"shanghai_educations" json:"shanghaiEducations"`
	Employments        []Employment       `bson:"employments" json:"employments"`
	Role               string             `bson:"role" json:"role"`
	Status             int64              `bson:"status" json:"status"`
	CreateTime         time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime         time.Time          `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime         time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}

type Education struct {
	Phase        string `bson:"phase" json:"phase"`
	School       string `bson:"school" json:"school"`
	Year         int64  `bson:"year" json:"year"`
	ProvinceCode string `bson:"province_code" json:"provinceCode"`
	ProvinceName string `bson:"province_name" json:"provinceName"`
	CityCode     string `bson:"city_code" json:"cityCode"`
	CityName     string `bson:"city_name" json:"cityName"`
}

type Employment struct {
	Organization string `bson:"organization" json:"organization"`
	Position     string `bson:"position" json:"position"`
	Industry     string `bson:"industry" json:"industry"`
	Entry        int64  `bson:"entry" json:"entry"`
	Departure    int64  `bson:"departure" json:"departure"`
	ProvinceCode string `bson:"province_code" json:"provinceCode"`
	ProvinceName string `bson:"province_name" json:"provinceName"`
	CityCode     string `bson:"city_code" json:"cityCode"`
	CityName     string `bson:"city_name" json:"cityName"`
}
