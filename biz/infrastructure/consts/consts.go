package consts

import "errors"

var PageSize int64 = 10

// 数据库相关
const (
	ID                        = "_id"
	UserID                    = "user_id"
	Status                    = "status"
	PlatformSendVerifyCodeUrl = "https://api.xhpolaris.com/platform/auth/send_verify_code"
	CreateTime                = "create_time"
	UpdateTime                = "update_time"
	DeleteTime                = "delete_time"
	ActivityId                = "activity_id"
	CheckIn                   = "check_in"
	Phone                     = "phone"
	Name                      = "name"
	DeleteStatus              = 1
	EffectStatus              = 0
)

// http
const (
	Post                   = "POST"
	PlatformSignInUrl      = "https://api.xhpolaris.com/platform/auth/sign_in"
	PlatformSetPasswordUrl = "https://api.xhpolaris.com/platform/auth/set_password"
	ContentTypeJson        = "application/json"
	CharSetUTF8            = "UTF-8"
	Beta                   = "beta"
	OpenApiCallUrl         = "https://api.xhpolaris.com/openapi/call/"
)

// 默认值
const (
	DefaultCount = 10
	AppId        = 15
)

// 上传体积相关上限（字节）。
// 路由层的 MaxRequestBodySize 必须不小于这里的最大值，否则 hertz 会在进入
// handler 之前就拒绝请求，前端只能看到一个没有上下文的错误。
const (
	// MaxRosterImportBytes 名册导入文件上限
	MaxRosterImportBytes = 10 << 20
	// requestBodyMargin 给 multipart 边界与其它表单字段留的余量
	requestBodyMargin = 1 << 20
)

// MaxRequestBodyBytes 返回路由层应配置的请求体上限。
func MaxRequestBodyBytes(uploadMaxBytes int64) int {
	limit := int64(MaxRosterImportBytes)
	if uploadMaxBytes > limit {
		limit = uploadMaxBytes
	}
	return int(limit) + requestBodyMargin
}

// dev mock auth
const (
	DevMockAccessToken = "mock-token"
	DevMockUserID      = "66a000000000000000000001"
	DevModeHeader      = "X-Alumni-Mode"
	DevModeValue       = "dev"
)

var ErrWxPhoneExchange = errors.New("微信手机号换取失败")
