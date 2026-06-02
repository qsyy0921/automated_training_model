# Image Proxy and Skill Evolution SDD

版本：v0.1  
日期：2026-06-02

## Image generation reverse proxy 应该写成 MCP 还是 Skill

结论：真实调用写成 MCP/tool adapter，Skill 只写使用流程。

原因：

- MCP/tool 负责真实 side effect：鉴权、限流、请求、下载 artifact、错误处理、审计。
- Skill 负责经验：什么时候生成图、提示词模板、输出保存到哪里、如何命名和复核。
- 反向代理 URL 和 key 不能写进 Skill，因为 Skill 会被复用、总结、同步或展示。

建议边界：

```text
skills/image-architecture-diagram/
  SKILL.md                只放工作流和提示词约束

mcp/image-generation-proxy
  generate_image(prompt, output_ref, policy)
  get_artifact(artifact_id)
  list_jobs()
```

## Provider 路由

| 能力 | 默认模型 | 入口 |
| --- | --- | --- |
| 文字规划、综合推理 | `mimo-v2.5-pro` | Python Agent Runtime / CLI LLM |
| 视觉理解 | `mimo-v2.5` | `vision-agent` |
| 图片生成 | ChatGPT 5.5 reverse proxy | MCP tool |

## Skill 自进化

默认关闭。开启后也只允许生成草稿，不允许自动启用。

```yaml
skill_evolution:
  enabled: false
  mode: draft_only
  draft_dir: data_lake/agents/skill_drafts
  require_human_approval: true
  strip_secrets: true
  allowed_sources:
    - successful_runtime_trace
    - approved_operator_note
```

流程：

```text
Runtime trace
  -> select successful repeated workflow
  -> summarize reusable steps
  -> strip secrets and raw private data
  -> draft SKILL.md
  -> human review
  -> enable skill
  -> audit promotion event
```

禁止：

- 从失败 trace 自动生成可启用 skill。
- 把 API key、cookie、QQ user id、私聊内容、原始训练数据复制进 skill。
- 让 skill 直接调用外部反向代理。
- 自动启用高风险写操作 skill。

当前代码落点：

- `workers/python/agent_runtime/skill_evolution.py`：默认关闭的配置契约。
- `internal/app/agentruntime/status.go`：通过 `/api/runtime/status` 暴露当前开关状态。
- `internal/app/skillapp`：管理 skill draft、列表、approve/reject 人工审批记录和 secret-like 内容拦截。
- `labelctl skill draft -id <skill-id> -summary <summary>`：手动写入 draft `SKILL.md`，不自动启用。
- `labelctl skill drafts`：列出草稿和审批状态。
- `labelctl skill approve-draft <skill-id>` / `reject-draft <skill-id>`：写入 `approval.json`，记录人工决策；即使 approved，`enabled=false`，后续启用/推广必须另走 promotion gate。
