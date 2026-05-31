# Go 主后端职责边界与中间件设计

版本：v0.1  
日期：2026-05-31  
定位：给视频标注智能体平台定义 Go 后端负责什么、哪些能力交给中间件、哪些能力交给模型 worker。

## 1. 总体结论

本平台应采用：

```text
Go 主后端 = 业务控制面 + 数据一致性 + API + 权限审计 + 任务编排 + Agent/Connector/MCP 管理
Python/模型服务 = GPU 推理 + SAM/YOLO/VLM/LLM 等重模型执行
前端/客户端 = 标注交互 + 审核工作台 + 多端入口
```

Go 不应该只做一个简单 HTTP server。它应该是平台的稳定核心，承担所有可审计、可恢复、可扩展、可权限控制的后端职责。

核心原则：

1. 正式标注数据只能由 Go 领域服务写入。
2. Agent、模型、群聊机器人默认只写 suggestion，不直接改正式标注。
3. 任何删除、覆盖、批量修改都必须经过 Go 权限和审计层。
4. 模型服务可以挂掉、重启、换实现，但 Go 后端的数据状态必须稳定。
5. 所有长任务都要可恢复、可取消、可重试、可追踪。

## 2. Go 具体负责什么

### 2.1 API Gateway

Go 是所有客户端的统一入口：

```text
Web UI
Desktop App
Mobile/PWA
WeChat/QQ/Feishu/Telegram Connector
External API
        ↓
Go API Gateway
```

职责：

- REST API。
- WebSocket / SSE 实时进度推送。
- 文件下载和静态资源访问授权。
- 前端 session 管理。
- API 版本管理。
- 请求限流、超时、日志、追踪 ID。
- 大文件上传分片管理。

推荐中间件：

- HTTP router：`chi` 或 `gin`。
- Request ID：每个请求生成 `request_id`。
- Recover：捕获 panic，避免服务崩溃。
- Logger：结构化日志。
- CORS：多端访问控制。
- Timeout：防止慢请求占用连接。
- BodyLimit：限制上传大小。
- RateLimit：防止机器人或外部入口打爆服务。
- Compression：静态文件和 JSON 压缩。
- Auth middleware：用户身份认证。
- RBAC middleware：权限检查。
- Audit middleware：记录关键操作。

### 2.2 领域模型和 DDD 核心

Go 负责维护正式业务对象。

建议领域上下文：

```text
Media Context
  Project
  Dataset
  Video
  Frame
  Segment

Tracking Context
  Track
  TrackBox
  TrackReview
  TrackDeleteQueue
  TrackCorrection

Annotation Context
  AnomalySegment
  AnomalyEvent
  EventObject
  AppearanceDescription
  LabelVersion

Workflow Context
  Task
  Job
  PipelineRun
  ModelSuggestion
  ReviewState

Agent Context
  AgentSession
  ToolCall
  Skill
  Memory
  MCPServer
  PermissionPolicy

Connector Context
  Channel
  Message
  Attachment
  IngestJob
```

Go 要保证：

- 视频、帧、轨迹、异常片段、异常事件、对象描述之间的关系一致。
- 删除 tracking 时有备份、有审计、有可恢复策略。
- 一个异常片段可以包含多个异常事件。
- 一个异常事件可以关联多个 object id。
- object 外貌描述属于异常事件内部的相关对象，而不是全局对象属性。
- 标注草稿和正式标注分离。
- 多人协作时不互相覆盖。

### 2.3 标注数据的唯一写入入口

Go 后端应提供正式写入接口：

```text
CreateAnomalySegment
UpdateAnomalySegment
DeleteAnomalySegment
CreateAnomalyEvent
UpdateAnomalyEvent
DeleteAnomalyEvent
AttachObjectToEvent
RemoveObjectFromEvent
SaveTrackReview
PurgeTrack
ExportDataset
```

模型和 Agent 不直接调用数据库写正式标注，只能走：

```text
CreateSuggestion
AcceptSuggestion
RejectSuggestion
```

这样可以避免自动标注污染人工标注。

### 2.4 视频、帧、轨迹渲染服务

Go 负责提供可审查的媒体服务：

- 视频列表。
- 单帧图像读取。
- 带 tracking box 的帧图渲染。
- 按 object id 高亮。
- 按异常片段锁定播放范围。
- 帧序列缩略图。
- 已删除 track 的预览。
- 导出审核前后对比图。

Go 不建议直接实现复杂视频解码算法。推荐调用：

- FFmpeg：转码、切片、抽帧、生成 proxy video。
- OpenCV/Python worker：复杂视觉处理。

Go 负责调度和缓存结果。

中间件/组件：

- FFmpeg CLI。
- Media cache directory。
- Object storage：MinIO / S3。
- CDN / Nginx 静态代理，后期使用。

### 2.5 任务和工作流编排

自动标注不是一次 API 调用，而是一组可恢复任务：

```text
视频导入
  -> 转码/抽帧
  -> 检测
  -> tracking
  -> 关键帧选择
  -> SAM 传播
  -> VLM 描述
  -> 异常候选生成
  -> 人工审核
  -> 导出训练数据
```

Go 负责：

- 创建任务。
- 记录任务状态。
- 分配 worker。
- 失败重试。
- 暂停/取消/恢复。
- 进度推送。
- 任务日志。
- 幂等处理。
- 任务依赖。

推荐中间件：

MVP：

- Go worker pool。
- SQLite/PostgreSQL task table。

中期：

- Redis + Asynq：简单后台任务、重试、延迟任务。
- NATS JetStream：事件驱动、轻量消息流。

高级：

- Temporal：复杂长工作流、可恢复执行、可观测 workflow history。

建议路径：

```text
第一阶段：DB task table + Go worker pool
第二阶段：Redis/Asynq
第三阶段：Temporal 或 NATS JetStream
```

### 2.6 Agent Runtime 编排

Go 应该负责 Agent 的运行边界，而不是把所有逻辑塞进大模型 prompt。

Go 管理：

- Agent session。
- Agent plan。
- Tool registry。
- Skill registry。
- MCP server registry。
- Memory retrieval。
- Permission policy。
- Tool call audit。
- Suggestion writeback。
- Human approval gate。

典型 Agent 流程：

```text
用户/群聊上传视频
  -> Go Connector Gateway 建立 IngestJob
  -> Go Agent Runtime 调用 KeyframeSkill
  -> Go 调用 ModelGateway 执行检测/SAM/VLM
  -> Go 写入 ModelSuggestion
  -> 人工在 Web UI 审核
  -> Go 将 accepted suggestion 转为正式标注
```

关键限制：

- `KeyframeAgent` 不能删除数据。
- `TrackingQAAagent` 只能写 suggestion。
- `ExportAgent` 只能读 accepted labels。
- `SelfEvolutionAgent` 只能生成 skill/config draft，不能直接修改生产代码和正式标注。

### 2.7 MCP 和 Skill 管理

Go 负责插件注册和权限隔离。

Skill 是可版本化的能力包：

```text
skill.yaml
prompt.md
schema.json
tools.json
tests/
examples/
```

Go 需要记录：

- skill id。
- version。
- input schema。
- output schema。
- allowed tools。
- required permissions。
- test status。
- owner。
- enable/disable 状态。

MCP server 是外部工具接口：

- 数据库查询 MCP。
- 文件检索 MCP。
- 模型服务 MCP。
- 标注工具 MCP。
- 训练任务 MCP。

Go 要做：

- MCP server 注册。
- tool allowlist。
- tool permission check。
- destructive tool 二次确认。
- tool call 日志。
- context budget 控制。

### 2.8 Memory 服务

Go 负责长期 memory 的结构化存储和检索。

Memory 类型：

- 用户偏好 memory。
- 项目 memory。
- 数据集 memory。
- 标注规范 memory。
- 模型错误模式 memory。
- 已审核样例 memory。
- Agent 经验 memory。

Memory 不应全都放进 prompt。Go 应该按任务检索最相关片段。

推荐中间件：

- PostgreSQL：结构化 memory。
- pgvector / Qdrant / Milvus：向量检索。
- Meilisearch / Typesense / Elasticsearch：关键词检索。
- Redis：短期 session cache。

建议 MVP：

```text
SQLite/PostgreSQL + FTS
```

中期：

```text
PostgreSQL + pgvector + Redis
```

大规模：

```text
PostgreSQL + Qdrant + Elasticsearch
```

### 2.9 多端 Connector Gateway

Go 负责多平台入口的稳定接入：

- 微信。
- QQ。
- 飞书。
- Telegram。
- Webhook。
- 邮件。
- Web 上传。
- 桌面端本地导入。

Connector Gateway 做标准化：

```text
External Message
  -> NormalizedMessage
  -> Attachment
  -> IngestJob
  -> Workspace Session
```

Go 负责：

- 平台 token。
- webhook 验签。
- 消息去重。
- 附件下载。
- 文件安全扫描。
- 用户身份绑定。
- 权限判断。
- 任务回执。

推荐中间件：

- Redis：消息去重、短期状态。
- Object storage：附件落盘。
- NATS/Asynq：异步下载和处理。
- OpenTelemetry：跨平台请求追踪。

### 2.10 权限、审计和版本

这是 Go 必须负责的部分。

权限模型：

```text
User
Role
Workspace
Project
Dataset
Video
Operation
```

关键操作：

- 删除 track。
- 批量删除。
- 接受模型标注。
- 覆盖人工标注。
- 导出训练集。
- 开启自动标注。
- 启用 self-evolution。
- 调用外部 MCP。

都必须写审计日志。

推荐中间件：

- Casbin：RBAC/ABAC 权限策略。
- OIDC/OAuth2：企业登录。
- Keycloak：自部署身份服务。
- Audit log table：不可变审计。
- Append-only event log：后续回放和追责。

### 2.11 训练任务管理

Go 不负责 PyTorch 训练本身，但负责训练任务生命周期：

- 创建训练实验。
- 锁定数据版本。
- 记录训练配置。
- 启动训练进程或提交到训练节点。
- 监控日志和指标。
- 记录 checkpoint。
- 对比实验指标。
- 生成报告。
- 推送进度到前端/群聊。

推荐中间件：

- MLflow 或自建 experiment table。
- MinIO/S3 存 checkpoint 和 artifact。
- Prometheus 采集 GPU/CPU/磁盘指标。
- Grafana 展示资源状态。
- Loki 存训练日志。

## 3. Go 不负责什么

Go 不应该直接负责：

- YOLO/SAM/VLM/LLM 的 GPU 推理。
- PyTorch 训练循环。
- 大规模视频逐帧像素级处理。
- 复杂 CUDA 算子。
- 模型权重管理细节。
- 大模型 prompt 的所有业务判断。

这些应放到：

```text
model-worker
training-worker
video-worker
```

Go 通过 HTTP/gRPC/队列调用它们。

## 4. 推荐中间件总表

| 类别 | MVP | 中期 | 大规模/企业版 |
|---|---|---|---|
| HTTP Router | chi/gin | chi/gin | API Gateway + chi/gin |
| 数据库 | SQLite | PostgreSQL | PostgreSQL 分库/只读副本 |
| ORM/SQL | sqlc/pgx | sqlc/pgx/ent | sqlc + migration discipline |
| 迁移 | goose/golang-migrate | goose/golang-migrate | GitOps migration |
| 缓存 | 内存 cache | Redis | Redis Cluster |
| 队列 | DB task table | Asynq / NATS | Temporal / Kafka / NATS |
| 对象存储 | 文件系统 | MinIO | S3/OSS/MinIO 集群 |
| 搜索 | SQLite FTS | Meilisearch/Typesense | Elasticsearch/OpenSearch |
| 向量库 | 暂不需要 | pgvector | Qdrant/Milvus |
| 权限 | 自定义 RBAC | Casbin | Casbin + OIDC/Keycloak |
| 日志 | slog/zap | Loki | Loki + SIEM |
| 指标 | expvar | Prometheus | Prometheus + Grafana |
| tracing | request_id | OpenTelemetry | Tempo/Jaeger |
| 错误监控 | 日志 | Sentry | Sentry + alerting |
| 反向代理 | 无 | Nginx/Traefik | Kong/Envoy/Traefik |
| 配置 | env/yaml | Viper | Vault/Consul |
| 文件上传 | 本地 | 分片上传 | tus/S3 multipart |
| 实时通信 | SSE | SSE/WebSocket | WebSocket Gateway |
| 模型通信 | HTTP | gRPC/HTTP | gRPC + queue + autoscaling |

## 5. 推荐 Go 服务拆分

初期不要拆太多微服务，先做模块化单体。

### 5.1 第一阶段：模块化单体

```text
labelserver
  /cmd/server
  /internal/api
  /internal/domain
  /internal/app
  /internal/repository
  /internal/infrastructure
  /internal/worker
  /internal/agent
  /internal/connector
  /internal/mcp
  /internal/skill
  /web
```

特点：

- 一个 Go 进程。
- 一个数据库。
- 文件系统或 MinIO。
- 本地 worker pool。
- 最快落地。

### 5.2 第二阶段：拆 Connector 和 Model Worker

```text
labelserver
connector-gateway
model-worker
training-worker
```

拆分原因：

- 群聊平台接入和主业务隔离。
- GPU 推理和主业务隔离。
- 训练任务不影响标注 UI。

### 5.3 第三阶段：事件驱动平台

```text
api-gateway
label-domain-service
workflow-service
agent-service
connector-gateway
model-gateway
training-service
```

只有当团队、数据和并发都变大后再拆到这一层。

## 6. Go 后端中间件链设计

HTTP 请求应经过：

```text
Recover
  -> RequestID
  -> RealIP
  -> Logger
  -> Metrics
  -> Tracing
  -> Timeout
  -> BodyLimit
  -> CORS
  -> Auth
  -> RBAC
  -> Audit
  -> Handler
```

关键写操作再加：

```text
IdempotencyKey
  -> ValidateInput
  -> DomainTransaction
  -> EventAppend
  -> Outbox
```

例如“彻底删除 track”：

```text
POST /api/videos/{video_id}/tracks/purge
  -> Auth
  -> RBAC: can_delete_track
  -> Audit: request received
  -> Validate: track id exists
  -> Backup original rows
  -> Transaction delete
  -> Append TrackPurged event
  -> Invalidate frame cache
  -> Return deleted count
```

## 7. 数据一致性设计

正式标注数据建议采用：

```text
PostgreSQL/SQLite transaction
  + append-only domain events
  + audit log
  + export snapshot
```

核心表：

```text
videos
frames
tracks
track_boxes
track_reviews
track_delete_queue
anomaly_segments
anomaly_events
event_objects
event_object_appearance
model_suggestions
label_versions
audit_logs
tasks
task_logs
agent_sessions
tool_calls
skills
memories
connectors
```

每次导出训练数据时生成 snapshot：

```text
snapshot_id
dataset_version
label_version
tracking_version
export_config
created_by
created_at
hash
```

这样训练实验可以追溯到具体标注版本。

## 8. Agent 和自动标注的安全边界

自动标注建议分三级：

### L1 Suggestion Only

Agent 只产生建议：

```text
candidate segment
candidate event
candidate object relation
candidate appearance
candidate reason
```

人工确认后才进入正式标注。

### L2 Assisted Apply

Agent 可以在人工点击“接受建议”后批量写入。

### L3 Controlled Automation

在低风险数据集上，Agent 可自动写入草稿，但不能写 accepted。

禁止：

- Agent 直接彻底删除 track。
- Agent 直接覆盖人工 accepted 标注。
- 群聊消息直接触发生产数据写入。
- self-evolution 自动修改核心 Go 后端代码并部署。

## 9. 中间件落地优先级

### 9.1 现在立刻需要

- Go HTTP router。
- structured logging。
- request id。
- recover。
- CORS。
- SQLite 或 PostgreSQL。
- migration。
- task table。
- file/object storage abstraction。
- audit log。
- SSE/WebSocket progress。

### 9.2 做自动标注前需要

- Redis 或 DB task queue。
- ModelGateway。
- worker heartbeat。
- task retry。
- suggestion table。
- artifact storage。
- OpenTelemetry 基础 tracing。

### 9.3 做多端接入前需要

- Connector Gateway。
- message dedup。
- attachment storage。
- user binding。
- permission mapping。
- platform webhook signature validation。

### 9.4 做自进化前需要

- skill registry。
- skill versioning。
- sandbox workspace。
- eval/replay test。
- approval workflow。
- rollback。
- immutable audit log。

## 10. 建议的技术选型

如果目标是工程稳定和后续可扩展，推荐：

```text
语言：Go 1.22+
HTTP：chi 或 gin
日志：slog 或 zap
配置：viper + env
数据库：PostgreSQL，单机轻量版可 SQLite
SQL：pgx + sqlc，或 ent
迁移：goose / golang-migrate
缓存：Redis
队列：Asynq 起步，复杂工作流再上 Temporal
对象存储：MinIO/S3
权限：Casbin
认证：JWT/OIDC，企业版接 Keycloak
实时：SSE 起步，复杂协同用 WebSocket
模型调用：gRPC/HTTP ModelGateway
可观测性：OpenTelemetry + Prometheus + Grafana + Loki
错误监控：Sentry
搜索：Meilisearch/Typesense，后期 Elasticsearch
向量检索：pgvector 起步，后期 Qdrant
反向代理：Traefik/Nginx
桌面端：Wails
移动端：PWA 起步，后期 Flutter/React Native
```

## 11. 为什么 Go 适合做主后端

Go 的优势正好对应本平台的核心需求：

- 并发任务和长连接简单稳定。
- 单二进制部署方便，适合本地科研工作站。
- 比 Python 更适合长期运行的 API 服务。
- 类型系统适合 DDD 和数据一致性。
- goroutine 适合任务调度、连接器、进度推送。
- 与 gRPC、OpenTelemetry、Prometheus、Kubernetes 生态契合。
- 可以被 Wails 直接打包成桌面应用后端。

Python 的优势在模型，不在业务控制面。因此 Python 应该是 worker，不应该是正式标注数据的唯一控制者。

## 12. 下一步工程拆分建议

从当前 `review_server.go` 迁移时建议按这个顺序：

1. 抽出 `MediaRepository`。
2. 抽出 `TrackingRepository`。
3. 抽出 `AnnotationRepository`。
4. 增加 `AuditService`。
5. 增加 `TaskService`。
6. 增加 `SuggestionService`。
7. 增加 `ModelGateway` 接口。
8. 增加 `AgentRuntime` 空壳。
9. 增加 `SkillRegistry`。
10. 增加 `ConnectorGateway`。

不要一开始就微服务化。先做一个干净的 Go 模块化单体，等任务量和多端入口真的复杂后再拆服务。

## 13. MQ、Redis 与任务系统怎么用

本平台会有大量异步任务：

```text
视频上传
转码抽帧
YOLO 检测
BoT-SORT tracking
SAM/SAM2 传播
VLM 描述
LLM 辅助标注
导出训练集
训练任务启动
训练日志采集
群聊消息回执
```

这些任务不能直接放在 HTTP handler 里同步执行。Go 后端应该把用户请求转成任务，再由 worker 执行。

### 13.1 Redis 应该负责什么

Redis 不应该存正式标注数据。它适合做短期、高并发、可丢失或可重建的状态。

适合放 Redis 的内容：

- API rate limit。
- 用户 session 短期状态。
- WebSocket/SSE 在线状态。
- 任务短期进度缓存。
- 消息去重 key。
- 分布式锁。
- Connector webhook 去重。
- 自动标注任务排队。
- 大文件上传临时状态。
- 热点视频 metadata cache。

不适合放 Redis 的内容：

- 正式标注。
- tracking 审核最终结果。
- 删除审计日志。
- 训练数据版本。
- accepted labels。

这些必须在 PostgreSQL/SQLite + 文件快照里落盘。

### 13.2 MQ 应该负责什么

MQ 用来解耦任务生产者和消费者。

推荐任务类型：

```text
media.transcode.requested
media.frames.extract.requested
tracking.detect.requested
tracking.track.requested
tracking.merge.requested
sam.propagate.requested
vlm.describe.requested
llm.suggest.requested
dataset.export.requested
training.run.requested
connector.attachment.received
```

事件命名建议：

```text
领域.动作.状态
```

例如：

```text
tracking.track.started
tracking.track.completed
tracking.track.failed
annotation.suggestion.created
annotation.label.accepted
```

### 13.3 中间件选型建议

不要一上来同时上 Kafka、Temporal、Kubernetes、Redis Cluster。当前更适合分阶段。

第一阶段，本地科研工作站：

```text
PostgreSQL/SQLite task table
Go worker pool
Redis 可选
```

优点：

- 简单。
- 好调试。
- 适合单机 GPU。
- 任务状态容易查。

第二阶段，多进程任务：

```text
Redis + Asynq
```

适合：

- 自动标注任务。
- 重试。
- 延迟任务。
- 后台批处理。
- 简单 worker 横向扩展。

第三阶段，事件流和多 worker：

```text
NATS JetStream
```

适合：

- 多模型 worker。
- 多客户端实时通知。
- 任务事件广播。
- 比 Kafka 轻，部署成本更低。

第四阶段，复杂长流程：

```text
Temporal
```

适合：

- 一个任务跨多个小时甚至多天。
- 需要强恢复能力。
- 需要明确 workflow history。
- 例如全量数据集自动标注和训练闭环。

Kafka 不是第一选择。除非后续达到大规模多团队、多数据源、海量事件吞吐，否则 Kafka 会增加过多运维成本。

## 14. AI 功能要不要做成微服务

结论：

```text
AI 推理执行层建议服务化；
AI 业务编排层不建议一开始微服务化。
```

也就是说，不要把每个 AI 能力都拆成独立业务微服务。更合理的是：

```text
Go 主后端
  -> ModelGateway
      -> detector-worker
      -> tracker-worker
      -> segment-worker
      -> vlm-worker
      -> llm-worker
      -> training-worker
```

Go 只认统一接口：

```text
SubmitModelJob
GetModelJobStatus
CancelModelJob
GetArtifacts
```

底层 worker 可以用 Python、C++、Go 或外部 API 实现。

### 14.1 为什么不建议一开始按 AI 能力拆很多微服务

如果一开始拆成：

```text
detection-service
tracking-service
sam-service
vlm-service
llm-service
training-service
keyframe-service
caption-service
qa-service
```

问题会很快出现：

- 服务太多，本地开发复杂。
- 每个服务都要配置、日志、监控、鉴权、重试。
- 数据传输成本高，视频帧和 mask 很大。
- 调试困难，一个标注结果出错需要跨多个服务查日志。
- 对单机 20GB 显存环境不友好。
- GPU 资源调度比业务拆分更重要。

当前阶段更适合：

```text
一个 Go 主服务
一个或少数几个 Python model-worker
一个 task queue
一个 artifact store
```

### 14.2 哪些 AI 能力适合独立 worker

适合独立 worker 的能力：

- YOLO / GroundingDINO / RT-DETR 检测。
- BoT-SORT / ByteTrack tracking。
- SAM / SAM2 mask 传播。
- VLM 图像或视频描述。
- LLM 标注建议。
- 训练任务。
- 批量数据质量检查。

这些能力的共同特点：

- 依赖 GPU 或大模型。
- 运行时间长。
- 容易 OOM。
- 依赖环境复杂。
- 需要单独重启，不应该拖垮 Go API。

### 14.3 哪些 AI 能力不应该独立成微服务

不建议单独拆成微服务的能力：

- 异常事件业务规则。
- 标注 schema 校验。
- label version 管理。
- accepted/rejected suggestion 状态。
- 删除 track 的最终写入。
- 权限判断。
- 审计。
- 导出数据版本管理。

这些必须留在 Go 领域层。否则业务一致性会被分散到多个服务里，后续很难维护。

## 15. 推荐的 AI 服务化形态

### 15.1 ModelGateway 统一入口

Go 内部定义统一接口：

```go
type ModelGateway interface {
    Submit(ctx context.Context, req ModelJobRequest) (ModelJobID, error)
    Status(ctx context.Context, id ModelJobID) (ModelJobStatus, error)
    Cancel(ctx context.Context, id ModelJobID) error
    Artifacts(ctx context.Context, id ModelJobID) ([]ArtifactRef, error)
}
```

上层业务不关心底层是：

- 本地 Python worker。
- 远程 GPU server。
- OpenAI API。
- Qwen API。
- 自部署 VLM。
- Docker container。

### 15.2 Worker 内部按模型能力分组

建议先做一个 `model-worker`，内部按 pipeline 分组：

```text
model-worker
  detection/
  tracking/
  segmentation/
  vlm/
  llm/
  training/
```

如果后面哪个能力资源压力很大，再拆出去。

拆分条件：

- 单个能力经常 OOM。
- 单个能力需要独占 GPU。
- 单个能力需要不同 CUDA/PyTorch 环境。
- 单个能力任务量远高于其他能力。
- 单个能力需要部署到远端机器。

例如 SAM2 和训练环境冲突时，可以拆：

```text
sam-worker
training-worker
```

### 15.3 Artifact 传递，而不是大对象直接 RPC

不要在服务之间传大量图像数组或视频帧。

正确方式：

```text
Go 创建任务
  -> 把输入文件路径 / artifact id 发给 worker
  -> worker 从 object storage 读取
  -> worker 输出 JSON/Mask/Video 到 object storage
  -> Go 只保存 artifact ref
```

Artifact 示例：

```json
{
  "artifact_id": "art_20260531_0001",
  "type": "tracking_csv",
  "uri": "s3://label-artifacts/project/video/tracking.csv",
  "sha256": "...",
  "created_by_job": "job_001"
}
```

这样可以避免服务间传输巨大 payload。

## 16. 推荐最终运行拓扑

### 16.1 单机科研版

```text
labelserver.exe
  Go API
  SQLite/PostgreSQL
  local task worker
  local file storage

model-worker.exe / python model_worker.py
  YOLO/SAM/VLM/LLM
  GPU
```

适合当前阶段。

### 16.2 工作站增强版

```text
Go labelserver
PostgreSQL
Redis
Asynq workers
MinIO
Python model-worker
Prometheus + Grafana
```

适合多人标注和全量自动标注。

### 16.3 团队服务器版

```text
Go labelserver
Go connector-gateway
PostgreSQL
Redis
NATS JetStream
MinIO
Multiple model-workers
training-worker
OpenTelemetry
Prometheus/Grafana/Loki
```

适合多端入口和多 GPU。

### 16.4 企业/平台版

```text
API Gateway
Go domain services
Temporal
NATS/Kafka
PostgreSQL cluster
Object storage
GPU worker pool
Kubernetes
OIDC/Keycloak
Audit/SIEM
```

这是远期，不是当前 MVP。

## 17. 对本项目的具体建议

针对当前 `video_label_tool`，建议路线是：

1. GitHub 仓库以 Go 模块化单体开局。
2. `cmd/labelserver` 作为主入口。
3. `internal/domain` 保存 DDD 实体和值对象。
4. `internal/app` 保存 use case。
5. `internal/repository` 保存 DB 和文件存储实现。
6. `internal/worker` 保存任务调度。
7. `internal/modelgateway` 定义 AI worker 统一接口。
8. `internal/agent` 只做编排和 suggestion，不写正式标注。
9. 第一版使用 SQLite + 文件系统。
10. 第二版切 PostgreSQL + Redis + Asynq + MinIO。
11. CLI 用 `cmd/labelctl`，只调用 API 或复用 app service，不绕过领域层改数据。
12. 大模型 API Key 通过 `SecretStore` 管理，默认环境变量，后续升级到加密数据库或 Vault。

不建议现在把所有 AI 功能拆成微服务。更好的做法是：

```text
Go 模块化单体
  + Redis/Asynq
  + 一个 Python model-worker
  + ModelGateway 抽象
```

等某个 AI 能力真的成为瓶颈，再按资源边界拆服务，而不是按概念边界拆服务。
