import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AnnotationWorkbenchPage } from "@pages/annotation-workbench/AnnotationWorkbenchPage";
import { apiClient } from "@shared/api/client";

export function AgentOverviewPage() {
  const [view, setView] = useState<"overview" | "review">("overview");
  const [qqText, setQQText] = useState("/bot-ping");
  const runtime = useQuery({ queryKey: ["runtime-status"], queryFn: () => apiClient.runtimeStatus() });
  const channels = useQuery({ queryKey: ["channels"], queryFn: () => apiClient.listChannels() });
  const agents = useQuery({ queryKey: ["agents"], queryFn: () => apiClient.listAgents() });
  const runs = useQuery({ queryKey: ["agent-runs"], queryFn: () => apiClient.listAgentRuns() });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => apiClient.listWorkflows() });
  const qqTest = useMutation({ mutationFn: (text: string) => apiClient.qqTestMessage(text) });

  const status = runtime.data?.runtime;
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
        <Metric label="Runs" value={String(runs.data?.runs.length ?? 0)} />
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

        <Panel title="QQ 测试">
          <div className="stack">
            <input value={qqText} onChange={(event) => setQQText(event.target.value)} />
            <button className="btn btn-primary" onClick={() => qqTest.mutate(qqText)} disabled={qqTest.isPending}>
              发送测试消息
            </button>
            <pre className="jsonPreview">{qqTest.data ? JSON.stringify(qqTest.data, null, 2) : "等待测试"}</pre>
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
