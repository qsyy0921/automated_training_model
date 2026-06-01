# Entry Points Test SDD

版本：v0.1  
日期：2026-06-02

## 目标

先把四类入口搭成同一个 Agent Runtime 的薄入口：

| 入口 | 当前状态 | 说明 |
| --- | --- | --- |
| CLI | ready | `labelctl runtime/channel/agent-run/governance`。 |
| Web | ready | 默认进入 Agent Overview，视频审核是二级工作台。 |
| Desktop | scaffolded | `cmd/agentdesktop` 和 `/api/desktop/status`，后续可替换为 Wails/Tauri。 |
| QQ | adapter-ready | NapCat/OneBot webhook 和 test-message API。 |

## 本地测试命令

启动服务后：

```powershell
go run .\cmd\labelctl runtime status
go run .\cmd\labelctl desktop status
go run .\cmd\labelctl channels
go run .\cmd\labelctl channel qq status
go run .\cmd\labelctl channel qq test /bot-ping
go run .\cmd\agentdesktop
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
| EP-004 | Web `/` | 首屏是 Agent Overview，不再只显示视频审核页面。 |
| EP-005 | `cmd/agentdesktop` | 可读取 `/api/desktop/status`。 |
