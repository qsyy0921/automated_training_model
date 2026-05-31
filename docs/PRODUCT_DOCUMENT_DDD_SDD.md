# 通用视频打标工具产品文档与 SDD

版本：v0.1  
日期：2026-05-31  
目标实现语言：Go 后端，Web 前端  
当前迁移来源：ShanghaiTech tracking 审核与对象级异常标注原型

## 1. 产品定位

本产品是一个通用的视频打标与审核平台，面向视频目标检测、追踪、分割、动作理解、异常检测等任务。它不只提供逐帧框选能力，还需要支持从已有 tracking 结果出发，完成轨迹审核、对象级异常事件标注、关键帧挑选、SAM/视频分割模型辅助传播、大模型辅助描述与一致性检查。

核心目标是把“人工逐帧标注”升级为“人机协同的视频标注工作台”：

```text
导入视频/帧/预标注
  -> 自动生成轨迹与候选异常片段
  -> 人工审核 tracking
  -> 人工标注对象级异常事件
  -> Agent 调用 VLM/LLM 给出建议
  -> SAM/视频模型复制关键帧标注
  -> 质量检查与导出训练数据
```

## 2. 目标用户与场景

### 2.1 目标用户

- 研究人员：构建视频异常检测、object query、track query、anomaly query 训练数据。
- 标注员：审核自动 tracking 结果，补充对象级事件标注。
- 算法工程师：导入检测/追踪/分割模型结果，批量修正并导出训练集。
- 项目管理员：分配标注任务，检查进度和一致性。

### 2.2 典型使用场景

- 视频异常检测数据集：如 ShanghaiTech、UCF-Crime、Avenue、UCSD Ped2。
- 多目标追踪数据集：审核 YOLO + BoT-SORT / ByteTrack / MOTR 结果。
- 视频实例分割：选关键帧框或点，调用 SAM/SAM2 传播 mask。
- 行为理解：对一个异常片段内多个事件、多对象关系进行标注。
- VLM 训练数据构建：把对象轨迹、异常原因、外貌描述整理为结构化 JSONL。

## 3. 产品范围

### 3.1 MVP 范围

MVP 以“已有视频 + 已有 tracking 标注”为输入，支持：

- 数据集导入：视频、帧目录、tracking CSV/MOT、帧级 mask。
- 视频列表浏览、搜索、类别筛选。
- 帧级播放：上一帧、下一帧、自动播放、倍速、锁定片段播放。
- 轨迹可视化：服务端渲染或前端 canvas 渲染 bbox、ID、类别颜色。
- 轨迹审核：通过、加入删除队列、保存删除队列、彻底删除源 tracking 数据、备份与恢复。
- 对象级异常标注：
  - 视频 -> 异常时间段 -> 异常事件 -> 相关对象列表。
  - 每个事件可绑定多个时间段。
  - 每个事件可绑定多个对象。
  - 每个对象在事件内拥有外貌/角色/行为描述。
- 标注保存、删除、导出 JSONL。
- 帧级异常 mask 导入，自动生成候选异常时间段。

### 3.2 增强范围

- 关键帧推荐：自动挑选最值得标注的帧。
- Agent 辅助标注：调用 VLM/LLM 生成候选描述、异常原因、对象关系。
- SAM/SAM2 辅助传播：从关键帧 bbox/点/mask 生成整段 mask/box。
- 质量检查：漏标、重复 ID、类别冲突、时间段冲突、异常事件缺对象等。
- 多用户任务分配与审计日志。
- 插件化模型服务：YOLO、BoT-SORT、SAM、GroundingDINO、YOLOWorld、Qwen-VL 等。

### 3.3 非目标

- MVP 不做完整训练平台。
- MVP 不强依赖某一个数据集格式。
- MVP 不要求自动标注直接覆盖人工结果，所有模型建议默认需要人工确认。

## 4. 术语表

| 术语 | 含义 |
| --- | --- |
| Dataset | 一个数据集工程，包含多个视频、标注配置和导出配置 |
| Video | 单个视频或帧序列 |
| Frame | 视频中的一帧，具有 frame index、timestamp、image path |
| Track | 一个跨帧对象轨迹，由 class_id + track_id 唯一标识 |
| Object | 某个具体标注对象，通常对应一个 Track，也可以是人工创建对象 |
| Segment | 视频中的时间片段，如异常片段 155-230 |
| Event | 一个语义事件，可属于一个或多个 Segment |
| Event Object | 事件相关对象，包含 object id、外貌、角色、行为描述 |
| Review Action | 对 tracking 的审核动作，如通过、删除、修正类别 |
| Annotation | 标注记录，包含检测框、轨迹审核、事件、对象描述等 |
| Keyframe | 被人工或算法选中的代表帧，用于标注或传播 |
| Propagation | 从关键帧标注传播到时间片段其他帧 |
| Agent Task | 由智能体执行的辅助任务，如生成描述、检查冲突、调用模型 |

## 5. 核心数据结构

### 5.1 对象级异常标注层级

产品必须支持如下层级：

```text
Video
  -> Anomaly Segment 1..N
      -> Anomaly Event 1..N
          -> Event Object 0..N
              -> track_id / object_id
              -> object class
              -> appearance description
              -> role in event
              -> action description
          -> event reason / description
          -> severity / confidence / review status
```

示例：

```json
{
  "video_id": "01_0014",
  "segments": [
    {
      "segment_id": "seg01",
      "start_frame": 155,
      "end_frame": 230,
      "events": [
        {
          "event_id": "01_0014-seg01-event01",
          "event_type": "bicycle",
          "reason_zh": "白色上衣黑色裤子的人骑自行车进入人行区域",
          "objects": [
            {
              "track_key": "0:6",
              "track_id": 6,
              "class_id": 0,
              "class_name": "person",
              "appearance_zh": "白色上衣，黑色裤子",
              "role_zh": "骑车的人"
            },
            {
              "track_key": "1:5",
              "track_id": 5,
              "class_id": 1,
              "class_name": "bicycle",
              "appearance_zh": "黑色自行车",
              "role_zh": "异常交通工具"
            }
          ]
        }
      ]
    }
  ]
}
```

### 5.2 Tracking 审核数据

Tracking 审核必须和异常事件标注分开，避免把“删除误检轨迹”和“对象异常”混在一起。

```json
{
  "video_id": "01_0016",
  "track_key": "80:23",
  "class_id": 80,
  "track_id": 23,
  "review_status": "deleted",
  "issue": "false_positive",
  "start_frame": 1,
  "end_frame": 337,
  "operator": "human",
  "created_at": "2026-05-31T12:00:00+08:00"
}
```

### 5.3 数据导出格式

MVP 提供三类导出：

- `tracking_clean.csv`：清洗后的 tracking 表。
- `object_anomaly.jsonl`：对象级异常训练数据。
- `review_audit.jsonl`：审核日志和删除记录。

建议 JSONL 每行对应一个事件对象，方便训练 object query / anomaly query：

```json
{
  "dataset": "ShanghaiTech",
  "video_id": "01_0014",
  "segment_id": "seg01",
  "event_id": "01_0014-seg01-event01",
  "start_frame": 155,
  "end_frame": 230,
  "event_type": "bicycle",
  "event_reason_zh": "白色上衣黑色裤子的人骑自行车进入人行区域",
  "track_key": "0:6",
  "track_id": 6,
  "class_id": 0,
  "class_name": "person",
  "appearance_zh": "白色上衣，黑色裤子",
  "role_zh": "骑车的人",
  "bbox_quality": "ok",
  "review_status": "accepted"
}
```

## 6. 功能需求

### 6.1 数据导入

支持导入：

- 视频文件：mp4、avi、mov。
- 帧序列：jpg/png。
- Tracking：MOT Challenge txt、CSV、自定义 JSON。
- 检测/分割结果：COCO JSON、YOLO txt、mask png、RLE。
- 帧级异常标签：mask 文件、frame range 文件。

导入时需要生成统一索引：

```text
dataset_id
video_id
frame_count
fps
width / height
frame_path / video_path
track_count
class_distribution
annotation_count
```

### 6.2 视频和帧浏览

必须支持：

- 按视频 ID 搜索。
- 按类别筛选视频。
- 显示每个视频的帧数、轨迹数、框数、标注数、异常帧数。
- 帧级展示中直接显示 tracking 框和 ID。
- 支持倍速播放：0.25x、0.5x、1x、2x、4x。
- 支持锁定片段播放：整段视频、异常片段、用户自定义片段。
- 支持点击画面中的对象选中对应 track。

### 6.3 Tracking 审核

用户可以：

- 选中一个 track。
- 查看该 track 的出现次数、起止帧、平均置信度、类别。
- 将 track 加入删除预览。
- 保存删除队列为审核记录。
- 彻底删除源 tracking 数据。
- 恢复未彻底删除的审核记录。
- 删除前自动备份原始 tracking 文件。

删除策略：

```text
加入删除预览：只在前端临时隐藏，不写盘。
保存删除队列：写入 review annotation，服务端过滤显示，但源 CSV 不变。
彻底删除数据：备份源 CSV，然后删除对应 track_key 的所有行，重载索引。
```

### 6.4 异常时间段管理

异常时间段来源：

- 帧级 mask 自动提取。
- 用户手动创建。
- 根据当前对象轨迹和锁定片段交集生成。

规则：

- 一个视频可以有多个异常时间段。
- 一个异常时间段可以有多个异常事件。
- 一个异常事件可以属于一个或多个异常时间段。
- 锁定某个异常时间段后，播放和帧滚动范围固定在该片段内。

### 6.5 异常事件标注

每个异常事件包含：

- 所属异常时间段。
- 异常类型。
- 异常原因/描述。
- 严重程度。
- 置信度。
- 相关对象列表。

默认行为：

- 每个异常时间段默认创建一个异常事件草稿。
- 如果一个片段内有多个异常，用户点击“新增属于本时间段的异常事件”。

### 6.6 事件对象标注

每个事件对象包含：

- object id / track id。
- object class。
- 上衣颜色。
- 下衣颜色。
- 上衣类型。
- 下衣类型。
- 携带物。
- 角色描述。
- 当前对象在该异常中的行为/外貌描述。

要求：

- 一个异常事件默认展示 5 个对象槽位。
- 用户可以新增、删除、清空对象槽位。
- 对象槽位横向排列，可左右滚动。
- 聚焦对象槽位时，A/D 在对象槽位内循环切换。
- 只有非 unknown 字段写入导出数据。

### 6.7 关键帧标注

关键帧来源：

- 用户手动选择。
- 模型推荐。
- 片段首帧、中间帧、尾帧。
- 轨迹速度峰值帧。
- 检测置信度低或 ID switch 风险高的帧。
- 异常 mask 边界帧。

关键帧用途：

- 标注 bbox。
- 标注点 prompt。
- 标注 mask。
- 调用 SAM/SAM2 传播。
- 生成事件描述候选。

### 6.8 SAM/SAM2 辅助传播

传播流程：

```text
用户选定片段
  -> 选择关键帧
  -> 选择对象 bbox/点/mask
  -> 调用 SAM/SAM2 得到关键帧 mask
  -> 在片段内传播 mask/box
  -> 生成候选轨迹或修正轨迹
  -> 用户预览
  -> 用户接受/拒绝/局部修正
```

传播结果必须进入候选层，不得直接覆盖人工标注。

### 6.9 Agent 辅助标注

Agent 不直接替代人工标注，它负责生成候选、检查一致性和降低重复劳动。

Agent 能力：

- 自动挑关键帧。
- 调用 VLM 描述当前帧、对象、事件。
- 根据 tracking 轨迹生成对象行为摘要。
- 根据帧级 mask 和对象轨迹推荐异常对象。
- 检查标注一致性：
  - 事件无对象。
  - 对象不在事件时间段内。
  - 同一个 track 同时被标成删除和异常。
  - 类别和描述冲突。
  - 时间段不覆盖对象出现帧。
- 调用 SAM/SAM2 执行传播任务。
- 生成中英文标签映射。

Agent 输出默认状态为 `suggested`，必须人工接受后才变成 `accepted`。

## 7. 非功能需求

### 7.1 性能

- 1000 个视频以内的数据集，视频列表加载小于 2 秒。
- 单帧图像加载小于 300 ms。
- 轨迹查询小于 100 ms。
- 保存单条标注小于 100 ms。
- 批量删除 100 条轨迹小于 2 秒。
- 模型任务异步执行，前端不阻塞。

### 7.2 可靠性

- 所有 destructive 操作必须有确认。
- 所有彻底删除必须备份。
- 标注保存必须是追加或事务式写入。
- 服务异常退出后标注不能损坏。
- 模型任务失败不影响人工标注。

### 7.3 可扩展性

- 支持多数据集。
- 支持多模型插件。
- 支持不同标注 schema。
- 支持导出为训练所需格式。

### 7.4 可审计性

每次修改必须记录：

- 操作人。
- 操作时间。
- 操作类型。
- 操作对象。
- 修改前后摘要。
- 关联任务 ID。

## 8. DDD 架构设计

### 8.1 Bounded Context

```text
Project Context
  管理项目、数据集、用户、任务。

Media Context
  管理视频、帧、播放片段、媒体索引。

Tracking Context
  管理检测框、轨迹、类别、tracking 审核和清洗。

Annotation Context
  管理异常片段、异常事件、事件对象、外貌描述。

Model Context
  管理模型、推理任务、SAM 传播、VLM/LLM 调用。

Agent Context
  管理 Agent workflow、任务编排、建议、质量检查。

Export Context
  管理数据导出、格式转换、版本快照。
```

### 8.2 聚合设计

#### Dataset Aggregate

- Dataset
- LabelSchema
- ClassMap
- ImportJob

不变量：

- dataset_id 唯一。
- class map 可版本化。

#### Video Aggregate

- Video
- FrameIndex
- PlaybackSegment

不变量：

- frame index 从 1 开始。
- segment start <= end。

#### Track Aggregate

- Track
- DetectionBox
- TrackReview
- TrackRevision

不变量：

- track_key = class_id + ":" + track_id。
- 同一个 video 内 track_key 唯一。
- 彻底删除必须先生成 backup revision。

#### Annotation Aggregate

- AnomalySegment
- AnomalyEvent
- EventObject
- AppearanceDescriptor

不变量：

- Event 必须至少绑定一个 Segment。
- accepted Event 建议至少有一个 EventObject。
- EventObject 的 track 必须在 event 时间范围内有交集。

#### AgentTask Aggregate

- AgentTask
- ToolCall
- Suggestion
- HumanDecision

不变量：

- Agent 只能生成 suggestion。
- suggestion 未被接受前不进入正式标注。

### 8.3 领域服务

- `TrackCleanerService`：删除、恢复、合并、拆分轨迹。
- `SegmentBuilderService`：从 mask 或人工选择生成异常片段。
- `EventAnnotationService`：创建事件、绑定对象、校验事件。
- `KeyframeSelectorService`：关键帧挑选。
- `PropagationService`：调用 SAM/SAM2 做传播。
- `AgentOrchestrator`：编排 LLM/VLM/SAM/质量检查。
- `ExportService`：导出训练数据。

### 8.4 Repository 接口

Go 后端中用接口隔离存储：

```go
type VideoRepository interface {
    List(ctx context.Context, filter VideoFilter) ([]VideoSummary, error)
    Get(ctx context.Context, id VideoID) (*Video, error)
    GetFrame(ctx context.Context, id VideoID, frame int) (*Frame, error)
}

type TrackRepository interface {
    ListTracks(ctx context.Context, videoID VideoID) ([]Track, error)
    ListBoxes(ctx context.Context, videoID VideoID, frame int) ([]DetectionBox, error)
    PurgeTracks(ctx context.Context, videoID VideoID, keys []TrackKey) (PurgeResult, error)
}

type AnnotationRepository interface {
    List(ctx context.Context, videoID VideoID) ([]Annotation, error)
    Save(ctx context.Context, ann Annotation) error
    Delete(ctx context.Context, videoID VideoID, annID AnnotationID) error
}
```

## 9. SDD：软件设计文档

### 9.1 总体架构

```text
Web UI
  -> REST / WebSocket API
      -> Application Services
          -> Domain Services / Aggregates
              -> Repositories
                  -> File Storage / DB / Object Storage
      -> Model Gateway
          -> YOLO / SAM / VLM / LLM / Custom Python Services
      -> Agent Runtime
          -> Workflow Engine / Tool Calls / Suggestion Store
```

### 9.2 后端技术选型

- Go 1.22+。
- HTTP 框架：标准库 `net/http` 优先，后期可换 chi。
- 存储：
  - MVP：文件系统 + JSONL + CSV。
  - 正式版：SQLite/PostgreSQL + 对象存储。
- 异步任务：Go worker pool + task table。
- 模型调用：
  - 本地 Python 推理服务。
  - HTTP/gRPC 模型网关。
  - 大模型 API。
- 前端：
  - MVP：原生 HTML/CSS/JS 或轻量框架。
  - 正式版：React/Vue/Svelte 均可。

### 9.3 服务分层

```text
/cmd/labelserver
  main.go

/internal/domain
  media/
  tracking/
  annotation/
  agent/
  model/
  export/

/internal/application
  video_service.go
  track_service.go
  annotation_service.go
  agent_service.go
  export_service.go

/internal/infrastructure
  filesystem/
  sqlite/
  modelgateway/
  sam/
  llm/

/internal/interfaces/http
  routes.go
  handlers_video.go
  handlers_track.go
  handlers_annotation.go
  handlers_agent.go

/web
  static/
  templates/
```

### 9.4 API 设计

#### 视频与帧

```http
GET /api/projects
GET /api/datasets
GET /api/videos?dataset_id=&class_id=&q=
GET /api/videos/{video_id}/meta
GET /api/videos/{video_id}/frames/{frame}.jpg
GET /api/videos/{video_id}/boxes?frame=
```

#### Tracking 审核

```http
POST /api/videos/{video_id}/tracks/{track_key}/review
POST /api/videos/{video_id}/tracks/purge
POST /api/videos/{video_id}/tracks/restore
POST /api/videos/{video_id}/tracks/merge
POST /api/videos/{video_id}/tracks/split
```

#### 异常标注

```http
GET  /api/videos/{video_id}/annotations
POST /api/videos/{video_id}/segments
POST /api/videos/{video_id}/events
POST /api/videos/{video_id}/events/{event_id}/objects
PUT  /api/videos/{video_id}/events/{event_id}
DELETE /api/videos/{video_id}/events/{event_id}
DELETE /api/videos/{video_id}/events/{event_id}/objects/{object_id}
```

#### Agent 与模型任务

```http
POST /api/videos/{video_id}/agent/keyframes
POST /api/videos/{video_id}/agent/suggest-event
POST /api/videos/{video_id}/agent/check-quality
POST /api/videos/{video_id}/propagation/sam
GET  /api/tasks/{task_id}
POST /api/suggestions/{suggestion_id}/accept
POST /api/suggestions/{suggestion_id}/reject
```

#### 导出

```http
POST /api/export
GET  /api/export/{job_id}
GET  /api/export/{job_id}/download
```

### 9.5 存储设计

MVP 文件布局：

```text
project_root/
  datasets/
    dataset.json
    videos/
    frames/
    tracking/
      raw/
      clean/
      backups/
    masks/
    annotations/
      review.jsonl
      anomaly_events.jsonl
      suggestions.jsonl
    exports/
```

正式数据库表：

- projects
- datasets
- videos
- frames
- tracks
- detection_boxes
- track_reviews
- anomaly_segments
- anomaly_events
- event_objects
- appearance_descriptors
- agent_tasks
- model_suggestions
- audit_logs
- export_jobs

### 9.6 并发与任务调度

模型任务采用异步模式：

```text
HTTP 请求创建 task
  -> task 入队
  -> worker 执行
  -> 写入 task result / suggestions
  -> 前端轮询或 WebSocket 接收进度
  -> 用户接受 suggestion
```

任务类型：

- `keyframe_selection`
- `vlm_frame_caption`
- `track_summary`
- `event_suggestion`
- `sam_propagation`
- `quality_check`
- `export`

### 9.7 错误处理

- API 返回结构化错误：

```json
{
  "error_code": "TRACK_NOT_FOUND",
  "message": "track 0:6 not found in video 01_0014",
  "recoverable": true
}
```

- 模型任务失败不影响主服务。
- 文件写入失败必须保留临时文件和日志。
- 删除失败必须可从 backup 恢复。

### 9.8 安全设计

- destructive 操作必须二次确认。
- 批量删除必须生成备份。
- 用户输入必须转义，避免 XSS。
- 模型调用日志不能泄露敏感路径或密钥。
- 多用户版本需要鉴权与权限控制。

## 10. Agent 工作流设计

### 10.1 Agent 角色

| Agent | 职责 |
| --- | --- |
| Keyframe Agent | 挑选关键帧 |
| Vision Agent | 调用 VLM 解释帧和对象 |
| Tracking QA Agent | 检查误检、重复 ID、短轨迹 |
| Anomaly Suggestion Agent | 根据片段和轨迹建议异常事件 |
| SAM Propagation Agent | 调用 SAM/SAM2 复制标注 |
| Consistency Agent | 检查标注一致性 |
| Export Agent | 检查导出格式和字段完整性 |

### 10.2 Agent 状态机

```text
Created
  -> Running
  -> ToolCalling
  -> WaitingHumanReview
  -> Accepted / Rejected / Failed
```

### 10.3 大模型辅助标注流程

```text
用户锁定异常片段
  -> Keyframe Agent 选 3-5 帧
  -> Vision Agent 对关键帧生成中文描述
  -> Tracking QA Agent 找候选异常对象
  -> Anomaly Agent 生成候选事件
  -> 用户编辑/接受
  -> Consistency Agent 检查冲突
  -> 保存正式标注
```

### 10.4 LLM/VLM Prompt 输入

输入必须结构化：

```json
{
  "video_id": "01_0014",
  "segment": {"start": 155, "end": 230},
  "keyframes": [155, 186, 230],
  "tracks": [
    {"track_key": "0:6", "class": "person", "boxes": "...", "appearance": "unknown"},
    {"track_key": "1:5", "class": "bicycle", "boxes": "...", "appearance": "unknown"}
  ],
  "task": "请用中文给出该异常片段中可能的异常事件和相关对象。"
}
```

输出必须是 JSON，不直接写正式标注：

```json
{
  "suggestions": [
    {
      "event_type": "bicycle",
      "reason_zh": "有人骑自行车进入人行区域",
      "objects": [
        {"track_key": "0:6", "role_zh": "骑车的人"},
        {"track_key": "1:5", "role_zh": "自行车"}
      ],
      "confidence": 0.72
    }
  ]
}
```

## 11. UI 设计

### 11.1 主布局

```text
左侧：视频列表 / 搜索 / 类别筛选 / 导出
中间上方：视频或帧级播放区，显示 tracking boxes
中间下方：异常片段预览 / 轨迹列表 / 关键帧列表
右侧：当前对象 / 轨迹审核 / 异常事件 / 对象槽位 / 已保存标注
```

### 11.2 交互原则

- 一切标注流程必须在网页端完成。
- 任何删除动作先进入预览，再保存或彻底删除。
- 所有保存按钮明确区分：
  - 保存审核记录。
  - 保存异常事件。
  - 彻底删除源数据。
- 所有长列表必须支持滚动。
- 横向对象槽位必须支持左右滚动和快捷键。
- 播放区可点击对象并同步右侧表单。

### 11.3 快捷键

| 快捷键 | 作用 |
| --- | --- |
| A | 在播放区聚焦时切换上一个视频 |
| D | 在播放区聚焦时切换下一个视频 |
| A/D | 在对象槽位聚焦时切换上/下一个对象槽位 |
| Ctrl+S | 保存当前视频待保存内容 |
| ← / → | 上一帧 / 下一帧 |
| Space | 播放 / 暂停 |

## 12. 质量控制

### 12.1 标注完整性检查

- 异常片段无异常事件。
- 异常事件无相关对象。
- EventObject 缺少 track id。
- EventObject 的 track 与事件时间段无交集。
- 同一个 track 同时被彻底删除和引用为异常对象。
- 类别与异常类型明显冲突。
- appearance 字段全部 unknown。

### 12.2 Tracking 质量检查

- 短轨迹。
- 低置信轨迹。
- 重复轨迹。
- 轨迹突然跳变。
- 类别切换。
- bbox 面积异常。
- 同一帧同类高 IoU 重叠。

### 12.3 模型建议质量

- suggestion confidence。
- 人工接受率。
- 被修改字段比例。
- SAM propagation IoU consistency。
- 关键帧覆盖率。

## 13. 迁移计划

### Phase 0：整理现有原型

- 保留当前 Go 单文件服务作为功能参考。
- 将数据读取、tracking 清洗、annotation 保存拆为接口。
- 固化当前 ShanghaiTech schema。

### Phase 1：DDD 重构 MVP

- 建立 Go 项目结构。
- 实现 Media / Tracking / Annotation 三个核心 Context。
- 完成现有前端功能迁移。
- 支持 CSV/MOT/JSONL 导入导出。

### Phase 2：Agent 与模型网关

- 增加任务队列。
- 接入关键帧选择。
- 接入 VLM/LLM 建议。
- 接入 SAM/SAM2 传播。
- 增加 suggestion review UI。

### Phase 3：多数据集和多用户

- 项目/数据集管理。
- 用户和权限。
- 任务分配。
- 标注一致性统计。

### Phase 4：训练数据闭环

- 导出 object query / track query / anomaly query 训练格式。
- 统计训练覆盖率。
- 根据训练失败样例回流标注任务。

## 14. 验收标准

### 14.1 MVP 验收

- 能导入一个包含 100+ 视频的数据集。
- 能正确显示视频、帧、bbox、track id。
- 能删除误检轨迹，并生成备份。
- 能创建多个异常片段、多个异常事件、多个事件对象。
- 能导出完整 JSONL。
- 所有核心流程可在网页端完成。

### 14.2 Agent 验收

- Agent 能为异常片段挑选关键帧。
- Agent 能生成候选异常事件和相关对象。
- Agent 建议不会自动覆盖人工标注。
- SAM 传播结果可预览、接受、拒绝。
- 质量检查能发现关键冲突。

### 14.3 性能验收

- 单帧切换流畅，无明显黑屏。
- 轨迹列表 1000 条以内可交互。
- 保存和删除操作有明确反馈。
- 长任务显示进度并可失败恢复。

## 15. 开发优先级

P0：

- DDD 目录重构。
- 视频/帧/track/annotation 的稳定数据模型。
- 当前网页功能完整迁移。
- 彻底删除源数据与备份。
- JSONL 导出。

P1：

- 关键帧选择。
- SAM 传播任务。
- Agent suggestion 存储与审核。
- 质量检查。

P2：

- 多用户。
- 权限。
- 数据集版本管理。
- 在线训练闭环。

## 16. 主要风险

- 自动 tracking 噪声会污染对象级异常标签。
- 大模型描述可能幻觉，必须人工确认。
- SAM 传播在遮挡和小目标场景可能漂移。
- 不同数据集 class id 不统一，需要 class map 版本化。
- 彻底删除源数据有风险，必须保留备份和审计。
- 如果 UI 过于拥挤，标注员效率会下降，需要模块折叠和滚动区域设计。

## 17. 当前项目落地建议

短期不建议一上来做复杂多用户系统。建议先把当前 ShanghaiTech 原型重构为通用单机版：

```text
Go 后端 DDD 分层
+ 文件系统存储
+ CSV/MOT/JSONL 适配器
+ 当前 Web UI
+ Agent task 接口占位
+ SAM task 接口占位
```

等 tracking 审核和对象级异常标注稳定后，再引入模型服务和多用户能力。

