import { z } from "zod";
import type { DatasetRecord } from "@entities/dataset/model";
import type {
  AgentRun,
  AgentSpec,
  AgentToolSpec,
  ArtifactManifestPayload,
  AuditEvent,
  ChannelStatus,
  ControlSurface,
  DataGovernancePolicy,
  EnforcementPoint,
  ReleasePolicy,
  GatewayAuthStatus,
  IntakeWorkflow,
  RuntimeSession,
  RuntimeSnapshot,
  RuntimeStatus,
  RuntimeTrace,
  RuntimeModelJob,
  RuntimeModelJobLogs,
  RuntimePolicy,
  WorkflowSpec
} from "@entities/agent/model";
import type { TaskRecord } from "@entities/task/model";
import type { Taxonomy } from "@entities/taxonomy/model";
import type { Box } from "@entities/track/model";
import type { VideoMeta, VideoSummary, AnnotationRecord } from "@entities/video/model";

const errorSchema = z.object({ error: z.string().optional() }).passthrough();

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    try {
      const parsed = errorSchema.parse(JSON.parse(text));
      throw new Error(parsed.error || text || `${res.status} ${res.statusText}`);
    } catch {
      throw new Error(text || `${res.status} ${res.statusText}`);
    }
  }
  const contentType = res.headers.get("content-type") || "";
  return contentType.includes("application/json") ? ((await res.json()) as T) : ((await res.text()) as T);
}

export const apiClient = {
  listVideos: async () => request<{ videos: VideoSummary[] }>("/api/videos"),
  taxonomy: async () => request<Taxonomy>("/api/taxonomy"),
  videoMeta: async (scene: string) => request<VideoMeta>(`/api/video/${scene}/meta`),
  frameBoxes: async (scene: string, frame: number) => request<{ scene: string; frame: number; boxes: Box[] }>(`/api/video/${scene}/boxes?frame=${frame}`),
  saveAnnotation: async (scene: string, payload: Partial<AnnotationRecord>) =>
    request<{ annotation: AnnotationRecord }>(`/api/video/${scene}/annotations`, { method: "POST", body: JSON.stringify(payload) }),
  deleteAnnotation: async (scene: string, id: string) =>
    request<{ deleted: boolean }>(`/api/video/${scene}/annotation/${id}`, { method: "DELETE" }),
  purgeTracks: async (scene: string, trackKeys: string[]) =>
    request<{ scene: string; track_keys: string[]; removed_rows: number }>(`/api/video/${scene}/purge-tracks`, {
      method: "POST",
      body: JSON.stringify({ track_keys: trackKeys })
    }),
  listDatasets: async () => request<{ datasets: DatasetRecord[] }>("/api/datasets"),
  registerFolderDataset: async (payload: Record<string, unknown>) =>
    request<{ dataset: DatasetRecord }>("/api/datasets/register-folder", { method: "POST", body: JSON.stringify(payload) }),
  registerManifestDataset: async (payload: Record<string, unknown>) =>
    request<{ dataset: DatasetRecord }>("/api/datasets/register-manifest", { method: "POST", body: JSON.stringify(payload) }),
  uploadArchiveDataset: async (formData: FormData) =>
    request<{ dataset: DatasetRecord; extract_root?: string }>("/api/datasets/upload-archive", { method: "POST", body: formData }),
  activateDataset: async (id: string) => request<{ dataset: DatasetRecord; active: boolean }>(`/api/datasets/${id}/activate`, { method: "POST", body: "{}" }),
  submitTask: async (path: string, payload: Record<string, unknown>) => {
    const result = await request<Record<string, unknown>>(path, { method: "POST", body: JSON.stringify(payload) });
    return { task: normalizeLifecycleTaskResponse(path, result) };
  },
  listTasks: async (limit = 30) => request<{ tasks: TaskRecord[] | null }>(`/api/tasks?limit=${limit}`),
  taskStatus: async (id: string) => {
    const result = await request<{ task: TaskRecord }>(`/api/tasks/${id}`);
    return result.task;
  },
  resumeTask: async (id: string) =>
    request<{ task: TaskRecord }>(`/api/tasks/${id}/resume`, { method: "POST", body: "{}" }),
  taskLogs: async (id: string, limit = 30) =>
    request<TaskRecord>(`/api/tasks/${id}/logs?limit=${limit}`),
  taskManifest: async (id: string) =>
    request<ArtifactManifestPayload>(`/api/tasks/${id}/manifest`),
  taskLineage: async (id: string) =>
    request<{ task_id: string; root_id: string; count: number; lineage: TaskRecord[] }>(`/api/tasks/${id}/lineage`),
  listAgents: async () => request<{ agents: AgentSpec[] }>("/api/agents"),
  listAgentTools: async () => request<{ tools: AgentToolSpec[] }>("/api/tools"),
  listWorkflows: async () => request<{ workflows: WorkflowSpec[] }>("/api/workflows"),
  submitAgentRun: async (payload: Record<string, unknown>) =>
    request<{ run: AgentRun }>("/api/agent-runs", { method: "POST", body: JSON.stringify(payload) }),
  listAgentRuns: async () => request<{ runs: AgentRun[] }>("/api/agent-runs"),
  listAuditEvents: async (limit = 30) => request<{ events: AuditEvent[] }>(`/api/audit-events?limit=${limit}`),
  runtimeStatus: async () => request<{ runtime: RuntimeStatus; snapshot: RuntimeSnapshot; gateway?: { auth?: GatewayAuthStatus } }>("/api/runtime/status"),
  runtimeSessions: async () => request<{ sessions: RuntimeSession[] }>("/api/runtime/sessions"),
  runtimeTraces: async (limit = 30) => request<{ traces: RuntimeTrace[] | null }>(`/api/runtime/traces?limit=${limit}`),
  runtimeModelJobs: async (limit = 30) => request<{ jobs: RuntimeModelJob[] | null }>(`/api/runtime/model-jobs?limit=${limit}`),
  runtimeModelJob: async (id: string) => request<{ job: RuntimeModelJob }>(`/api/runtime/model-jobs/${encodeURIComponent(id)}`),
  runtimeModelJobLogs: async (id: string, limit = 30) =>
    request<RuntimeModelJobLogs>(`/api/runtime/model-jobs/${encodeURIComponent(id)}/logs?limit=${limit}`),
  runtimeModelJobManifest: async (id: string) =>
    request<ArtifactManifestPayload>(`/api/runtime/model-jobs/${encodeURIComponent(id)}/manifest`),
  runtimeModelJobLineage: async (id: string) =>
    request<{ job_id: string; root_id: string; count: number; lineage: RuntimeModelJob[] }>(`/api/runtime/model-jobs/${encodeURIComponent(id)}/lineage`),
  cancelRuntimeModelJob: async (id: string) =>
    request<{ job: RuntimeModelJob }>(`/api/runtime/model-jobs/${encodeURIComponent(id)}/cancel`, { method: "POST" }),
  resumeRuntimeModelJob: async (id: string) =>
    request<{ job: RuntimeModelJob }>(`/api/runtime/model-jobs/${encodeURIComponent(id)}/resume`, { method: "POST" }),
  runtimeIntakeWorkflows: async (limit = 30) =>
    request<{ workflows: IntakeWorkflow[] | null }>(`/api/runtime/intake/workflows?limit=${limit}`),
  runtimeIntakeWorkflow: async (id: string) =>
    request<{ workflow: IntakeWorkflow }>(`/api/runtime/intake/workflows/${encodeURIComponent(id)}`),
  runtimeSend: async (text: string) =>
    request<Record<string, unknown>>("/api/channels/qq/test-message", {
      method: "POST",
      body: JSON.stringify({
        id: `web_runtime_${Date.now()}`,
        channel: "qq",
        account_id: "default",
        peer: { channel: "qq", account_id: "default", kind: "direct", id: "web-runtime" },
        sender_id: "web-runtime",
        text
      })
    }),
  desktopStatus: async () => request<{ desktop: Record<string, unknown> }>("/api/desktop/status"),
  listChannels: async () => request<{ channels: ChannelStatus[] }>("/api/channels"),
  qqStatus: async () => request<Record<string, unknown>>("/api/channels/qq/status"),
  qqTestMessage: async (text: string) =>
    request<Record<string, unknown>>("/api/channels/qq/test-message", {
      method: "POST",
      body: JSON.stringify({
        id: `web_${Date.now()}`,
        channel: "qq",
        account_id: "default",
        peer: { channel: "qq", account_id: "default", kind: "direct", id: "web-test" },
        sender_id: "web-test",
        text
      })
    }),
  listEnforcementPoints: async () => request<{ enforcement_points: EnforcementPoint[] }>("/api/governance/enforcement-points"),
  listDataGovernancePolicies: async () => request<{ data_policies: DataGovernancePolicy[] }>("/api/governance/data-policies"),
  listReleasePolicies: async () => request<{ release_policies: ReleasePolicy[] }>("/api/governance/release-policies"),
  listRuntimePolicies: async () => request<{ runtime_policies: RuntimePolicy[] }>("/api/governance/runtime-policies"),
  getControlSurface: async () => request<{ control_surface: ControlSurface }>("/api/governance/control-surface")
};

function normalizeLifecycleTaskResponse(path: string, result: Record<string, unknown>): TaskRecord {
  if (result.task && typeof result.task === "object") {
    return result.task as TaskRecord;
  }
  if (result.run && typeof result.run === "object") {
    const run = result.run as Record<string, unknown>;
    if (typeof run.task_id === "string") {
      return {
        id: run.task_id,
        type: path.includes("/evaluation/") ? "evaluation.run" : "training.run",
        status: String(run.status || "queued")
      };
    }
  }
  if (result.job && typeof result.job === "object") {
    const job = result.job as Record<string, unknown>;
    if (typeof job.task_id === "string") {
      return {
        id: job.task_id,
        type: "autolabel.job",
        status: String(job.status || "queued")
      };
    }
  }
  if (result.deployment && typeof result.deployment === "object") {
    const deployment = result.deployment as Record<string, unknown>;
    if (typeof deployment.task_id === "string") {
      return {
        id: deployment.task_id,
        type: "deployment.run",
        status: String(deployment.status || "queued")
      };
    }
  }
  throw new Error("lifecycle task response missing task id");
}
