# Project Done

版本：v0.1  
日期：2026-05-31

## 已完成

- [x] 将项目定位从单一视频标注工具扩展为“小模型训练到部署”的工程平台。
- [x] 建立 Go 主后端 DDD / 六边形基础目录。
- [x] 整理根目录，只保留 `cmd/ internal/ web/ docs/ ops/ go.mod README.md` 等稳定入口。
- [x] 实现 `labelserver` Go HTTP 服务。
- [x] 实现 merge/csv tracking 数据读取。
- [x] 实现帧图读取和 canvas bbox 渲染。
- [x] 实现帧级异常 mask 自动切分为异常片段。
- [x] 实现 tracking 审核、删除预览、保存删除队列。
- [x] 实现彻底删除 tracking CSV 行并自动备份。
- [x] 实现对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 多个相关对象。
- [x] 实现数据接入三种方式：本地目录、Zip 上传、Manifest 注册。
- [x] 实现数据集激活，并可切换当前 media/annotation repository。
- [x] 预留 Provider/API Key 查询边界。
- [x] 增加 `autolabel/training/evaluation/modelregistry/deployment` 领域模型。
- [x] 增加 `lifecycleapp`，统一自动标注、训练、评估、模型注册、部署任务入口。
- [x] 增加生命周期 HTTP API：
  - `POST /api/autolabel/jobs`
  - `POST /api/training/runs`
  - `POST /api/evaluation/runs`
  - `POST /api/models/register`
  - `POST /api/deployments`
  - `GET /api/tasks/{id}`
  - `DELETE /api/tasks/{id}`
- [x] 修复项目级 PowerShell UTF-8 脚本，降低中文乱码概率。
- [x] 前端从单个大文件拆成平台化模块：
  - `app`
  - `features`
  - `entities`
  - `infrastructure`
  - `shared`
  - `state`
  - `css`

## 当前限制

- [ ] 生命周期任务目前仍通过 in-memory/noop gateway 模拟排队，尚未真正调度 Python worker。
- [ ] 前端仍是原生 ES Modules，尚未引入 TypeScript、组件测试和构建流水线。
- [ ] 数据版本、标注版本、模型版本目前只有边界设计，尚未形成完整持久化模型。

