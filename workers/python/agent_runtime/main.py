from __future__ import annotations

import argparse
import json
import sys

from agent_runtime.contracts import RuntimeRequest, RuntimeResult
from agent_runtime.intent import classify_intent


def run_runtime(request: RuntimeRequest) -> RuntimeResult:
    intent = classify_intent(request)
    if intent.kind == "health_check":
        return RuntimeResult(status="ok", intent=intent, reply_text="pong")
    if intent.kind == "identify_actor":
        reply = (
            f"channel={request.channel} account={request.account_id} "
            f"peer={request.peer_kind}:{request.peer_id} sender={request.sender_id}"
        )
        return RuntimeResult(status="ok", intent=intent, reply_text=reply)
    if intent.kind == "data_intake":
        return RuntimeResult(
            status="planned",
            intent=intent,
            reply_text=f"已收到 {len(request.attachments)} 个附件，将先进入隔离区并生成 Data Intake Plan。",
            plan=[
                {"kind": "intake.quarantine", "params": {"count": str(len(request.attachments))}},
                {"kind": "intake.plan", "params": {"skill_id": intent.skill_id}},
            ],
        )
    if intent.kind == "submit_dry_run":
        return RuntimeResult(
            status="planned",
            intent=intent,
            reply_text=f"准备提交 dry-run workflow，dataset={intent.dataset_id}",
            plan=[
                {
                    "kind": "workflow.submit_run",
                    "params": {
                        "workflow_id": "data-to-deployment-lifecycle",
                        "dataset_id": intent.dataset_id,
                        "dry_run": "true",
                    },
                }
            ],
        )
    return RuntimeResult(
        status="planned",
        intent=intent,
        reply_text="已进入 Python Agent Runtime；下一步会接入 LLM planner 和 tool executor。",
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Automated Training Model Python Agent Runtime")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--request-json", help="JSON-encoded runtime request")
    source.add_argument("--request-file", help="Path to a JSON runtime request file")
    args = parser.parse_args(argv)

    try:
        raw = args.request_json
        if args.request_file:
            with open(args.request_file, "r", encoding="utf-8-sig") as handle:
                raw = handle.read()
        payload = json.loads(raw or "{}")
        request = RuntimeRequest.from_dict(payload)
        result = run_runtime(request)
    except Exception as exc:
        result = RuntimeResult(status="failed", intent=classify_intent(RuntimeRequest.from_dict({})), reply_text=str(exc))
    print(json.dumps(result.to_dict(), ensure_ascii=False))
    return 0 if result.status != "failed" else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

