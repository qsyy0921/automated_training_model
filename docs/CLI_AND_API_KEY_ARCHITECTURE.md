# CLI 与大模型 API Key 接入设计

版本：v0.1  
日期：2026-05-31  
主后端：Go

## 1. 目标

平台后续需要同时支持：

- Web 标注界面。
- 桌面端。
- 手机端。
- 群聊入口。
- CLI 管理。
- 大模型 API Key 管理。
- 本地模型 worker。
- 远程模型 provider。

因此 CLI 和 API Key 不能作为临时脚本处理，必须纳入 Go 主后端架构。

## 2. CLI 应该负责什么

CLI 是运维、批处理和科研实验入口。

推荐命令：

```text
labelctl health
labelctl videos
labelctl video <scene>
labelctl import dataset --path ...
labelctl tracking run --model yolo26x --tracker botsort
labelctl tracking qa --video 01_0014
labelctl labels export --version v1 --format jsonl
labelctl tasks list
labelctl tasks logs <task_id>
labelctl providers list
labelctl providers set-key openai
labelctl model test openai --model gpt-4.1
labelctl worker start model
```

CLI 不应该绕过 Go 后端直接改数据。它应该调用 API 或复用 app service。

## 3. CLI 分层

```text
cmd/labelctl
  -> api client
  -> app service, local mode only
  -> infrastructure adapters
```

两种运行模式：

### Remote Mode

```text
labelctl -> HTTP API -> labelserver
```

适合管理已运行服务。

### Local Mode

```text
labelctl --local -> app service -> repository
```

适合离线批处理，但仍必须走同一套应用服务，不直接改 CSV/DB。

## 4. API Key 管理原则

API Key 是敏感数据，不能写进 Git 仓库，也不能写入普通日志。

原则：

1. 默认从环境变量读取。
2. 支持后续接入系统 secret store。
3. 数据库只保存 key ref，不保存明文。
4. 前端和 CLI 只能看到 masked key。
5. 日志中永远不打印明文 key。
6. Agent tool call 不允许读取明文 key。

## 5. Provider 模型

Provider 示例：

```json
{
  "id": "openai",
  "type": "openai",
  "display_name": "OpenAI",
  "base_url": "https://api.openai.com/v1",
  "api_key_ref": "env:OPENAI_API_KEY",
  "enabled": true
}
```

支持 provider：

- OpenAI。
- Qwen/DashScope。
- OpenRouter。
- Local OpenAI-compatible endpoint。
- 自定义 HTTP/gRPC model-worker。

## 6. SecretStore 抽象

Go 后端定义：

```go
type SecretStore interface {
    PutAPIKey(ctx context.Context, providerID string, displayName string, plaintext string) (APIKeySecret, error)
    GetAPIKey(ctx context.Context, ref string) (string, error)
    ListAPIKeys(ctx context.Context) ([]APIKeySecret, error)
    DeleteAPIKey(ctx context.Context, ref string) error
}
```

实现路线：

```text
第一阶段：EnvSecretStore
第二阶段：EncryptedDBSecretStore
第三阶段：Vault / cloud secret manager
```

## 7. 与 Python AI Worker 的关系

Python worker 不直接持有所有 API Key。

推荐流程：

```text
Python worker 请求执行某个 model job
Go ModelGateway 选择 provider
Go 读取 secret
Go 调用外部 LLM
或 Go 临时下发短期 token 给可信 worker
```

对于本地 GPU worker：

```text
YOLO/SAM/training worker 不需要 LLM key
VLM/LLM worker 需要 provider token 时应由 Go 控制
```

## 8. 已落地骨架

当前已加入：

- `cmd/labelctl`：最小 CLI。
- `domain/provider`：Provider 和 APIKeySecret 领域模型。
- `app.ProviderService`：Provider/Secret 应用服务。
- `infrastructure/secrets.EnvStore`：环境变量 secret store。
- `infrastructure/providerrepo.MemoryRepository`：MVP provider repository。

后续可扩展：

- `labelctl providers list`。
- `labelctl providers set-key`。
- `POST /api/providers`。
- `GET /api/providers`。
- `GET /api/secrets`。
- encrypted DB secret backend。

