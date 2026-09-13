package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 数组字段一旦序列化成 null，前端按数组处理（.map / .length）就会整页崩。
// 这条曾经真实发生过：库里没有 employments 字段时 Go 零值是 nil → JSON null
// → 管理端用户详情页白屏。这里从 JSON 层面锁住「永远是数组」。
func TestAdminUserArraysAreNeverNull(t *testing.T) {
	chapters := chapterFixture(chapter.CodeShanghai)
	service, _, _ := newTestAdminService(nil, chapters, nil)

	// 完全不设置 Educations / Employments 等切片字段
	item := &user.User{ID: primitive.NewObjectID(), Name: "空数据用户", Status: 0}
	result := service.mapAdminUser(item, "")

	if result.Educations == nil || result.Employments == nil ||
		result.HomeEducations == nil || result.ShanghaiEducations == nil {
		t.Fatalf("切片字段不能为 nil: %+v", result)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, field := range []string{"educations", "employments", "homeEducations", "shanghaiEducations"} {
		if strings.Contains(string(data), `"`+field+`":null`) {
			t.Fatalf("字段 %s 不应序列化为 null: %s", field, data)
		}
	}
}

// 用户端档案接口同样必须返回数组（契约 2.3）
func TestProfileArraysAreNeverNull(t *testing.T) {
	service := &UserService{
		UserMapper:    &fakeUserMapper{},
		ChapterMapper: &fakeChapterMapper{items: []*chapter.Chapter{}},
		RosterMapper:  &fakeRosterMapper{},
	}
	item := &user.User{ID: primitive.NewObjectID(), Name: "空数据用户"}
	result := service.mapProfile(context.Background(), item)

	if result.Educations == nil || result.Employments == nil {
		t.Fatalf("切片字段不能为 nil: %+v", result)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, field := range []string{"educations", "employments"} {
		if strings.Contains(string(data), `"`+field+`":null`) {
			t.Fatalf("字段 %s 不应序列化为 null: %s", field, data)
		}
	}
}
