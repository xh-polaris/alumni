package adaptor

import (
	"context"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/service-idl-gen-go/kitex_gen/basic"
)

// userMetaKey 是已解析身份的 context key。
//
// 认证中间件解析出身份后放入 context，后续的 service 层直接复用，
// 避免每个 handler 都重复解析 JWT；测试也可以用同一个入口注入身份，
// 从而在不访问真实 JWT 私钥的前提下覆盖分会权限逻辑。
type userMetaKey struct{}

// WithUserMeta 把一个已解析的用户身份写入 context。
func WithUserMeta(ctx context.Context, meta *basic.UserMeta) context.Context {
	return context.WithValue(ctx, userMetaKey{}, meta)
}

// WithUserID 按用户 ID 构造并写入身份，AppId 固定为业务 AppId。
func WithUserID(ctx context.Context, userID string) context.Context {
	meta := &basic.UserMeta{}
	meta.UserId = userID
	meta.AppId = basic.APP(consts.AppId)
	meta.SessionUserId = userID
	meta.SessionAppId = meta.AppId
	meta.IsLogin = userID != ""
	return WithUserMeta(ctx, meta)
}

// extractUserMetaOverride 返回中间件或测试预先写入的身份。
func extractUserMetaOverride(ctx context.Context) (*basic.UserMeta, bool) {
	meta, ok := ctx.Value(userMetaKey{}).(*basic.UserMeta)
	if !ok || meta == nil {
		return nil, false
	}
	return meta, true
}
