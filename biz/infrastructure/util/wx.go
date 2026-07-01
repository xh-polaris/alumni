package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
)

// WxClient 微信 API 客户端，管理 access_token 的获取与缓存
type WxClient struct {
	client      *http.Client
	appId       string
	appSecret   string
	accessToken string
	expiresAt   time.Time
	mu          sync.Mutex
}

var wxClient *WxClient

func NewWxClient() *WxClient {
	cfg := config.GetConfig()
	if wxClient == nil {
		wxClient = &WxClient{
			client:    &http.Client{Timeout: 10 * time.Second},
			appId:     cfg.Wx.AppId,
			appSecret: cfg.Wx.AppSecret,
		}
	}
	return wxClient
}

func (w *WxClient) getAccessToken() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.accessToken != "" && time.Now().Before(w.expiresAt) {
		return w.accessToken, nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		w.appId, w.appSecret)
	resp, err := w.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("获取 access_token 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 access_token 响应失败: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 access_token 响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取 access_token 失败: %s", result.ErrMsg)
	}

	w.accessToken = result.AccessToken
	// 提前 5 分钟过期，避免临界情况
	w.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return w.accessToken, nil
}

// GetPhoneNumber 通过微信 code 换取手机号
func (w *WxClient) GetPhoneNumber(code string) (string, error) {
	token, err := w.getAccessToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=%s", token)
	reqBody, _ := json.Marshal(map[string]string{"code": code})

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("创建手机号请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取手机号请求失败: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取手机号响应失败: %w", err)
	}

	var result struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo *struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return "", fmt.Errorf("解析手机号响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("换取手机号失败: %s", result.ErrMsg)
	}
	if result.PhoneInfo == nil {
		return "", fmt.Errorf("未返回手机号数据")
	}

	// purePhoneNumber 不带区号，优先使用
	if result.PhoneInfo.PurePhoneNumber != "" {
		return result.PhoneInfo.PurePhoneNumber, nil
	}
	return result.PhoneInfo.PhoneNumber, nil
}
