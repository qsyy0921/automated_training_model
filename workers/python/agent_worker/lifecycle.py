from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from agent_worker.contracts import JobArtifact, JobEnvelope, JobLog, JobResult, WorkerHeartbeat, utc_now_iso


def run_lifecycle_job(job: JobEnvelope, logs: list[JobLog], started: str) -> JobResult:
    action = (job.action or "").strip()
    try:
        request = parse_request_json(job)
    except ValueError as exc:
        finished = utc_now_iso()
        message = str(exc)
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="invalid lifecycle request"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )

    try:
        plan = build_lifecycle_plan(action, request)
    except ValueError as exc:
        finished = utc_now_iso()
        message = str(exc)
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="invalid lifecycle recipe"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )
    if job.dry_run:
        return complete_lifecycle_dry_run(job, action, plan, logs, started)
    return complete_lifecycle_execution(job, action, request, plan, logs, started)


def complete_lifecycle_dry_run(job: JobEnvelope, action: str, plan: dict[str, Any], logs: list[JobLog], started: str) -> JobResult:
    finished = utc_now_iso()
    lifecycle_logs = list(logs)
    lifecycle_logs.append(JobLog(at=started, level="info", message=f"normalized request for {action}"))
    for step in plan.get("steps", []):
        lifecycle_logs.append(JobLog(at=started, level="info", message=f"plan step: {step.get('id', '-')}: {step.get('title', '-') }"))
    lifecycle_logs.append(JobLog(at=finished, level="info", message=plan["summary"]))
    return JobResult(
        task_id=job.task_id,
        status="completed",
        message=plan["summary"],
        artifacts=[
            JobArtifact(
                name=f"{job.task_id}-{action}-plan",
                uri=f"artifact://lifecycle/{job.task_id}/{action}.plan.json",
                kind=f"{action}.plan",
                metadata={
                    "action": action,
                    "dataset_id": str(plan.get("dataset_id", "")),
                    "model_id": str(plan.get("model_id", "")),
                    "target": str(plan.get("target", "")),
                    "dry_run": "true",
                    "step_count": str(len(plan.get("steps", []))),
                },
            )
        ],
        logs=lifecycle_logs,
        heartbeat=WorkerHeartbeat(at=finished, status="completed", message=f"{action} dry-run recipe ready"),
        retryable=False,
        attempt=1,
        max_attempts=1,
        started_at=started,
        finished_at=finished,
    )


def complete_lifecycle_execution(
    job: JobEnvelope,
    action: str,
    request: dict[str, Any],
    plan: dict[str, Any],
    logs: list[JobLog],
    started: str,
) -> JobResult:
    bundle_dir = resolve_bundle_dir(job, action)
    bundle_dir.mkdir(parents=True, exist_ok=True)
    request_path = bundle_dir / "request.json"
    plan_path = bundle_dir / "plan.json"
    result_path = bundle_dir / "result.json"

    write_json(request_path, request)
    write_json(plan_path, plan)

    execution = build_execution_record(job, action, request, plan, bundle_dir, started)
    write_json(result_path, execution)

    finished = execution["finished_at"]
    lifecycle_logs = list(logs)
    lifecycle_logs.append(JobLog(at=started, level="info", message=f"normalized request for {action}"))
    lifecycle_logs.append(JobLog(at=started, level="info", message=f"execution bundle root: {bundle_dir}"))
    for step in plan.get("steps", []):
        lifecycle_logs.append(JobLog(at=started, level="info", message=f"execute step: {step.get('id', '-')}: {step.get('title', '-')}"))
    lifecycle_logs.append(JobLog(at=finished, level="info", message=execution["summary"]))

    metadata = lifecycle_artifact_metadata(job, action, plan, "false")
    return JobResult(
        task_id=job.task_id,
        status="completed",
        message=execution["summary"],
        artifacts=[
            JobArtifact(
                name=f"{job.task_id}-{action}-request",
                uri=str(request_path),
                kind=f"{action}.request",
                metadata={**metadata, "role": "request"},
            ),
            JobArtifact(
                name=f"{job.task_id}-{action}-plan",
                uri=str(plan_path),
                kind=f"{action}.plan",
                metadata={**metadata, "role": "plan"},
            ),
            JobArtifact(
                name=f"{job.task_id}-{action}-result",
                uri=str(result_path),
                kind=f"{action}.result",
                metadata={**metadata, "role": "result", "execution_mode": "materialized-recipe"},
            ),
        ],
        logs=lifecycle_logs,
        heartbeat=WorkerHeartbeat(at=finished, status="completed", message=f"{action} execution bundle ready"),
        retryable=False,
        attempt=1,
        max_attempts=1,
        started_at=started,
        finished_at=finished,
    )


def parse_request_json(job: JobEnvelope) -> dict[str, Any]:
    raw = (job.params or {}).get("request_json", "").strip()
    if not raw:
        raise ValueError("request_json is required for lifecycle worker jobs")
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid request_json: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError("request_json must decode to an object")
    return value


def build_lifecycle_plan(action: str, request: dict[str, Any]) -> dict[str, Any]:
    if action == "training.run":
        return build_training_plan(request)
    if action == "evaluation.run":
        return build_evaluation_plan(request)
    if action == "deployment.run":
        return build_deployment_plan(request)
    if action == "autolabel.run":
        return build_autolabel_plan(request)
    raise ValueError(f"unsupported lifecycle action: {action}")


def resolve_bundle_dir(job: JobEnvelope, action: str) -> Path:
    artifact_root = (job.params or {}).get("artifact_root", "").strip() or "data_lake/runtime/lifecycle"
    root = Path(artifact_root)
    if not root.is_absolute():
        root = repo_root() / root
    return root / action / job.task_id


def build_execution_record(
    job: JobEnvelope,
    action: str,
    request: dict[str, Any],
    plan: dict[str, Any],
    bundle_dir: Path,
    started: str,
) -> dict[str, Any]:
    finished = utc_now_iso()
    return {
        "task_id": job.task_id,
        "workflow_id": job.workflow_id,
        "agent_id": job.agent_id,
        "tool_id": job.tool_id,
        "action": action,
        "dry_run": False,
        "execution_mode": "materialized-recipe",
        "bundle_dir": str(bundle_dir),
        "dataset_id": str(plan.get("dataset_id", "")),
        "model_id": str(plan.get("model_id", "")),
        "target": str(plan.get("target", "")),
        "summary": execution_summary(action, plan),
        "started_at": started,
        "finished_at": finished,
        "request": request,
        "plan": plan,
    }


def execution_summary(action: str, plan: dict[str, Any]) -> str:
    if action == "training.run":
        return (
            "training execution bundle materialized: "
            f"dataset={plan.get('dataset_id', '')} target={plan.get('target_task', '')} model={plan.get('model_family', '')}"
        )
    if action == "evaluation.run":
        return (
            "evaluation execution bundle materialized: "
            f"dataset={plan.get('dataset_id', '')} model={plan.get('model_id', '')} split={plan.get('split', '')}"
        )
    if action == "deployment.run":
        return (
            "deployment execution bundle materialized: "
            f"model={plan.get('model_id', '')} target={plan.get('target', '')} runtime={plan.get('runtime', '')}"
        )
    if action == "autolabel.run":
        task_types = ",".join(string_list(plan.get("task_types")))
        return f"autolabel execution bundle materialized: dataset={plan.get('dataset_id', '')} task_types={task_types}"
    return f"{action} execution bundle materialized"


def lifecycle_artifact_metadata(job: JobEnvelope, action: str, plan: dict[str, Any], dry_run: str) -> dict[str, str]:
    return {
        "action": action,
        "dataset_id": str(plan.get("dataset_id", "")),
        "model_id": str(plan.get("model_id", "")),
        "target": str(plan.get("target", "")),
        "dry_run": dry_run,
        "step_count": str(len(plan.get("steps", []))),
        "task_id": job.task_id,
    }


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def build_training_plan(request: dict[str, Any]) -> dict[str, Any]:
    dataset_id = required_string(request, "dataset_id")
    target_task = required_string(request, "target_task")
    model_family = required_string(request, "model_family")
    split_config = optional_string(request, "split_config") or "default"
    output_registry = optional_string(request, "output_registry") or "data_lake/models"
    training_config = string_map(request.get("training_config"))
    return {
        "kind": "training.run",
        "dataset_id": dataset_id,
        "target_task": target_task,
        "model_family": model_family,
        "split_config": split_config,
        "output_registry": output_registry,
        "training_config": training_config,
        "summary": f"training dry-run recipe ready: dataset={dataset_id} target={target_task} model={model_family}",
        "steps": [
            {"id": "dataset-freeze", "title": "freeze dataset and annotation version"},
            {"id": "trainer-config", "title": "render trainer config and output registry"},
            {"id": "resource-check", "title": "validate worker runtime, device and artifact paths"},
            {"id": "train-plan", "title": "emit executable training recipe without side effects"},
        ],
    }


def build_evaluation_plan(request: dict[str, Any]) -> dict[str, Any]:
    dataset_id = required_string(request, "dataset_id")
    model_id = required_string(request, "model_id")
    split_name = optional_string(request, "split") or "validation"
    metrics = string_list(request.get("metrics")) or ["mAP", "recall"]
    return {
        "kind": "evaluation.run",
        "dataset_id": dataset_id,
        "model_id": model_id,
        "split": split_name,
        "metrics": metrics,
        "save_visuals": bool(request.get("save_visuals", False)),
        "failure_mining": bool(request.get("failure_mining", False)),
        "summary": f"evaluation dry-run recipe ready: dataset={dataset_id} model={model_id} split={split_name}",
        "steps": [
            {"id": "model-resolve", "title": "resolve model artifact and checkpoint set"},
            {"id": "eval-config", "title": "normalize metrics, split and failure mining settings"},
            {"id": "output-plan", "title": "reserve evaluation report and optional visuals output"},
        ],
    }


def build_deployment_plan(request: dict[str, Any]) -> dict[str, Any]:
    model_id = required_string(request, "model_id")
    target = required_string(request, "target")
    runtime = optional_string(request, "runtime") or "python-worker"
    strategy = optional_string(request, "strategy") or "dry-run"
    replicas = int(request.get("replicas") or 1)
    if replicas <= 0:
        raise ValueError("replicas must be greater than 0")
    return {
        "kind": "deployment.run",
        "model_id": model_id,
        "model_version": optional_string(request, "model_version"),
        "target": target,
        "runtime": runtime,
        "strategy": strategy,
        "replicas": replicas,
        "summary": f"deployment dry-run recipe ready: model={model_id} target={target} runtime={runtime}",
        "steps": [
            {"id": "release-check", "title": "validate model metadata and deployment target"},
            {"id": "runtime-spec", "title": "normalize runtime, replica and resource settings"},
            {"id": "rollout-plan", "title": "emit rollout/rollback plan without mutating serving infra"},
        ],
    }


def build_autolabel_plan(request: dict[str, Any]) -> dict[str, Any]:
    dataset_id = required_string(request, "dataset_id")
    task_types = string_list(request.get("task_types"))
    if not task_types:
        raise ValueError("task_types is required")
    video_ids = string_list(request.get("video_ids"))
    return {
        "kind": "autolabel.run",
        "dataset_id": dataset_id,
        "task_types": task_types,
        "video_ids": video_ids,
        "model_profile": optional_string(request, "model_profile") or "default",
        "require_review": bool(request.get("require_review", False)),
        "summary": f"autolabel dry-run recipe ready: dataset={dataset_id} task_types={','.join(task_types)}",
        "steps": [
            {"id": "input-scan", "title": "resolve target videos and task types"},
            {"id": "model-route", "title": "select worker model profile and prompt package"},
            {"id": "draft-output", "title": "plan draft annotations and review queue outputs"},
        ],
    }


def required_string(request: dict[str, Any], key: str) -> str:
    value = optional_string(request, key)
    if not value:
        raise ValueError(f"{key} is required")
    return value


def optional_string(request: dict[str, Any], key: str) -> str:
    value = request.get(key, "")
    return str(value).strip() if value is not None else ""


def string_map(value: Any) -> dict[str, str]:
    if not isinstance(value, dict):
        return {}
    return {str(k): str(v) for k, v in value.items() if str(k).strip() and str(v).strip()}


def string_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    out: list[str] = []
    for item in value:
        text = str(item).strip()
        if text:
            out.append(text)
    return out
