# 管理后台后端实现设计

## 状态

- 日期：2026-06-25
- 状态：待实现
- 范围：`alumni-backend`
- 目标：为现有 `alumni-admin` 提供可脱离 mock 的管理端后端接口，并补齐资讯实体。

## 背景

现有后端已经有用户端接口：

- `/user/sign_in`
- `/user/*`
- `/activity/*`

管理端前端已经完成基础 CRUD 页面，活动 CRUD 已接入现有 `/activity/*` 接口；用户、报名、资讯、管理端 session 仍依赖 mock。后端需要补齐对应接口。

## 总体方案

采用方案 A：在现有 Go/Hertz 后端中新增自定义 `/admin/*` REST 路由。

不重生成 IDL，不改现有用户端接口，不新建管理后端服务。新增代码沿用当前分层：

- controller：处理 Hertz 请求、绑定 query/body/path 参数、统一响应。
- service：封装管理端业务逻辑。
- mapper：封装 MongoDB 访问。
- model：定义 MongoDB 文档结构。

## 认证与权限

继续使用现有登录体系：

- 登录仍调用 `POST /user/sign_in`。
- 管理端请求继续传 `Authorization`。
- `/admin/*` 第一阶段做最小校验：请求必须带 `Authorization`。

用户角色字段本次一并落库，但第一阶段不实现复杂 RBAC。`role=admin` 用于标识管理员身份；是否开放管理后台由部署方控制账号发放和前端入口。后续如要严格校验管理员，可在统一管理端 middleware 中将校验从“有 token”升级为“token 对应用户 role=admin 且 status=0”。

## 统一响应格式

管理端新增接口统一返回当前前端已经适配的格式：

成功：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

失败：

```json
{
  "code": 40400,
  "message": "资源不存在",
  "data": null
}
```

## 用户 role 字段

在 `user` 集合新增业务角色字段：

| API 值 | 中文 | 说明 |
| --- | --- | --- |
| `admin` | 管理员 | 可以作为管理后台管理员身份 |
| `guest` | 嘉宾 | 活动嘉宾、特邀人员 |
| `alumni` | 校友 | 已确认校友身份 |
| `user` | 普通用户 | 默认角色 |

约束：

- 老数据没有 `role` 时按 `user` 返回。
- 新注册用户默认写入 `role=user`。
- 管理端用户接口允许修改 `role`。
- 前端的原 `user/admin` 二值角色需要同步改为四值角色。

## 资讯 Article 实体

新增 Mongo collection：`article`。

字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `_id` | ObjectID | 资讯 ID |
| `title` | string | 标题 |
| `summary` | string | 简介 |
| `cover` | string | 封面 URL |
| `wechat_url` | string | 微信公众号链接 |
| `source` | string | 来源 |
| `author` | string | 作者 |
| `publish_time` | DateTime/null | 发布时间 |
| `sort_order` | int64 | 排序值 |
| `publish_status` | string | `draft`、`published`、`offline` |
| `deleted` | bool | 是否软删除 |
| `create_time` | DateTime | 创建时间 |
| `update_time` | DateTime | 更新时间 |
| `delete_time` | DateTime/null | 删除时间 |

创建规则：

- 新建资讯默认 `publish_status=draft`。
- 发布时设置 `publish_status=published`；如果 `publish_time` 为空，自动写当前时间。
- 下架时设置 `publish_status=offline`。
- 删除为软删除：`deleted=true`，写入 `delete_time`。
- 恢复时：`deleted=false`，`publish_status=offline`，清空删除语义。

列表排序：

1. `sort_order` 降序
2. `publish_time` 降序
3. `create_time` 降序

## 管理端接口

### Session

- `GET /admin/session`

返回当前管理端会话：

```json
{
  "id": "string",
  "name": "string",
  "avatar": "string",
  "phone": "string",
  "role": "admin"
}
```

第一阶段无法从 token 稳定反查用户时，返回一个固定管理端 session，保证前端认证流可用。

### Users

- `GET /admin/users`
- `GET /admin/users/:id`
- `PATCH /admin/users/:id`
- `PATCH /admin/users/:id/role`
- `PATCH /admin/users/:id/status`
- `DELETE /admin/users/:id`
- `POST /admin/users/:id/restore`

列表参数：

- `page`
- `pageSize`
- `keyword`
- `role`
- `status`

返回字段对齐前端 `User` 类型：

- `id`
- `avatar`
- `name`
- `gender`
- `birthday`
- `phone`
- `wxId`
- `hometown`
- `homeEducations`
- `shanghaiEducations`
- `employments`
- `role`
- `status`
- `deleted`
- `createTime`

软删除：

- `DELETE` 设置 `status=1` 并写 `delete_time`。
- `restore` 设置 `status=0` 并清空删除语义。

### Registrations

- `GET /admin/registrations`
- `POST /admin/registrations`
- `PATCH /admin/registrations/:id`
- `DELETE /admin/registrations/:id`
- `POST /admin/registrations/:id/check-in`
- `POST /admin/registrations/:id/cancel-check-in`

列表参数：

- `activityId`
- `page`
- `pageSize`
- `keyword`
- `checkIn`

返回字段对齐前端 `Registration` 类型：

- `id`
- `activityId`
- `userId`
- `name`
- `phone`
- `checkIn`
- `checkInTime`
- `deleted`
- `createTime`

报名删除为软删除。旧数据没有删除字段时按未删除处理。

### Articles

- `GET /admin/articles`
- `GET /admin/articles/:id`
- `POST /admin/articles`
- `PATCH /admin/articles/:id`
- `DELETE /admin/articles/:id`
- `POST /admin/articles/:id/restore`
- `POST /admin/articles/:id/publish`
- `POST /admin/articles/:id/offline`

列表参数：

- `page`
- `pageSize`
- `keyword`
- `status`

`status=deleted` 时查询已删除资讯；其他状态按 `publish_status` 过滤。

## 前端同步点

后端实现完成后，`alumni-admin` 需要同步：

- `User.role` 类型改为 `admin | guest | alumni | user`。
- 用户列表角色筛选改为四个选项。
- 用户角色操作从“设为管理员/取消管理员”改为选择四种角色。
- mock 数据同步四角色，避免本地 mock 与真实接口不一致。

## 验证

后端验证：

- `go test ./...`
- `go build ./...`

前后端联调验证：

- `GET /admin/session` 能建立管理端会话。
- 用户列表可分页、筛选、修改四种角色。
- 报名列表可查询、创建、编辑、删除、签到和取消签到。
- 资讯可创建、编辑、发布、下架、删除、恢复。
- 现有 `/activity/*` 和 `/user/sign_in` 不回归。

## 非目标

本次不做：

- 复杂 RBAC。
- 操作审计。
- 批量导入导出。
- 富文本资讯正文。
- 微信公众号自动同步。
