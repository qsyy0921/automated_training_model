# Agent 架构图

这些图替换之前的两张 PNG。架构以 Mermaid 作为可维护源文件，imagegen PNG 作为视觉版参考。当前目标不是做一个页面功能，而是设计一个 **从数据采集到模型部署的 CLI-first Agent 助手**：CLI 是主入口，Go 后端是稳定控制面，Web 控制台只负责可视化、审核和运维。

## 0. Imagegen 视觉版

![全流程模型训练与部署 Agent 助手架构图](assets/agent-lifecycle-overview-imagegen.png)

![CLI-first Agent 运行时架构图](assets/agent-cli-runtime-imagegen.png)

![从数据采集到模型部署的 Agent 闭环](assets/agent-data-to-deploy-loop-imagegen.png)

## 1. 总体分层架构

```mermaid
flowchart TD
  subgraph entry["入口层"]
    cli["CLI / TUI<br/>agent ask / agent run / agent auto"]
    web["Web 控制台<br/>可视化 / 审核 / 运维"]
    api["API / Webhook<br/>外部系统集成"]
    cron["定时任务 / CI<br/>批处理触发"]
    ide["IDE / 脚本<br/>开发者工作流"]
  end

  subgraph routing["会话与路由层"]
    gateway["Gateway<br/>认证 / 租户 / 限流"]
    session["Session Runner<br/>会话状态 / 恢复"]
    router["任务路由<br/>工作流选择 / Agent 选择"]
    guards["命令守卫<br/>Schema / 权限 / 预算"]
    context["上下文管理<br/>Prompt / Memory / 压缩"]
  end

  subgraph core["Agent 核心层"]
    planner["Planner<br/>目标拆解"]
    orchestrator["Workflow Orchestrator<br/>生命周期编排"]
    executor["Tool Executor<br/>流式工具执行"]
    memory["Memory<br/>短期 / 长期 / 向量检索"]
    subagent["Subagent 协作<br/>受限并发 / 受限深度"]
  end

  subgraph lifecycle["生命周期 Agent 层"]
    collect["数据采集 Agent"]
    govern["数据治理 Agent"]
    label["标注 / 合成 Agent"]
    train["训练 Agent"]
    eval["评估 Agent"]
    release["发布 Agent"]
    deploy["部署 Agent"]
    monitor["监控反馈 Agent"]
  end

  subgraph tools["执行与能力层"]
    gotools["Go 控制工具<br/>编排 / 配置 / 元数据"]
    pyworkers["Python Worker<br/>数据处理 / 标注 / 训练"]
    wasm["Rust / WASM<br/>高性能解析 / 几何 / 图像"]
    registry["文件与对象存储工具"]
    modelgw["模型网关<br/>Provider 路由"]
    deployctl["部署控制器<br/>灰度 / 回滚"]
    metrics["监控采集器<br/>指标 / 日志 / 追踪"]
  end

  subgraph state["数据与状态层"]
    lake["Data Lake"]
    dataset["Dataset Registry<br/>数据集版本"]
    feature["Feature / Vector Store"]
    model["Model Registry<br/>模型版本"]
    artifact["Artifact Store<br/>产物版本"]
    audit["Audit Store<br/>审计日志"]
    lineage["Lineage Catalog<br/>血缘目录"]
    queue["Queue Store<br/>消息队列"]
  end

  subgraph governance["治理与安全强制路径"]
    auth["Auth<br/>认证与授权"]
    schema["Schema<br/>模式与契约"]
    policy["Policy<br/>策略与合规"]
    approval["Approval<br/>人工 / 策略审批"]
    sandbox["Sandbox<br/>执行隔离"]
    budget["Budget<br/>配额与成本"]
    rollback["Rollback<br/>恢复与回滚"]
  end

  entry --> gateway --> session --> router --> guards --> context --> planner
  planner --> orchestrator --> executor
  orchestrator --> memory
  orchestrator --> subagent
  orchestrator --> collect --> govern --> label --> train --> eval --> release --> deploy --> monitor
  executor --> gotools
  executor --> pyworkers
  executor --> wasm
  executor --> registry
  executor --> modelgw
  executor --> deployctl
  executor --> metrics
  tools --> state
  lifecycle -.产生元数据 / 指标 / 模型 / 数据版本.-> state
  governance -.每次请求与工具调用必须经过.-> routing
  governance -.每个生命周期节点必须经过.-> lifecycle
  governance -.每次执行必须经过.-> tools
```

## 2. CLI-first 运行时架构

```mermaid
flowchart LR
  user["用户 / 脚本 / CI"] --> cli["CLI Agent 主入口"]
  cli --> qe["Query Engine<br/>会话编排"]
  qe --> planner["Planner<br/>任务规划"]
  planner --> wf["Workflow Orchestrator<br/>生命周期编排"]
  wf --> gate["强制治理门<br/>权限 / Schema / 预算 / 审批"]
  gate --> exec["Streaming Tool Executor<br/>并行 / 流式 / 可观测"]
  exec --> state["App State<br/>状态持久化"]
  state --> qe

  exec --> tools["工具注册表<br/>Go / Python / Rust-WASM / MCP"]
  exec --> model["模型网关<br/>OpenAI 兼容 / 本地模型 / 路由器"]
  exec --> files["文件与对象存储"]
  exec --> deploy["部署控制器"]
  exec --> monitor["监控采集器"]

  wf --> report["运行报告<br/>过程 / 结果 / 成本 / 风险"]
  report --> cli

  web["Web 控制台"] -.只读/审核/运维.-> state
  api["REST API / Webhook"] -.外部集成.-> wf
```

设计取舍：

- CLI 是主交互面，负责对话、规划、运行和自动化。
- Go 后端是控制面，负责 Gateway、治理、状态、工作流、审计和部署元数据。
- Web 使用 TypeScript + React，定位为控制台和人工审核界面，不重写主 Agent 对话面。
- Rust / WASM 只放到性能敏感模块，例如几何计算、轨迹解析、视频帧索引和大文件解析。
- Python Worker 承接训练、评估、模型推理和数据处理生态。

## 3. 数据到部署闭环

```mermaid
flowchart TD
  request["目标输入<br/>自然语言 / 脚本 / API"] --> plan["Agent 规划<br/>拆解生命周期任务"]
  plan --> collect["1 数据采集<br/>本地目录 / 上传包 / Manifest / 对象存储 / 流数据"]
  collect --> sourceGate["来源准入<br/>租户 / 授权 / 同意 / Manifest"]
  sourceGate --> profile["2 数据画像<br/>Schema / 规模 / 缺失 / 分布 / 隐私风险"]
  profile --> govern["3 数据治理<br/>脱敏 / 去重 / 质量 / 血缘"]
  govern --> version["4 数据集版本<br/>冻结快照 / Split 隔离 / Dataset Registry"]
  version --> label["5 标注与合成<br/>人工复核 / 自动标注 / 主动学习"]
  label --> train["6 训练执行<br/>训练配方 / GPU 配额 / Checkpoint"]
  train --> eval["7 评估验证<br/>离线指标 / 安全 / 回归 / 成本延迟"]
  eval --> releaseGate{"发布门禁<br/>阈值 / 审批 / 回滚方案"}
  releaseGate -->|通过| deploy["8 发布部署<br/>灰度 / A-B / 生产别名 / 回滚控制器"]
  releaseGate -->|失败| quarantine["候选隔离<br/>失败样本 / 主动学习队列"]
  deploy --> monitor["9 线上监控<br/>质量 / 漂移 / 成本 / 延迟 / 异常"]
  monitor --> feedback["反馈回流<br/>失败样本 / 新数据 / 下一轮训练"]
  feedback --> sourceGate
  quarantine --> feedback

  version -.血缘.-> lineage["Lineage Catalog"]
  train -.产物.-> artifact["Artifact Store"]
  eval -.报告.-> audit["Audit / Report Store"]
  deploy -.发布事件.-> model["Model Registry"]
  monitor -.指标.-> metrics["Metrics / Traces"]
```

## 4. 强制治理路径

```mermaid
flowchart LR
  request["用户请求"] --> ingress["入口检查<br/>认证 / 租户 / Schema / 预算"]
  ingress --> planner["任务规划"]
  planner --> toolguard["工具预检<br/>allowlist / 参数 / 权限"]
  toolguard --> modelguard["模型预检<br/>脱敏 / 能力 / 成本 / 保留策略"]
  modelguard --> execguard["执行预检<br/>沙箱 / 文件 / 网络 / 资源"]
  execguard --> worker["隔离执行<br/>Go 工具 / Python Worker / WASM"]
  worker --> egress["结果出站检查<br/>隐私 / 安全 / 审计"]
  egress --> response["结果 / 产物 / 报告"]

  subgraph controls["不可绕过控制面"]
    auth["Auth / Tenant"]
    schema["Schema Registry"]
    policy["Policy Registry"]
    approval["Approval Queue"]
    budget["Budget / Quota"]
    audit["Audit / Trace"]
    recovery["Recovery / Rollback"]
  end

  controls -.强制注入.-> ingress
  controls -.强制注入.-> toolguard
  controls -.强制注入.-> modelguard
  controls -.强制注入.-> execguard
  controls -.强制注入.-> egress
```

## 5. 参考源码吸收的原则

- `E:\agent\cc`：保留 CLI-first、QueryEngine、工具权限提示、会话恢复和流式工具执行的运行时模式。
- `E:\agent\Hermes`：吸收 Gateway、Session Store、插件/技能、记忆提供者和多平台入口的分层方式。
- `E:\agent\openclaw`：吸收统一 Gateway、插件化能力、MCP/工具边界、沙箱与安全默认值。

落到本项目后，核心不是复制通用聊天 Agent，而是把这些模式收敛到模型工程生命周期：采集、治理、标注、训练、评估、发布、部署、监控和反馈回流。
