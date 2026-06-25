# 用户端公开资讯接口与移动端接入设计

## 状态

- 日期：2026-06-25
- 状态：已确认并实现
- 范围：`alumni-backend`、`alumni-app`

## 目标

管理端创建并发布资讯后，用户端移动端可以看到同一份真实数据。用户端只读取公开资讯，不复用管理端接口。

## 后端公开接口

新增无需登录的公开资讯接口：

- `GET /articles?page=1&pageSize=10`
- `GET /articles/:id`

查询规则：

- 只返回 `publish_status=published`
- 排除 `deleted=true`
- 排序与管理端一致：`sort_order DESC, publish_time DESC, create_time DESC`

列表响应：

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "pageSize": 10
}
```

详情响应：

```json
{
  "id": "string",
  "title": "string",
  "summary": "string",
  "cover": "string",
  "wechatUrl": "string",
  "source": "string",
  "author": "string",
  "publishTime": 1234567890
}
```

公开接口不返回草稿、下架、删除状态和管理字段。

## 移动端改造

新增资讯 API：

- `src/api/article/article-interface.ts`
- `src/api/article/article.ts`

改造 `src/pages/news/index.vue`：

- 页面进入时加载第一页资讯
- 有数据时展示资讯卡片
- 无数据时保留当前“暂无资讯”空状态
- 点击卡片打开 `wechatUrl`
- H5 使用 `window.open`
- 小程序等非 H5 环境复制链接并提示用户打开

## 移动端请求地址

现有 `src/api/request.ts` 写死线上地址，需要改为环境变量优先：

```ts
const BASE_URL = import.meta.env.VITE_API_BASE_URL || "https://api.xhpolaris.com/alumni";
```

H5 本地开发在 `vite.config.ts` 增加代理：

- `/articles`
- `/activity`
- `/user`
- `/sts`

本地 H5 使用 `.env.local`：

```env
VITE_API_BASE_URL=
```

真机或小程序如需连接本机后端，`VITE_API_BASE_URL` 应设置为电脑局域网 IP，例如 `http://192.168.x.x:8888`。

## 验收

- 管理端发布后的资讯能通过 `GET /articles` 查询到。
- 草稿、下架和删除资讯不会出现在用户端。
- 移动端资讯页能展示真实资讯。
- 点击资讯可跳转或复制微信公众号链接。
