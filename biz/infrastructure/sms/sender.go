// Package sms 封装 xhpolaris 中台的模板短信能力。
//
// 后端只依赖 Sender 接口，具体实现（中台 HTTP 接口）在 platform.go 中。
// 这样生日任务可以脱离网络进行测试，也方便将来替换短信通道。
package sms

import "context"

// Message 是一条模板短信。IdempotencyKey 由调用方生成并保证唯一，
// 中台需要基于它做幂等，避免重试导致重复发送。
type Message struct {
	// AppID 业务方在中台注册的应用标识。
	AppID int
	// TemplateCode 中台模板编码。
	TemplateCode string
	// Phone 接收手机号。
	Phone string
	// Params 模板参数，例如 {"name": "张三"}。
	Params map[string]string
	// IdempotencyKey 幂等键，同一键中台只应真正发送一次。
	IdempotencyKey string
}

// Result 是中台返回的发送回执。
type Result struct {
	// MessageID 中台消息 ID，用于后台排查。
	MessageID string
	// Status 中台发送状态，成功时为 sent。
	Status string
}

// Sender 发送模板短信。
type Sender interface {
	Send(ctx context.Context, message Message) (Result, error)
}

// StatusSent 表示中台确认短信已提交成功。
const StatusSent = "sent"
