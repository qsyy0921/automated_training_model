# 项目目录结构

版本：v0.4  
日期：2026-06-01

## 1. 根目录原则

项目根目录只保留长期稳定入口，避免脚本、配置、测试数据、临时文件散落。

```text
F:\automated_training_model
  cmd/        Go 可执行程序入口
  internal/   Go 后端核心代码
  web/        Vite + React + TypeScript 前端工程
  docs/       产品、SDD、架构和维护文档
  ops/        部署、脚本、配置、迁移、工具
  go.mod
  README.md
```

## 2. Go 后端结构

Go 后端采用 DDD / 六边形架构。

```text
internal/
  api/
    httpapi/              HTTP handler / DTO adapter
  app/
    annotationapp/        标注用例
    datasetapp/           数据集注册与管理用例
    lifecycleapp/         自动标注、训练、评估、部署生命周期任务
    mediaapp/             视频、帧、轨迹读取用例
    providerapp/          Provider/API key 查询边界
    workspaceapp/         当前数据集运行时切换
    workflowapp/          Agent / worker workflow 边界
  domain/
    annotation/
    autolabel/
    dataset/
    deployment/
    evaluation/
    media/
    modelregistry/
    provider/
    tracking/
    training/
    workflow/
  infrastructure/
    config/
    datasetrepo/
    datasetruntime/
    jsonannotation/
    mergecsv/
    middleware/
    modelgateway/
    providerrepo/
    queue/
    secrets/
  trigger/
    http/
  types/
```

依赖方向：

```text
api -> app -> domain
trigger -> api/app/infrastructure
infrastructure -> app ports + domain
```

禁止：

```text
domain -> infrastructure
domain -> api
app -> concrete postgres/redis/python worker
```

## 3. 前端结构

前端采用 FSD / 前端 DDD。

```text
web/
  package.json
  vite.config.ts
  tsconfig.json
  index.html
  src/
    app/        应用入口、Provider、全局 store、全局样式
    pages/      页面组合层
    widgets/    大块业务 UI
    features/   可交互业务能力
    entities/   业务实体模型
    shared/     API client、UI 基础组件、工具函数、配置
```

`web/dist/` 是构建产物，不进 Git。Go 服务会优先服务 `web/dist`，没有构建产物时回退到 `web/index.html`。

## 4. ops 目录

```text
ops/
  configs/      非敏感配置模板
  deployments/  Docker/K8s/服务部署配置
  migrations/   数据库迁移脚本
  scripts/      本地开发、构建、运维脚本
  testdata/     极小测试数据
  tools/        数据转换、QA、benchmark、一次性迁移工具
```

## 5. 数据与模型文件规则

不进入 Git：

- 真实视频、帧图、tracking CSV
- token cache
- checkpoint
- 模型权重
- API key
- `.env.local`
- `data_lake/`
- `tmp/`
- `web/dist/`
- `web/node_modules/`

只把最小可复现样例放入 `ops/testdata/`。
