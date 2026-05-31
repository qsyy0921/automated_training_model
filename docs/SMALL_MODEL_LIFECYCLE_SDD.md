# 小模型训练到部署平台 SDD

版本：v0.1  
日期：2026-05-31  
主后端：Go  
当前落地重点：视频标注与 tracking 审核

## 1. 产品定位

本项目不再只定位为单点视频标注工具，而是定位为“小模型训练到部署”的完整数据闭环平台。平台面向检测、追踪、分割、异常检测、动作识别、VLM 蒸馏等小模型场景，提供从数据接入、人工/自动标注、训练、评估、压缩、部署到线上反馈回流的统一工作台。

当前阶段先实现标注相关能力，因为它是后续 object query、track query、anomaly query 和小模型训练质量的基础。

```text
Dataset Registry
  -> Labeling / Review Workbench
  -> Auto Label Agent
  -> Training Job
  -> Evaluation Report
  -> Model Registry
  -> Deployment
  -> Monitoring / Feedback
```

## 2. 核心边界

### 2.1 Go Control Plane

Go 负责稳定业务、权限、数据一致性和任务编排：

- Dataset Registry：数据集登记、激活、版本、索引。
- Annotation Service：人工标注、审核、删除、导出。
- Workflow Service：训练/评估/自动标注任务生命周期。
- Model Registry：模型版本、指标、artifact、部署状态。
- Deployment Control：部署目标、灰度策略、回滚。
- Connector Gateway：Web、桌面、移动端、微信、QQ、飞书、Telegram 等入口。
- Agent Runtime：大模型辅助标注、质量检查、自动迭代策略。
- Memory / Skill / MCP：长期记忆、技能化流程和外部工具接入。

### 2.2 Python / Model Data Plane

Python worker 负责重 GPU 和模型相关能力：

- YOLO / RT-DETR 检测。
- BoT-SORT / ByteTrack / MOTR 追踪。
- SAM / SAM2 分割传播。
- GroundingDINO / YOLOWorld 开放词汇检测。
- VLM/LLM 视觉理解和标注建议。
- 训练、评估、量化、蒸馏。

Go 不直接绑定 PyTorch 环境，而是通过任务队列和 ModelGateway 调度 worker。

## 3. 当前已落地的 DDD 上下文

```text
internal/domain
  annotation   对象级异常事件、轨迹审核记录
  dataset      本地目录、上传压缩包、manifest 数据源
  media        视频、帧、异常片段
  tracking     box、track、class id、object id
  provider     LLM/VLM provider 和 key 引用
  workflow     异步任务状态
```

```text
internal/app
  annotationapp  标注用例
  datasetapp     数据集登记用例
  mediaapp       视频读取和 tracking 查询用例
  providerapp    模型 provider/key 查询用例
  workspaceapp   激活数据集，切换当前媒体和标注 repository
  workflowapp    TaskQueue / ModelGateway 端口
```

依赖方向：

```text
api -> app -> domain
infrastructure -> app ports + domain
trigger -> api/app/infrastructure
```

## 4. 三种数据接入方式

### 4.1 注册本地文件夹

适合研究机器、NAS、服务器本地盘。用户输入：

- `merge_root`：包含 `csv/`、`vis_videos/` 或 `browser_videos/`。
- `frame_root`：帧图目录。
- `mask_root`：可选，帧级异常 mask。
- `annotation_root`：可选，人工标注输出目录。

优点：

- 不复制大数据。
- 本地训练/标注最快。
- 适合 TB 级数据。

### 4.2 上传压缩包

适合小数据集、样例任务、团队临时共享。当前实现：

- Web 端上传 zip。
- Go 后端保存到 `data_lake/uploads/`。
- 自动解压到 `extracted/`。
- 尝试识别 `csv/`、`frames/`、`testframemask/` 等目录。
- 登记为 upload dataset，并可激活。

限制：

- 不适合超大视频全集。
- 大数据应使用本地文件夹或 manifest。

### 4.3 Manifest / 大数据索引

适合对象存储、分布式文件、Parquet/DuckDB/PostgreSQL 索引。manifest 可以先包含：

```json
{
  "merge_root": "F:/dataset/merge",
  "frame_root": "F:/dataset/frames",
  "mask_root": "F:/dataset/testframemask",
  "annotation_root": "F:/dataset/annotations"
}
```

后续可升级为：

- video table。
- frame table。
- box table。
- track table。
- mask/artifact URI。
- sharding 和 checksum。

## 5. 训练到部署闭环

### 5.1 数据版本

每次标注保存、tracking 删除、自动标注确认，都产生可追踪变更。后续训练 job 必须绑定：

- dataset version。
- annotation version。
- tracking source version。
- split config。
- preprocessing config。

### 5.2 训练 Job

训练任务不是直接由前端跑脚本，而是通过 Workflow：

```text
POST /api/workflows/training
  -> enqueue training job
  -> Python training worker
  -> checkpoint / metrics / logs
  -> Model Registry
```

### 5.3 评估与误差分析

评估输出：

- precision / recall / IoU / IDF1 / AUC。
- per-video 指标。
- failure cases。
- 可视化视频。
- 待人工复核样本列表。

### 5.4 模型发布

模型通过 Model Registry 进入部署：

- artifact path。
- model card。
- runtime spec。
- input/output schema。
- quantization state。
- deployment target。
- rollback version。

## 6. 后续微服务拆分

现在已经按边界拆出 Go app/domain/infrastructure。后续可独立拆分：

- `dataset-service`：数据索引和版本。
- `label-service`：标注和审核。
- `workflow-service`：任务调度。
- `model-worker-python`：GPU 任务。
- `agent-service`：LLM/VLM 工具调用和 memory。
- `deployment-service`：模型发布。

拆分触发条件：

- 某模块需要独立扩缩容。
- 某模型 worker 依赖冲突。
- 单机队列无法满足吞吐。
- 需要多团队权限隔离。

## 7. 当前实现状态

已实现：

- DDD/六边形目录。
- Go HTTP server。
- 本地 merge/csv 数据读取。
- 帧图读取和 canvas bbox 渲染。
- 帧级 mask 异常片段解析。
- tracking 删除审核、保存、彻底删除 CSV 且备份。
- 对象级异常事件标注。
- 注册本地目录、上传 zip、注册 manifest。
- 数据集激活用例。

预留但未完成：

- PostgreSQL / Redis / MinIO。
- Python model-worker 协议。
- SAM/SAM2 自动传播。
- 训练/评估/部署 API。
- 多用户权限和审计。
