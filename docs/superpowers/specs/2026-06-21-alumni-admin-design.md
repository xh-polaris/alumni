# 校友平台管理后台前后端规范

## 1. 文档信息

- 状态：已确认设计
- 日期：2026-06-21
- 适用范围：`alumni-admin` 管理端前端与 `alumni-backend` 管理接口
- 目标：以简单、快速、可维护为原则，实现用户、活动、活动报名和资讯的基础 CRUD 管理

## 2. 背景与目标

现有系统包含：

- `alumni-app`：Vue 3 + uni-app 用户端。
- `alumni-backend`：Go + Hertz + MongoDB 后端。
- 已存在 `user`、`activity`、`register` 三个 MongoDB 集合。
- 用户端已有资讯入口，但当前仅展示“暂无资讯”，没有资讯实体和接口。

本项目新增独立管理端 `alumni-admin`，并在现有 `alumni-backend` 中增加 `/admin/*` 管理接口。管理端继续复用现有手机号登录和 JWT 体系，由后端根据登录用户 ID 查询管理员角色并完成授权。

第一阶段目标：

1. 管理用户资料、角色和状态。
2. 管理活动及其报名数据。
3. 管理外链型资讯并在用户端展示已发布资讯。
4. 提供清晰、安全、统一的管理 API。

## 3. 范围与非目标

### 3.1 第一阶段范围

- 管理员登录及权限校验。
- 用户查询、查看、编辑、停用、恢复、软删除和角色调整。
- 活动查询、创建、查看、编辑、软删除和恢复。
- 活动报名查询、创建、查看、编辑、软删除、签到和取消签到。
- 资讯查询、创建、查看、编辑、发布、下架、软删除和恢复。
- 活动及资讯封面上传。
- 服务端分页、关键词搜索、状态筛选和时间筛选。
- 前后端关键逻辑测试。

### 3.2 第一阶段非目标

- 富文本资讯正文。
- 微信公众号内容自动同步。
- 数据分析大屏。
- 批量导入导出。
- 复杂 RBAC 和细粒度权限配置。
- 多租户。
- 操作审计日志页面。
- UI 个性化主题。

## 4. 总体架构

### 4.1 工程边界

- `alumni-admin`：新增的独立 React 管理端工程，与 `alumni-app` 同级。
- `alumni-backend`：保留现有服务，在同一进程内增加管理接口、管理员鉴权和资讯模块。
- MongoDB：保留 `user`、`activity`、`register` 集合，新增 `article` 集合。
- 对象存储：继续通过现有 `/sts/apply` 获取上传签名，前端直传对象存储。

不新建独立管理后端服务，避免重复部署、认证、数据库连接和监控配置。

### 4.2 管理端技术栈

- React 19
- TypeScript
- Vite
- Material UI
- MUI Data Grid
- TanStack Query
- React Router
- React Hook Form
- Zod
- Axios
- Zustand，仅保存登录态和必要的全局会话信息
- Vitest + React Testing Library
- pnpm

管理端只使用 Material UI 作为 UI 组件体系，不引入 shadcn/ui 和 TanStack Table，减少重复能力和样式维护成本。

## 5. 认证与授权

### 5.1 登录流程

1. 管理端调用现有 `POST /user/sign_in`，使用手机号配合密码或验证码登录。
2. 登录成功后保存访问令牌，并立即调用 `GET /admin/session`。
3. 后端验证 JWT，从令牌提取用户 ID，再从 `user` 集合查询最新角色和状态。
4. 仅 `role=admin` 且 `status=0` 的用户可建立管理会话。
5. 普通用户收到 `403`；无效或过期令牌收到 `401`。

前端传入的角色字段不作为授权依据。每次 `/admin/*` 请求均由后端校验当前用户的真实角色和状态。

### 5.2 管理员安全规则

- 管理员不能停用或删除自己。
- 管理员不能取消自己的管理员角色。
- 系统至少保留一个正常状态的管理员。
- 修改角色、停用、恢复和删除用户均记录操作管理员 ID。

### 5.3 请求头

```http
Authorization: <access-token>
```

沿用现有服务的令牌格式，不额外增加前缀要求。

## 6. 数据模型

### 6.1 通用字段约定

- MongoDB 中时间字段使用 BSON DateTime。
- API 中时间统一使用 Unix 秒级时间戳。
- `create_time` 在创建时写入，之后不修改。
- `update_time` 在每次写操作时更新。
- `delete_time` 仅在软删除时写入，恢复时清空。
- 管理写操作增加 `created_by` 或 `updated_by`，值为操作管理员的用户 ID；旧数据允许为空。
- 所有软删除数据默认不出现在公开接口和普通管理列表中。

### 6.2 用户 `user`

保留现有字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `_id` | ObjectID | 用户 ID，与现有登录体系用户 ID 一致 |
| `avatar` | string | 头像 URL |
| `name` | string | 姓名 |
| `gender` | int64 | 沿用现有性别编码 |
| `birthday` | DateTime | 生日 |
| `phone` | string | 手机号 |
| `wx_id` | string | 微信号 |
| `hometown` | string | 籍贯 |
| `home_educations` | Education[] | 家乡教育经历 |
| `shanghai_educations` | Education[] | 上海教育经历 |
| `employments` | Employment[] | 工作经历 |
| `status` | int64 | `0` 正常，`1` 停用或已删除 |
| `create_time` | DateTime | 创建时间 |
| `update_time` | DateTime | 更新时间 |
| `delete_time` | DateTime/null | 软删除时间 |

新增字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `role` | string | `user` 或 `admin`；旧数据缺省按 `user` 处理 |
| `updated_by` | string | 最后操作管理员 ID |

`Education`：

- `phase`：教育阶段。
- `school`：学校。
- `year`：年份。

`Employment`：

- `organization`：单位。
- `position`：职位。
- `industry`：行业。
- `entry`：入职时间，沿用现有整数时间表示。
- `departure`：离职时间，沿用现有整数时间表示。

用户停用和删除在第一阶段均表现为不可登录、不可访问管理接口。软删除额外写入 `delete_time`，以区分停用与删除；恢复删除时清空 `delete_time` 并将 `status` 设为 `0`。

### 6.3 活动 `activity`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `_id` | ObjectID | 活动 ID |
| `cover` | string | 封面 URL |
| `name` | string | 活动名称 |
| `location` | string | 地区 |
| `exact_location` | string | 详细地址，沿用现有 JSON 字符串表示 |
| `sponsor` | string | 主办方 |
| `start` | int64 | 活动开始时间，Unix 秒 |
| `description` | string | 活动介绍 |
| `register_start` | DateTime | 报名开始时间 |
| `register_end` | DateTime | 报名截止时间 |
| `contact` | string | 联系方式 |
| `limit` | int64 | 人数限制，`-1` 表示不限制 |
| `status` | int64 | `0` 有效，`1` 已删除 |
| `create_time` | DateTime | 创建时间 |
| `update_time` | DateTime | 更新时间 |
| `delete_time` | DateTime/null | 软删除时间 |
| `created_by` | string | 创建管理员 ID |
| `updated_by` | string | 最后操作管理员 ID |

报名人数和签到人数不冗余写入活动文档，由 `register` 集合按活动 ID 统计。

### 6.4 活动报名 `register`

保留现有字段并补充状态和签到时间：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `_id` | ObjectID | 报名 ID |
| `activity_id` | string | 活动 ID |
| `user_id` | string | 关联用户 ID；管理员代录时可为空 |
| `name` | string | 报名姓名 |
| `phone` | string | 手机号；兼容旧数据中的 `-1` |
| `check_in` | bool | 是否签到 |
| `check_in_time` | DateTime/null | 签到时间；取消签到时清空 |
| `status` | int64 | `0` 有效，`1` 已删除；旧数据缺省按 `0` 处理 |
| `create_time` | DateTime | 创建时间 |
| `update_time` | DateTime | 更新时间 |
| `delete_time` | DateTime/null | 软删除时间 |
| `created_by` | string | 管理员代录时记录管理员 ID |
| `updated_by` | string | 最后操作管理员 ID |

同一活动下，标准化后的姓名和手机号完全相同时视为重复报名。手机号为空和旧值 `-1` 在重复校验时视为同一种“无手机号”状态。

### 6.5 资讯 `article`

资讯采用外链卡片模式，不保存正文。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `_id` | ObjectID | 资讯 ID |
| `title` | string | 必填，去除首尾空格后 1 至 100 个字符 |
| `summary` | string | 必填，去除首尾空格后 1 至 500 个字符 |
| `cover` | string | 可选，封面 URL |
| `wechat_url` | string | 必填，合法 HTTP(S) URL |
| `source` | string | 可选，最多 100 个字符 |
| `author` | string | 可选，最多 100 个字符 |
| `publish_time` | DateTime | 展示发布时间；首次发布时为空则自动使用当前时间 |
| `sort_order` | int64 | 默认 `0`，数值越大越靠前 |
| `publish_status` | string | `draft`、`published` 或 `offline` |
| `create_time` | DateTime | 创建时间 |
| `update_time` | DateTime | 更新时间 |
| `delete_time` | DateTime/null | 软删除时间 |
| `created_by` | string | 创建管理员 ID |
| `updated_by` | string | 最后操作管理员 ID |

`wechat_url` 第一阶段接受任意合法 HTTP(S) URL，以兼容公众号短链和域名变化；管理端文案提示该字段应填写微信公众号文章链接。后端不抓取、不解析、不代理目标页面。

公开资讯按 `sort_order DESC, publish_time DESC, create_time DESC` 排序，只返回 `publish_status=published` 且未删除的数据。

## 7. 管理端页面规范

### 7.1 全局布局

- 固定侧边栏：用户管理、活动管理、资讯管理。
- 顶部栏：当前管理员、退出登录。
- 内容区：页面标题、筛选区、操作区和数据表格。
- 使用 MUI Data Grid 服务端分页。
- 简单数据使用 Dialog 或 Drawer 编辑；活动和资讯使用独立编辑页面。
- 页码、关键词和筛选条件同步到 URL 查询参数。
- 删除、停用、取消管理员和下架操作必须二次确认。

### 7.2 登录页

- 支持现有手机号加密码或验证码的登录方式。
- 登录成功后调用 `/admin/session` 验证管理员权限。
- 非管理员显示“当前账号无管理权限”，清除令牌并停留在登录页。
- Token 过期时清除会话并跳转登录页，保留原目标地址用于登录后返回。

### 7.3 用户管理

列表字段：头像、姓名、手机号、性别、籍贯、角色、状态、创建时间、操作。

功能：

- 按姓名或手机号搜索。
- 按角色和状态筛选。
- 查看用户详情。
- 编辑基本信息、教育经历和工作经历。
- 停用和恢复用户。
- 设置或取消管理员角色。
- 软删除用户。

用户密码不在本管理端中查看或修改。

### 7.4 活动管理

列表字段：封面、名称、主办方、活动时间、报名时间、人数限制、报名人数、签到人数、状态、操作。

功能：

- 按活动名称或主办方搜索。
- 按状态和活动时间筛选。
- 创建、查看、编辑、软删除和恢复活动。
- 上传封面。
- 点击报名人数进入该活动的报名管理页。

表单校验：

- 名称必填。
- 活动时间必填。
- 报名开始时间不得晚于报名截止时间。
- 报名截止时间不得晚于活动开始时间。
- 人数限制只能为 `-1` 或正整数。

### 7.5 报名管理

路由为活动的子页面，不设置一级侧边栏入口。

列表字段：姓名、手机号、关联用户、签到状态、签到时间、报名时间、操作。

页面顶部展示报名总数、签到数和未签到数。

功能：

- 按姓名或手机号搜索。
- 按签到状态筛选。
- 管理员手动新增报名。
- 编辑姓名和手机号。
- 软删除报名。
- 签到和取消签到。

### 7.6 资讯管理

列表字段：封面、标题、来源、作者、发布时间、发布状态、排序值、创建时间、操作。

功能：

- 按标题或来源搜索。
- 按发布状态筛选。
- 创建、查看、编辑、软删除和恢复资讯。
- 草稿发布、已发布下架、下架后重新发布。
- 新窗口预览公众号文章链接。
- 上传封面。

资讯编辑表单不包含正文编辑器。

## 8. API 规范

### 8.1 通用响应

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

分页响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "pageSize": 20
  }
}
```

错误响应：

```json
{
  "code": 40001,
  "message": "invalid request",
  "data": null
}
```

管理接口使用真实 HTTP 状态码：

- `400`：请求格式或参数错误。
- `401`：未登录、Token 无效或 Token 过期。
- `403`：非管理员或操作受保护资源。
- `404`：目标数据不存在或已删除。
- `409`：重复数据或状态冲突。
- `422`：请求格式正确但不满足业务规则。
- `500`：未预期服务端错误。

### 8.2 通用分页与筛选

- `page`：从 `1` 开始，默认 `1`。
- `pageSize`：默认 `20`，允许 `10`、`20`、`50`、`100`，最大 `100`。
- `keyword`：去除首尾空格后参与模糊匹配。
- `status`：模块对应的状态值。
- `startTime`、`endTime`：Unix 秒级时间戳。
- `includeDeleted`：默认 `false`；仅管理接口可用。

分页参数非法时返回 `400`，不静默修正。

### 8.3 会话接口

#### `GET /admin/session`

返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "user-id",
    "name": "管理员",
    "avatar": "https://example.com/avatar.png",
    "phone": "13800000000",
    "role": "admin"
  }
}
```

### 8.4 用户管理接口

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/admin/users` | 分页查询用户，支持 `keyword`、`role`、`status`、`includeDeleted` |
| GET | `/admin/users/:id` | 查询用户详情 |
| PATCH | `/admin/users/:id` | 修改基本资料、教育经历和工作经历 |
| PATCH | `/admin/users/:id/role` | 修改 `role` |
| PATCH | `/admin/users/:id/status` | 设置正常或停用状态 |
| DELETE | `/admin/users/:id` | 软删除用户 |
| POST | `/admin/users/:id/restore` | 恢复已删除用户 |

角色修改请求：

```json
{ "role": "admin" }
```

状态修改请求：

```json
{ "status": 1 }
```

### 8.5 活动管理接口

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/admin/activities` | 分页查询活动，支持关键词、状态和活动时间筛选 |
| POST | `/admin/activities` | 创建活动 |
| GET | `/admin/activities/:id` | 查询活动详情和报名统计 |
| PATCH | `/admin/activities/:id` | 部分更新活动 |
| DELETE | `/admin/activities/:id` | 软删除活动 |
| POST | `/admin/activities/:id/restore` | 恢复活动 |

活动列表和详情响应增加：

- `registrationCount`：有效报名数。
- `checkInCount`：有效且已签到的报名数。

删除活动不会级联删除报名记录。已删除活动不能新增报名或签到；恢复活动后原有报名记录继续有效。

### 8.6 报名管理接口

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/admin/registrations` | 按 `activityId` 分页查询，支持关键词和签到状态筛选 |
| POST | `/admin/registrations` | 管理员手动新增报名 |
| GET | `/admin/registrations/:id` | 查询报名详情 |
| PATCH | `/admin/registrations/:id` | 修改姓名、手机号或关联用户 |
| DELETE | `/admin/registrations/:id` | 软删除报名 |
| POST | `/admin/registrations/:id/check-in` | 签到并写入 `check_in_time` |
| POST | `/admin/registrations/:id/cancel-check-in` | 取消签到并清空 `check_in_time` |

查询接口必须提供 `activityId`，缺失时返回 `400`。新增报名时必须校验活动存在且未删除。

### 8.7 资讯管理接口

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/admin/articles` | 分页查询资讯，支持关键词、发布状态和发布时间筛选 |
| POST | `/admin/articles` | 创建资讯，默认状态为 `draft` |
| GET | `/admin/articles/:id` | 查询资讯详情 |
| PATCH | `/admin/articles/:id` | 部分更新资讯 |
| DELETE | `/admin/articles/:id` | 软删除资讯 |
| POST | `/admin/articles/:id/restore` | 恢复资讯，恢复后状态固定为 `offline` |
| POST | `/admin/articles/:id/publish` | 发布或重新发布资讯 |
| POST | `/admin/articles/:id/offline` | 下架资讯 |

发布前必须再次校验标题、简介和公众号链接。首次发布且 `publish_time` 为空时，后端写入当前时间；重新发布保留原发布时间，除非管理员在编辑请求中明确修改。

### 8.8 用户端资讯接口

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/articles` | 查询已发布、未删除资讯，支持分页 |
| GET | `/articles/:id` | 查询已发布资讯详情 |

公开响应包含：`id`、`title`、`summary`、`cover`、`wechatUrl`、`source`、`author`、`publishTime`。不返回角色、操作人或删除信息。

### 8.9 文件上传

管理端继续使用 `POST /sts/apply` 获取签名并直传对象存储。上传成功后，前端将对象 URL 写入活动或资讯表单。后端保存前校验 URL 非空时必须是合法 HTTP(S) URL。

## 9. 前端数据流与状态管理

- Axios 实例统一添加令牌、请求超时和错误转换。
- TanStack Query 管理列表、详情和会话数据。
- 写操作成功后使对应列表和详情 Query 失效并重新获取。
- Zustand 仅保存访问令牌和当前管理员的最小会话信息，不缓存业务列表。
- React Hook Form 管理表单状态，Zod 与后端规则保持一致。
- 列表筛选先写入 URL，再由 URL 参数构造 Query Key 和 API 请求。
- 连续关键词输入使用 300 毫秒防抖。
- 提交期间禁用重复提交；后端仍需保证业务操作幂等或冲突可识别。

建议路由：

```text
/login
/users
/users/:id
/activities
/activities/new
/activities/:id
/activities/:id/edit
/activities/:id/registrations
/articles
/articles/new
/articles/:id
/articles/:id/edit
```

## 10. 错误处理

- `401`：清除本地会话并跳转登录页。
- `403`：展示无权限页面；登录阶段收到 `403` 时清除令牌。
- `404`：详情页展示数据不存在，并提供返回列表操作。
- `409`：在表单或操作位置展示冲突原因，例如重复报名。
- `422`：将字段错误映射到具体表单字段；无法映射时显示全局错误提示。
- `500` 和网络错误：显示可重试提示，不丢失当前表单输入。
- 列表请求失败：保留筛选条件并提供重试按钮。
- 写操作失败：不主动关闭 Dialog、Drawer 或编辑页。

后端日志记录请求路径、管理员 ID、目标实体 ID、业务错误码和追踪 ID，但不得记录访问令牌、验证码和完整手机号等敏感信息。

## 11. 数据库索引

为保证列表和重复校验性能，增加以下索引：

### `user`

- `phone` 单字段索引，是否唯一沿用现有生产数据约束，不在本次强制切换。
- `role + status + create_time` 复合索引。
- `delete_time` 单字段索引。

### `activity`

- `status + create_time` 复合索引。
- `start` 单字段索引。

### `register`

- `activity_id + status + create_time` 复合索引。
- `activity_id + check_in + status` 复合索引。
- `activity_id + name + phone + status` 复合索引，用于重复检查；由于旧数据存在 `-1` 和潜在重复记录，第一阶段不设置唯一约束。

### `article`

- `publish_status + delete_time + sort_order + publish_time` 复合索引。
- `create_time` 单字段索引。

索引创建脚本必须可重复执行。上线前先扫描旧数据，避免唯一约束引发部署失败。

## 12. 后端模块边界

沿用现有分层方式：

- `mapper`：实体定义和 MongoDB 访问，不包含 HTTP 逻辑。
- `service`：业务规则、授权后的资源操作和状态转换。
- `controller`：参数绑定、调用 Service、构造统一响应。
- `router`：注册 `/admin/*` 路由和管理员中间件。

新增管理 API 不直接复用缺少权限语义的用户端 Controller。可复用 Mapper 和纯业务函数，但管理入口必须经过管理员中间件。

资讯模块独立为 `article` Mapper、Service 和 Controller。用户端公开查询与管理写操作共享查询逻辑，但使用不同 Controller 和响应模型。

## 13. 测试规范

### 13.1 后端测试

必须覆盖：

- 无 Token、无效 Token、普通用户、停用管理员和正常管理员。
- 用户分页筛选、资料修改、角色修改、自我保护和最后管理员保护。
- 活动创建校验、编辑、软删除、恢复和报名统计。
- 报名新增、重复检查、编辑、软删除、签到和取消签到。
- 资讯字段校验、草稿、发布、下架、软删除和恢复。
- 公开资讯只返回已发布、未删除数据，并符合排序规则。
- 所有列表的分页边界、关键词筛选和软删除过滤。

Service 和 Mapper 编写单元测试，管理 HTTP API 编写集成测试。测试数据必须隔离，不依赖生产数据库。

### 13.2 前端测试

必须覆盖：

- 登录成功、非管理员拒绝和 Token 过期退出。
- 四类数据列表的分页、搜索和筛选。
- 表单必填、时间关系、人数限制和 URL 校验。
- 新增、修改、删除、恢复和状态操作后的缓存刷新。
- 网络失败、无权限、数据不存在和业务冲突展示。
- 危险操作确认框。

使用 Vitest 和 React Testing Library。MUI Data Grid 不测试其内部实现，只测试传入参数、用户操作和业务结果。

## 14. 验收标准

1. 管理员可以通过现有手机号体系登录；普通用户不能访问 `/admin/*`。
2. 用户、活动、报名和资讯的第一阶段管理功能均完整可用。
3. 所有列表使用服务端分页，并支持本文规定的基础筛选。
4. 所有删除均为软删除，支持本文规定的恢复操作。
5. 活动报名统计与有效报名数据一致。
6. 资讯发布后出现在用户端列表，点击能够打开配置的微信公众号文章链接。
7. 页面刷新后登录状态和 URL 中的列表筛选条件得到保留。
8. 所有危险操作均有确认提示。
9. 后端对所有管理接口执行管理员角色校验，不信任前端角色数据。
10. 前后端构建、类型检查和测试全部通过。

## 15. 实施顺序

1. 后端补充用户角色字段、管理员会话接口和管理鉴权中间件。
2. 后端实现用户管理 API。
3. 后端完善活动和报名管理 API。
4. 后端新增资讯实体、管理 API 和用户端公开查询 API。
5. 创建 `alumni-admin` 并完成登录、布局和统一请求层。
6. 按用户、活动、报名、资讯的顺序实现管理页面。
7. 补充测试、接口联调和验收。

前端开发可在管理 API 契约确定后使用 Mock 数据并行进行，但最终验收必须连接真实后端。
