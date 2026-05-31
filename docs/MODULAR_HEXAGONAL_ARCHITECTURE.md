# 模块化单体与六边形架构落地说明

版本：v0.2  
日期：2026-05-31  
仓库：`_video_label_tool`

## 1. 当前选择

当前项目采用模块化单体，而不是一开始就拆成大量微服务。

代码边界按六边形架构整理在 `internal/` 下：

```text
internal/api
internal/app
internal/domain
internal/infrastructure
internal/trigger
internal/types
```

这样做的目的：

- 根目录更整洁。
- Go 的 `internal` 机制能限制外部 import。
- 领域层和基础设施层分离。
- 后续可以平滑拆出 Python worker、Connector Gateway、Training Worker。

根目录的非核心支撑文件统一放入 `ops/`：

```text
ops/configs
ops/deployments
ops/migrations
ops/scripts
ops/testdata
ops/tools
```

因此根目录只保留 `cmd/`、`internal/`、`web/`、`docs/`、`ops/` 和少量工程配置文件。

## 2. 核心依赖方向

推荐依赖方向：

```text
api -> app -> domain
trigger -> api/app/infrastructure
infrastructure -> app ports + domain
types -> domain only when necessary
```

禁止：

```text
domain -> infrastructure
domain -> api
app -> concrete postgres/redis/python worker
```

## 3. 当前模块

### `internal/domain`

领域模型：

```text
media       视频、帧、视频摘要
tracking    track、box、class 映射
workflow    task/job 状态
provider    LLM/VLM provider 与 API key 引用
```

### `internal/app`

应用服务和端口：

```text
MediaService
ProviderService
VideoRepository
TaskQueue
ModelGateway
SecretStore
ProviderRepository
```

### `internal/infrastructure`

端口实现：

```text
mergecsv       读取现有 merge/csv 数据
middleware     HTTP 中间件
queue          内存任务队列 MVP
modelgateway   Noop ModelGateway
providerrepo   内存 provider repository
secrets        环境变量 SecretStore
config         配置
```

### `internal/api`

HTTP API：

```text
GET /healthz
GET /api/videos
GET /api/video/{scene}/meta
GET /api/video/{scene}/boxes?frame=...
GET /api/video/{scene}/frame/{frame}.jpg
GET /api/video/{scene}/preview
GET /api/providers
GET /api/secrets
```

### `internal/trigger`

启动触发器：

```text
internal/trigger/http
```

### `cmd`

可执行程序：

```text
cmd/labelserver
cmd/labelctl
```

## 4. 当前已落地接口

### `VideoRepository`

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
internal/infrastructure/mergecsv.Repository
```

后续可替换为：

```text
PostgreSQL + object storage
Parquet/DuckDB index
remote media service
```

### `ModelGateway`

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

### `SecretStore`

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
internal/infrastructure/secrets.EnvStore
```

后续实现：

```text
EncryptedDBSecretStore
VaultSecretStore
CloudSecretStore
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

## 6. 中间件路线

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

## 7. GitHub 当前提交范围

提交范围：

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

## 8. 下一阶段开发顺序

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
