# Docker 镜像与 GitHub Actions

## 1. 发布仓库

把 **`pier/` 内的内容作为独立 GitHub 仓库的根目录**，例如 `your-account/pier`。根目录应直接包含 `Dockerfile`、`server/`、`web/` 和 `.github/workflows/image.yml`，不要在仓库根目录外再套一层 `pier/`，否则 GitHub 不会发现其中的 workflow。

镜像默认推送到 `ghcr.io/<owner>/<repo>`，名称自动转成小写。GHCR 使用仓库自带的 `GITHUB_TOKEN`，无需额外设置发布密码。工作流仅在发布 job 授予 `packages: write`。

在 GitHub 的 Actions 页面确认允许运行工作流。首次发布后，到 GitHub Packages 的包设置将镜像可见性设为 **Public**，这样其他服务器可以匿名拉取；私有镜像需要先 `docker login ghcr.io`。

## 2. 触发发布

```sh
git tag v1.0.0
git push origin v1.0.0
```

发布前执行 Go 测试、静态检查、前端类型检查/构建和镜像运行验收，随后推送 `linux/amd64` 与 `linux/arm64` 多架构镜像。

| 触发 | 镜像标签 |
| --- | --- |
| 正式版本 `v1.0.0` | `v1.0.0`、`1.0.0`、`1.0`、`latest`、`sha-...` |
| 预发布 `v1.1.0-rc.1` | 对应预发布标签和 `sha-...`，不更新 `latest` |
| 推送 main / 在 main 手动运行 | `latest`、`edge`、`sha-...` |
| Pull request | 只验证，不登录 Registry、不推送镜像 |

默认 Compose 使用 `latest`，由 main 构建和正式版本发布更新；预发布 tag 不更新它。需要固定版本时，可改用已发布的完整版本号。首次没有正式 tag 时，也可以在 main 上手动运行 Actions 生成 `latest`。工作流完成页会列出实际发布的完整镜像名称。

## 3. 使用 Compose

本仓库默认镜像为 `ghcr.io/crayon-hua/pier:latest`，成功完成镜像工作流后即可使用：

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

如果已有 `whaee-hub`，直接启动。不存在时先创建：

```sh
docker network create whaee-hub
docker compose up -d
```

打开 `http://服务器IP:18099`，首次创建管理员，没有默认密码。无需在服务器安装 Go、Node 或 upy。

仓库自带 `compose.yaml`，默认即可使用。也可以复制 `.env.example` 为 `.env`，通过 `PIER_IMAGE` 指定其他镜像或固定版本：

```sh
cp .env.example .env
# 默认 PIER_IMAGE=ghcr.io/crayon-hua/pier:latest
docker compose pull
docker compose up -d
```

持久化目录：

```text
/data/pier/
├── state/                 # 管理员密码哈希、项目配置、任务和日志
│   ├── pier.json
│   └── logs/
└── apps/<project-id>/     # 各应用的 releases、current、shared、backups
```

更新容器不会丢失这些文件。应用的静态站点目录为 `/data/pier/apps/<project-id>/current`，Nginx/Caddy 可直接读取或只读挂载它；Pier 控制台不自动提供业务网站的域名入口。

## 4. 管理 Docker / Compose 应用

普通配置无需 Docker socket，可管理静态应用、本地部署包、版本、备份与清理。若要由 Pier 操作宿主机 Docker，使用额外覆盖文件：

```sh
docker compose -f compose.yaml -f compose.docker.yaml up -d
```

它只额外添加：

```yaml
services:
  pier:
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

镜像已包含 Docker CLI、Compose 和 Buildx，无需安装。Socket 使控制台能够管理宿主机 Docker，应仅让可信管理员访问控制台；无需 `privileged: true`。

**项目路径在宿主机和容器内必须相同**。Docker daemon 按宿主机路径解析 bind mount，所以示例特意使用 `/data/pier:/data/pier`。如果修改为 `/srv/pier`，同时修改两边挂载路径、`PIER_DATA_DIR=/srv/pier/state` 和 `PIER_PROJECTS_DIR=/srv/pier/apps`。不要只改成 `/data/pier:/app/data` 再期望宿主机 Docker 自动识别 `/app/data`。

Docker 应用健康检查中的 `127.0.0.1` 指向 Pier 容器自身。检查宿主机映射端口时，清单应使用类似：

```yaml
healthcheck:
  url: http://host.docker.internal:3000/health
  timeout: 60s
```

也可通过共同 Docker 网络中的应用服务名访问。业务容器的 network、port、volume 仍由自己的清单/Compose 文件决定。

容器版不管理宿主机 systemd；需要该功能时，请在宿主机运行 Go 二进制版本。备份脚本在 Pier 容器内执行，镜像附带 shell、bash、tar、gzip 和 curl；脚本依赖的数据库客户端等额外工具需自行派生镜像安装。

## 5. 同步发布到 Docker Hub（可选）

在仓库 Settings → Secrets and variables → Actions 中配置：

| 类型 | 名称 | 值 |
| --- | --- | --- |
| Variable | `DOCKERHUB_USERNAME` | Docker Hub 用户名 |
| Variable（可选） | `DOCKERHUB_IMAGE` | 完整仓库名，如 `yourname/pier`，默认同名 |
| Secret | `DOCKERHUB_TOKEN` | 有推送权限的 Docker Hub access token |

在 Docker Hub 创建对应仓库，然后重新推送新 tag 或手动运行 workflow。配置后同时发布 GHCR 与 Docker Hub，可把 Compose 的镜像替换为：

```yaml
image: yourname/pier:v1.0.0
```

## 6. 本地构建与验收

在 `pier/` 下执行：

```sh
docker build -t pier:local .
node scripts/docker-smoke.mjs pier:local
```

验收脚本创建随机命名的容器和 volume，实际验证登录、内嵌页面、本地 ZIP 发布、软链、日志、健康检查和容器重建后数据保留，并清理自己创建的资源。不会挂载宿主机 Docker socket。

本地使用该镜像：

```sh
PIER_IMAGE=pier:local docker compose up -d
```

如需 HTTPS，在前方设置反向代理，保留 Host，并挂载自定义配置设置 `secure_cookie: true`。升级前等待当前任务结束，长任务可增加 `stop_grace_period`。不要同时运行两个容器写同一份状态目录。

相关官方文档：[GitHub 镜像发布](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)、[Docker GitHub Actions](https://docs.docker.com/build/ci/github-actions/)。
