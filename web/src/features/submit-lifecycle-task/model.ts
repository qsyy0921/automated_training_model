export type LifecycleTaskKind = "autolabel" | "training" | "evaluation" | "deployment";

export const lifecycleTaskEndpoint: Record<LifecycleTaskKind, string> = {
  autolabel: "/api/autolabel/jobs",
  training: "/api/training/runs",
  evaluation: "/api/evaluation/runs",
  deployment: "/api/deployments"
};

