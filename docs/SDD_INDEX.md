# SDD 文档索引

当前保留这些有用文档：

| 文档 | 用途 |
| --- | --- |
| [SYSTEM_DESIGN_DOCUMENT.md](SYSTEM_DESIGN_DOCUMENT.md) | 当前统一 SDD 总纲，描述产品目标、分层、治理、工作流和实现状态。 |
| [AGENT_ARCHITECTURE_DIAGRAMS.md](AGENT_ARCHITECTURE_DIAGRAMS.md) | 三张 imagegen 架构图与可维护 Mermaid 源图。 |
| [AGENT_SYSTEM_DESIGN.md](AGENT_SYSTEM_DESIGN.md) | Agent Serving 与 Model/Data Training 的领域边界和治理 contract。 |
| [AGENT_RUNTIME_SDD.md](AGENT_RUNTIME_SDD.md) | Agent Runtime、Channel Router、Intent Router 和 QQ/NapCat 通信验证设计。 |
| [AGENT_RUNTIME_MVP_SDD.md](AGENT_RUNTIME_MVP_SDD.md) | Agent Runtime MVP 的当前范围、模块边界、Mimo/HF/ShanghaiTech 设计和未完成项。 |
| [AGENT_RUNTIME_MVP_ATDD.md](AGENT_RUNTIME_MVP_ATDD.md) | Agent Runtime MVP 的验收场景、证据命令和完成状态矩阵。 |
| [AGENT_RUNTIME_MVP_TDD.md](AGENT_RUNTIME_MVP_TDD.md) | Agent Runtime MVP 的单元、组件、smoke 测试策略和提交前测试清单。 |
| [AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md](AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md) | Mimo Agent Runtime 用于 HuggingFace 模型安装、校验和数据测试规划的中文 Prompt 与 tool-call JSON 约束。 |
| [REFERENCE_AGENT_RUNTIME_ALIGNMENT.md](REFERENCE_AGENT_RUNTIME_ALIGNMENT.md) | 对 OpenClaw、cc、Hermes 三个参考项目的架构取舍和本项目落点。 |
| [SUB_AGENT_STRATEGY_SDD.md](SUB_AGENT_STRATEGY_SDD.md) | 什么时候使用 sub-agent、什么时候不用，以及 Go/Python runtime 的调度规则。 |
| [INTENT_TOOL_SKILL_MCP_SDD.md](INTENT_TOOL_SKILL_MCP_SDD.md) | Intent 识别后如何映射到 Skill、Tool、MCP、Workflow 和治理执行。 |
| [IMAGE_PROXY_AND_SKILL_EVOLUTION_SDD.md](IMAGE_PROXY_AND_SKILL_EVOLUTION_SDD.md) | 图片生成反向代理为何作为 MCP/tool，skill 自进化如何默认关闭并人工审批。 |
| [ENTRYPOINTS_TEST_SDD.md](ENTRYPOINTS_TEST_SDD.md) | CLI、Web、桌面端、QQ 入口的最小测试计划。 |
| [CODE_ARCHITECTURE.md](CODE_ARCHITECTURE.md) | 当前代码结构如何对应三张架构图。 |
| [INTERFACE_DESIGN.md](INTERFACE_DESIGN.md) | CLI、本地客户端、Web 前端三类入口的界面设计和职责边界。 |
| [REMOTE_CONNECTION_SDD.md](REMOTE_CONNECTION_SDD.md) | 以本机 Gateway 为中心的 Web、CLI、桌面端和 Channel 远程连接策略与 SDD 测试计划。 |
| [QQ_CHANNEL_SDD.md](QQ_CHANNEL_SDD.md) | 当前阶段 QQ 消息入口的 SDD，定义 Channel Adapter、路由、群策略和治理边界。 |
| [CHANNEL_DATA_INGEST_SDD.md](CHANNEL_DATA_INGEST_SDD.md) | 通过 QQ 等 Channel 上传数据并由 LLM Agent 规划入湖、审核和工作流触发的设计。 |
| [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) | 仓库目录规则和长期结构。 |
| [FRONTEND_ARCHITECTURE.md](FRONTEND_ARCHITECTURE.md) | TypeScript/React 前端分层。 |
| [WASM_ACCELERATION.md](WASM_ACCELERATION.md) | Rust/WASM 加速层边界。 |
| [LLM_PROVIDER_SETUP.md](LLM_PROVIDER_SETUP.md) | LLM provider 环境变量和 CLI 使用。 |
| [PROJECT_TODO.md](PROJECT_TODO.md) | 下一阶段工程待办。 |
| [PROJECT_DONE.md](PROJECT_DONE.md) | 已完成记录和当前限制。 |

已删除的旧文档主要是 2026-05-31 的 v0.1 阶段性长文档，内容被当前统一 SDD、Agent 架构图和代码架构文档覆盖。
