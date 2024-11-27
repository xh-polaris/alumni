package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
}

// Provider 提供controller依赖的对象
type Provider struct {
	Config          *config.Config
	UserService     service.UserService
	ActivityService service.ActivityService
}

func Get() *Provider {
	return provider
}

var ApplicationSet = wire.NewSet(
	service.UserServiceSet,
	service.ActivityServiceSet,
)

var InfrastructureSet = wire.NewSet(
	config.NewConfig,
	user.NewMongoMapper,
	activity.NewMongoMapper,
	register.NewMongoMapper,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
