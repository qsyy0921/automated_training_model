from __future__ import annotations

import argparse
import json
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
    if job.dry_run:
        finished = utc_now_iso()
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
                    metadata={
                        "workflow_id": job.workflow_id,
                        "agent_id": job.agent_id,
                        "tool_id": job.tool_id,
                        "dataset_id": job.dataset_id,
                    },
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


def health_payload() -> dict[str, object]:
    return {
        "worker": "automated-training-python-worker",
        "status": "ok",
        "heartbeat": WorkerHeartbeat(at=utc_now_iso(), status="ok", message="worker process started").__dict__,
        "capabilities": ["dry-run", "heartbeat", "logs", "artifacts", "retry-contract"],
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


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
