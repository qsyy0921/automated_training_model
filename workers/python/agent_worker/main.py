from __future__ import annotations

import argparse
import json
import sys

from agent_worker.contracts import JobEnvelope, JobResult


def run_job(job: JobEnvelope) -> JobResult:
    if not job.task_id:
        return JobResult(task_id="", status="failed", message="task_id is required")
    if job.dry_run:
        return JobResult(
            task_id=job.task_id,
            status="completed",
            message=f"dry-run accepted for {job.agent_id}/{job.action}",
        )
    return JobResult(
        task_id=job.task_id,
        status="failed",
        message="real worker execution is not wired yet",
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Automated Training Model Python worker")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--job-json", help="JSON-encoded job envelope")
    source.add_argument("--job-file", help="Path to a JSON job envelope file")
    args = parser.parse_args(argv)

    try:
        raw = args.job_json
        if args.job_file:
            with open(args.job_file, "r", encoding="utf-8-sig") as handle:
                raw = handle.read()
        payload = json.loads(raw or "{}")
        job = JobEnvelope.from_dict(payload)
        result = run_job(job)
    except Exception as exc:
        result = JobResult(task_id="", status="failed", message=str(exc))
    print(json.dumps(result.to_dict(), ensure_ascii=False))
    return 0 if result.status != "failed" else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
