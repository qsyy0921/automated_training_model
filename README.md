# automated_training_model

面向“小模型从数据到部署”的工程平台。当前阶段先落地视频数据接入、tracking 审核、对象级异常标注、自动标注和训练任务入口；后续继续扩展到训练、评估、压缩、发布、线上监控和反馈回流。

Go 是主后端，采用 DDD / 六边形架构。Python、YOLO、Tracking、SAM、VLM、LLM 与训练脚本作为 worker 或模型服务接入，由 Go 控制面负责数据、任务、状态、权限和生命周期编排。

## 当前架构

```text
cmd/                      Go 可执行程序入口
internal/api/             HTTP/API 适配层
internal/app/             应用服务与端口接口
internal/domain/          领域模型
internal/infrastructure/  存储、队列、模型网关、中间件实现
internal/trigger/         服务启动与外部触发器
internal/types/           API DTO
web/                      Vite + React + TypeScript 前端工程
docs/                     产品、SDD、架构和维护文档
ops/                      部署、脚本、配置、迁移和工具
```

前端采用 FSD / 前端 DDD 分层：

```text
web/src/
  app/        应用入口、Provider、全局状态
  pages/      页面组合层
  widgets/    大块业务 UI 组合
  features/   可交互业务能力
  entities/   视频、轨迹、异常事件、数据集、任务等业务实体
  shared/     API client、设计系统组件、工具函数、配置
```

## 当前功能

- 视频 / 帧 / tracking 数据浏览。
- Canvas 渲染 tracking 框和对象 ID。
- tracking 审核、删除预览、彻底删除源 tracking CSV 并自动备份。
- 对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 多个相关对象。
- 帧级异常 mask 自动切分为候选异常片段。
- 三种数据接入方式：
  - 注册本地文件夹，适合研究机器和内网服务器。
  - 上传 zip，适合小型数据集和团队临时共享。
  - 注册 manifest，适合大数据索引、对象存储、Parquet/DuckDB/PostgreSQL 后续扩展。
- Provider/API Key、任务队列、模型网关、Agent workflow 后端边界已预留。
- 自动标注、训练、评估、模型注册、部署的 lifecycle API 已接入 Go 控制面。

## 本机运行

先构建前端：

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
  -data-root F:\automated_training_model\data_lake
```

打开：

```text
http://127.0.0.1:7870/
```

开发前端时可以单独运行 Vite：

```powershell
cd F:\automated_training_model\web
npm run dev
```

Vite 开发服务器会把 `/api` 代理到 `http://127.0.0.1:7870`。

## Docker

当前本机服务没有使用 Docker。Docker 配置保留在 `ops/deployments/docker`，需要 Docker Desktop Engine 启动后使用：

```powershell
docker compose -f .\ops\deployments\docker\docker-compose.yml up --build
```

## 维护入口

- 长期开发提示词：`docs/CODEX_GO_PROMPT.md`
- 当前待办：`docs/PROJECT_TODO.md`
- 已完成记录：`docs/PROJECT_DONE.md`
- 前端架构：`docs/FRONTEND_ARCHITECTURE.md`
- 小模型训练到部署 SDD：`docs/SMALL_MODEL_LIFECYCLE_SDD.md`
