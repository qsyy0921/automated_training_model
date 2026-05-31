# 大数据处理架构设计

版本：v0.1  
日期：2026-05-31  
主后端：Go  
适用范围：视频标注、tracking 审核、对象级异常标注、自动标注、训练数据生成、模型评估与数据治理。

## 1. 为什么需要大数据架构

视频标注平台的数据增长会非常快。真正占空间和吞吐的不是表单标注，而是这些数据：

```text
原始视频
转码后 proxy video
逐帧图片
tracking box
人工标注版本
模型自动标注 suggestion
SAM mask
ViT/CLIP/Qwen visual token cache
训练样本 manifest
训练 checkpoint
训练日志和指标
可视化视频
导出数据集快照
```

如果所有内容都按普通文件夹 + CSV 管理，短期能跑，但后续会遇到：

- 全量统计慢。
- 数据版本不可追溯。
- 训练集导出难复现。
- 标注修订后无法知道影响了哪些训练样本。
- embedding/cache 文件太多，小文件 IO 成为瓶颈。
- 多模型 worker 并发读写时容易乱。
- 前端检索某个 object 或异常片段时延迟高。

因此需要一套“数据湖 + 元数据服务 + 任务队列 + 批处理/流处理”的架构。

## 2. 总体数据架构

推荐采用分层数据湖：

```text
Go Metadata Control Plane
  PostgreSQL / SQLite
  Task / Audit / Version / Lineage

Object Storage / Data Lake
  raw
  bronze
  silver
  gold
  features
  artifacts

Batch / Stream Processing
  DuckDB / Polars / Spark
  Redis / Asynq / NATS

Model Workers
  Detection
  Tracking
  SAM
  VLM
  Training
```

Go 不直接做大规模全表扫描和图像矩阵处理。Go 负责：

- 数据目录规范。
- 元数据登记。
- 任务调度。
- 数据版本。
- 访问控制。
- 审计。
- 血缘。
- 导出 manifest。
- 查询 API。

大批量扫描和计算交给：

- DuckDB / Polars：单机或工作站级分析。
- Spark / Flink：集群级批处理和流处理。
- Python/Ray：GPU/模型并行。
- Parquet/Iceberg：大规模分析表。

## 3. 数据湖分层

### 3.1 Raw Layer

保存原始输入，不允许覆盖。

```text
data_lake/raw/
  dataset=shanghaitech/
    split=test/
      video_id=01_0014/
        source.mp4
        source.sha256
        metadata.json
```

内容：

- 原始视频。
- 原始帧级 mask。
- 原始外部 tracking 文件。
- 上传来源。
- sha256。
- 摄像头信息。

规则：

- Raw 只追加，不修改。
- 删除也应做 tombstone 记录，不直接物理删除。

### 3.2 Bronze Layer

保存机器生成但未审核的数据。

```text
data_lake/bronze/
  tracking_run=yolo26x_botsort_v1/
    dataset=shanghaitech/
      video_id=01_0014/
        boxes.parquet
        tracks.csv
        vis.mp4
        run_config.json
```

内容：

- YOLO 检测框。
- BoT-SORT/ByteTrack 轨迹。
- YOLOWorld stroller 检测。
- SAM 结果。
- VLM caption。
- 自动异常 suggestion。

规则：

- Bronze 是模型输出，不等于 GT。
- 每次模型运行都要有 `run_id`。
- 保留模型版本、参数、输入数据 hash。

### 3.3 Silver Layer

保存人工审核后的高质量数据。

```text
data_lake/silver/
  dataset=shanghaitech/
    label_version=v20260531_reviewed/
      tracking/
        video_id=01_0014/boxes.parquet
      annotations/
        video_id=01_0014/events.jsonl
      masks/
        video_id=01_0014/frame_mask.parquet
```

内容：

- 删除误检后的 tracking。
- 修正类别后的 tracking。
- 人工审核后的对象级异常标注。
- 对象外貌描述。
- 异常原因/描述。

规则：

- Silver 是训练 object query / anomaly query 的主要来源。
- 每次保存生成 label version。
- Accepted label 不允许被模型直接覆盖。

### 3.4 Gold Layer

保存训练/评估直接使用的数据集快照。

```text
data_lake/gold/
  task=object_query_training/
    export_id=oq_20260531_001/
      manifest.parquet
      samples/
      config.json
      stats.json
      lineage.json
```

内容：

- 训练 manifest。
- 数据切分。
- 样本索引。
- 统计结果。
- 导出配置。
- 血缘。

规则：

- 模型训练只能依赖 Gold snapshot。
- 每个实验必须记录对应 `export_id`。

### 3.5 Features Layer

保存大模型视觉特征和 embedding。

```text
data_lake/features/
  extractor=qwen3vl_vit_720p_stride4/
    dataset=shanghaitech/
      video_id=01_0014/
        part-000.parquet
        index.json
```

内容：

- ViT patch tokens。
- object crop embeddings。
- track embeddings。
- scene embeddings。
- memory bank vectors。

建议：

- 小规模可以继续用 `.pt`。
- 数据变大后应转成分块格式，例如 Parquet、Arrow IPC、Zarr 或 safetensors shard。
- 避免每帧一个小文件。

## 4. 表格式选择

### 4.1 Parquet

Parquet 适合存 tracking、标注、统计、特征索引、训练 manifest。

适合：

- 列式扫描。
- 只读取部分列。
- 按 video_id/frame_id 过滤。
- 压缩存储。
- 批量统计。

表例：

```text
boxes.parquet
  dataset
  split
  video_id
  frame_idx
  track_id
  class_id
  class_name
  x
  y
  w
  h
  conf
  source_run_id
  review_status
```

### 4.2 Iceberg

当数据版本很多、数据集很大、多人并发写入时，应考虑 Iceberg。

Iceberg 适合：

- 大型分析表。
- schema evolution。
- snapshot。
- time travel。
- partition evolution。
- 多计算引擎共享。

当前阶段不必立刻上 Iceberg，但目录和 manifest 设计要为它留空间。

### 4.3 JSONL

JSONL 适合人工标注事件，因为结构可变：

```json
{
  "video_id": "01_0014",
  "segment_id": "seg01",
  "event_id": "event01",
  "start_frame": 155,
  "end_frame": 230,
  "anomaly_type": "bicycle",
  "reason_cn": "白色上衣黑色裤子的人骑黑色自行车",
  "objects": [
    {
      "track_id": 6,
      "class_name": "person",
      "appearance_cn": {
        "upper_color": "白色",
        "lower_color": "黑色",
        "vehicle": "黑色自行车"
      }
    }
  ]
}
```

但导出训练集时应转成 Parquet，方便统计和批处理。

## 5. 元数据数据库

正式系统建议使用 PostgreSQL。

MVP 可以 SQLite 起步，但要保持 repository 接口，方便迁移。

PostgreSQL 负责：

- 项目/数据集/视频元数据。
- track 索引。
- anomaly segment/event/object。
- label version。
- task/job 状态。
- audit log。
- model run。
- artifact reference。
- user/role/permission。

不建议 PostgreSQL 存：

- 原始视频二进制。
- 大量帧图。
- mask 大数组。
- ViT token 大矩阵。

这些放对象存储或分块文件。

## 6. 对象存储

数据量上来后，文件系统目录会越来越难管理。推荐使用 MinIO/S3 兼容对象存储。

对象存储负责：

- 原始视频。
- proxy video。
- 抽帧图片。
- 可视化视频。
- SAM mask。
- feature shard。
- checkpoint。
- 导出数据集。

Go 后端只保存：

```text
artifact_id
uri
sha256
size_bytes
content_type
created_by_job
lineage
```

这样以后可以从本地文件系统平滑迁移到 MinIO/S3。

## 7. 批处理技术

### 7.1 当前阶段：DuckDB / Polars

当前数据规模在单机工作站上，优先使用：

- DuckDB：直接查询 Parquet，适合统计和导出。
- Polars：高性能 dataframe，适合 Python 数据处理。

典型任务：

- 每个视频 track 数统计。
- 每个类别 box 数统计。
- recall/IoU 全量评估。
- 训练 manifest 生成。
- 按异常片段切样本。
- 检查 label schema。
- 查找疑似误检 track。

### 7.2 中期：Spark

当数据量达到 TB 级、单机统计变慢时，引入 Spark。

适合任务：

- 全量视频特征统计。
- 多数据集 join。
- 大规模导出训练集。
- 多版本标签对比。
- 海量 embedding metadata 处理。

### 7.3 流处理：Flink / Spark Structured Streaming

如果多端持续上传视频，需要流处理。

适合任务：

- 群聊接入后的持续 ingest。
- 实时自动标注队列。
- 实时质量监控。
- 大量 worker 事件聚合。

当前阶段更推荐 NATS/Redis/Asynq + Go worker，Flink 可以后置。

## 8. 消息队列与事件流

### 8.1 Redis + Asynq

适合当前阶段：

- 简单任务队列。
- 重试。
- 延迟执行。
- worker pool。
- 与 Go 集成简单。

任务例子：

```text
tracking.generate
stroller.detect
sam.propagate
vlm.describe
dataset.export
training.start
```

### 8.2 NATS JetStream

适合中期事件流：

- 多 worker 订阅。
- 任务状态广播。
- 多端实时通知。
- 轻量部署。

事件例子：

```text
video.ingested
tracking.completed
annotation.accepted
dataset.exported
training.metric.created
```

### 8.3 Kafka

Kafka 适合更大规模：

- 多团队共享事件流。
- 极高吞吐。
- 长期事件保留。
- 与 Flink/Spark 大规模集成。

当前不建议第一阶段直接上 Kafka，运维成本偏高。

## 9. AI 服务与大数据的关系

AI 功能不应直接读散乱目录。它们应该通过 Go 下发任务和 artifact。

正确方式：

```text
Go 创建 ModelJob
  -> job 输入是 artifact URI / manifest URI
  -> model-worker 读取对象存储
  -> model-worker 输出 artifact
  -> Go 登记 artifact 和 lineage
```

不要：

```text
前端直接调用 Python 脚本
Python 脚本直接写正式标注
模型 worker 任意扫全盘目录
```

模型 worker 可以很多，但它们都应该服从同一个数据合同：

```text
输入：manifest + artifact refs
输出：artifact refs + metrics + logs
状态：running/completed/failed/cancelled
```

## 10. 大数据下的查询设计

前端标注需要低延迟，不能每次扫 Parquet。

建议：

```text
交互查询：PostgreSQL 索引表
大规模统计：DuckDB/Spark 扫 Parquet
媒体读取：Object Storage / file server
向量检索：pgvector/Qdrant
```

### 10.1 PostgreSQL 索引表

用于 UI：

- 视频列表。
- 每个视频 frame count。
- track 列表。
- 当前帧 boxes。
- 异常片段。
- 异常事件。
- 已保存标注。
- 删除队列。

### 10.2 Parquet 分析表

用于全量统计：

- class 分布。
- track 长度分布。
- 标注覆盖率。
- 模型 recall/IoU。
- 数据集版本差异。

### 10.3 向量库

用于 memory 和智能检索：

- 查找相似异常。
- 查找相似 object appearance。
- 查找相似轨迹。
- Agent 检索历史标注规范。

## 11. 分区策略

所有大表都应有明确分区。

推荐分区：

```text
dataset=shanghaitech
split=test
video_id=01_0014
run_id=yolo26x_botsort_v2
label_version=v20260531
export_id=oq_20260531_001
```

不要只按日期分区，因为标注系统最常见查询是：

- 某个视频。
- 某个数据集。
- 某个标签版本。
- 某个 tracking run。
- 某个训练导出版本。

## 12. 血缘与版本

每个训练样本都应该能追溯：

```text
sample_id
  -> video_id
  -> frame_idx
  -> tracking_run_id
  -> label_version
  -> feature_extractor_version
  -> export_id
  -> transform_config
```

每个模型结果也要能追溯：

```text
experiment_id
  -> export_id
  -> checkpoint
  -> metrics
  -> code_version
  -> config
```

这对科研非常重要，否则后面无法解释某次指标提升到底来自模型、标签还是数据切分。

## 13. 数据质量任务

大数据架构里必须有自动 QA。

建议任务：

- 空视频检查。
- 无 tracking 视频检查。
- 同一人重复 track 检查。
- track 突然跳变检查。
- box 过小/过大检查。
- class id 异常检查。
- frame mask 与 anomaly segment 对齐检查。
- label schema 检查。
- track 删除后是否仍被事件引用。
- export snapshot 是否完整。

这些 QA 任务由 Go 创建，DuckDB/Polars/Spark 执行，结果写回 `quality_reports`。

## 14. 本项目推荐落地路线

### Phase 1：单机数据湖

```text
Go labelserver
SQLite/PostgreSQL
文件系统 data_lake
Parquet/JSONL export
DuckDB 统计
DB task table 或 Asynq
```

目标：

- 先把数据层级和版本管起来。
- 替代散乱 CSV 和视频目录。
- 每次标注和导出都可追溯。

### Phase 2：工作站增强

```text
PostgreSQL
Redis + Asynq
MinIO
DuckDB/Polars batch jobs
Python model-worker
Prometheus/Grafana
```

目标：

- 支持全量自动标注。
- 支持更大视频集。
- 支持训练数据稳定导出。
- 支持任务失败重试。

### Phase 3：实验室/团队

```text
PostgreSQL
MinIO/S3
NATS JetStream
Spark
Iceberg
Qdrant/pgvector
多 GPU model-worker
```

目标：

- 多人协作。
- 多数据集。
- TB 级数据。
- 多模型对比。
- 大规模检索。

### Phase 4：平台化

```text
Kubernetes
Temporal
Iceberg Catalog
Spark/Flink
Trino
Kafka/NATS
GPU worker pool
OIDC/Keycloak
OpenTelemetry stack
```

目标：

- 企业级多租户。
- 自动伸缩。
- 长周期工作流。
- 大规模多端数据接入。

## 15. 对当前仓库的具体设计建议

当前 `automated_training_model` 不应该立即引入全套大数据栈。建议先在 Go 后端里抽象出这些接口：

```go
type ArtifactStore interface {
    Put(ctx context.Context, obj ArtifactObject) (ArtifactRef, error)
    Get(ctx context.Context, ref ArtifactRef) (io.ReadCloser, error)
    Stat(ctx context.Context, ref ArtifactRef) (ArtifactInfo, error)
}

type TableStore interface {
    WriteParquet(ctx context.Context, table string, rows any) (ArtifactRef, error)
    Query(ctx context.Context, query AnalyticsQuery) (QueryResult, error)
}

type TaskQueue interface {
    Enqueue(ctx context.Context, task TaskSpec) (TaskID, error)
    Cancel(ctx context.Context, id TaskID) error
    Status(ctx context.Context, id TaskID) (TaskStatus, error)
}
```

第一版实现：

```text
ArtifactStore = local filesystem
TableStore = DuckDB + Parquet
TaskQueue = DB task table
```

第二版替换：

```text
ArtifactStore = MinIO
TableStore = DuckDB/Polars + Parquet
TaskQueue = Redis + Asynq
```

第三版替换：

```text
TableStore = Iceberg + Spark/Trino
TaskQueue/EventBus = NATS/Kafka
Workflow = Temporal
```

这样架构可以逐步升级，不需要一次性重写。

## 16. 最关键的取舍

1. Go 负责大数据控制面，不负责大数据计算内核。
2. PostgreSQL 负责元数据，不负责存视频和大矩阵。
3. Object storage 负责大文件，不负责业务关系。
4. Parquet/Iceberg 负责分析数据，不负责前端低延迟交互。
5. Redis/Asynq 适合当前任务队列，Kafka/Flink 不应过早引入。
6. AI worker 通过 artifact 和 manifest 通信，不直接改正式标注。
7. 所有训练数据必须从 Gold snapshot 导出，不能从散乱中间目录直接训练。

