# automated_training_model

面向“小模型从数据到部署”的工程平台。当前阶段先落地视频数据接入、tracking 审核、对象级异常标注和自动标注预留接口；后续继续扩展到训练、评估、压缩、发布和线上监控。

Go 是主后端，采用 DDD / 六边形架构。Python/模型能力作为 worker 或模型服务接入，用于 YOLO/Tracking/SAM/VLM/LLM/训练任务。

目录约定：

```text
cmd/                      Go 可执行程序入口
internal/api/             HTTP/API 适配层
internal/app/             应用服务与端口接口
internal/domain/          领域模型
internal/infrastructure/  中间件、存储、队列、模型网关实现
internal/trigger/         启动入口和外部触发器
internal/types/           API DTO 和共享类型
docs/                     产品与架构文档
ops/                      部署、脚本、配置、迁移、工具和小测试数据
web/                      前端工程目录
```

## 当前功能

- 视频/帧/tracking 数据浏览。
- 前端 Canvas 渲染 tracking 框和对象 ID。
- tracking 审核、删除预览、保存审核记录、彻底删除源 tracking CSV 并自动备份。
- 对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 多个相关对象。
- 帧级异常 mask 自动切分为候选异常片段。
- 三种数据接入方式：
  - 注册本地文件夹：适合研究机器和内网服务器。
  - 上传 zip 压缩包：适合小型数据集和团队临时共享。
  - 注册 manifest：适合大数据索引、对象存储、Parquet/DuckDB/PostgreSQL 后续扩展。
- Provider/API Key、任务队列、模型网关、Agent workflow 的后端边界已预留。

## 平台目标

```text
数据接入
  -> 标注/审核
  -> 自动标注与模型辅助
  -> 数据版本管理
  -> 小模型训练
  -> 评估与误差分析
  -> 压缩/蒸馏/量化
  -> 部署发布
  -> 线上反馈回流
```

## 本机运行

```powershell
F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe run .\cmd\labelserver `
  -addr 127.0.0.1:7870 `
  -merge-root F:\keyan\token_compression\data\shanghai\new_tracking\merge `
  -frame-root F:\keyan\token_compression\data\shanghai\data\testing\frames `
  -annotation-root F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review
```

打开：

```text
http://127.0.0.1:7870/
```

## Docker 构建

需要 Docker Desktop Engine 已启动。

```powershell
docker build -f .\ops\deployments\docker\Dockerfile -t video-label-tool-labelserver .
docker run --rm -p 7870:7870 `
  -v F:\keyan\token_compression\data\shanghai\new_tracking\merge:/data/merge `
  -v F:\keyan\token_compression\data\shanghai\data\testing\frames:/data/frames `
  video-label-tool-labelserver
```

Docker Compose：

```powershell
docker compose -f .\ops\deployments\docker\docker-compose.yml up --build
```
