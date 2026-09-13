# 三端改造接口契约（2026-06 分会改造）

本文件是三端改造期间的**唯一接口契约**。服务端、管理端、用户端在实现与联调时以本文为准；
如与服务端代码不一致，视为缺陷，需要修正代码或本文。

所有分会在首期固定为四个：宁波、上海、杭州、北京。分会以 `chapterId`（Mongo ObjectId 字符串）标识，
用户端与管理端都通过 `GET /chapters`、`GET /admin/chapters` 动态获取，不硬编码 ID。

---

## 1. 身份与权限模型

| 字段 | 取值 | 说明 |
| --- | --- | --- |
| `memberRole` | `pending` / `alumni` / `guest` | 用户身份，与旧 `role` 解耦 |
| `adminRole` | `none` / `chapter_admin` / `super_admin` | 管理权限，与身份完全独立 |
| `adminChapterId` | 分会 ID 或 `""` | 仅 `chapter_admin` 有值；**不随个人分会切换而变化** |
| `verificationMethod` | `none` / `roster` / `manual` | 认证方式 |
| `chapterId` | 分会 ID 或 `""` | 个人所属分会，用户可在个人中心切换 |

规则：

1. 分会管理员只能读写 `chapterId == adminChapterId` 的人员、活动、报名、资讯、联络人。
2. 超级管理员可读写全部数据，并可任命/撤销分会管理员；不能通过接口创建 `super_admin`。
3. 只有 `memberRole` 为 `alumni` 或 `guest` 的用户可报名活动；`pending` 与未登录一律拒绝（服务端强校验）。
4. 资讯默认返回全部分会；活动默认按用户所属分会筛选，但用户可切换。

---

## 2. 公共接口（用户端使用，无 `code/message/data` 包装）

### 2.1 `GET /chapters`

返回启用的分会列表：

```json
[
  {
    "id": "6650...",
    "code": "shanghai",
    "name": "上海分会",
    "contact": {
      "name": "李老师",
      "wechat": "sh_alumni",
      "phone": "13800000000",
      "description": "负责上海分会日常联络",
      "qrCodeUrl": "https://.../qr.png"
    },
    "status": 0
  }
]
```

`code` 取值：`ningbo` / `shanghai` / `hangzhou` / `beijing`。

### 2.2 `POST /user/register`

请求：

```json
{
  "authId": "13800000000",
  "authType": "phone",
  "verifyCode": "123456",
  "password": "******",
  "name": "张三",
  "graduationYear": 2018,
  "birthDate": "2000-05-20",
  "chapterId": "6650..."
}
```

响应：

```json
{
  "id": "6651...",
  "accessToken": "...",
  "accessExpire": 1780000000,
  "memberRole": "alumni",
  "autoVerified": true,
  "chapterContact": {
    "id": "6650...",
    "code": "shanghai",
    "name": "上海分会",
    "contact": { "name": "李老师", "wechat": "sh_alumni", "phone": "", "description": "", "qrCodeUrl": "https://.../qr.png" }
  }
}
```

- 姓名（规范化后）+ 毕业年份 + 出生日期三项与校友名册完全匹配：`memberRole=alumni`、`autoVerified=true`、`verificationMethod=roster`。
- 任一项不匹配：注册成功但 `memberRole=pending`、`autoVerified=false`，`verificationMethod=none`。
- 出生日期必须为 `YYYY-MM-DD`；毕业年份为四位年份。

### 2.3 `GET /user/profile`

```json
{
  "id": "6651...",
  "avatar": "",
  "name": "张三",
  "gender": 1,
  "birthday": 958665600,
  "birthDate": "2000-05-20",
  "phone": "13800000000",
  "wxId": "",
  "hometown": "宁波北仑",
  "chapterId": "6650...",
  "chapterName": "上海分会",
  "graduationYear": 2018,
  "memberRole": "alumni",
  "verificationMethod": "roster",
  "profileComplete": true,
  "educations": [
    { "phase": "本科", "school": "同济大学", "year": 2018,
      "provinceCode": "310000", "provinceName": "上海市",
      "cityCode": "310100", "cityName": "上海市" }
  ],
  "employments": [
    { "organization": "澄明设计", "position": "产品设计师", "industry": "科学研究和技术服务业",
      "entry": 2021, "departure": 0,
      "provinceCode": "310000", "provinceName": "上海市",
      "cityCode": "310100", "cityName": "上海市" }
  ]
}
```

`profileComplete` 为 `false` 表示旧客户端注册、缺少分会或认证资料，用户端必须引导补全
（补充分会 + 毕业年份 + 出生日期）。登录接口本身保持旧行为不变。

### 2.4 `PATCH /user/profile`

可更新字段（全部可选，使用指针语义，未传即不修改）：
`avatar`、`name`、`gender`、`phone`、`wxId`、`hometown`、`chapterId`、
`graduationYear`、`birthDate`。

- `chapterId` 变更即“切换分会”，人员归属立即转移。
- `graduationYear` 必须为四位年份（`< 1900` 返回 400）；`birthDate` 必须为 `YYYY-MM-DD`。
- 当用户仍处于 `pending` 且提交后姓名、毕业年份、出生日期三者齐备时，
  服务端会**重新执行一次名册匹配**：命中则升级为 `alumni`（`verificationMethod=roster`），
  未命中保持 `pending`。已认证身份不会被覆盖。
  这是旧客户端注册用户补全资料的唯一入口，无需额外的“重新核验”接口。

返回最新 `GET /user/profile` 结构。

### 2.5 `PUT /user/educations`

```json
{ "educations": [ { "phase": "本科", "school": "同济大学", "year": 2018,
  "provinceCode": "310000", "provinceName": "上海市", "cityCode": "310100", "cityName": "上海市" } ] }
```

这是一次**整体替换**，请求体必须包含调用方希望保留的全部记录。

服务端只接受新增这些学段：`大专`、`本科`、`硕士`、`博士`、`其他`。
判定规则（`validateEducationPhases`）：提交项要么是上述白名单学段，
要么与库中**已存在**的记录（学段 + 学校 + 毕业年份）完全一致；
否则返回 400。因此：

- 新增中小学记录会被拒绝；
- 已存在的中小学记录可以原样提交并保留；
- 客户端若想把旧记录清掉，只需在请求体中不再包含它们（整体替换语义）。

服务端不会自动删除旧中小学记录，它们会继续通过 `GET /user/profile` 返回。
返回最新 `GET /user/profile` 结构。

**行政区划约定**：`provinceCode` / `cityCode` 使用 GB/T 2260 六位代码，
名称使用简称（`浙江省`、`杭州市`）。直辖市（北京/上海/天津/重庆）的市层使用市辖区
（如 `310104 徐汇区`）；旧数据迁移时上海记录预填 `310100 上海市`。
香港、澳门没有地级代码，使用约定的分组代码（`810100 香港岛` 等）。
代码与名称必须成对提交，服务端不做校验也不做反查，因此客户端随包发布的
`regions.ts` 是唯一的行政区划来源。

### 2.6 `PUT /user/employments`

```json
{ "employments": [ { "organization": "澄明设计", "position": "产品设计师", "industry": "...",
  "entry": 2021, "departure": 0,
  "provinceCode": "310000", "provinceName": "上海市", "cityCode": "310100", "cityName": "上海市" } ] }
```

返回最新 `GET /user/profile` 结构。

### 2.7 `GET /articles?chapterId=&page=1&pageSize=10`

不传 `chapterId` 返回全部分会。响应：

```json
{ "items": [ { "id": "...", "title": "...", "summary": "...", "cover": "...",
  "wechatUrl": "...", "source": "...", "author": "...",
  "chapterId": "6650...", "chapterName": "上海分会", "publishTime": 1780000000 } ],
  "total": 12, "page": 1, "pageSize": 10 }
```

### 2.8 `GET /articles/:id`

单个 `Article`（结构同上）。

### 2.9 `GET /activities?chapterId=&page=1&pageSize=10`

```json
{ "items": [ { "id": "...", "cover": "", "name": "...", "location": "上海",
  "exactLocation": "{\"name\":\"...\"}", "sponsor": "...",
  "chapterId": "6650...", "chapterName": "上海分会",
  "start": 1780000000, "registerStart": 1779000000, "registerEnd": 1779500000,
  "description": "...", "contact": "...", "limit": 50, "status": 0,
  "registrationCount": 12 } ],
  "total": 3, "page": 1, "pageSize": 10 }
```

### 2.10 `GET /activities/:id`

`{ ...PublicActivity, registrationCount }`（同上单条结构）。

### 2.10.1 `POST /upload`（图片上传）

用于替换「粘贴图片 URL」的输入方式。**需要登录**（与其它需鉴权接口一致的
`Authorization`；dev 演示身份同样可用）。

- 请求：`multipart/form-data`
  - `file`（必填）：图片文件
  - `scope`（可选）：仅用于在存储路径上加一层可读前缀，服务端会严格清洗
    （只保留小写字母/数字/短横线，最长 24 字符）；建议值：`avatar`、`activity`、
    `article`、`chapter`
- 限制：单文件最大 5 MB；只接受 **PNG / JPG / WEBP / GIF**。
  类型由**内容嗅探**判定，扩展名也据此生成，不信任客户端文件名。
- 响应：**裸 JSON**（与其它公共接口一致，不套 `code/message/data`）：

```json
{
  "url": "http://localhost:8888/files/202609/chapter/57e17923-....png",
  "key": "202609/chapter/57e17923-....png",
  "size": 361,
  "contentType": "image/png"
}
```

拿到 `url` 后直接写入业务字段（`avatar` / `cover` / `qrcodeUrl`）即可，无需再处理。

错误：`400`（未选文件 / 非图片 / 超过大小，`{code,message}`）、`401`（未登录）、
`500`（写盘失败）。

### 2.10.2 `GET /files/<key>`

托管已上传的图片。无需登录（与对象存储的公开读一致），
返回 `Cache-Control: public, max-age=31536000, immutable`（文件名含 uuid，内容不可变）。
含 `..` 的路径一律 404。

> **线上说明**：生产环境更推荐客户端直传对象存储 —— 现有 `POST /sts/apply`
> 会返回带签名的 PUT URL。`/upload` 是本地联调可用的兜底实现（存本机磁盘 +
> `/files` 静态托管），生产可继续使用 `/sts/apply`，或把 `/upload` 换成 COS 驱动。

### 2.11 旧接口（保持兼容）

`POST /user/sign_up`、`POST /user/sign_in`、`GET /user/info`、`POST /user/update_info`、
`POST /user/update_edu`、`POST /user/update_employment`、`POST /activity/get_many`、
`POST /activity/get`、`POST /activity/register`、`POST /activity/check_in`、
`POST /activity/get_register`、`POST /activity/create`、`POST /activity/update`。

旧客户端注册的用户落在 `memberRole=pending`；下次登录后通过 `GET /user/profile` 的
`profileComplete=false` 感知并强制补全。`POST /activity/register` 已强制校验身份。

---

## 3. 管理端接口（统一 `{code,message,data}` 包装，`code=0` 表示成功）

所有 `/admin/*` 请求必须带 `Authorization`。分会作用域由服务端强制：
分会管理员的查询自动追加 `chapter_id == adminChapterId`，跨分会写入返回 403。

### 3.1 会话

`GET /admin/session`

```json
{ "id": "6651...", "name": "李老师", "avatar": "", "phone": "13800000000",
  "role": "admin", "adminRole": "chapter_admin",
  "adminChapterId": "6650...", "adminChapterName": "上海分会" }
```

超级管理员 `adminRole=super_admin`、`adminChapterId=""`、`adminChapterName=""`。

### 3.2 人员

`GET /admin/users?page=1&pageSize=20&keyword=&chapterId=&memberRole=&verificationMethod=&graduationYear=&status=`

- `memberRole`：`pending` / `alumni` / `guest`（兼容旧值 `user` → `pending`）。
- `verificationMethod`：`none` / `roster` / `manual`。
- `graduationYear`：四位年份。
- `status`：`0` 正常 / `1` 停用 / `deleted` 已删除。
- 分会管理员的 `chapterId` 参数被忽略（服务端强制本分会）。

`data` 为 `{items:[AdminUser], total, page, pageSize}`：

```json
{ "id": "...", "avatar": "", "name": "张三", "gender": 1, "birthday": 958665600,
  "phone": "13800000000", "wxId": "", "hometown": "宁波北仑",
  "chapterId": "6650...", "chapterName": "上海分会", "graduationYear": 2018,
  "memberRole": "alumni", "adminRole": "none", "adminChapterId": "",
  "verificationMethod": "roster",
  "educations": [ { "phase": "本科", "school": "同济大学", "year": 2018,
    "provinceCode": "310000", "provinceName": "上海市", "cityCode": "310100", "cityName": "上海市" } ],
  "employments": [ ... ],
  "status": 0, "deleted": false, "createTime": 1780000000 }
```

`GET /admin/users/:id` → 单个 `AdminUser`。

`PATCH /admin/users/:id`（`AdminUserUpdate`，全部可选）：

```json
{ "avatar": "", "name": "张三", "gender": 1, "birthday": 958665600, "phone": "138...",
  "wxId": "", "hometown": "宁波北仑", "chapterId": "6650...",
  "educations": [ ... ], "employments": [ ... ] }
```

`PATCH /admin/users/:id/role` → `{ "role": "alumni" | "guest" | "pending" }`
（用户身份；改为 `alumni`/`guest` 时 `verificationMethod=manual`，`verifiedBy` 记当前管理员）。

`PATCH /admin/users/:id/status` → `{ "status": 0 | 1 }`。
`DELETE /admin/users/:id`、`POST /admin/users/:id/restore`。

### 3.3 管理员配置（仅超级管理员）

`GET /admin/admins` → `AdminUser[]`（`adminRole != none` 的用户）。

`POST /admin/admins/assign`

```json
{ "userId": "6651...", "adminRole": "chapter_admin" | "none", "adminChapterId": "6650..." }
```

- `adminRole=chapter_admin` 时 `adminChapterId` 必须存在且有效。
- `adminRole=none` 为撤销，`adminChapterId` 被清空。
- 分会管理员调用返回 403；无法通过本接口产生 `super_admin`。

### 3.4 分会

`GET /admin/chapters` → `Chapter[]`（含 `contact`，含停用分会）。
分会管理员只返回本分会。

`PATCH /admin/chapters/:id/contact`

```json
{ "name": "李老师", "wechat": "sh_alumni", "phone": "138...", "description": "...", "qrCodeUrl": "https://..." }
```

分会管理员只能改本分会，跨分会 403。

### 3.5 校友名册（仅超级管理员）

`GET /admin/roster?page=1&pageSize=20`

```json
{ "items": [ { "id": "...", "name": "张三", "normalizedName": "张三",
  "graduationYear": 2018, "birthDate": "2000-05-20", "createTime": 1780000000 } ],
  "total": 120, "page": 1, "pageSize": 20 }
```

`POST /admin/roster/import`，`multipart/form-data`，字段名 `file`，接受 `.xlsx` 或 UTF-8 `.csv`，
表头固定为 `姓名,毕业年份,出生日期`（顺序固定，必须完全一致）。

```json
{ "created": 100, "updated": 15, "duplicates": 2, "failed": 1,
  "errors": [ { "row": 7, "message": "姓名或毕业年份格式不正确" } ] }
```

`row` 为文件中的行号（表头为第 1 行）。同一文件内重复记录计入 `duplicates`；
数据库唯一索引为 `(normalized_name, graduation_year, birth_date)`；未出现在新文件中的旧记录不删除。
名册只保存出生日期，不保存身份证号。

### 3.6 活动

- `GET /admin/activities?page=&pageSize=&keyword=&status=&chapterId=`
- `GET /admin/activities/:id`
- `POST /admin/activities`
- `PATCH /admin/activities/:id`
- `DELETE /admin/activities/:id`
- `POST /admin/activities/:id/restore`

```json
{ "id": "...", "cover": "", "name": "夏日交流会", "location": "上海", "exactLocation": "...",
  "sponsor": "...", "chapterId": "6650...", "chapterName": "上海分会",
  "start": 1780000000, "registerStart": 1779000000, "registerEnd": 1779500000,
  "description": "...", "contact": "...", "limit": 50,
  "status": 0, "deleted": false,
  "registrationCount": 12, "checkInCount": 3, "createTime": 1780000000 }
```

写入体（`AdminActivityInput`）：`cover, name, location, exactLocation, sponsor, chapterId,
start, registerStart, registerEnd, description, contact, limit`（时间字段为秒级时间戳）。
分会管理员提交的 `chapterId` 被服务端覆盖为 `adminChapterId`。

**管理端活动接口不再使用旧的 `/activity/create`、`/activity/update`。**

### 3.7 报名

- `GET /admin/registrations?page=&pageSize=&activityId=&chapterId=&keyword=&checkIn=`
- `POST /admin/registrations`（`activityId, userId, name, phone`）
- `PATCH /admin/registrations/:id`（`userId, name, phone`）
- `DELETE /admin/registrations/:id`
- `POST /admin/registrations/:id/check-in`、`POST /admin/registrations/:id/cancel-check-in`

列表 `data` 为 `{items:[AdminRegistration], total, page, pageSize, checked}`：

```json
{ "id": "...", "activityId": "...", "activityName": "夏日交流会",
  "chapterId": "6650...", "chapterName": "上海分会",
  "userId": "...", "name": "张三", "phone": "138...",
  "checkIn": false, "checkInTime": null, "deleted": false, "createTime": 1780000000 }
```

`activityId` 与 `chapterId` 都可选；都不传时返回本分会（超级管理员为全部）报名。
报名数据的分会取自活动所属分会，禁止用其他分会的活动 ID 绕过权限。

### 3.8 资讯

- `GET /admin/articles?page=&pageSize=&keyword=&status=&chapterId=`
- `GET /admin/articles/:id`、`POST /admin/articles`、`PATCH /admin/articles/:id`、
  `DELETE /admin/articles/:id`、`POST /admin/articles/:id/restore`、
  `POST /admin/articles/:id/publish`、`POST /admin/articles/:id/offline`

`AdminArticle` 增加 `chapterId`、`chapterName`。写入体必须带 `chapterId`（分会管理员由服务端覆盖）。

---

## 4. 错误约定

| 场景 | HTTP | `message` |
| --- | --- | --- |
| 未登录 / token 失效 | 401 | 登录已失效 |
| 身份或分会越权 | 403 | 当前账号无管理权限 / 当前身份无权执行此操作 |
| 参数错误 | 400 | 请求参数错误 |
| 资源不存在 | 404 | 资源不存在 |
| 服务异常 | 500 | 服务异常 |

旧 `core_api` 接口仍使用 `{code,msg}` 业务错误码，不做改动。
