package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
)

// PlatformSender 通过 xhpolaris 中台的模板短信接口发送短信。
//
// 中台接口约定：
//
//	POST {Endpoint}
//	Authorization: {APIKey}
//	{"appId":15,"phone":"138...","templateCode":"...","params":{"name":"张三"},"idempotencyKey":"birthday:<userId>:2026"}
//
// 响应：{"success":true,"messageId":"...","status":"sent"}
// 其中 status 省略时按 success 推断为 sent。
type PlatformSender struct {
	config *config.Config
	client *http.Client
}

var _ Sender = (*PlatformSender)(nil)

// NewPlatformSender 构造中台短信发送器。
func NewPlatformSender(cfg *config.Config) *PlatformSender {
	return &PlatformSender{config: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

type platformRequest struct {
	AppID          int               `json:"appId"`
	Phone          string            `json:"phone"`
	TemplateCode   string            `json:"templateCode"`
	Params         map[string]string `json:"params"`
	IdempotencyKey string            `json:"idempotencyKey"`
}

type platformResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

func (s *PlatformSender) Send(ctx context.Context, message Message) (Result, error) {
	if strings.TrimSpace(s.config.BirthdaySms.Endpoint) == "" {
		return Result{}, fmt.Errorf("短信中台地址未配置")
	}
	payload, err := json.Marshal(platformRequest{
		AppID:          message.AppID,
		Phone:          message.Phone,
		TemplateCode:   message.TemplateCode,
		Params:         message.Params,
		IdempotencyKey: message.IdempotencyKey,
	})
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.BirthdaySms.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(s.config.BirthdaySms.APIKey); key != "" {
		request.Header.Set("Authorization", key)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{}, fmt.Errorf("短信中台返回 %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed platformResponse
	if err = json.Unmarshal(body, &parsed); err != nil {
		return Result{}, fmt.Errorf("解析短信中台响应失败: %w", err)
	}
	if !parsed.Success {
		message := parsed.Message
		if message == "" {
			message = "短信中台返回失败"
		}
		return Result{}, fmt.Errorf("%s", message)
	}
	status := parsed.Status
	if status == "" {
		status = StatusSent
	}
	return Result{MessageID: parsed.MessageID, Status: status}, nil
}
