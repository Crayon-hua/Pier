# 来源与改动

`server/internal/engine/` 和 `server/cmd/pier-crypt/` 提取自用户提供的 upy 工作区（Go module: `github.com/AaronConlon/upy`）。保留原始注释、测试和命名以便追溯；本地 import 已改为 `pier/internal/engine/`，没有通过 replace 或运行时路径依赖原仓库。

主要适配：

- 允许仅本地部署的项目省略 GitHub release 配置。
- 增加清单版本和相对路径校验、ZIP 条目/解压总量限制。
- 相同版本强制重发保留原有 previousVersion。
- Unix 专用测试增加平台条件；独立加密 CLI 改名为 pier-crypt。
- Pier 的认证、项目、任务、HTTP API、持久化与 Vue 控制台为新增实现。

本次输入的源码根目录没有独立 LICENSE 文件。本文只记录代码来源，不替原作者授予或更改许可证；对外再分发时应核对上游实际许可证和依赖许可。
