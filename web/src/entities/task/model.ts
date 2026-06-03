export type TaskKind = "autolabel.job" | "training.run" | "evaluation.run" | "model.register" | "deployment.run";

export interface TaskRecord {
  id: string;
  type: TaskKind | string;
  status: string;
  message?: string;
  progress?: number;
  progress_percent?: number;
  retryable?: boolean;
  attempt?: number;
  max_attempts?: number;
  worker_heartbeat?: {
    at: string;
    status: string;
    message?: string;
  };
  artifacts?: Array<{
    name: string;
    uri: string;
    kind?: string;
    metadata?: Record<string, string>;
  }>;
  stdout?: string;
  stderr?: string;
  logs?: Array<{
    at: string;
    level: string;
    message: string;
  }> | null;
  metadata?: Record<string, string>;
  created_at?: string;
  started_at?: string;
  finished_at?: string;
  updated_at?: string;
}
