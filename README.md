# tiny-feed

视频 Feed 应用的极简版。从 [feedsystem_video_go](https://github.com/LeoninCS/feedsystem_video_go) 精简而来：

- **去掉了 Redis 缓存层**：原本用来做限流和点赞计数缓存，简化后直接走 MySQL
- **去掉了 RabbitMQ + Worker 进程**：原本用来做点赞 / 关注 / 评论的异步事件处理，简化后同步写库（用 DB 唯一索引 + 原子 SQL 保证正确性）
- **去掉了 pprof**：开发/排障用，单服务场景不需要
- **去掉了站内信模块**
- **保留最基础的功能**：账号注册/登录、视频发布/查询/删除、Timeline/关注/点赞数/热度/标签 Feed、评论、点赞、关注

后端：Go 1.24 + Gin + GORM + MySQL 8。
前端：Vue 3 + Vite + Pinia + Vue Router。

---

## 方式一：Docker Compose（推荐）

一键起 MySQL + 后端 + 前端：

```bash
docker compose up -d --build
```

打开 http://localhost:8081 即可。

端口：
- `3306` — MySQL
- `8080` — 后端 API（直连调试用）
- `8081` — 前端（nginx 反代到后端）

要彻底重置数据：`docker compose down -v`。

> `JWT_SECRET` 当前是 `please-change-me` 占位值，生产前请改成你自己的固定值。

---

## 方式二：本地直接启动

### 准备 MySQL

```sql
CREATE DATABASE feedsystem DEFAULT CHARSET utf8mb4;
```

后端启动时会自动跑 GORM AutoMigrate 建表。

### 启动后端

```bash
cd apps/backend
go mod download
go run ./cmd
```

环境变量（与 `configs/config.yaml` 等价，env 优先级更高）：

- `MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_DATABASE`
- `SERVER_PORT`（默认 8080）
- `JWT_SECRET`（强烈建议设一个固定值）

### 启动前端

```bash
cd apps/frontend
npm ci
npm run dev
```

开发模式监听 5173，Vite 自动把 `/api/...` 代理到 `http://localhost:8080`。

生产构建：

```bash
npm run build      # 产物在 apps/frontend/dist
```

`apps/frontend/nginx.conf` 是参考 nginx 配置。

---

## 常用接口（POST 形式）

鉴权：受保护接口都要在请求头带 `Authorization: Bearer <token>`。

账号：
- `/account/register`、`/account/login`、`/account/refresh`（用 `X-Refresh-Token` header）、`/account/changePassword`、`/account/findByID`、`/account/findByUsername`、`/account/getProfile`
- 需登录：`/account/logout`、`/account/rename`、`/account/updateProfile`

视频：`/video/listByAuthorID`、`/video/getDetail`，需登录：`/video/publish`、`/video/delete`（仅作者本人能删）

点赞（全部需登录）：`/like/like`、`/like/unlike`、`/like/isLiked`、`/like/listMyLikedVideos`

评论：`/comment/listAll`，需登录：`/comment/publish`、`/comment/delete`（仅作者本人能删）

关注：`/social/getAllFollowers`、`/social/getAllVloggers`，需登录：`/social/follow`、`/social/unfollow`、`/social/getCounts`

Feed：`/feed/listLatest`、`/feed/listLikesCount`、`/feed/listByTag`，需登录：`/feed/listByFollowing`、`/feed/listByPopularity`

健康检查：`GET /healthz`

---

## 部署规格

后端是 Go 静态二进制（alpine 镜像），前端是 nginx 静态文件，工作负载很轻；主要资源消耗在 MySQL。

### 最小（开发 / 个人 / 日活 < 100）

| 组件 | CPU | 内存 | 存储 |
|------|-----|------|------|
| 后端 + 前端（同一台 1C1G 即可） | 1 vCPU | 512 MB | 5 GB |
| MySQL | 1 vCPU | 512 MB | 5 GB |

### 推荐（中等流量，日活 1k-10k）

| 组件 | CPU | 内存 | 存储 |
|------|-----|------|------|
| 后端 + 前端 | 1-2 vCPU | 1 GB | 10 GB |
| MySQL | 2 vCPU | 2 GB | 20 GB SSD |

### 大流量（10k+）

后端可以水平扩展（多实例 + nginx upstream），MySQL 升级到独立机器或托管 RDS，按 QPS 加分库分表（当前没分表，按 `account.id`、`video.id` 范围拆）。

### 软件依赖

- OS：Linux 任意发行版（amd64 / arm64 都行）
- 容器化部署：Docker 20+、Docker Compose v2（推荐）或 Kubernetes
- 直跑后端：glibc 或 musl 兼容的 Linux（alpine 镜像版自带 musl）
- MySQL：8.0+（用 5.7 也行但没测过）
- 端口：3306（MySQL）、8080（后端）、80/443（前端，对外）

### 部署后的健康检查

```bash
curl http://<host>:8080/healthz
# {"status":"ok"}
```

这个端点不依赖 MySQL，用于负载均衡探活。如果 MySQL 挂了，业务接口会报 500，但 `/healthz` 仍然 200——把 LB 健康检查指向 `/healthz`，业务接口 500 时仍然能 200，方便排查到底是哪一层挂了。
