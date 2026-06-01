# Automated Training Model

从数据采集到模型部署的 **CLI-first Agent 助手**。项目目标不是只做一个标注页面，而是把数据接入、数据治理、人工复核、自动标注、训练、评估、发布、部署、监控和反馈回流串成一个可审计、可恢复、可扩展的模型工程闭环。

![全流程模型训练与部署 Agent 助手架构图](docs/assets/agent-lifecycle-overview-imagegen.png)

## 核心定位

Automated Training Model 面向“小模型训练到部署”的真实工程流程：

- **核心是 Agent**：CLI 负责对话、规划、执行和自动化，Web 只是控制台和审核面。
- **后端使用 Go**：Go 承接 Gateway、会话、治理、工作流、状态、审计、模型注册和部署控制面。
- **前端使用 TypeScript + React**：用于数据浏览、视频审核、Agent 控制台、运行状态和人工审批。
- **Python 负责模型执行**：训练、评估、自动标注、VLM、tracking、segmentation 等模型生态放在 worker 层。
- **Rust / WASM 只做局部加速**：用于轨迹几何、IoU、视频帧索引、mask/RLE 等确定性高性能计算，不作为主前端框架。

## 架构总览

系统拆成两个核心域：

```text
Agent Serving Platform
  CLI / Web / API / Cron
  Gateway / Session Runner / Planner / Tool Executor
  Governance / Approval / Sandbox / Audit / Memory

Model & Data Training Platform
  Data Collection / Data Governance / Labeling
  Training / Evaluation / Release / Deployment / Monitoring
  Dataset Registry / Model Registry / Artifact Store / Lineage Catalog
```

两者只通过显式契约连接：workflow request、dataset version、model artifact、evaluation report、promotion event、audit event。运行时会话状态不会直接流入训练数据，训练数据必须经过治理、脱敏、去重、质量检查、版本冻结和血缘记录。

更多架构图见：[docs/AGENT_ARCHITECTURE_DIAGRAMS.md](docs/AGENT_ARCHITECTURE_DIAGRAMS.md)。

## 架构图

### 全流程模型训练与部署 Agent 助手架构图

![全流程模型训练与部署 Agent 助手架构图](docs/assets/agent-lifecycle-overview-imagegen.png)

### CLI-first Agent 运行时架构图

![CLI-first Agent 运行时架构图](docs/assets/agent-cli-runtime-imagegen.png)

### 从数据采集到模型部署的 Agent 闭环

![从数据采集到模型部署的 Agent 闭环](docs/assets/agent-data-to-deploy-loop-imagegen.png)

## 当前能力

- 视频、帧、tracking 数据浏览与 Canvas 渲染。
- Tracking 审核、删除预览、源 CSV 硬删除和自动备份。
- 对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 相关对象。
- 帧级 anomaly mask 自动切分候选异常片段。
- 数据接入：本地目录、上传 zip、manifest 注册。
- Agent registry、tool registry、workflow registry、run API、audit API。
- 默认全流程工作流：`data-to-deployment-lifecycle`。
- Governance control surface：强制检查点、数据治理、发布治理、运行策略、Schema、预算、租户隔离和恢复策略。
- 模型注册元数据持久化到 `data_lake/models/models.json`，模型权重和 checkpoint 不进入 Git。

## Agent 生命周期

默认主工作流覆盖完整模型工程链路：

```text
collect
  -> profile
  -> govern_data
  -> curate
  -> label_or_review
  -> train
  -> evaluate
  -> release
  -> deploy
  -> monitor
  -> report
```

CLI 是主入口：

```powershell
go run .\cmd\labelctl agent run -workflow data-to-deployment-lifecycle -dataset workspace-dataset -dry-run=true
go run .\cmd\labelctl runtime status
go run .\cmd\labelctl channel qq test /bot-ping
go run .\cmd\labelctl skill draft -id qq-data-intake-demo -summary "QQ 上传图片后进入隔离区、视觉检查、生成 Data Intake Plan"
go run .\cmd\agentdesktop
go run .\cmd\labelctl governance all
go run .\cmd\labelctl workflows
go run .\cmd\labelctl runs
```

如需 LLM 规划能力，配置 OpenAI-compatible endpoint：

```powershell
$env:LLM_BASE_URL="http://127.0.0.1:11434/v1"
$env:LLM_MODEL="qwen2.5"
$env:LLM_API_KEY=""

go run .\cmd\labelctl agent "注册一个本地数据集并创建从数据采集到部署的 dry-run 工作流"
```

## 技术栈

| 层 | 技术 | 责任 |
| --- | --- | --- |
| CLI | Go | Agent 主入口、工作流提交、治理查询、LLM 规划 |
| Backend | Go / DDD / Hexagonal | 控制面、API、注册表、审计、队列边界、模型注册 |
| Frontend | React / TypeScript / Vite | 数据审核、Agent 控制台、运行与治理可视化 |
| Acceleration | Rust / WASM | 热路径几何与轨迹计算 |
| Workers | Python | 模型推理、训练、评估、报告生成 |
| Storage | JSON MVP -> PostgreSQL / MinIO / Redis / NATS | 元数据、产物、任务状态、队列和血缘 |

## 项目结构

```text
cmd/                      Go 可执行程序入口
internal/api/             HTTP/API 适配层
internal/app/             应用服务与端口接口
internal/cli/             CLI Agent 命令与规划逻辑
internal/domain/          领域模型
internal/infrastructure/  存储、队列、模型网关、中间件实现
internal/trigger/         服务启动与外部触发器
web/                      Vite + React + TypeScript 前端工程
workers/python/           Python worker 契约与执行入口
skills/                   Agent skills
docs/                     产品、SDD、架构和维护文档
ops/                      部署、脚本、配置、迁移和工具
```

## 本机运行

先启用 PowerShell UTF-8 模式，避免中文文档、脚本输出和 Python/Go 输出在 Windows PowerShell 里乱码：

```powershell
cd F:\automated_training_model
. .\ops\scripts\utf8.ps1
.\ops\scripts\encoding-doctor.ps1
```

构建前端：

```powershell
cd F:\automated_training_model\web
npm install
npm run build
```

启动 Go 服务：

```powershell
cd F:\automated_training_model
F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe run .\cmd\labelserver `
  -addr 127.0.0.1:7870 `
  -merge-root F:\keyan\token_compression\data\shanghai\new_tracking\merge `
  -frame-root F:\keyan\token_compression\data\shanghai\data\testing\frames `
  -mask-root F:\keyan\token_compression\data\shanghai\data\testframemask `
  -annotation-root F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review `
  -web-root F:\automated_training_model\web `
  -data-root F:\automated_training_model\data_lake `
  -model-root F:\automated_training_model\data_lake\models `
  -agent-root F:\automated_training_model\data_lake\agents
```

打开：

```text
http://127.0.0.1:7870/
```

前端开发：

```powershell
cd F:\automated_training_model\web
npm run dev
```

Vite 会把 `/api` 代理到 `http://127.0.0.1:7870`。

## 文档入口

- [SDD 文档索引](docs/SDD_INDEX.md)
- [统一 SDD 总纲](docs/SYSTEM_DESIGN_DOCUMENT.md)
- [Agent 架构图](docs/AGENT_ARCHITECTURE_DIAGRAMS.md)
- [Agent 系统设计](docs/AGENT_SYSTEM_DESIGN.md)
- [Agent Runtime 设计](docs/AGENT_RUNTIME_SDD.md)
- [Sub-Agent 使用策略](docs/SUB_AGENT_STRATEGY_SDD.md)
- [Intent / Tool / Skill / MCP 设计](docs/INTENT_TOOL_SKILL_MCP_SDD.md)
- [图片生成代理与 Skill 自进化](docs/IMAGE_PROXY_AND_SKILL_EVOLUTION_SDD.md)
- [入口测试 SDD](docs/ENTRYPOINTS_TEST_SDD.md)
- [代码架构](docs/CODE_ARCHITECTURE.md)
- [三端界面设计](docs/INTERFACE_DESIGN.md)
- [远程连接策略](docs/REMOTE_CONNECTION_SDD.md)
- [Channel 数据接入](docs/CHANNEL_DATA_INGEST_SDD.md)
- [前端架构](docs/FRONTEND_ARCHITECTURE.md)
- [WASM 加速层](docs/WASM_ACCELERATION.md)
- [LLM Provider 设置](docs/LLM_PROVIDER_SETUP.md)
- [当前待办](docs/PROJECT_TODO.md)
- [完成记录](docs/PROJECT_DONE.md)

## 当前阶段

这是一个正在演进中的工程平台。当前已经完成控制面骨架、Agent/Tool/Workflow 注册表、治理模型、Web 控制台和视频审核基础能力；下一阶段重点是 durable queue、真实 Python worker runner、artifact manifest、lineage catalog、run log stream 和更严格的策略执行。
