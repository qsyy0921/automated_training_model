# Mimo Provider 接入说明

版本：v0.1  
日期：2026-05-31

## 1. 接入方式

Mimo 同时提供：

```text
OpenAI-compatible:
https://token-plan-cn.xiaomimimo.com/v1

Anthropic-compatible:
https://token-plan-cn.xiaomimimo.com/anthropic
```

当前 Go 后端内置两个 provider：

```text
mimo_anthropic
mimo_openai
```

二者都只保存 API key 引用：

```text
env:ANTHROPIC_AUTH_TOKEN
```

不会把明文 token 写入配置文件、数据库或 Git。

## 2. 模型约定

根据当前策略：

```text
综合推理默认模型：mimo-v2.5-pro
视觉模型：mimo-v2.5
```

也就是：

- 普通标注规则分析、异常原因生成、标注质检：用 `mimo-v2.5-pro`。
- 需要图片/关键帧视觉理解：用 `mimo-v2.5`。

## 3. PowerShell 设置

不要把真实 token 写入仓库。可以在本地 PowerShell 里设置：

```powershell
$env:ANTHROPIC_BASE_URL="https://token-plan-cn.xiaomimimo.com/anthropic"
$env:ANTHROPIC_AUTH_TOKEN="你的真实 token"
$env:ANTHROPIC_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_SONNET_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_OPUS_MODEL="mimo-v2.5-pro"
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL="mimo-v2.5-pro"
```

仓库中只保留：

- `.env.example`
- `scripts/set-mimo-env.example.ps1`

## 4. 后续工作

后续需要实现：

1. `ModelGateway` 的 Anthropic-compatible client。
2. `ModelGateway` 的 OpenAI-compatible client。
3. provider health check。
4. model capability registry。
5. 按任务自动选择 `mimo-v2.5-pro` 或 `mimo-v2.5`。
6. CLI：
   - `labelctl providers`
   - `labelctl secrets`
   - `labelctl model test mimo_anthropic`

