# Pier 栈桥

轻量的单机应用部署控制台。Vue 3 + TypeScript + Arco Design Vue，Go 后端；提取 upy 的发布引擎，提供用户名密码登录和统一 Web 管理。

第一版面向一台服务器上的多个应用。项目独立构建，不依赖原 upy 源码目录，也不需要安装 upy。参考 perf-server 的 `server/ + web/` 组织方式，保留分层结构，省去 Redis、外部数据库、RBAC 和第三方登录。

## 功能

| 功能 | 实现 |
| --- | --- |
| 登录 | 首次创建管理员、bcrypt 密码哈希、HttpOnly 会话 Cookie、改密、退出、登录限速 |
| 项目 | 创建、编辑、搜索、按类型筛选、移除登记（保留磁盘产物） |
| 发布 | GitHub Release 指定版本 / latest、本地 ZIP / 加密包上传、强制重发 |
| 部署引擎 | 静态网站、Docker 单容器、Docker Compose、systemd 二进制服务 |
| 版本 | 当前/上一版本、历史版本、手动回退、激活或健康检查失败自动尝试回滚 |
| 任务 | 后台执行、每项目互斥、日志自动刷新/下载、任务状态持久化、重启后标记待确认 |
| 运维 | 备份脚本执行、归档下载与轮转、磁盘统计、Bundle 缓存/旧版本/临时目录清理 |
| 自动化 | 项目级 Bearer / GitHub HMAC Webhook、Bark / 出站 Webhook 通知 |
| 加密 | upy 兼容的 AES-256-GCM 包解密，独立 `pier-crypt` 加解密工具 |

控制台使用真实 API 数据，不预置项目或伪造发布记录。已部署状态代表成功发布的记录，不是持续健康监控。

## Docker 部署（推荐）

默认使用 `ghcr.io/crayon-hua/pier:latest`，无需安装 Go、Node 或 upy。在服务器上保存以下内容为 `compose.yaml`：

```yaml
services:
  pier:
    image: ghcr.io/crayon-hua/pier:latest
    container_name: pier
    restart: unless-stopped
    networks:
      - whaee-hub
    ports:
      - "18099:8099"
    volumes:
      - /data/pier:/data/pier
    environment:
      TZ: Asia/Shanghai
    stop_grace_period: 5m

networks:
  whaee-hub:
    external: true
```

启动（已有 `whaee-hub` 网络时跳过创建）：

```sh
docker network inspect whaee-hub >/dev/null 2>&1 || docker network create whaee-hub
docker compose pull
docker compose up -d
```

访问 `http://服务器IP:18099`，首次创建管理员用户名与密码，没有默认密码。管理员、配置和日志保存在 `/data/pier/state`，部署版本、持久化数据和备份保存在 `/data/pier/apps`，重建容器会保留这些数据。

更新到最新镜像：

```sh
docker compose pull
docker compose up -d
```

`latest` 在 main 分支构建或正式版本发布成功后更新，预发布 tag 不更新它。也可将镜像标签固定为已发布的版本，例如 `:v1.0.0`。镜像首次发布后需在 GitHub Packages 中设为 Public，才能匿名拉取；私有镜像需要先执行 `docker login ghcr.io`。

若要让 Pier 管理宿主机 Docker / Compose 应用，克隆本仓库后使用：

```sh
docker compose -f compose.yaml -f compose.docker.yaml up -d
```

覆盖文件会挂载 Docker socket，并配置宿主机健康检查地址。宿主机与容器内的项目路径须保持一致，因此示例使用 `/data/pier:/data/pier`。容器版不管理宿主机 systemd；完整说明见 [镜像发布与 Compose 部署](docs/docker.md)。

## 源码启动与开发

需要 Go 1.26+、Node.js 22.12+。实际服务部署推荐 Linux；macOS 可用于静态/Docker 部署。Windows 可运行控制台和前后端开发，但静态部署需要系统允许创建符号链接，systemd 仅支持 Linux，备份脚本需要 Unix 环境。

在项目根目录安装前端依赖并构建：

```sh
cd web
npm ci
cd ..
node scripts/build.mjs
```

启动后端（默认监听 `127.0.0.1:8099`）：

```sh
cd server
go run ./cmd/server
# 或运行已构建的 bin/pier（Windows 为 bin/pier.exe）
```

打开 <http://127.0.0.1:8099>。首次打开设置管理员用户名与密码，密码至少 12 字节、最多 72 字节。没有默认密码。

开发时可在另一个终端启动 Vite：

```sh
cd web
npm run dev
```

访问 <http://127.0.0.1:5317>，`/api` 自动代理到 Go 后端。修改后端端口时同步修改 `web/vite.config.ts` 的代理地址。

## 第一次部署

1. 在控制台新建项目，标识填 `hello-pier`，类型选“静态站点”，GitHub 仓库留空。
2. 在项目根目录运行 `node scripts/bundle-example.mjs`，生成 `examples/hello-pier.zip`。
3. 进入项目，点击“发布新版本”，上传该 ZIP。
4. 从任务日志确认发布结果。控制台管理部署，业务站点由 Nginx/Caddy 提供访问。

示例包布局：

```text
bundle.zip
├── upy.yaml
└── dist/
    └── index.html
```

```yaml
name: hello-pier        # 必须与控制台项目标识一致
version: v1.0.0
type: static
static:
  directory: dist
```

新版本修改 `version` 后重新打包上传。GitHub 版本发布时，清单版本必须与 Release tag 一致（兼容有无 v 前缀）。包内清单兼容 `upy.yaml`、`manifest.yaml`、`bundle.yaml`、`deploy.yaml`，优先读取前者。

Docker / systemd 清单见 `examples/docker.upy.yaml`、`examples/system.upy.yaml`。Compose 包中需附带 Compose 文件；单容器包需附带 Dockerfile 和构建文件；system 包需附带目标 Linux 架构的可执行文件并保留执行权限。

## 项目高级配置

项目设置中的 YAML 支持 upy 的以下配置。`name` 与 `release` 由表单统一生成。私有仓库 Token 单独保存，API 不回传原 Token；留空保留，勾选“清除已有 Token”才会删除。

```yaml
bundles:
  keep: 3
service:
  user: my-api
  group: my-api
  unit: upy-my-api.service
crypto:
  password: your-encrypted-bundle-password
backup:
  script: backup.sh
  retention: 7
  timeout: 15m
notify:
  bark:
    - your-device-key
webhook:
  secret: use-a-random-secret-of-at-least-24-bytes
```

这些配置可能含解密密码或通知密钥，仅面向管理员。systemd 用户必须预先存在，Pier 不会自动以 root 代替。Docker/Compose 和 systemd 切换可能停止旧服务，不承诺零停机。

把 `examples/backup.sh` 放到项目根目录并赋予执行权限，按需调整备份内容。脚本使用 `UPY_PROJECT_ROOT`、`UPY_BACKUP_DIR`、`UPY_CURRENT_VERSION`、`UPY_TIMESTAMP` 等变量，输出到 `backups/`。环境变量/持久化数据仍在服务器上维护：Docker 的 `shared/.env`，systemd 的 `shared/service.env` 和 `shared/data/`。

当前和上一个版本会受清理保护；成功发布后自动裁剪更早的版本。手动回退是重新部署本机保留的版本；它不会回退数据库或持久化数据。

## 服务配置和目录

复制 `server/configs/local.yaml.example` 为 `server/configs/local.yaml`：

```yaml
listen: 127.0.0.1:8099
data_dir: ./data
projects_dir: ./apps
web_dir: ../web/dist
secure_cookie: false
max_upload_mb: 256
```

在 `server/` 下执行 `go run ./cmd/server -config configs/local.yaml`。相对路径按进程工作目录解析。

`PIER_LISTEN`、`PIER_DATA_DIR`、`PIER_PROJECTS_DIR`、`PIER_WEB_DIR` 可覆盖配置。首次部署也可通过 `PIER_ADMIN_USERNAME` / `PIER_ADMIN_PASSWORD` 环境变量初始化管理员，已经初始化后不会覆盖账户。

```text
server/
├── data/                         # 私有元数据，目录 0700
│   ├── pier.json                 # 管理员哈希、项目配置和任务记录，0600
│   ├── pier.pid
│   ├── logs/<task-id>.log
│   └── uploads/                  # 上传暂存，任务结束后删除
└── apps/<project-id>/            # 可独立配置到 /srv/pier/apps
    ├── deploy.yaml               # 服务端配置，0600
    ├── state.json
    ├── current -> releases/...
    ├── releases/<version>/
    ├── bundles/
    ├── shared/
    └── backups/
```

私有数据与业务目录分开，便于 Nginx 访问静态文件而不开放凭据目录。反代 `current` 时，确保所有父目录可被 Web 服务用户穿越。

生产部署用 HTTPS 反向代理，设置 `secure_cookie: true`，代理保留原始 `Host`。`/api` 的请求体上限也应在代理层匹配。初始设置应在本机或受控访问下完成，再对外开放控制台。服务进程只需要被管理应用对应的权限；systemd 操作需要适当的服务管理权限。

`scripts/build.mjs` 会把前端嵌入 Go 二进制，发布后不需要 Node。重新构建前端后需重新构建二进制以更新嵌入内容。配置的 `web_dir` 若有 `index.html` 则优先使用该目录。

## Webhook 和加密

配置项目 `webhook.secret` 后，CI 可以发送：

```sh
curl -X POST https://pier.example.com/api/v1/webhooks/my-api \
  -H "Authorization: Bearer $PIER_WEBHOOK_SECRET" \
  -H 'Content-Type: application/json' \
  -d '{"target":"v1.0.0"}'
```

GitHub Webhook 使用相同 secret，事件选 Releases；服务校验 `X-Hub-Signature-256`，只处理正式 Release 的 `published`，忽略 draft/prerelease。没有配置密钥时入口默认关闭。

加密工具兼容 upy 包格式，CI 推荐通过环境变量提供密码：

```sh
PIER_CRYPT_PASSWORD='your-password' ./pier-crypt bundle.zip
PIER_CRYPT_PASSWORD='your-password' ./pier-crypt -d bundle.zip.enc restored.zip
```

加密的 Release 资产在项目表单中设置正确资产名，例如 `bundle.zip.enc`，高级 YAML 设置 `crypto.password`。

## 代码结构

```text
server/
├── cmd/server/                  # HTTP 服务入口 + 隔离 worker 入口
├── cmd/pier-crypt/              # 独立加密 CLI
├── configs/
└── internal/
    ├── auth/                    # 管理员登录、会话、限速
    ├── config/                  # Pier 服务配置
    ├── store/                   # JSON 原子持久化
    ├── project/                 # 项目配置和目录管理
    ├── task/                    # 后台任务、子进程、日志
    ├── runner/                  # 编排调用提取出的 upy 核心
    ├── server/                  # API 路由与请求处理
    ├── webembed/                # 前端静态资源嵌入
    └── engine/                  # 从 upy 提取的部署/备份/清理等核心
web/src/
├── api/                         # 类型与 HTTP 客户端
├── router/                      # 页面路由和登录守卫
├── stores/                      # 轻量认证状态
├── components/                  # 项目表单、任务日志
└── views/                       # 登录、概览、项目、详情、任务、设置
```

后台任务通过当前可执行文件的 `worker` 子命令执行，敏感输入走 stdin，避免 upy 的包级配置/日志在并发项目间串扰。项目操作同时受持久化任务状态和文件锁保护。

第一版使用单进程 JSON 仓储，适合小规模单机管理，不支持多个控制台进程共同写同一份数据。服务端会锁定数据目录。正常关闭时等待执行中任务完成；强制关闭或主机重启后，任务标记为“结果待确认”，需查看实际状态。会话仅存内存，服务重启需重新登录。

## 验证

```sh
cd server
go test ./...
go vet ./...
cd ../web
npm run build
cd ..
node scripts/bundle-example.mjs
node scripts/build.mjs
node scripts/smoke.mjs
```

`smoke.mjs` 使用临时目录和 18099 端口，真实验证登录、两次静态发布、手动回退、健康检查失败自动回滚、备份及安全清理。需在支持软链和 Unix 脚本的环境运行。Windows + Ubuntu/WSL 可交叉编译 `server/bin/pier-linux-amd64` 后运行 `node scripts/smoke.mjs --wsl`。

上游的 Unix 文件锁、shell、systemd 测试通过 `!windows` build tag 区分；Linux CI 会执行这些测试。Docker/systemd 的单元测试包含命令替身，不等同于真实生产 Docker 或 systemd 验收。

## 来源

部署核心来自 [AaronConlon/upy](https://github.com/AaronConlon/upy)。修改与来源说明见 [NOTICE.md](NOTICE.md)。组件使用 [Arco Design Vue](https://arco.design/vue/docs/start)。
