# 第一版验证记录

日期：2026-11-03（Asia/Shanghai）。

- Windows / Go 1.26.2：`go test ./...` 和 `go vet ./...` 通过。Unix 专用测试由 build tag 排除。
- Ubuntu / WSL：交叉编译并运行 `go test ./internal/...`，包括部署、备份、清理、通知、GitHub 下载模拟、项目文件锁，以及新增 API / 认证 / 持久化测试，全部通过。
- 前端：`vue-tsc --noEmit` 和 Vite 生产构建通过。
- 实际浏览器：首次设置页、用户名密码登录、概览、项目创建、项目详情和本地包发布弹窗已检查。
- Linux 真实进程验收：创建管理员 → 登录 → 创建项目 → 上传并部署 v1 → 发布 v2 → 手动回退 v1 → 健康检查失败自动回滚 → 执行备份 → 查看日志 → 缓存清理后当前版本保留。

测试使用独立临时数据，未操作实际业务项目。

边界：没有连接真实 GitHub 私有仓库，未在真实 Docker daemon 或 systemd 业务服务上部署；相关流程通过移植的命令替身与 HTTP 模拟测试。Windows 本机缺少创建符号链接权限，完整部署流程在 Ubuntu/WSL 中验证。

## Docker 镜像交付验证

- 使用本机 Docker Desktop / Linux Engine 29.0.1，实际构建 `Dockerfile` 为 `pier:smoke`（另提供本地标签 `pier:local`）。
- `scripts/docker-smoke.mjs` 通过：容器启动、管理员创建与登录、内嵌前端、真实 ZIP 静态部署、版本软链、健康检查、Docker/Compose CLI 与独立加密工具检查。
- 删除并重建容器、复用同一数据卷后，管理员、项目版本、静态内容和任务日志均保留。测试容器与临时数据卷已清理。
- `compose.yaml` 与叠加 `compose.docker.yaml` 均通过 `docker compose config`。
- GitHub 镜像 workflow 通过 actionlint v1.7.7 校验；新增容器内 systemd 限制测试通过。
- 本地已验证 amd64 镜像；arm64 构建由工作流负责。未向远程 Registry 推送，需用户将项目上传至自己的仓库后触发。宿主机 Docker socket 管理业务容器和宿主机 systemd 不在本次镜像验收范围内。
