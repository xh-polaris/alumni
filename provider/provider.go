package provider

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/application/service"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/activity"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/article"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/birthday"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/chapter"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/register"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/rpc/platform_sts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/seed"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/sms"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
	seed.EnsureDevData(provider.Config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = provider.ChapterMapper.EnsureDefaults(ctx)
	for _, ensure := range provider.indexEnsurers() {
		_ = ensure(ctx)
	}
	_ = provider.UserService.MigrateLegacy(ctx)
	go provider.BirthdayService.Start(context.Background())
}

// Provider 提供controller依赖的对象
type Provider struct {
	Config          *config.Config
	UserService     service.UserService
	ActivityService service.ActivityService
	AdminService    service.AdminService
	ArticleService  service.ArticleService
	StsService      service.StsService
	ChapterMapper   *chapter.MongoMapper
	RosterMapper    *roster.MongoMapper
	BirthdayMapper  *birthday.MongoMapper
	BirthdayService service.BirthdayService
	UploadService   service.UploadService
}

// indexEnsurers 汇总各集合的索引初始化，启动时统一执行。
// 索引是自动认证与生日短信幂等的前提；失败只记录不阻塞启动，
// 因为首次部署时集合可能尚未创建。
func (p *Provider) indexEnsurers() []func(context.Context) error {
	ensurers := []func(context.Context) error{
		p.UserService.UserMapper.EnsureIndexes,
		p.ChapterMapper.EnsureIndexes,
		p.RosterMapper.EnsureIndexes,
		p.BirthdayMapper.EnsureIndexes,
		p.ActivityService.ActivityMapper.EnsureIndexes,
		p.ActivityService.RegisterMapper.EnsureIndexes,
		p.ArticleService.ArticleMapper.EnsureIndexes,
	}
	return ensurers
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
	service.BirthdayServiceSet,
	service.UploadServiceSet,
)

var RpcSet = wire.NewSet(
	platform_sts.PlatformStsSet,
)

// InfrastructureSet 把具体的 Mongo mapper 绑定到服务层依赖的窄接口上，
// 服务层因此可以在不连接 MongoDB 的情况下被测试。
var InfrastructureSet = wire.NewSet(
	config.NewConfig,
	user.NewMongoMapper,
	wire.Bind(new(user.IMongoMapper), new(*user.MongoMapper)),
	article.NewMongoMapper,
	wire.Bind(new(article.IMongoMapper), new(*article.MongoMapper)),
	activity.NewMongoMapper,
	wire.Bind(new(activity.IMongoMapper), new(*activity.MongoMapper)),
	register.NewMongoMapper,
	wire.Bind(new(register.IMongoMapper), new(*register.MongoMapper)),
	chapter.NewMongoMapper,
	wire.Bind(new(chapter.IMongoMapper), new(*chapter.MongoMapper)),
	roster.NewMongoMapper,
	wire.Bind(new(roster.IMongoMapper), new(*roster.MongoMapper)),
	birthday.NewMongoMapper,
	wire.Bind(new(birthday.IMongoMapper), new(*birthday.MongoMapper)),
	sms.NewPlatformSender,
	wire.Bind(new(sms.Sender), new(*sms.PlatformSender)),
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
