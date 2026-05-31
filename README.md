# video_label_tool labelserver

Go 主后端，采用 DDD / 六边形架构。

目录约定：

```text
api             HTTP/API 适配层
app             应用服务与端口接口
domain          领域模型
infrastructure 具体中间件、存储、队列、模型网关实现
trigger         启动入口和外部触发器
types           API DTO 和共享类型
cmd             可执行程序入口
docs            产品与架构文档
```

第一版先适配现有 `merge` 目录，提供视频列表、视频详情、帧级 boxes、帧图读取和预览视频读取接口。

## 本机运行

```powershell
F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe run .\cmd\labelserver `
  -addr 127.0.0.1:7870 `
  -merge-root F:\keyan\token_compression\data\shanghai\new_tracking\merge `
  -frame-root F:\keyan\token_compression\data\shanghai\data\testing\frames `
  -annotation-root F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review
```

## Docker 构建

需要 Docker Desktop Engine 已启动。

```powershell
docker build -f .\deployments\docker\Dockerfile -t video-label-tool-labelserver .
docker run --rm -p 7870:7870 `
  -v F:\keyan\token_compression\data\shanghai\new_tracking\merge:/data/merge `
  -v F:\keyan\token_compression\data\shanghai\data\testing\frames:/data/frames `
  video-label-tool-labelserver
```

Docker Compose：

```powershell
docker compose -f .\deployments\docker\docker-compose.yml up --build
```
