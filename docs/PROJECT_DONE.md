# Project Done

版本：v0.2  
日期：2026-06-01

## 已完成

- [x] 将项目定位从单一视频标注工具扩展为“小模型训练到部署”的工程平台。
- [x] 建立 Go 主后端 DDD / 六边形基础目录。
- [x] 将项目迁移到独立根目录 `F:\automated_training_model`，避免继续受旧 ShanghaiTech 任务目录影响。
- [x] 配置 GitHub remote：`https://github.com/qsyy0921/automated_training_model.git`。
- [x] 实现 `labelserver` Go HTTP 服务。
- [x] 实现 merge/csv tracking 数据读取。
- [x] 实现帧图读取和 bbox overlay 数据 API。
- [x] 实现帧级异常 mask 自动切分为异常片段。
- [x] 实现 tracking 审核、删除预览、保存删除队列。
- [x] 实现彻底删除 tracking CSV 行并自动备份。
- [x] 实现对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 多个相关对象。
- [x] 实现数据接入三种方式：本地目录、zip 上传、manifest 注册。
- [x] 实现数据集激活，并可切换当前 media/annotation repository。
- [x] 预留 Provider/API Key 查询边界。
- [x] 增加 `autolabel/training/evaluation/modelregistry/deployment` 领域模型。
- [x] 增加 `lifecycleapp`，统一自动标注、训练、评估、模型注册、部署任务入口。
- [x] 增加 lifecycle HTTP API：
  - `POST /api/autolabel/jobs`
  - `POST /api/training/runs`
  - `POST /api/evaluation/runs`
  - `POST /api/models/register`
  - `POST /api/deployments`
  - `GET /api/tasks/{id}`
  - `DELETE /api/tasks/{id}`
- [x] 修复项目级 PowerShell UTF-8 脚本，降低中文乱码概率。
- [x] 前端从原生 ES Modules 迁移到 Vite + React + TypeScript。
- [x] 前端按 FSD / 前端 DDD 拆分：
  - `app`
  - `pages`
  - `widgets`
  - `features`
  - `entities`
  - `shared`
- [x] 前端接入 TanStack Query 管理服务端状态。
- [x] 前端接入 Zustand 管理 UI/draft 状态。
- [x] 增加平台型工作台视觉风格和集中设计 tokens。
- [x] Go 服务支持优先服务 `web/dist` 构建产物。

## 当前限制

- [ ] lifecycle 任务目前仍通过 in-memory/noop gateway 模拟排队，尚未真正调度 Python worker。
- [ ] Zod 只作为依赖接入，API runtime schema 尚未完整覆盖。
- [ ] 前端仍有少量 `alert/confirm`，后续需要统一 toast/dialog。
- [ ] 数据版本、标注版本、模型版本目前只有边界设计，尚未形成完整持久化模型。
