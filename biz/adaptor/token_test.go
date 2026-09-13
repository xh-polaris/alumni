package adaptor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
)

// 一个格式合法、但签名无意义的 JWT：ParseUnverified 只解 payload。
const sampleToken = "eyJhbGciOiJFUzI1NiJ9.eyJ1c2VySWQiOiJhYmMiLCJhcHBJZCI6MTV9.c2ln"

// configPrefix 覆盖 go-zero ServiceConf 校验要求的必填项；各用例只追加 State 与 Auth。
const configPrefix = `Name: test
ListenOn: 0.0.0.0:0
Wx:
  AppId: "test"
  AppSecret: "test"
Mongo:
  URL: mongodb://127.0.0.1:27017
  DB: test
Cache:
  - Host: 127.0.0.1:6379
    Type: node
BirthdaySms:
  Enabled: false
  Endpoint: "http://127.0.0.1/unused"
  TemplateCode: "TEST"
`

// authBase 是 Auth 段的必填项，各用例在其后追加 PublicKey / AllowUnverifiedToken。
const authBase = `Auth:
  SecretKey: "test"
  PublicKey: ""
  AccessExpire: 7200
`

func loadConfig(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configPrefix+body), 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}
	t.Setenv("CONFIG_PATH", path)
	if _, err := config.NewConfig(); err != nil {
		t.Fatalf("加载临时配置失败: %v", err)
	}
}

// 「不验签解析」是一条提权通道，必须严格受 AllowUnverifiedToken + 非生产环境双重约束。
func TestParseTokenClaimsGuardsUnverifiedFallback(t *testing.T) {
	cases := []struct {
		name    string
		config  string
		allowed bool
	}{
		{
			name:    "本地联调：显式开启且非生产，允许",
			config:  "State: dev\n" + authBase + "  AllowUnverifiedToken: true\n",
			allowed: true,
		},
		{
			name:    "未显式开启，拒绝",
			config:  "State: dev\n" + authBase + "  AllowUnverifiedToken: false\n",
			allowed: false,
		},
		{
			name:    "生产环境即使开启也拒绝",
			config:  "State: prod\n" + authBase + "  AllowUnverifiedToken: true\n",
			allowed: false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			loadConfig(t, testCase.config)
			claims, err := parseTokenClaims(context.Background(), sampleToken)
			if testCase.allowed {
				if err != nil {
					t.Fatalf("应允许不验签解析，实际 %v", err)
				}
				if claims == nil {
					t.Fatalf("应返回 claims")
				}
				return
			}
			if err == nil {
				t.Fatalf("应拒绝，实际通过")
			}
		})
	}
}

func TestParseTokenClaimsRejectsEmptyToken(t *testing.T) {
	loadConfig(t, "State: dev\n"+authBase+"  AllowUnverifiedToken: true\n")
	if _, err := parseTokenClaims(context.Background(), "  "); err == nil {
		t.Fatalf("空 Authorization 应被拒绝")
	}
}
