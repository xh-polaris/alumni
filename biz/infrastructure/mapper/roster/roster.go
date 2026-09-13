package roster

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Entry struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	NormalizedName string             `bson:"normalized_name" json:"normalizedName"`
	Name           string             `bson:"name" json:"name"`
	GraduationYear int64              `bson:"graduation_year" json:"graduationYear"`
	BirthDate      string             `bson:"birth_date" json:"birthDate"`
	CreateTime     time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime     time.Time          `bson:"update_time,omitempty" json:"updateTime"`
}
