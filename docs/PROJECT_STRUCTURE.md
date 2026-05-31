# 项目目录结构

版本：v0.3  
日期：2026-05-31

## 1. 整理原则

项目采用 Go 主后端 + DDD / 六边形架构。根目录只保留少数长期稳定的入口：

```text
labelserver/
  cmd/        可执行程序入口
  internal/   Go 后端核心代码
  web/        前端工程
  docs/       产品、SDD、架构和运维文档
  ops/        部署、脚本、配置、迁移、工具和小测试数据
  go.mod
  README.md
```

这样做的目的：

- 根目录不再被 `configs/deployments/scripts/tools/testdata` 等支撑目录打散。
- Go 业务代码集中在 `internal/`，利用 Go 的 `internal` 机制保护内部实现。
- 运维和工程辅助内容集中在 `ops/`，便于后续 Docker、K8s、CI、迁移脚本和离线工具扩展。

## 2. 顶层目录职责

### `cmd/`

Go 可执行程序入口。

```text
cmd/labelserver  后端服务入口
cmd/labelctl     CLI 管理工具入口
```

### `internal/`

核心后端代码，采用 DDD / 六边形架构。

```text
internal/
  api/             HTTP/API 适配层
  app/             应用服务与端口接口
  domain/          领域模型
  infrastructure/  存储、队列、模型网关、中间件等端口实现
  trigger/         服务启动、CLI、Webhook、定时任务等触发器
  types/           API DTO 和跨边界轻量类型
```

### `web/`

前端工程目录。后续二次元风格 Web UI、桌面端壳、移动端适配都从这里扩展。

### `docs/`

产品文档、SDD、架构设计、中间件、大数据、Mimo Provider、CLI/API Key 等说明。

### `ops/`

工程支撑目录，不承载核心业务代码。

```text
ops/
  configs/      非敏感配置模板
  deployments/  Docker/K8s/服务部署配置
  migrations/   数据库迁移脚本
  scripts/      本地开发、构建、运维脚本
  testdata/     极小测试数据
  tools/        数据转换、QA、benchmark、一次性迁移工具
```

注意：

- 真实 API key、`.env.local`、模型权重、视频、tracking CSV、token cache、checkpoint 都不进入 Git。
- `ops/testdata/` 只放最小可复现样例，不放 ShanghaiTech 全量数据。

## 3. `internal` 六边形结构

```text
internal/
  api/
    httpapi/
  app/
  domain/
    media/
    provider/
    tracking/
    workflow/
  infrastructure/
    config/
    mergecsv/
    middleware/
    modelgateway/
    providerrepo/
    queue/
    secrets/
  trigger/
    http/
  types/
```

依赖方向：

```text
api -> app -> domain
trigger -> api/app/infrastructure
infrastructure -> app ports + domain
```

禁止：

```text
domain -> infrastructure
domain -> api
app -> concrete postgres/redis/python worker
```

## 4. 文件放置规则

| 内容 | 放置位置 |
|---|---|
| 业务实体和值对象 | `internal/domain/<context>` |
| use case / 应用服务 | `internal/app` |
| HTTP handler | `internal/api/httpapi` |
| 数据库/Redis/MinIO/Python worker 适配 | `internal/infrastructure` |
| 服务启动编排 | `internal/trigger` |
| Go main | `cmd/<binary>` |
| 前端工程 | `web` |
| Docker/K8s | `ops/deployments` |
| SQL migration | `ops/migrations` |
| 非敏感配置模板 | `ops/configs` |
| 开发脚本 | `ops/scripts` |
| 小测试数据 | `ops/testdata` |
| 一次性工具 | `ops/tools` |

## 5. 不建议的结构

不建议新建：

```text
handlers/
services/
models/
utils/
```

原因：

- `services` 容易变成所有业务混杂。
- `models` 容易混淆 DB model、API DTO、domain model。
- `utils` 容易失控。

如果确实需要工具函数，应尽量放在具体上下文里，而不是全局 `utils`。
