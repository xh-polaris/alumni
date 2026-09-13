package adaptor

import (
	"context"
	"errors"
	"strings"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util/log"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/json"
	"github.com/golang-jwt/jwt/v4"
	"github.com/xh-polaris/service-idl-gen-go/kitex_gen/basic"
)

const hertzContext = "hertz_context"

func InjectContext(ctx context.Context, c *app.RequestContext) context.Context {
	return context.WithValue(ctx, hertzContext, c)
}

func ExtractContext(ctx context.Context) (*app.RequestContext, error) {
	c, ok := ctx.Value(hertzContext).(*app.RequestContext)
	if !ok {
		return nil, errors.New("hertz context not found")
	}
	return c, nil
}

func ExtractMeta(ctx context.Context) (*basic.UserMeta, *basic.Extra) {
	return ExtractUserMeta(ctx), ExtractExtra(ctx)
}

// IsDevModeRequest 判断请求是否要求使用开发/演示身份。
//
// 这条通道允许携带 mock-token 直接获得超级管理员会话，因此**必须在生产环境彻底关闭**：
// 只有 State 明确不是 prod 时才生效，避免线上任何人通过一个请求头提权。
func IsDevModeRequest(ctx context.Context) bool {
	if isProductionState() {
		return false
	}
	c, err := ExtractContext(ctx)
	if err != nil {
		return false
	}
	return string(c.GetHeader(consts.DevModeHeader)) == consts.DevModeValue
}

// isProductionState 判断当前是否为生产环境。
// 配置缺失时按非生产处理（本地开发与测试都不加载配置），但 State=prod 时一律拒绝 dev 通道。
func isProductionState() bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}
	return IsProductionState(cfg.State)
}

// IsProductionState 是判断生产环境的纯函数，便于单测覆盖而无需加载配置。
func IsProductionState(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "prod")
}

// parseTokenClaims 解析访问令牌并返回 claims。
//
// 正常路径用 Auth.PublicKey 验签。本地联调常常没有公钥，此时所有真实令牌都会验签失败，
// 表现为「登录成功但后续接口 401」。为避免这种误导，当满足**全部**条件时退化为只解 claims：
//   - 显式配置 Auth.AllowUnverifiedToken: true
//   - Auth.PublicKey 为空
//   - State 不是 prod
//
// 这与已有的 dev 通道（X-Alumni-Mode: dev + mock-token）是同一信任级别，
// 仅用于本地联调；生产环境必须配置真实公钥并把开关关掉。
func parseTokenClaims(ctx context.Context, tokenString string) (jwt.Claims, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, errors.New("authorization header is empty")
	}
	cfg := config.GetConfig()
	publicKey := ""
	allowUnverified := false
	if cfg != nil {
		publicKey = strings.TrimSpace(cfg.Auth.PublicKey)
		allowUnverified = cfg.Auth.AllowUnverifiedToken && !IsProductionState(cfg.State)
	}
	if publicKey != "" {
		token, err := jwt.Parse(tokenString, func(_ *jwt.Token) (interface{}, error) {
			return jwt.ParseECPublicKeyFromPEM([]byte(publicKey))
		})
		if err != nil {
			return nil, err
		}
		if !token.Valid {
			return nil, errors.New("token is not valid")
		}
		return token.Claims, nil
	}
	if !allowUnverified {
		return nil, errors.New("auth public key is not configured")
	}
	log.CtxError(ctx, "warn=auth_unverified_token 未配置 Auth.PublicKey 且开启 AllowUnverifiedToken，"+
		"当前只解析 JWT claims 不验签；仅限本地联调，生产环境必须配置公钥")
	// ParseUnverified 不会校验签名，也不会把 token.Valid 置为 true，
	// 因此这里直接返回 claims，调用方不再检查 Valid。
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}
	return token.Claims, nil
}

func ExtractUserMeta(ctx context.Context) (user *basic.UserMeta) {
	user = new(basic.UserMeta)
	if meta, ok := extractUserMetaOverride(ctx); ok {
		return meta
	}
	var err error
	defer func() {
		if err != nil {
			log.CtxInfo(ctx, "extract user meta fail, err=%v", err)
		}
	}()
	c, err := ExtractContext(ctx)
	if err != nil {
		return
	}
	tokenString := c.GetHeader("Authorization")
	if IsDevModeRequest(ctx) && string(tokenString) == consts.DevMockAccessToken {
		user.UserId = consts.DevMockUserID
		user.AppId = basic.APP(consts.AppId)
		user.SessionUserId = user.UserId
		user.SessionAppId = user.AppId
		user.IsLogin = true
		return
	}
	claims, err := parseTokenClaims(ctx, string(tokenString))
	if err != nil {
		return
	}
	data, err := json.Marshal(claims)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, user)
	if err != nil {
		return
	}
	user.IsLogin = user.UserId != ""
	if user.SessionUserId == "" {
		user.SessionUserId = user.UserId
	}
	if user.SessionAppId == 0 {
		user.SessionAppId = user.AppId
	}
	if user.SessionDeviceId == "" {
		user.SessionDeviceId = user.DeviceId
	}
	log.CtxInfo(ctx, "userMeta=%s", util.JSONF(user))
	return
}

func ExtractExtra(ctx context.Context) (extra *basic.Extra) {
	extra = new(basic.Extra)
	var err error
	defer func() {
		if err != nil {
			log.CtxInfo(ctx, "extract extra fail, err=%v", err)
		}
	}()
	c, err := ExtractContext(ctx)
	if err != nil {
		return
	}
	extra.ClientIP = c.ClientIP()
	err = c.Bind(extra)
	if err != nil {
		return
	}
	log.CtxInfo(ctx, "extra=%s", util.JSONF(extra))
	return
}
