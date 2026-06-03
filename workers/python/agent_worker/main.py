from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

from agent_worker.contracts import JobArtifact, JobEnvelope, JobLog, JobResult, WorkerHeartbeat, utc_now_iso


def run_job(job: JobEnvelope) -> JobResult:
    started = utc_now_iso()
    logs = [JobLog(at=started, level="info", message=f"accepted job action={job.action or '-'} tool={job.tool_id or '-'}")]
    if not job.task_id:
        finished = utc_now_iso()
        return JobResult(
            task_id="",
            status="failed",
            message="task_id is required",
            logs=logs + [JobLog(at=finished, level="error", message="task_id is required")],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="invalid job envelope"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )
    if job.action == "download_hf":
        return run_hf_snapshot_job(job, verify_only=False, logs=logs, started=started)
    if job.action == "verify_hf":
        return run_hf_snapshot_job(job, verify_only=True, logs=logs, started=started)
    if job.action == "smoke_locateanything":
        return run_locateanything_smoke_job(job, logs=logs, started=started)
    if job.dry_run:
        finished = utc_now_iso()
        artifact_metadata = {
            "workflow_id": job.workflow_id,
            "agent_id": job.agent_id,
            "tool_id": job.tool_id,
            "dataset_id": job.dataset_id,
        }
        for key, value in (job.params or {}).items():
            if key and value:
                artifact_metadata[str(key)] = str(value)
        return JobResult(
            task_id=job.task_id,
            status="completed",
            message=f"dry-run accepted for {job.agent_id}/{job.action}",
            logs=logs + [JobLog(at=finished, level="info", message="dry-run completed without side effects")],
            artifacts=[
                JobArtifact(
                    name=f"{job.task_id}-dry-run-plan",
                    uri=f"artifact://dry-run/{job.task_id}",
                    kind="dry-run-plan",
                    metadata=artifact_metadata,
                )
            ],
            heartbeat=WorkerHeartbeat(at=finished, status="completed", message="dry-run completed"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )
    finished = utc_now_iso()
    return JobResult(
        task_id=job.task_id,
        status="failed",
        message="real worker execution is not wired yet",
        logs=logs + [JobLog(at=finished, level="error", message="real worker execution is not wired yet")],
        heartbeat=WorkerHeartbeat(at=finished, status="failed", message="real worker execution is not wired"),
        retryable=False,
        started_at=started,
        finished_at=finished,
    )


def run_hf_snapshot_job(job: JobEnvelope, verify_only: bool, logs: list[JobLog], started: str) -> JobResult:
    repo_id = (job.params or {}).get("repo_id", "").strip() or "nvidia/LocateAnything-3B"
    local_dir = (job.params or {}).get("local_dir", "").strip()
    manifest = (job.params or {}).get("manifest", "").strip()
    if not local_dir or not manifest:
        finished = utc_now_iso()
        message = "local_dir and manifest are required for HuggingFace worker jobs"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="invalid HuggingFace worker params"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )

    command = [
        sys.executable,
        str(repo_download_script()),
        "--repo-id",
        repo_id,
        "--local-dir",
        local_dir,
        "--manifest",
        manifest,
    ]
    if verify_only:
        command.append("--verify-only")
    try:
        env = os.environ.copy()
        env.setdefault("HF_HUB_DISABLE_PROGRESS_BARS", "1")
        completed = subprocess.run(
            command,
            cwd=str(repo_root()),
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=hf_worker_timeout_seconds(),
            env=env,
        )
    except subprocess.TimeoutExpired as exc:
        finished = utc_now_iso()
        timeout_message = f"huggingface worker timed out after {hf_worker_timeout_seconds()}s"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=timeout_message,
            logs=logs + [JobLog(at=finished, level="error", message=timeout_message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="huggingface worker timed out"),
            retryable=True,
            started_at=started,
            finished_at=finished,
        )

    finished = utc_now_iso()
    stdout = (completed.stdout or "").strip()
    stderr = (completed.stderr or "").strip()
    action_name = "校验" if verify_only else "下载"
    base_logs = list(logs)
    summary = parse_json_payload(stdout)
    if summary:
        base_logs.append(JobLog(at=finished, level="info", message=summary_log_line(action_name, summary)))
    elif stdout:
        for line in summarize_process_output(stdout):
            base_logs.append(JobLog(at=finished, level="info", message=f"{action_name}输出: {line}"))
    if stderr:
        for line in summarize_process_output(stderr):
            base_logs.append(JobLog(at=finished, level="warn", message=f"{action_name}告警: {line}"))

    if completed.returncode != 0:
        message = first_line(stderr) or first_line(stdout) or f"huggingface {action_name} failed"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=base_logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message=f"huggingface {action_name} failed"),
            retryable=True,
            attempt=1,
            max_attempts=3,
            started_at=started,
            finished_at=finished,
        )

    message = f"HuggingFace 模型{action_name}完成：repo={repo_id}"
    if verify_only:
        message += f" complete={summary.get('complete', True)}"
    artifacts = [
        JobArtifact(name=f"{job.task_id}-manifest", uri=manifest, kind="manifest", metadata={"repo_id": repo_id}),
        JobArtifact(name=f"{job.task_id}-model-dir", uri=local_dir, kind="model-dir", metadata={"repo_id": repo_id}),
    ]
    metadata = {
        "repo_id": repo_id,
        "local_dir": local_dir,
        "manifest": manifest,
        "complete": str(summary.get("complete", True)).lower(),
    }
    artifacts[0].metadata = metadata
    return JobResult(
        task_id=job.task_id,
        status="completed",
        message=message,
        logs=base_logs + [JobLog(at=finished, level="info", message=message)],
        artifacts=artifacts,
        heartbeat=WorkerHeartbeat(at=finished, status="completed", message=f"huggingface {action_name} completed"),
        retryable=False,
        attempt=1,
        max_attempts=3,
        started_at=started,
        finished_at=finished,
    )


def run_locateanything_smoke_job(job: JobEnvelope, logs: list[JobLog], started: str) -> JobResult:
    model_dir = (job.params or {}).get("model_dir", "").strip()
    data_root = (job.params or {}).get("data_root", "").strip()
    output = (job.params or {}).get("output", "").strip()
    if not model_dir or not data_root or not output:
        finished = utc_now_iso()
        message = "model_dir, data_root and output are required for LocateAnything smoke worker jobs"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="invalid LocateAnything smoke params"),
            retryable=False,
            started_at=started,
            finished_at=finished,
        )

    command = [
        sys.executable,
        str(repo_locateanything_smoke_script()),
        "--model-dir",
        model_dir,
        "--data-root",
        data_root,
        "--output",
        output,
    ]
    try:
        completed = subprocess.run(
            command,
            cwd=str(repo_root()),
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=hf_worker_timeout_seconds(),
            env=os.environ.copy(),
        )
    except subprocess.TimeoutExpired:
        finished = utc_now_iso()
        timeout_message = f"locateanything smoke worker timed out after {hf_worker_timeout_seconds()}s"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=timeout_message,
            logs=logs + [JobLog(at=finished, level="error", message=timeout_message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="LocateAnything smoke timed out"),
            retryable=True,
            attempt=1,
            max_attempts=3,
            started_at=started,
            finished_at=finished,
        )

    finished = utc_now_iso()
    stdout = (completed.stdout or "").strip()
    stderr = (completed.stderr or "").strip()
    base_logs = list(logs)
    summary = parse_json_payload(stdout)
    if summary:
        base_logs.append(JobLog(at=finished, level="info", message=locateanything_summary_log_line(summary)))
    elif stdout:
        for line in summarize_process_output(stdout):
            base_logs.append(JobLog(at=finished, level="info", message=f"smoke 输出: {line}"))
    if stderr:
        for line in summarize_process_output(stderr):
            base_logs.append(JobLog(at=finished, level="warn", message=f"smoke 告警: {line}"))

    if completed.returncode != 0:
        message = first_line(stderr) or first_line(stdout) or "locateanything smoke failed"
        return JobResult(
            task_id=job.task_id,
            status="failed",
            message=message,
            logs=base_logs + [JobLog(at=finished, level="error", message=message)],
            heartbeat=WorkerHeartbeat(at=finished, status="failed", message="LocateAnything smoke failed"),
            retryable=True,
            attempt=1,
            max_attempts=3,
            started_at=started,
            finished_at=finished,
        )

    message = (
        f"LocateAnything-3B smoke 完成：status={summary.get('status', 'ok')} "
        f"model_load={summary_bool(summary, 'model_load')} real_inference={summary_bool(summary, 'real_inference')}"
    )
    return JobResult(
        task_id=job.task_id,
        status="completed",
        message=message,
        logs=base_logs + [JobLog(at=finished, level="info", message=message)],
        artifacts=[
            JobArtifact(name=f"{job.task_id}-smoke-report", uri=output, kind="smoke-report", metadata={"model_dir": model_dir, "data_root": data_root}),
            JobArtifact(name=f"{job.task_id}-model-dir", uri=model_dir, kind="model-dir", metadata={"model_dir": model_dir}),
        ],
        heartbeat=WorkerHeartbeat(at=finished, status="completed", message="LocateAnything smoke completed"),
        retryable=False,
        attempt=1,
        max_attempts=3,
        started_at=started,
        finished_at=finished,
    )


def health_payload() -> dict[str, object]:
    return {
        "worker": "automated-training-python-worker",
        "status": "ok",
        "heartbeat": WorkerHeartbeat(at=utc_now_iso(), status="ok", message="worker process started").__dict__,
        "capabilities": ["dry-run", "heartbeat", "logs", "artifacts", "retry-contract", "download_hf", "verify_hf", "smoke_locateanything"],
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Automated Training Model Python worker")
    parser.add_argument("--health", action="store_true", help="Print worker heartbeat and exit")
    source = parser.add_mutually_exclusive_group(required=False)
    source.add_argument("--job-json", help="JSON-encoded job envelope")
    source.add_argument("--job-file", help="Path to a JSON job envelope file")
    args = parser.parse_args(argv)

    if args.health:
        print(json.dumps(health_payload(), ensure_ascii=False))
        return 0
    if not args.job_json and not args.job_file:
        parser.error("one of --job-json, --job-file or --health is required")

    try:
        raw = args.job_json
        if args.job_file:
            with open(args.job_file, "r", encoding="utf-8-sig") as handle:
                raw = handle.read()
        payload = json.loads(raw or "{}")
        job = JobEnvelope.from_dict(payload)
        result = run_job(job)
    except Exception as exc:
        now = utc_now_iso()
        result = JobResult(
            task_id="",
            status="failed",
            message=str(exc),
            logs=[JobLog(at=now, level="error", message=str(exc))],
            heartbeat=WorkerHeartbeat(at=now, status="failed", message="job parsing failed"),
            retryable=False,
            started_at=now,
            finished_at=now,
        )
    print(json.dumps(result.to_dict(), ensure_ascii=False))
    return 0 if result.status != "failed" else 1


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def repo_download_script() -> Path:
    return repo_root() / "skills" / "huggingface-model-downloader" / "scripts" / "download_hf_snapshot.py"


def repo_locateanything_smoke_script() -> Path:
    return repo_root() / "workers" / "python" / "agent_worker" / "locateanything_smoke.py"


def hf_worker_timeout_seconds() -> int:
    raw = os.getenv("AGENT_RUNTIME_MODEL_WORKER_TIMEOUT_SECONDS", "").strip()
    if not raw:
        return 3600
    try:
        value = int(raw)
    except ValueError:
        return 3600
    return value if value > 0 else 3600


def summarize_process_output(text: str) -> list[str]:
    lines = []
    for line in text.splitlines():
        line = line.strip()
        if line:
            lines.append(first_line(line))
        if len(lines) >= 3:
            break
    return lines


def parse_json_payload(text: str) -> dict[str, object]:
    text = text.strip()
    if not text:
        return {}
    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end >= start:
        text = text[start : end + 1]
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return {}
    return value if isinstance(value, dict) else {}


def first_line(text: str) -> str:
    text = (text or "").strip()
    if not text:
        return ""
    line = text.splitlines()[0].strip()
    return line[:240] + ("..." if len(line) > 240 else "")


def summary_log_line(action_name: str, summary: dict[str, object]) -> str:
    parts = [f"{action_name}摘要"]
    if summary.get("repo_id"):
        parts.append(f"repo={summary['repo_id']}")
    if summary.get("file_count") is not None:
        parts.append(f"files={summary['file_count']}")
    if summary.get("remote_file_count") is not None:
        parts.append(f"remote_files={summary['remote_file_count']}")
    if summary.get("complete") is not None:
        parts.append(f"complete={summary['complete']}")
    return " ".join(parts)


def locateanything_summary_log_line(summary: dict[str, object]) -> str:
    parts = ["smoke 摘要"]
    if summary.get("status"):
        parts.append(f"status={summary['status']}")
    parts.append(f"model_load={summary_bool(summary, 'model_load')}")
    parts.append(f"real_inference={summary_bool(summary, 'real_inference')}")
    return " ".join(parts)


def summary_bool(summary: dict[str, object], key: str) -> str:
    completed = summary.get("completed")
    if isinstance(completed, dict) and key in completed:
        return str(completed.get(key)).lower()
    return "unknown"


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
