package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/seed"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/rpc/platform_sts"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
	seed.EnsureDevData(provider.Config)
}

// Provider 提供controller依赖的对象
type Provider struct {
	Config          *config.Config
	UserService     service.UserService
	ActivityService service.ActivityService
	AdminService    service.AdminService
	ArticleService  service.ArticleService
	StsService      service.StsService
}

func Get() *Provider {
	return provider
}

var ApplicationSet = wire.NewSet(
	service.UserServiceSet,
	service.ActivityServiceSet,
	service.AdminServiceSet,
	service.ArticleServiceSet,
	service.StsServiceSet,
)

var RpcSet = wire.NewSet(
	platform_sts.PlatformStsSet,
)

var InfrastructureSet = wire.NewSet(
	config.NewConfig,
	user.NewMongoMapper,
	article.NewMongoMapper,
	activity.NewMongoMapper,
	register.NewMongoMapper,
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
