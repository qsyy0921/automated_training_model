import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@shared/api/client";
import { Button } from "@shared/ui/Button";
import { Panel } from "@shared/ui/Panel";
import type { WorkflowSpec } from "@entities/agent/model";

interface Props {
  visible: boolean;
  currentScene: string;
}

export function AgentControlPanel({ visible, currentScene }: Props) {
  const queryClient = useQueryClient();
  const [workflowID, setWorkflowID] = useState("");
  const [datasetID, setDatasetID] = useState("shanghaitech-original");
  const [dryRun, setDryRun] = useState(true);

  const agents = useQuery({ queryKey: ["agents"], queryFn: () => apiClient.listAgents(), enabled: visible });
  const tools = useQuery({ queryKey: ["agent-tools"], queryFn: () => apiClient.listAgentTools(), enabled: visible });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => apiClient.listWorkflows(), enabled: visible });
  const runs = useQuery({ queryKey: ["agent-runs"], queryFn: () => apiClient.listAgentRuns(), enabled: visible, refetchInterval: visible ? 3000 : false });
  const audit = useQuery({ queryKey: ["audit-events"], queryFn: () => apiClient.listAuditEvents(24), enabled: visible, refetchInterval: visible ? 5000 : false });

  const workflowList = workflows.data?.workflows || [];
  const selectedWorkflow = useMemo(() => workflowList.find((item) => item.id === workflowID) || workflowList[0], [workflowID, workflowList]);

  useEffect(() => {
    if (!workflowID && workflowList[0]) setWorkflowID(workflowList[0].id);
  }, [workflowID, workflowList]);

  const submitRun = useMutation({
    mutationFn: (workflow: WorkflowSpec) =>
      apiClient.submitAgentRun({
        workflow_id: workflow.id,
        dataset_id: datasetID,
        scene: currentScene,
        dry_run: dryRun,
        params: { source: "web-agent-panel" }
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agent-runs"] });
      queryClient.invalidateQueries({ queryKey: ["audit-events"] });
    }
  });

  if (!visible) return null;

  return (
    <section className="agentPanel">
      <Panel title="Agent 控制面">
        <div className="metricStrip">
          <span>
            <b>{agents.data?.agents.length || 0}</b>
            <small>智能体</small>
          </span>
          <span>
            <b>{tools.data?.tools.length || 0}</b>
            <small>工具</small>
          </span>
          <span>
            <b>{workflowList.length}</b>
            <small>工作流</small>
          </span>
          <span>
            <b>{runs.data?.runs.length || 0}</b>
            <small>运行记录</small>
          </span>
        </div>
        <div className="formGrid two">
          <label>
            <span>工作流</span>
            <select value={selectedWorkflow?.id || ""} onChange={(event) => setWorkflowID(event.target.value)}>
              {workflowList.map((workflow) => (
                <option key={workflow.id} value={workflow.id}>
                  {workflow.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>数据集 ID</span>
            <input value={datasetID} onChange={(event) => setDatasetID(event.target.value)} />
          </label>
        </div>
        <div className="agentRunBar">
          <label className="inlineCheck">
            <input type="checkbox" checked={dryRun} onChange={(event) => setDryRun(event.target.checked)} />
            <span>只做调度预演</span>
          </label>
          <Button variant="primary" onClick={() => selectedWorkflow && submitRun.mutate(selectedWorkflow)} disabled={!selectedWorkflow || submitRun.isPending}>
            提交工作流
          </Button>
        </div>
      </Panel>

      <Panel title="智能体与工具">
        <div className="agentGrid">
          {(agents.data?.agents || []).map((agent) => (
            <article key={agent.id} className="agentCard">
              <div>
                <b>{agent.name}</b>
                <small>{agent.runtime || agent.kind}</small>
              </div>
              <p>{agent.description}</p>
              <div className="badgeRow compact">
                {(agent.capabilities || []).slice(0, 4).map((capability) => (
                  <span key={capability} className="tagPill">
                    {capability}
                  </span>
                ))}
              </div>
            </article>
          ))}
        </div>
      </Panel>

      <Panel title="工作流详情">
        {selectedWorkflow ? (
          <div className="workflowDetail">
            <div>
              <b>{selectedWorkflow.name}</b>
              <small>{selectedWorkflow.description}</small>
            </div>
            <ol>
              {(selectedWorkflow.steps || []).map((step) => (
                <li key={step.id}>
                  <b>{step.name}</b>
                  <small>{step.agent_id || "human"} · {step.action}</small>
                </li>
              ))}
            </ol>
          </div>
        ) : (
          <p className="empty">暂无工作流。</p>
        )}
      </Panel>

      <Panel title="运行与审计">
        <div className="runList">
          {(runs.data?.runs || []).slice(0, 6).map((run) => (
            <div key={run.id} className="runRow">
              <b>{run.workflow_id}</b>
              <small>{run.status} · {run.task_id}</small>
            </div>
          ))}
        </div>
        <div className="auditList">
          {(audit.data?.events || []).slice(0, 6).map((event) => (
            <div key={event.id} className="auditRow">
              <b>{event.action}</b>
              <small>{event.resource_type}:{event.resource_id}</small>
            </div>
          ))}
        </div>
      </Panel>
    </section>
  );
}
