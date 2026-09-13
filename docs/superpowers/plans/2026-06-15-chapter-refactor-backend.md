# 三端改造 · 服务端实现报告

对应方案：`仑中校友会平台三端改造方案`。
接口契约：`docs/superpowers/specs/2026-06-15-chapter-refactor-api-contract.md`（唯一权威来源）。

本轮在已有的分会改造半成品之上补齐了数据模型、权限中间件、筛选能力、
自动认证、生日短信、可测试性与观测埋点。全部改动集中在 `alumni-backend`。

---

## 1. 数据模型

| 集合 | 状态 | 说明 |
| --- | --- | --- |
| `chapter` | 已有，本轮补索引 | `code` 唯一索引；启动时 `EnsureDefaults` 初始化宁波/上海/杭州/北京四个分会 |
| `user` | 扩展 | `chapter_id`、`graduation_year`、`birth_date`、`member_role`、`admin_role`、`admin_chapter_id`、`verification_method`、`verified_at`、`verified_by`、`educations`（省市代码+名称）、`employments`（省市代码+名称） |
| `alumni_roster` | 已有，本轮补唯一索引 | `(normalized_name, graduation_year, birth_date)` 唯一复合索引 |
| `birthday_sms_log` | 已有，本轮补唯一索引 | `(user_id, year)` 唯一索引，保证一年只发送一次 |
| `activity` / `article` / `register` | 扩展 | 均带 `chapter_id`，本轮补索引 |

`EnsureIndexes` 由各 mapper 暴露，`provider.Init` 在初始化分会与执行迁移之前统一调用。
索引失败只记录不阻塞启动（首次部署时集合可能尚不存在）。

**管理员的分会范围与个人分会彻底分离**：`admin_chapter_id` 只在任命时写入，
用户在自己的个人中心切换 `chapter_id` 不会影响管理范围。

## 2. 身份、认证与权限

- 身份 `member_role`：`pending` / `alumni` / `guest`；权限 `admin_role`：`none` / `chapter_admin` / `super_admin`。两套字段完全独立。
- `effectiveMemberRole` / `effectiveAdminRole` 负责旧 `role` 字段的兼容读取，
  `MigrateLegacy` 在启动时把旧数据固化为新字段：
  `user → pending`、`alumni → alumni`、`guest → guest`、
  `admin → member_role=alumni + admin_role=super_admin`（保持原有全局权限）。
- 旧上海教育经历迁移时预填 `310000/上海市` 与 `310100/上海市`，其余旧记录省市留空，旧中小学记录不删除。
- **分会权限中间件** `requireScope` 覆盖人员、活动、报名、资讯、分会联络人的全部读写路径；
  越权返回 403 并写出 `metric=admin_scope_rejected` 日志。
- 分会管理员的列表查询由服务端强制追加 `chapter_id == admin_chapter_id`，
  前端传入的 `chapterId` 被忽略（`ListUsers`、`ListActivities`、`ListArticles`、`ListRegistrations`、`ListChapters`）。
- 报名数据的分会归属来自**活动**（`register.chapter_id = activity.chapter_id`），
  因此无法用其他分会的活动 ID 读取或操作报名。
- 管理员配置：`GET /admin/admins` + `POST /admin/admins/assign`。
  仅超级管理员可调用；接口无法产生 `super_admin`；已存在的超级管理员不可被该接口降级或覆盖。

## 3. 注册与自动认证

`POST /user/register`（`UserService.RegisterProfile`）：

1. 校验手机号注册、验证码、密码、姓名、四位毕业年份、`YYYY-MM-DD` 出生日期、有效的启用分会。
2. 调用中台 `sign_up` + `set_password`，落地本地用户。
3. 名册匹配：姓名仅做首尾空格清理 + Unicode NFC 规范化（`normalizeRosterName`），**不做模糊匹配**；
   与 `graduation_year`、`birth_date` 三项完全一致才命中。
4. 命中 → `member_role=alumni`、`verification_method=roster`、`verified_at=now`；
   未命中 → 注册成功但 `member_role=pending`、`verification_method=none`。
5. 返回会话、`memberRole`、`autoVerified`、`chapterContact`（含联络人姓名、说明、微信号、二维码）。

`GET /user/profile` 返回 `profileComplete`（`chapter_id != "" && graduation_year > 0 && birth_date != ""`）。
旧客户端注册的用户因此可以在登录后被用户端识别并强制补全。
`PATCH /user/profile` 接受 `graduationYear` / `birthDate`，并在用户仍处于 `pending` 时
重新执行名册匹配（`applyRosterVerification`）——填错一次不会永久停留在待认证，
已认证身份也不会被覆盖。

## 4. 教育经历与工作经历

- 教育经历合并为 `educations`，工作经历为 `employments`，两者都保存省市**代码与名称**。
- `PUT /user/educations` 的学段白名单为 `大专`/`本科`/`硕士`/`博士`/`其他`。
  兼容策略（`validateEducationPhases`）：提交项只要是白名单学段，
  或与库中已存在的记录（学段+学校+毕业年份）完全一致，即通过；
  这意味着新增中小学记录被拒绝，而旧记录可以原样往返、由用户或管理员清理。
- 旧的 `home_educations` / `shanghai_educations` 字段保留读取兼容，
  `GET /user/profile` 与 `GET /admin/users/:id` 在 `educations` 为空时回退返回旧记录。

## 5. 名册导入

`POST /admin/roster/import`（仅超级管理员，`multipart/form-data`，字段 `file`）：

- 支持 `.xlsx`（内置 zip + XML 解析，含共享字符串与内联字符串）与 UTF-8 `.csv`（兼容 BOM）。
- 表头必须严格为 `姓名,毕业年份,出生日期`；出生日期支持 `YYYY-MM-DD` 与 `YYYY/MM/DD`，以及 Excel 序列号。
- 文件内重复（规范化姓名+年份+出生日期）计入 `duplicates` 并给出行号；
  格式错误计入 `failed`。批量 upsert，未出现在新文件中的旧记录不删除。
- 返回 `{created, updated, duplicates, failed, errors:[{row,message}]}`，行号为文件实际行号（表头为第 1 行）。
- 名册只保存出生日期，不采集或保存完整身份证号。
- 唯一索引是自动认证的安全底线：同一三项组合不可能存在两条记录，`Match` 因此要求命中数恰为 1。

## 6. 生日短信

- 候选人群由 `user.BirthdayCandidatesFilter()` 定义：`member_role=alumni`、
  状态正常、未软删除、手机号有效。嘉宾、待认证、停用、已删除用户一律排除。
  该筛选条件被抽成独立函数，以便在不连接 MongoDB 的情况下测试。
- 每天北京时间 09:00 由 `BirthdayService.Start` 触发 `Run`。
- 生日按身份证公历日期（`birth_date`）计算，回退到旧 `birthday` 时间戳；
  2 月 29 日在非闰年于 2 月 28 日发送（`birthdayMatches`）。
- 发送通过 `sms.Sender` 接口，实现为 `sms.PlatformSender`（调用中台模板短信接口）。
  请求携带 `appId`、`phone`、`templateCode`、`params.name` 与幂等键 `birthday:<userId>:<year>`；
  中台需返回 `messageId` 与状态，`messageId` 落库供排查。
- 单用户单日最多 3 次尝试（首次 + 两次重试），次数持久化在发送日志中；
  次数用尽后当天不再发送，且**不会**被误标记为已发送（`ErrBirthdayAttemptsExhausted`）。
- 同一 `(user_id, year)` 已有 `sent` 记录时直接跳过，重复执行不会重复发送。

## 7. 可测试性重构

服务层原先直接依赖具体的 `*MongoMapper`，无法脱离 MongoDB 测试。本轮：

- 为 `user`/`chapter`/`roster`/`birthday`/`article` 补齐 mapper 接口（`activity`/`register` 已有）。
- 服务层字段改为窄接口，`provider.InfrastructureSet` 用 `wire.Bind` 绑定具体实现。
- 新增 `adaptor.WithUserID` / `WithUserMeta` 身份注入口：
  认证中间件与测试都通过同一入口提供已解析身份，避免重复解析 JWT，
  也让分会权限逻辑可以在没有 JWT 私钥的情况下被完整覆盖。
- `provider/wire_gen.go` 为手工同步（`wire` 生成结果一致）；本环境无法联网运行 `wire`，
  但 `provider.go` 的 provider/bind 集合已更新，`make wire` 可重现同样结果。

## 8. 观测埋点

| 日志键 | 含义 |
| --- | --- |
| `metric=admin_scope_rejected` | 越权拒绝数（含管理员、其分会、目标分会） |
| `metric=roster_import` | 名册导入新增/更新/重复/失败数 |
| `metric=birthday_sms` | 生日短信候选数、成功数、失败数、重复发送拦截数 |

待认证积压可由 `GET /admin/users?memberRole=pending` 的 `total` 直接得到。

## 8.1 图片上传

`POST /upload`（multipart，字段 `file` + 可选 `scope`）+ `GET /files/<key>` 静态托管。

- **为什么不用现成的 `/sts/apply`**：那是客户端直传对象存储的路径，依赖 `platform.sts`
  的 Kitex RPC，本地没有服务发现，跑不通。因此 `/upload` 提供「存本机磁盘 + `/files` 托管」
  的本地兜底，接口形状（拿到 `url` 直接写业务字段）与直传方案一致；
  线上仍可继续用 `/sts/apply`，或把驱动换成 COS。契约里已写明这一点。
- **安全**：类型由**内容嗅探**（`http.DetectContentType`）判定，扩展名据此生成，
  不信任客户端文件名；`scope` 会被清洗（只留小写字母/数字/短横线，≤24 字符）；
  `GET /files` 显式拒绝含 `..` 的路径，并对最终绝对路径再做一次前缀校验（两道防线）。
- **默认值**：`Upload.Dir=./output/upload`、`MaxBytes=5MB`、`PublicBaseURL` 留空时
  返回相对路径 `/files/xxx`。三个字段都标了 `,optional`。
- 测试：`biz/application/service/upload_test.go` 覆盖落盘与 URL 形状、scope 清洗、
  目录穿越拒绝、非图片/空文件/超限拒绝、默认目录与相对 URL 兜底。

## 9. 测试

`go test ./...` 全绿，`go vet ./...` 干净。`biz/application/service` 下共 **44** 个用例：

- `chapter_scope_test.go` — 分会管理员不能读取/修改/删除其他分会的人员、活动、报名、联络人（含伪造 ID）；
  看不到管理员配置与名册；不能任命管理员；停用与非管理员被拒；
  超级管理员可见全部并可任命/撤销分会管理员；不能任命 `super_admin`；
  人员列表的身份、认证方式、毕业年份筛选与旧角色值兼容；认证操作写入 `manual` 与审计信息。
- `identity_test.go` — 报名身份门控（未登录/待认证拒绝，校友/嘉宾通过，身份缺失按待认证处理）、
  报名时间窗与活动状态、名额上限、报名继承活动分会；
  名册命中自动成为校友、未命中保持待认证、已认证不被降级；
  毕业年份与出生日期非法值被拒；教育阶段白名单与旧记录放行；
  名册导入计数与逐行行号、表头/扩展名校验、斜杠日期规范化；
  生日闰年规则、同年度不重复发送、单日重试上限、用尽后不误判为已发送、候选人筛选排除规则、发送回执落库；
  公开活动按分会筛选与下架过滤。

覆盖方式为内存版 mapper（`fakes_test.go`）与内存短信发送器，不依赖 MongoDB、Redis 或短信中台。

## 10. 发布顺序与开关

1. 服务端（含迁移）→ 2. 管理端 → 3. 用户端 → 4. 开启生日定时任务。

生日任务由 `BirthdaySms.Enabled` 控制，默认 `false`；`Endpoint`、`APIKey`、`TemplateCode`
在 `etc/config.yaml` 配置。中台侧模板短信接口需要保证幂等键生效并返回 `messageId`。

## 11. 契约偏差与遗留

- 旧 `core_api`（`/user/sign_up`、`/activity/*` 等）保持原样，仍返回 `{code,msg}`；
  新接口统一使用 `{code,message,data}` 包装（`/admin/*`）或直接返回数据（公开接口）。
- `GET /user/info` 与 `POST /user/update_edu` 等旧资料接口未接入 `educations` 新模型，
  仅新接口（`GET/PATCH /user/profile`、`PUT /user/educations`）使用新模型；
  这是刻意的兼容取舍，用户端迁移完成后可下线旧接口。
- 生日短信的“中台模板短信能力”由后端按约定调用实现，中台实现不在本仓库内。
