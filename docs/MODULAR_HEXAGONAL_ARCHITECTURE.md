# 模块化单体与六边形架构落地说明

版本：v0.1  
日期：2026-05-31  
仓库：`video_label_tool`

## 1. 当前选择

当前项目不再继续扩展单文件 Go 服务，而是拆成可扩展的模块化单体：

```text
api
app
domain
infrastructure
trigger
types
```

这不是最终拒绝微服务，而是先把边界设计好。后续任何模块都可以按边界拆成独立服务。

## 2. 目录职责

### 2.1 `domain`

领域层，不依赖外部框架。

当前包含：

```text
domain/media       视频、帧、视频摘要
domain/tracking    track、box、class 映射
domain/workflow    task/job 状态
domain/provider    LLM/VLM provider 与 API key 引用
```

后续增加：

```text
domain/annotation  异常片段、异常事件、事件对象、外貌描述
domain/agent       agent session、tool call、skill、memory
domain/dataset     dataset version、snapshot、export manifest
```

### 2.2 `app`

应用层，定义 use case 和端口接口。

当前包含：

```text
MediaService
ProviderService
VideoRepository
TaskQueue
ModelGateway
SecretStore
ProviderRepository
```

原则：

- app 层只依赖 domain。
- app 层不关心 CSV、PostgreSQL、Redis、Python worker 的具体实现。
- 正式标注写入必须通过 app service。

### 2.3 `infrastructure`

基础设施层，实现外部依赖。

当前包含：

```text
mergecsv          适配现有 merge/csv 数据
middleware        HTTP 中间件
queue             内存任务队列 MVP
modelgateway      Noop ModelGateway
providerrepo      内存 provider repository
secrets           环境变量 SecretStore
config            配置读取
```

后续增加：

```text
postgres          PostgreSQL repository
redisqueue        Redis/Asynq task queue
minio             object storage
nats              event bus
pythonworker      Python model-worker client
duckdb            analytics table store
otel              OpenTelemetry
casbin            RBAC/ABAC
```

### 2.4 `api`

API 适配层。

当前包含：

```text
api/httpapi
  GET /healthz
  GET /api/videos
  GET /api/video/{scene}/meta
  GET /api/video/{scene}/boxes?frame=...
  GET /api/video/{scene}/frame/{frame}.jpg
  GET /api/video/{scene}/preview
```

后续增加：

```text
/api/annotations
/api/tracks/review
/api/tasks
/api/model-jobs
/api/providers
/api/secrets
/api/exports
/api/agents
/api/skills
/api/memory
```

### 2.5 `trigger`

外部触发器和进程入口。

当前包含：

```text
trigger/http
```

后续增加：

```text
trigger/cli
trigger/connector
trigger/scheduler
trigger/webhook
```

### 2.6 `types`

API DTO 和跨层轻量类型。

注意：

- domain 是业务事实。
- types 是 API 形状。
- 不要把 API DTO 当领域模型。

## 3. 当前已落地接口

### 3.1 `VideoRepository`

```go
type VideoRepository interface {
    ListVideos(ctx context.Context) ([]media.VideoSummary, error)
    GetVideo(ctx context.Context, scene string) (*media.Video, error)
    GetBoxes(ctx context.Context, scene string, frame int) ([]tracking.Box, error)
    OpenFrame(ctx context.Context, scene string, frame int) (ReadSeekCloser, string, error)
    PreviewPath(ctx context.Context, scene string) (string, error)
}
```

当前实现：

```text
infrastructure/mergecsv.Repository
```

后续可替换为：

```text
PostgreSQL + object storage
Parquet/DuckDB index
remote media service
```

### 3.2 `ModelGateway`

```go
type ModelGateway interface {
    Submit(ctx context.Context, taskType string, payload map[string]string) (string, error)
    Status(ctx context.Context, id string) (*workflow.Task, error)
    Cancel(ctx context.Context, id string) error
}
```

后续所有 AI 功能都走这个网关：

- YOLO detection。
- BoT-SORT tracking。
- SAM/SAM2 segmentation。
- VLM caption。
- LLM suggestion。
- training worker。

### 3.3 `SecretStore`

```go
type SecretStore interface {
    PutAPIKey(...)
    GetAPIKey(...)
    ListAPIKeys(...)
    DeleteAPIKey(...)
}
```

当前实现：

```text
EnvStore
```

后续实现：

```text
EncryptedDBSecretStore
VaultSecretStore
CloudSecretStore
```

## 4. 中间件路线

当前 HTTP middleware：

```text
Recover
RequestID
CORS
Logger
```

下一步应增加：

```text
Auth
RBAC
Audit
RateLimit
Timeout
Metrics
Tracing
BodyLimit
IdempotencyKey
```

## 5. AI 微服务拆分策略

现在先不把每个 AI 功能拆成业务微服务，而是：

```text
Go labelserver
  -> ModelGateway
      -> Python model-worker
```

Python worker 内部按能力组织：

```text
detection
tracking
segmentation
vlm
llm
training
```

满足以下条件再拆微服务：

- 某能力需要独立 CUDA/PyTorch 环境。
- 某能力经常 OOM。
- 某能力需要远程 GPU。
- 某能力任务量远高于其他能力。
- 某能力需要独立伸缩和发布。

## 6. GitHub 当前提交范围

首次提交只包含：

- Go 后端骨架。
- CLI 骨架。
- Docker/Docker Compose 配置。
- 架构文档。
- UTF-8 终端脚本。

不提交：

- ShanghaiTech 视频。
- tracking CSV。
- 可视化视频。
- 模型权重。
- token cache。
- 训练 checkpoint。
- API Key。

这些通过 `.gitignore` 排除。

## 7. 下一阶段开发顺序

1. `annotation` 领域与 API。
2. track review / purge 的领域化。
3. PostgreSQL repository。
4. Redis/Asynq task queue。
5. Python model-worker 协议。
6. provider/key 管理 API。
7. CLI 批处理命令。
8. Web UI 接入新 API。
9. object storage abstraction。
10. 数据集 export snapshot。

