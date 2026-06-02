import { z } from "zod";
import type { DatasetRecord } from "@entities/dataset/model";
import type {
  AgentRun,
  AgentSpec,
  AgentToolSpec,
  AuditEvent,
  ChannelStatus,
  ControlSurface,
  DataGovernancePolicy,
  EnforcementPoint,
  ReleasePolicy,
  GatewayAuthStatus,
  RuntimeSession,
  RuntimeSnapshot,
  RuntimeStatus,
  RuntimeTrace,
  RuntimeModelJob,
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
  submitTask: async (path: string, payload: Record<string, unknown>) =>
    request<{ task: TaskRecord }>(path, { method: "POST", body: JSON.stringify(payload) }),
  taskStatus: async (id: string) => request<TaskRecord>(`/api/tasks/${id}`),
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
