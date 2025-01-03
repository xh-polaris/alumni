package consts

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
