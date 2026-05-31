export type TaskKind = "autolabel.job" | "training.run" | "evaluation.run" | "model.register" | "deployment.run";

export interface TaskRecord {
  id: string;
  type: TaskKind | string;
  status: string;
  message?: string;
  progress?: number;
  created_at?: string;
  updated_at?: string;
}

