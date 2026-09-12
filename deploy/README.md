# 单服务器部署：Cloudflare Tunnel

本文使用文档示例 IP `203.0.113.10`、域名 `example.com` 和目录 `/opt/tiny-feed`。部署时替换为自己的地址；实际配置写入不提交到 Git 的 `.env.production`。
生产配置是仓库根目录的 `compose.prod.yaml`，Compose 项目名固定为 `tiny-feed-prod`。
MySQL 和后端只在容器网络内通信；前端映射到服务器回环地址 `127.0.0.1:8081` 供排查。
Cloudflare Tunnel 通过出站连接提供 HTTPS 入口，无需开放服务器的入站 80/443。
启用下文的独立上传入口后，视频/封面的上传通过 `upload.example.com` 的 HTTPS 入口直连服务器；网站页面、业务接口和媒体播放仍走 Tunnel。

## 可选整站 HTTP 直连入口

仅在明确接受明文传输时启用。HTTP 不加密登录凭证或文件内容；使用 IP 不代替云服务商要求的备案、接入或访问授权。必须从 `http://203.0.113.10/` 打开整个网站，不能让 HTTPS 页面调用 HTTP 上传地址。

在 `.env.production` 中设置 `DIRECT_HTTP_HOST=203.0.113.10`（只填 IP，不含协议），放通服务器 TCP 80，然后执行：

```bash
docker compose --env-file .env.production -f compose.prod.yaml --profile direct-http build frontend-http
docker compose --env-file .env.production -f compose.prod.yaml --profile direct-http up -d --no-deps frontend-http
docker compose --env-file .env.production -f compose.prod.yaml --profile direct-http up -d --no-deps upload-gateway
curl --fail http://203.0.113.10/healthz
```

需要先启动 MySQL 和后端。网关已有环境变量变化时，上面的命令会重建网关容器，应避开在途上传。仅修改 Caddyfile 时，可以用 `docker compose --env-file .env.production -f compose.prod.yaml exec -T upload-gateway caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile` 平滑加载。

`frontend-http` 使用独立镜像 `tiny-feed-frontend:direct-http`，构建时固定 `VITE_UPLOAD_API_BASE=/api`，不受 HTTPS 上传地址影响。页面、登录、上传、封面和视频均走 IP 同源；API 和 `/static/` 由 Caddy 直接转发到后端，支持媒体 Range 分段读取。文件上限仍为视频 300 MiB、封面 10 MiB，原 HTTPS 前端使用自己的镜像。

IP 地址与原域名不共享浏览器登录状态，需要在 HTTP 入口重新登录。启用前检查健康接口、登录、真实上传和媒体分段响应；如果 IP 也返回云平台阻断页面，需先处理云平台限制，不能仅凭 DNS 或端口连通认定入口可用。

停用时先把 `DIRECT_HTTP_HOST` 改回 `localhost` 并更新网关，再停止 `frontend-http`。默认只匹配 localhost，不启用公网 HTTP 站点。

## 独立 HTTPS 上传入口

1. 在 Cloudflare 为 `upload.example.com` 新增 A 记录 `203.0.113.10`，代理状态选“仅 DNS”。保留根域名现有的 Tunnel 记录。
2. 服务器允许公网 TCP 80/443。`upload-gateway` 使用 Caddy 自动签发、续期可信证书；80 仅用于证书验证和 HTTPS 跳转，文件上传由 443 接收。上传路由保留后端 JWT 鉴权，跨域仅允许 `https://example.com`。
3. 启动网关，等待证书签发并检查：

```bash
docker compose --env-file .env.production -f compose.prod.yaml --profile direct-upload up -d --no-deps upload-gateway
curl --fail https://upload.example.com/healthz
```

4. 在 `.env.production` 设置 `VITE_UPLOAD_API_BASE=https://upload.example.com/api`，重新构建、部署前端。该变量是**构建参数**，仅修改服务器环境文件不会改变已构建的页面。其他 API 仍使用原有同源 `/api`。

```bash
docker buildx build --platform linux/amd64 --load \
  --build-arg VITE_UPLOAD_API_BASE=https://upload.example.com/api \
  -t tiny-feed-frontend:production apps/frontend
```

视频上限为 300 MiB，封面为 10 MiB；网关额外预留 multipart 边界空间。独立入口仅代理视频/封面上传及健康检查，不提供登录、业务数据或文件浏览。上传失败时不会自动退回 Tunnel。

Caddy 证书存储在 `tiny-feed-prod_upload-caddy-data` 卷中，配置存储在 `tiny-feed-prod_upload-caddy-config` 卷中，更新时保留这两个卷。不要用 Cloudflare Origin CA 证书替代公网可信证书，浏览器直连不信任 Origin CA。

回退时先清空 `VITE_UPLOAD_API_BASE` 并重建、热更新前端，再停用独立上传网关；清空配置后页面恢复同源上传与 95 MiB 上限。只恢复环境文件无法回退已加载的前端资源。

## 首次启动

在服务器的项目目录（示例为 `/opt/tiny-feed`）准备 `.env.production`，参考 `.env.production.example`。
三个密码/密钥分别使用 `openssl rand -hex 32` 生成，文件权限设置为 `600`，不提交 Git。

```bash
cd /opt/tiny-feed
docker compose --env-file .env.production -f compose.prod.yaml config --quiet
docker compose --env-file .env.production -f compose.prod.yaml up -d --build
docker compose --env-file .env.production -f compose.prod.yaml ps
```

默认启动 MySQL、后端、前端。Tunnel 使用单独的 `tunnel` profile，在凭证准备好后启用。

## 配置 Cloudflare

域名需要已注册，并按 Cloudflare 提示完成 DNS 接入，站点状态为 Active。
中国内地服务器对外提供网站服务前需按接入商要求完成备案；Tunnel 不代替备案。

在 Cloudflare 控制台进入 **Networking → Tunnels**，为这台服务器创建独立 Tunnel，例如 `tiny-feed-server`。
环境选择 Docker，复制安装命令中 `--token` 后面的 Token。
在服务器交互终端中执行以下命令，在隐藏输入提示中粘贴整条 Docker 安装命令或 Token。脚本只提取和保存 Token，不会执行粘贴的命令：

```bash
bash /opt/tiny-feed/deploy/save-tunnel-token.sh
```

父目录权限 `700` 限制宿主机上的访问，Token 文件通过只读 secret 挂载供非 root 容器读取。

```bash
cd /opt/tiny-feed
docker compose --env-file .env.production -f compose.prod.yaml --profile tunnel up -d --no-build --pull never
```

在 Tunnel 的 **Routes → Add route → Published application** 中设置根域名 `example.com`，子域名和路径留空；Service type 选择 HTTP，Service URL 填 `frontend:80`（合并输入框填 `http://frontend:80`）。
`cloudflared` 与 `frontend` 位于同一 Docker 网络，因此服务地址使用 `frontend:80`。
控制台会自动创建指向 `<Tunnel ID>.cfargotunnel.com` 的 DNS 记录，无需给根域名添加服务器 IP 的 A 记录。
若已有冲突记录，请先检查用途，再按控制台提示处理。

## 服务器无法拉取镜像时

在可以访问 Docker Hub 的电脑上为服务器的架构构建，例如服务器架构为 `linux/amd64`：

```bash
docker buildx build --platform linux/amd64 --load -t tiny-feed-backend:production apps/backend
docker buildx build --platform linux/amd64 --load -t tiny-feed-frontend:production apps/frontend
docker pull --platform linux/amd64 mysql:8.0
docker pull --platform linux/amd64 cloudflare/cloudflared:latest
```

使用 `docker save` 导出这四个镜像，通过 SSH 传输并在服务器上 `docker load`。
同时上传项目代码和部署配置，排除 `.git`、`.env`、`.env.production`、`.secrets`、`node_modules` 和开发上传目录 `.run`；密钥只在服务器生成。
然后在服务器执行：

```bash
cd /opt/tiny-feed
docker compose --env-file .env.production -f compose.prod.yaml up -d --no-build --pull never
```

## 验证

```bash
docker compose --env-file .env.production -f compose.prod.yaml logs --tail=100 cloudflared
curl https://example.com/api/healthz
curl -H 'Content-Type: application/json' -d '{"limit":1}' https://example.com/api/feed/listLatest
```

`/api/healthz` 只验证进程响应；Feed 接口还会访问数据库。用浏览器继续检查登录、上传与播放。

## 更新与数据

先备份数据库和上传文件，再导入新版本的后端、前端镜像，并重新创建后端和前端容器：

```bash
docker compose --env-file .env.production -f compose.prod.yaml up -d --no-build --pull never --force-recreate backend frontend
```

前端 Nginx 需要跟随后端一起重建，以重新解析后端容器地址。单机更新期间会有短暂中断。

持久化卷包括 `tiny-feed-prod_mysql-data` 和 `tiny-feed-prod_backend-uploads`。
持久化卷不是备份：数据库应做一致性逻辑备份，视频/封面应备份上传卷，并将备份保存在服务器之外。
不要运行 `docker compose down -v`，这会删除数据卷。已有 MySQL 数据卷时，修改环境变量不会自动更改数据库账户密码。

排查启动问题：

```bash
docker compose --env-file .env.production -f compose.prod.yaml ps
docker compose --env-file .env.production -f compose.prod.yaml logs --tail=100
docker stats --no-stream
```

Cloudflare 配置参考：[官方步骤](https://developers.cloudflare.com/tunnel/setup/)。

通过 Tunnel 上传时，Cloudflare Free 套餐的单次请求体上限为 100 MB，前端将视频限制在 95 MiB，为表单请求体预留空间；封面限制为 10 MiB。启用独立直连入口后，视频按后端上限 300 MiB 校验，不经过 Cloudflare 的上传限制。目前仍是整文件上传，尚无分片续传。[限制说明](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/4xx-client-error/error-413/)

## 手机浏览器

- 手机导航位于底部，使用安全区留白；页面跟随 VisualViewport 高度变化，兼容地址栏、横竖屏和软键盘。
- 视频默认有声。Safari 等浏览器若拦截首次有声自动播放，页面显示“点击有声播放”，由用户轻点后启动；不强制切成静音。
- 推荐使用 MP4（H.264）视频。MOV 是容器格式，手机拍摄的 HEVC 视频是否可播取决于目标设备的解码支持；当前后端不转码。封面支持 JPG、PNG、WebP，HEIC 照片需先转换。
- 上传页面显示当前文件大小、阶段和百分比；100% 表示浏览器已发送请求体，仍需等待服务器确认，不表示视频已发布。
- 手机文件提供方返回空 MIME 或 `application/octet-stream` 时，前端按已支持的文件扩展名补全类型；服务器仍执行扩展名、类型和大小校验。
- 上传前会为即将过期的登录凭证续期，同时保存服务端返回的新 access/refresh token。续期遇到临时网络错误会保留登录状态；凭证失效时显示重新登录入口。
- 失败原因持续显示在发布页面。同一页面重试封面或发布时复用已上传的视频；更换文件或离开页面后不再复用，这不是跨页面断点续传。
- 已用 Chromium / WebKit 检查 320–430px 手机竖屏、844×390 横屏及桌面布局，覆盖连续滑动、评论输入、登录、发布、401 刷新重试和有声播放。模拟键盘检查不替代 iPhone Safari / vivo 浏览器真机验证。

## 上传回归

`deploy/tests/upload-regression.cjs` 仅允许连接本机地址，会创建隔离测试账号并发布测试视频。使用独立 MySQL、后端和前端测试容器，不得连接生产数据库。测试包括真实上传与文件逐字节比对、连续两次凭证续期、空/通用 MIME、封面失败重试、续期临时失败和凭证失效拦截。仅临时故障响应由浏览器注入，其余接口均使用真实服务。

先准备隔离的 Compose JSON 配置，其中 `services.backend.environment.JWT_SECRET` 是测试密钥；启动服务并将前端绑定到 `127.0.0.1:18082`。安装 Playwright 及 Chromium/WebKit 后执行：

```bash
QA_COMPOSE_FILE=/path/to/test-compose.json \
QA_VIDEO_FILE=/path/to/sample.mp4 \
QA_BROWSER=chromium node deploy/tests/upload-regression.cjs
```

再用 `QA_BROWSER=webkit` 运行。可通过 `QA_PLAYWRIGHT_PATH` 指定 Playwright 模块位置、`PLAYWRIGHT_BROWSERS_PATH` 指定浏览器安装位置、`QA_ARTIFACT_DIR` 保存失败提示截图。测试完成后移除仅供测试的容器和数据。

直连回归时，在隔离环境添加 `upload-gateway` 服务，挂载 `deploy/Caddyfile.upload`，设置 `UPLOAD_SITE_ADDRESS=http://:80`、`UPLOAD_ALLOWED_ORIGIN=http://127.0.0.1:18082`，仅绑定 `127.0.0.1:18083:80`。测试前端以 `VITE_UPLOAD_API_BASE=http://127.0.0.1:18083/api` 构建，再给脚本增加 `QA_UPLOAD_ORIGIN=http://127.0.0.1:18083`。脚本额外检查跨域预检、未登录上传返回 401、其他来源/业务路由被拒绝，以及所有文件请求确实走独立入口。本机 HTTP 配置只用于隔离测试；HTTPS 主站使用的独立上传入口仍须提供 HTTPS。

整站 HTTP 回归：测试容器使用 `frontend-http` 服务名和对应镜像，网关设置 `DIRECT_HTTP_HOST=127.0.0.1`、`UPLOAD_SITE_ADDRESS=http://upload.localhost`，仅映射 `127.0.0.1:18083:80`。给脚本设置 `QA_BASE_URL=http://127.0.0.1:18083`、`QA_UPLOAD_ORIGIN=http://127.0.0.1:18083` 和 `QA_VERIFY_MEDIA=1`，另保持上述测试数据库与视频文件参数。除了上传回归，还验证首页/详情媒体同源、默认有声播放和拖动进度，以及 HEAD、Range、If-Range、尾部范围和无效范围响应。

只有静态前端改动时，可先加载新版镜像，再将新哈希资源复制进运行容器，最后原子替换 `index.html`；保留旧哈希资源供已打开的页面使用。这样无需重启 Nginx，不会因更新中断在途上传。需同时同步源代码与镜像，确保后续重建使用同一版本。
