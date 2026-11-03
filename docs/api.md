# API

前缀 `/api/v1`。除首次初始化、登录、初始化状态和单独密钥保护的 Webhook，其他 API 均需 `pier_session` Cookie。请求/响应使用 JSON，失败返回 `{"message":"说明"}`。POST/PUT/DELETE 校验同源 Origin，未开启跨域 CORS。

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| GET | `/auth/status` | 是否已初始化 |
| POST | `/auth/setup` | 创建唯一管理员，`username/password` |
| POST | `/auth/login` | 用户名密码登录 |
| GET | `/auth/me` | 当前用户 |
| POST | `/auth/logout` | 注销当前会话 |
| POST | `/auth/password` | `oldPassword/newPassword`，撤销所有会话 |
| GET | `/system` | 平台、版本、上传上限和数据目录 |
| GET/POST | `/projects` | 列表/创建 |
| GET/PUT/DELETE | `/projects/{id}` | 详情/修改/仅移除登记 |
| GET | `/projects/{id}/releases` | GitHub Release 列表 |
| POST | `/projects/{id}/upload` | multipart 的 `file` 和可选 `force=true` |
| POST | `/projects/{id}/tasks` | 创建后台任务 |
| GET | `/projects/{id}/backups/{name}` | 下载归档 |
| GET | `/tasks?projectId=...` | 最近 200 条任务 |
| GET | `/tasks/{id}` | 任务状态 |
| GET | `/tasks/{id}/logs` | `{"text":"..."}`，最多最后 2 MiB |
| POST | `/webhooks/{id}` | 项目独立密钥 / GitHub HMAC 认证 |

项目创建/更新字段：`id,name,description,type,repository,asset,githubToken,clearToken,configYaml`。更新时路径 id 为准。`githubToken` 留空保留已有值；`clearToken=true` 清除。响应只返回 `hasToken`，不会回传 Token。

任务示例（返回 202 和任务对象）：

```json
{"kind":"deploy","target":"v1.0.0","force":false}
```

`kind` 支持 `deploy / rollback / backup / clean`；rollback 的 target 为本地版本，clean 的 target 为 `bundles / releases / tmp`。同一项目已有 queued/running 任务时返回 409。

任务状态：`queued / running / succeeded / failed / interrupted`。`interrupted` 表示控制台无法确认最终结果，不能理解为已完成回滚。
