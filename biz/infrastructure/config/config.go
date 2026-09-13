package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"os"

	"github.com/zeromicro/go-zero/core/service"

	"github.com/zeromicro/go-zero/core/conf"
)

var config *Config

type Auth struct {
	SecretKey    string
	PublicKey    string
	AccessExpire int64
	// AllowUnverifiedToken 仅用于本地联调：当 PublicKey 未配置时，
	// 退化为「不验签、只解 claims」。必须显式开启，且 State=prod 时一律忽略。
	// 标记 optional，保证旧配置文件缺少该字段时仍可启动。
	AllowUnverifiedToken bool `json:",optional"`
}

// Wx / BirthdaySms 的字段全部标记 optional：它们只在对应能力被使用时才需要，
// 缺失时应当使用零值继续启动，而不是直接拒绝启动。
type Wx struct {
	AppId     string `json:",optional"`
	AppSecret string `json:",optional"`
}

type BirthdaySms struct {
	Enabled      bool   `json:",optional"`
	Endpoint     string `json:",optional"`
	APIKey       string `json:",optional"`
	TemplateCode string `json:",optional"`
}

// Upload 是图片上传的本地存储配置。
//
// 线上更推荐客户端直传对象存储（现有 POST /sts/apply 会返回带签名的 PUT URL）；
// 本地联调没有中台 STS 服务，因此这里提供「存本地磁盘 + 静态托管」的兜底方案。
type Upload struct {
	// Dir 本地存储根目录，默认 ./output/upload
	Dir string `json:",optional"`
	// PublicBaseURL 拼接对外 URL 的前缀，例如 http://localhost:8888；
	// 留空时返回相对路径 /files/xxx，由调用方自行补全。
	PublicBaseURL string `json:",optional"`
	// MaxBytes 单文件大小上限，默认 5MB
	MaxBytes int64 `json:",optional"`
}

type Config struct {
	service.ServiceConf
	ListenOn string
	State    string
	// MetricsListenOn 是 Prometheus 指标端口。gopkg/kitex/client 会在 init 阶段
	// 硬编码占用 :9091，因此本地联调把它错开，保证 go test 仍能绑定 :9091。
	// 标记 optional 以兼容不含该字段的旧配置，缺省回落到 :9091。
	MetricsListenOn string `json:",optional"`
	Wx              Wx     `json:",optional"`
	Auth            Auth
	BirthdaySms     BirthdaySms `json:",optional"`
	Upload          Upload      `json:",optional"`
	Mongo           struct {
		URL string
		DB  string
	}
	Cache cache.CacheConf
}

func NewConfig() (*Config, error) {
	c := new(Config)
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "etc/config.yaml"
	}
	err := conf.Load(path, c)
	if err != nil {
		return nil, err
	}
	err = c.SetUp()
	if err != nil {
		return nil, err
	}
	config = c
	return c, nil
}

func GetConfig() *Config {
	return config
}
