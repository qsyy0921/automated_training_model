import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AnnotationWorkbenchPage } from "@pages/annotation-workbench/AnnotationWorkbenchPage";
import { apiClient } from "@shared/api/client";

export function AgentOverviewPage() {
  const [view, setView] = useState<"overview" | "review">("overview");
  const [qqText, setQQText] = useState("/bot-ping");
  const [selectedJobId, setSelectedJobId] = useState("");
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const runtime = useQuery({ queryKey: ["runtime-status"], queryFn: () => apiClient.runtimeStatus() });
  const channels = useQuery({ queryKey: ["channels"], queryFn: () => apiClient.listChannels() });
  const sessions = useQuery({ queryKey: ["runtime-sessions"], queryFn: () => apiClient.runtimeSessions(), refetchInterval: 3000 });
  const traces = useQuery({ queryKey: ["runtime-traces"], queryFn: () => apiClient.runtimeTraces(12), refetchInterval: 3000 });
  const modelJobs = useQuery({ queryKey: ["runtime-model-jobs"], queryFn: () => apiClient.runtimeModelJobs(8), refetchInterval: 3000 });
  const lifecycleTasks = useQuery({ queryKey: ["lifecycle-tasks"], queryFn: () => apiClient.listTasks(8), refetchInterval: 3000 });
  const modelJobLogs = useQuery({
    queryKey: ["runtime-model-job-logs", selectedJobId],
    queryFn: () => apiClient.runtimeModelJobLogs(selectedJobId, 30),
    enabled: selectedJobId !== "",
    refetchInterval: selectedJobId ? 3000 : false
  });
  const modelJobManifest = useQuery({
    queryKey: ["runtime-model-job-manifest", selectedJobId],
    queryFn: () => apiClient.runtimeModelJobManifest(selectedJobId),
    enabled: selectedJobId !== "",
    refetchInterval: selectedJobId ? 3000 : false
  });
  const taskLogs = useQuery({
    queryKey: ["lifecycle-task-logs", selectedTaskId],
    queryFn: () => apiClient.taskLogs(selectedTaskId, 30),
    enabled: selectedTaskId !== "",
    refetchInterval: selectedTaskId ? 3000 : false
  });
  const taskManifest = useQuery({
    queryKey: ["lifecycle-task-manifest", selectedTaskId],
    queryFn: () => apiClient.taskManifest(selectedTaskId),
    enabled: selectedTaskId !== "",
    refetchInterval: selectedTaskId ? 3000 : false
  });
  const intakeWorkflows = useQuery({ queryKey: ["runtime-intake-workflows"], queryFn: () => apiClient.runtimeIntakeWorkflows(6), refetchInterval: 3000 });
  const desktop = useQuery({ queryKey: ["desktop-status"], queryFn: () => apiClient.desktopStatus() });
  const agents = useQuery({ queryKey: ["agents"], queryFn: () => apiClient.listAgents() });
  const runs = useQuery({ queryKey: ["agent-runs"], queryFn: () => apiClient.listAgentRuns() });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => apiClient.listWorkflows() });
  const qqTest = useMutation({ mutationFn: (text: string) => apiClient.qqTestMessage(text) });

  const status = runtime.data?.runtime;
  const auth = runtime.data?.gateway?.auth;
  const recentRuns = useMemo(() => runs.data?.runs.slice(0, 5) ?? [], [runs.data?.runs]);

  if (view === "review") {
    return (
      <div className="pageSwitcher">
        <button className="btn btn-secondary" onClick={() => setView("overview")}>
          返回 Agent Overview
        </button>
        <AnnotationWorkbenchPage />
      </div>
    );
  }

  return (
    <main className="overviewShell">
      <header className="overviewHeader">
        <div>
          <p className="eyebrow">Automated Training Model</p>
          <h1>Agent Runtime 控制台</h1>
          <p>CLI、Web、桌面端和 QQ 都连接同一个 Gateway；Go 控制面负责状态、权限、审计和 workflow，Python runtime 负责后续 LLM planner。</p>
        </div>
        <div className="overviewActions">
          <button className="btn btn-primary" onClick={() => setView("review")}>
            Review Workbench
          </button>
          <button className="btn btn-secondary" onClick={() => runtime.refetch()}>
            刷新状态
          </button>
        </div>
      </header>

      <section className="overviewMetrics">
        <Metric label="Runtime" value={runtime.isLoading ? "loading" : status?.runtime ?? "offline"} />
        <Metric label="Agents" value={String(agents.data?.agents.length ?? 0)} />
        <Metric label="Workflows" value={String(workflows.data?.workflows.length ?? 0)} />
        <Metric label="Sessions" value={String(runtime.data?.snapshot.session_count ?? sessions.data?.sessions.length ?? 0)} />
      </section>

      <section className="overviewGrid">
        <Panel title="入口状态">
          <div className="overviewList">
            {(status?.entry_points ?? []).map((entry) => (
              <div className="overviewRow" key={entry.id}>
                <strong>{entry.name}</strong>
                <span>{entry.transport}</span>
                <small>{entry.status}{entry.endpoint ? ` · ${entry.endpoint}` : ""}</small>
              </div>
            ))}
            {channels.data?.channels.map((channel) => (
              <div className="overviewRow" key={channel.id}>
                <strong>{channel.name}</strong>
                <span>{channel.runtime}</span>
                <small>{channel.status} · {channel.inbound_endpoint}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Gateway Auth">
          <div className="overviewList">
            <div className="overviewRow">
              <strong>{auth?.remote_requires_token ? "Remote guarded" : "Remote open"}</strong>
              <span>token {auth?.token_configured ? "configured" : "missing"}</span>
              <small>loopback {auth?.loopback_bypass ? "bypass" : "requires token"} · origins {(auth?.allowed_origins ?? []).join(", ") || "default"}</small>
            </div>
          </div>
        </Panel>

        <Panel title="模型路由">
          <div className="overviewList">
            {(status?.provider_routes ?? []).map((route) => (
              <div className="overviewRow" key={route.id}>
                <strong>{route.id}</strong>
                <span>{route.provider} / {route.model}</span>
                <small>{route.use_case}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Sub-Agents">
          <div className="overviewList">
            {(status?.sub_agents ?? []).map((agent) => (
              <div className="overviewRow" key={agent.id}>
                <strong>{agent.name}</strong>
                <span>{agent.model_route}</span>
                <small>{agent.status} · {(agent.capabilities ?? []).join(", ")}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="入口联调">
          <div className="stack">
            <input value={qqText} onChange={(event) => setQQText(event.target.value)} />
            <button className="btn btn-primary" onClick={() => qqTest.mutate(qqText)} disabled={qqTest.isPending}>
              通过 QQ Adapter 发送
            </button>
            <small>Web 通过 QQ test-message 进入同一个 Agent Runtime；CLI 可用 runtime/status/sessions/traces，桌面端复用 Gateway API。</small>
            <pre className="jsonPreview">{qqTest.data ? JSON.stringify(qqTest.data, null, 2) : "等待测试"}</pre>
          </div>
        </Panel>

        <Panel title="Runtime Sessions">
          <div className="overviewList">
            {(sessions.data?.sessions ?? []).length === 0 ? <p className="empty">暂无 session</p> : null}
            {(sessions.data?.sessions ?? []).slice(0, 6).map((session) => (
              <div className="overviewRow" key={session.key}>
                <strong>{session.agent_id}</strong>
                <span>{session.channel} / {session.peer_kind}:{session.peer_id}</span>
                <small>{session.last_intent ?? "unknown"} · {session.last_status ?? "idle"} · {session.message_count} messages</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Runtime Traces">
          <div className="overviewList">
            {(traces.data?.traces ?? []).length === 0 ? <p className="empty">暂无 trace</p> : null}
            {(traces.data?.traces ?? []).slice(0, 6).map((trace) => (
              <div className="overviewRow" key={trace.id}>
                <strong>{trace.intent}</strong>
                <span>{trace.status} · {(trace.tool_ids ?? []).join(", ") || "no tool"}</span>
                <small>{traceSummary(trace)}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Model Jobs">
          <div className="overviewList">
            {(modelJobs.data?.jobs ?? []).length === 0 ? <p className="empty">no model jobs</p> : null}
            {(modelJobs.data?.jobs ?? []).slice(0, 6).map((job) => (
              <button
                className={`overviewRow overviewRowButton${selectedJobId === job.id ? " active" : ""}`}
                key={job.id}
                type="button"
                onClick={() => setSelectedJobId(job.id)}
              >
                <strong>{job.repo_id}</strong>
                <span>{job.status} · {job.progress_percent ?? 0}%</span>
                <small>
                  {job.message || job.error || job.id}
                  {job.resumable ? " · resumable" : ""}
                  {job.cancel_requested ? " · cancel requested" : ""}
                </small>
              </button>
            ))}
          </div>
        </Panel>

        <Panel title="Model Job Logs">
          <div className="overviewList">
            {selectedJobId === "" ? <p className="empty">选择一个 model job 查看日志</p> : null}
            {selectedJobId !== "" ? (
              <div className="overviewRow">
                <strong>{modelJobLogs.data?.job_id ?? selectedJobId}</strong>
                <span>{modelJobLogs.isLoading ? "loading" : `${modelJobLogs.data?.status ?? "unknown"} · ${modelJobLogs.data?.progress_percent ?? 0}%`}</span>
                <small>Gateway: /api/runtime/model-jobs/{selectedJobId}/logs</small>
              </div>
            ) : null}
            {selectedJobId !== "" && modelJobLogs.data ? (
              <div className="overviewRow">
                <strong>worker</strong>
                <span>
                  retryable={String(modelJobLogs.data.retryable ?? false)} · attempt={modelJobLogs.data.attempt ?? 0}/{modelJobLogs.data.max_attempts ?? 0}
                </span>
                <small>
                  {modelJobLogs.data.worker_heartbeat
                    ? `heartbeat ${compactDateTime(modelJobLogs.data.worker_heartbeat.at)} · ${modelJobLogs.data.worker_heartbeat.status} · ${modelJobLogs.data.worker_heartbeat.message || ""}`
                    : "暂无 worker heartbeat"}
                </small>
              </div>
            ) : null}
            {selectedJobId !== "" && (modelJobLogs.data?.artifacts?.length ?? 0) > 0 ? (
              <div className="overviewRow">
                <strong>artifacts</strong>
                <span>{modelJobLogs.data?.artifacts?.length ?? 0}</span>
                <small>{modelJobLogs.data?.artifacts?.[0]?.uri}</small>
              </div>
            ) : null}
            {selectedJobId !== "" && modelJobLogs.data?.metadata?.artifact_manifest ? (
              <div className="overviewRow">
                <strong>manifest</strong>
                <span>artifact manifest</span>
                <small>{modelJobLogs.data.metadata.artifact_manifest}</small>
              </div>
            ) : null}
            {selectedJobId !== "" && modelJobManifest.data?.manifest?.artifact_summary ? (
              <div className="overviewRow">
                <strong>summary</strong>
                <span>{modelJobManifest.data.manifest.artifact_summary.artifact_count ?? 0} artifacts</span>
                <small>{artifactSummaryText(modelJobManifest.data.manifest.artifact_summary)}</small>
              </div>
            ) : null}
            {selectedJobId !== "" && modelJobLogs.data?.stdout ? (
              <pre className="jsonPreview">{modelJobLogs.data.stdout}</pre>
            ) : null}
            {selectedJobId !== "" && modelJobLogs.data?.stderr ? (
              <pre className="jsonPreview">{modelJobLogs.data.stderr}</pre>
            ) : null}
            {(modelJobLogs.data?.logs ?? []).length === 0 && selectedJobId !== "" && !modelJobLogs.isLoading ? <p className="empty">暂无日志</p> : null}
            {(modelJobLogs.data?.logs ?? []).slice(-8).map((log, index) => (
              <div className="logRow" key={`${log.at}-${index}`}>
                <span>{compactDateTime(log.at)}</span>
                <strong>{log.level}</strong>
                <small>{log.message}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Lifecycle Tasks">
          <div className="overviewList">
            {(lifecycleTasks.data?.tasks ?? []).length === 0 ? <p className="empty">no lifecycle tasks</p> : null}
            {(lifecycleTasks.data?.tasks ?? []).slice(0, 6).map((task) => (
              <button
                className={`overviewRow overviewRowButton${selectedTaskId === task.id ? " active" : ""}`}
                key={task.id}
                type="button"
                onClick={() => setSelectedTaskId(task.id)}
              >
                <strong>{task.type}</strong>
                <span>{task.status} · {task.progress_percent ?? 0}%</span>
                <small>{task.message || task.id}</small>
              </button>
            ))}
          </div>
        </Panel>

        <Panel title="Lifecycle Task Logs">
          <div className="overviewList">
            {selectedTaskId === "" ? <p className="empty">选择一个 lifecycle task 查看日志</p> : null}
            {selectedTaskId !== "" ? (
              <div className="overviewRow">
                <strong>{taskLogs.data?.task_id ?? selectedTaskId}</strong>
                <span>{taskLogs.isLoading ? "loading" : `${taskLogs.data?.status ?? "unknown"} · ${taskLogs.data?.progress_percent ?? 0}%`}</span>
                <small>Gateway: /api/tasks/{selectedTaskId}/logs</small>
              </div>
            ) : null}
            {selectedTaskId !== "" && taskLogs.data ? (
              <div className="overviewRow">
                <strong>worker</strong>
                <span>
                  retryable={String(taskLogs.data.retryable ?? false)} · attempt={taskLogs.data.attempt ?? 0}/{taskLogs.data.max_attempts ?? 0}
                </span>
                <small>
                  {taskLogs.data.worker_heartbeat
                    ? `heartbeat ${compactDateTime(taskLogs.data.worker_heartbeat.at)} · ${taskLogs.data.worker_heartbeat.status} · ${taskLogs.data.worker_heartbeat.message || ""}`
                    : "暂无 worker heartbeat"}
                </small>
              </div>
            ) : null}
            {selectedTaskId !== "" && (taskLogs.data?.artifacts?.length ?? 0) > 0 ? (
              <div className="overviewRow">
                <strong>artifacts</strong>
                <span>{taskLogs.data?.artifacts?.length ?? 0}</span>
                <small>{taskLogs.data?.artifacts?.[0]?.uri}</small>
              </div>
            ) : null}
            {selectedTaskId !== "" && taskLogs.data?.metadata?.artifact_manifest ? (
              <div className="overviewRow">
                <strong>manifest</strong>
                <span>artifact manifest</span>
                <small>{taskLogs.data.metadata.artifact_manifest}</small>
              </div>
            ) : null}
            {selectedTaskId !== "" && taskManifest.data?.manifest?.artifact_summary ? (
              <div className="overviewRow">
                <strong>summary</strong>
                <span>{taskManifest.data.manifest.artifact_summary.artifact_count ?? 0} artifacts</span>
                <small>{artifactSummaryText(taskManifest.data.manifest.artifact_summary)}</small>
              </div>
            ) : null}
            {selectedTaskId !== "" && taskLogs.data?.stdout ? (
              <pre className="jsonPreview">{taskLogs.data.stdout}</pre>
            ) : null}
            {selectedTaskId !== "" && taskLogs.data?.stderr ? (
              <pre className="jsonPreview">{taskLogs.data.stderr}</pre>
            ) : null}
            {(taskLogs.data?.logs ?? []).length === 0 && selectedTaskId !== "" && !taskLogs.isLoading ? <p className="empty">暂无日志</p> : null}
            {(taskLogs.data?.logs ?? []).slice(-8).map((log, index) => (
              <div className="logRow" key={`${log.at}-${index}`}>
                <span>{compactDateTime(log.at)}</span>
                <strong>{log.level}</strong>
                <small>{log.message}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Intake Workflows">
          <div className="overviewList">
            {(intakeWorkflows.data?.workflows ?? []).length === 0 ? <p className="empty">no intake workflows</p> : null}
            {(intakeWorkflows.data?.workflows ?? []).slice(0, 6).map((workflow) => (
              <div className="overviewRow" key={workflow.id}>
                <strong>{workflow.plan.dataset_name || workflow.plan.id}</strong>
                <span>{workflow.status}</span>
                <small>
                  {workflow.id} · scans={workflow.scan_reports?.length ?? 0}
                  {workflow.approval_required ? " · approval required" : ""}
                </small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="桌面端">
          <div className="overviewRow">
            <strong>{String(desktop.data?.desktop.status ?? "unknown")}</strong>
            <span>{String(desktop.data?.desktop.profile ?? "local-desktop")}</span>
            <small>{String(desktop.data?.desktop.gateway ?? "/api/desktop/status")}</small>
          </div>
        </Panel>

        <Panel title="最近 Runs">
          <div className="overviewList">
            {recentRuns.length === 0 ? <p className="empty">暂无 run</p> : null}
            {recentRuns.map((run) => (
              <div className="overviewRow" key={run.id}>
                <strong>{run.workflow_id}</strong>
                <span>{run.status}</span>
                <small>{run.id} · {run.dataset_id || "no dataset"}</small>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Skill 自进化">
          <div className="overviewRow">
            <strong>{status?.skill_evolution.current_mode ?? "disabled"}</strong>
            <span>默认关闭</span>
            <small>{(status?.skill_evolution.controls ?? []).join(" / ")}</small>
          </div>
        </Panel>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="overviewMetric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="overviewPanel">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

function traceSummary(trace: { reply_text?: string; error?: string; session_key: string; metadata?: Record<string, string> }) {
  const metadata = trace.metadata ?? {};
  if (metadata.plan_id) {
    const parts = [`plan=${metadata.plan_id}`];
    if (metadata.dataset_name) parts.push(`dataset=${metadata.dataset_name}`);
    if (metadata.source_uri) parts.push(`source=${metadata.source_uri}`);
    return parts.join(" · ");
  }
  return trace.reply_text || trace.error || trace.session_key;
}

function compactDateTime(value: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("zh-CN", { hour12: false });
}

function artifactSummaryText(summary: {
  role_counts?: Record<string, number>;
  execution_mode_counts?: Record<string, number>;
  primary_artifact?: { role?: string; execution_mode?: string; uri?: string };
}) {
  const parts: string[] = [];
  if (summary.primary_artifact?.role) parts.push(`primary=${summary.primary_artifact.role}`);
  if (summary.primary_artifact?.execution_mode) parts.push(`mode=${summary.primary_artifact.execution_mode}`);
  if (summary.role_counts && Object.keys(summary.role_counts).length > 0) {
    parts.push(`roles ${Object.entries(summary.role_counts).map(([key, value]) => `${key}=${value}`).join(", ")}`);
  }
  if (summary.execution_mode_counts && Object.keys(summary.execution_mode_counts).length > 0) {
    parts.push(`modes ${Object.entries(summary.execution_mode_counts).map(([key, value]) => `${key}=${value}`).join(", ")}`);
  }
  if (summary.primary_artifact?.uri) parts.push(summary.primary_artifact.uri);
  return parts.join(" · ");
}
