# Entry Points Test SDD

版本：v0.1  
日期：2026-06-02

## 目标

先把四类入口搭成同一个 Agent Runtime 的薄入口：

| 入口 | 当前状态 | 说明 |
| --- | --- | --- |
| CLI | ready | `labelctl agent` 交互式 Agent Runtime CLI；兼容 `runtime/channel/agent-run/governance` 一次性命令。 |
| Web | ready | 默认进入 Agent Overview，视频审核是二级工作台。 |
| Desktop | ready | `cmd/agentdesktop` 复用 Gateway API，支持 status、sessions、traces、jobs、send 和原始 JSON；后续可替换为 Wails/Tauri。 |
| QQ | adapter-ready | NapCat/OneBot webhook、test-message API 和可选 WebSocket reader。 |

## 本地测试命令

启动服务后：

```powershell
. .\ops\scripts\resolve-go.ps1
$go = Resolve-Go
& $go build -o .\bin\labelctl.exe .\cmd\labelctl
.\bin\labelctl.exe -addr http://127.0.0.1:7870 agent
.\bin\labelctl.exe -addr http://127.0.0.1:7870 agent "请帮我规划 ShanghaiTech 数据接入"
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime status
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime sessions
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime traces
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime model-jobs
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime job <job_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime cancel-job <job_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime resume-job <job_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime intake
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime approve-intake <workflow_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 runtime register-intake <workflow_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 desktop status
.\bin\labelctl.exe -addr http://127.0.0.1:7870 dataset list
.\bin\labelctl.exe -addr http://127.0.0.1:7870 models list
.\bin\labelctl.exe -addr http://127.0.0.1:7870 deploy task <task_id>
.\bin\labelctl.exe -addr http://127.0.0.1:7870 logs traces
.\bin\labelctl.exe -addr http://127.0.0.1:7870 doctor
.\bin\labelctl.exe -addr http://127.0.0.1:7870 channel qq test /bot-ping
```

`labelctl agent` 当前是结构化 Agent shell，参考 `ccb` / Claude Code 的可观察交互方式，启动时展示 gateway、session、entry points 和模型路由；交互命令支持 `/status`、`/sessions`、`/traces`、`/jobs`、`/doctor`、`/json`、`/clear`、`/ping`、`/exit`。其中 `/traces` 会按 agent/tool tree 摘要展示最新 trace，`/doctor` 会检查 gateway、runtime 和本机 LLM/Mimo 环境变量。

远程 Gateway 连接需要 token：

```powershell
$env:ATM_GATEWAY_TOKEN="replace_with_local_secret"
.\bin\labelctl.exe -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN runtime status
go run .\cmd\agentdesktop -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN
go run .\cmd\agentdesktop -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN sessions
go run .\cmd\agentdesktop -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN traces
go run .\cmd\agentdesktop -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN jobs
go run .\cmd\agentdesktop -addr https://atm.example.com -token $env:ATM_GATEWAY_TOKEN send /bot-ping
```

本机 loopback smoke 默认不需要 token；如果设置 `ATM_GATEWAY_REQUIRE_TOKEN_FOR_LOOPBACK=true`，CLI 和桌面端也必须带 `-token`。

Mimo 模式验收：

```powershell
. .\ops\scripts\load-mimo-env.ps1
# 在同一个 PowerShell 会话中启动 labelserver 后：
.\bin\labelctl.exe -addr http://127.0.0.1:7870 agent "请用一句话说明你是不是正在通过 Mimo planner 工作。不要调用工具。"
```

预期：`/status` 显示 `planner=python mimo=true token=true`，自然语言回复不是固定规则兜底句。

控制命令验收：`/ping`、`/status`、`channel qq test /bot-ping`、QQ OneBot `/bot-ping` 走 Go 本地 fast-path，即使启用 Mimo，也不应等待 Python/Mimo planner。

也可以直接运行一键 smoke test。默认不会主动回发真实 QQ；如果要使用当前 shell 里的 NapCat outbound 配置，增加 `-UseConfiguredQQOutbound`。

```powershell
.\ops\scripts\smoke-agent-entrypoints.ps1
.\ops\scripts\smoke-agent-entrypoints.ps1 -UseConfiguredQQOutbound
.\ops\scripts\smoke-runtime-mvp.ps1
```

`smoke-runtime-mvp.ps1` 是当前推荐的 Runtime MVP 验收脚本。它额外覆盖：

- CLI `agent` 交互式入口和 `runtime send /bot-ping`。
- Web/Gateway 可查询 runtime status、sessions、traces、model-jobs 和 intake workflows，并支持查询单个 model job、请求取消、手动 resume、审批/注册 intake workflow。
- 桌面端复用 `/api/desktop/status`、`/api/runtime/sessions`、`/api/runtime/traces`、`/api/runtime/model-jobs` 和 QQ test-message runtime path。
- QQ test-message 和 OneBot webhook 都进入同一个 Agent Runtime。
- 普通文本进入 `planner-agent`。
- 图片附件进入 `vision-agent`。
- ShanghaiTech original 数据附件进入 `data-intake-agent`，trace 包含 `workflow_id`、`intake.plan`、`dataset_name=shanghaitech-original` 和 source uri。

启用真实 QQ 回发时，先配置 NapCat OneBot HTTP API：

```powershell
. .\ops\scripts\set-qq-napcat-env.example.ps1
```

QQ/NapCat webhook 模拟：

```powershell
$body = @{
  post_type = "message"
  message_type = "private"
  message_id = "m1"
  user_id = 10001
  message = "/bot-status"
} | ConvertTo-Json -Depth 5

Invoke-RestMethod http://127.0.0.1:7870/api/channels/qq/onebot -Method Post -ContentType "application/json" -Body $body
```

图片消息模拟：

```powershell
$body = @{
  post_type = "message"
  message_type = "group"
  message_id = "m2"
  group_id = 20001
  user_id = 10001
  message = @(
    @{ type = "text"; data = @{ text = "看一下这张异常帧" } },
    @{ type = "image"; data = @{ file = "frame.png"; url = "http://example/frame.png" } }
  )
} | ConvertTo-Json -Depth 8

Invoke-RestMethod http://127.0.0.1:7870/api/channels/qq/onebot -Method Post -ContentType "application/json" -Body $body
```

## 验收

| ID | 检查 | 预期 |
| --- | --- | --- |
| EP-001 | `GET /api/runtime/status` | 返回 entry points、provider routes、sub-agents、skill evolution。 |
| EP-002 | `labelctl channel qq test /bot-ping` | 返回 `pong`。 |
| EP-003 | QQ OneBot private `/bot-status` | 返回 runtime ready 和 OneBot send payload。 |
| EP-003b | 设置 `QQ_ONEBOT_OUTBOUND_ENABLED=true` | webhook 处理后主动调用 NapCat `/send_msg`。 |
| EP-003c | `qqbot.RunWebSocketClient` 本地 fake OneBot WS | 读取 private `/bot-ping` event，经 normalizer 后在同一 WebSocket 写回 `send_msg`。 |
| EP-004 | Web `/` | 首屏是 Agent Overview，不再只显示视频审核页面。 |
| EP-005 | `cmd/agentdesktop` | 可读取 `/api/desktop/status`，并通过 sessions/traces/jobs/send 命令复用同一 Gateway runtime。 |
| EP-005b | `labelctl dataset/models/deploy/logs/doctor` | 领域命令组复用现有 Gateway API，不绕过 Agent Runtime / lifecycle / task 边界。 |
| EP-006 | `labelctl skill draft ...` | 写入 draft-only `SKILL.md`，`enabled=false`。 |
| EP-007 | `smoke-agent-entrypoints.ps1` | 自动启动服务并验证 CLI、QQ webhook、desktop 和 skill draft。 |
| EP-008 | `smoke-runtime-mvp.ps1` | 验证四入口同 runtime、sub-agent routing、model-jobs API、`vlm.inspect` trace 和带 metadata 的 ShanghaiTech `intake.plan`。 |
