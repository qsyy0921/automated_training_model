# 项目目录结构

版本：v0.1  
日期：2026-05-31

## 1. 核心代码目录

项目主代码使用 DDD / 六边形架构，核心目录固定为：

```text
api/
app/
domain/
infrastructure/
trigger/
types/
```

### `api/`

外部 API 适配层。

当前：

```text
api/httpapi
```

职责：

- HTTP REST API。
- WebSocket/SSE。
- 请求/响应 DTO 转换。
- API 错误格式。
- 不写业务规则。

### `app/`

应用层，也叫 use case 层。

职责：

- 编排业务流程。
- 定义端口接口。
- 调用 domain。
- 调用 repository/model gateway/task queue 等端口。

示例：

```text
MediaService
ProviderService
VideoRepository
TaskQueue
ModelGateway
SecretStore
```

### `domain/`

领域层。

职责：

- 核心业务对象。
- 值对象。
- 领域规则。
- 不依赖数据库、HTTP、Redis、Python。

当前：

```text
domain/media
domain/tracking
domain/workflow
domain/provider
```

后续：

```text
domain/annotation
domain/agent
domain/dataset
domain/memory
```

### `infrastructure/`

基础设施层，实现 app 定义的端口。

当前：

```text
infrastructure/mergecsv       读取现有 merge/csv 数据
infrastructure/middleware     HTTP 中间件
infrastructure/queue          内存任务队列 MVP
infrastructure/modelgateway   模型网关 MVP
infrastructure/providerrepo   provider 存储 MVP
infrastructure/secrets        API key/secret 存储
infrastructure/config         配置
```

后续：

```text
infrastructure/postgres
infrastructure/redisqueue
infrastructure/minio
infrastructure/nats
infrastructure/pythonworker
infrastructure/duckdb
infrastructure/otel
infrastructure/casbin
```

### `trigger/`

触发器层。

职责：

- HTTP server 启动。
- CLI local mode。
- webhook 入口。
- scheduler。
- connector gateway。

当前：

```text
trigger/http
```

### `types/`

跨边界 DTO 和轻量共享类型。

注意：

- `domain` 是业务事实。
- `types` 是传输形状。
- 不要把复杂业务规则写进 `types`。

## 2. 工程支撑目录

除了核心六边形目录，还需要一些工程目录。

### `cmd/`

Go 可执行程序入口。

当前：

```text
cmd/labelserver  后端服务入口
cmd/labelctl     CLI 管理工具入口
```

为什么不放进 `trigger/`：

- `cmd` 是 Go 社区约定。
- 一个仓库可以有多个二进制。
- `trigger` 是运行时触发器实现，`cmd` 是 main 入口。

### `docs/`

产品、架构、SDD、设计文档。

当前包括：

```text
PRODUCT_DOCUMENT_DDD_SDD.md
GO_BACKEND_AGENT_PLATFORM_ARCHITECTURE.md
GO_BACKEND_RESPONSIBILITY_AND_MIDDLEWARE.md
BIG_DATA_PROCESSING_ARCHITECTURE.md
CLI_AND_API_KEY_ARCHITECTURE.md
MIMO_PROVIDER_SETUP.md
MODULAR_HEXAGONAL_ARCHITECTURE.md
PROJECT_STRUCTURE.md
```

### `scripts/`

开发和运维脚本。

当前：

```text
build.ps1
docker-build.ps1
utf8.ps1
set-mimo-env.example.ps1
```

原则：

- 脚本不能保存真实 API key。
- 脚本不能绕过 app/domain 直接改正式数据，除非明确是迁移脚本。

### `deployments/`

部署配置。

当前：

```text
deployments/docker/Dockerfile
deployments/docker/docker-compose.yml
```

后续：

```text
deployments/k8s
deployments/systemd
deployments/windows-service
```

### `configs/`

非敏感配置样例。

用于：

- provider 配置模板。
- worker 配置模板。
- 本地开发配置模板。

真实 `.env` 和 API key 不进入 Git。

### `migrations/`

数据库迁移。

后续引入 PostgreSQL/SQLite migration：

```text
migrations/000001_init.sql
migrations/000002_annotation.sql
```

### `web/`

前端工程。

后续可以放：

```text
web/app
web/components
web/styles
web/package.json
```

如果前端单独成仓库，也可以只保留生成后的静态资源目录。

### `tools/`

开发辅助工具。

例如：

- 数据转换工具。
- QA 检查工具。
- benchmark 工具。
- 一次性迁移工具。

### `testdata/`

小型测试数据。

只放极小样例，不放真实 ShanghaiTech 视频和大 CSV。

## 3. 当前推荐树

```text
labelserver/
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
  cmd/
    labelserver/
    labelctl/
  configs/
  deployments/
    docker/
  docs/
  migrations/
  scripts/
  testdata/
  tools/
  web/
  go.mod
  README.md
```

## 4. 文件放置规则

### 业务实体放哪里

放 `domain/<context>/`。

例如：

```text
AnomalyEvent -> domain/annotation
TrackBox -> domain/tracking
Provider -> domain/provider
```

### 业务流程放哪里

放 `app/`。

例如：

```text
SaveAnomalyEvent
PurgeTrack
ExportDataset
RunAutoLabelJob
```

### HTTP 接口放哪里

放 `api/httpapi/`。

### 数据库、Redis、MinIO、Python worker 放哪里

放 `infrastructure/`。

### 启动程序放哪里

放 `cmd/`。

### 服务启动编排放哪里

放 `trigger/`。

### Docker/K8s 放哪里

放 `deployments/`。

## 5. 不建议的结构

不建议：

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

