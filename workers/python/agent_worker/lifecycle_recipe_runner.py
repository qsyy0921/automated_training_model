from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run a repo-owned lifecycle execution recipe.")
    parser.add_argument("--action", required=True, help="Lifecycle action, for example training.run")
    parser.add_argument("--recipe", default="default", help="Recipe id")
    parser.add_argument("--bundle-dir", required=True, help="Bundle directory containing request.json and plan.json")
    args = parser.parse_args(argv)

    bundle_dir = Path(args.bundle_dir).resolve()
    request = read_json(bundle_dir / "request.json")
    plan = read_json(bundle_dir / "plan.json")
    started_at = now_iso()

    print(f"recipe start action={args.action} recipe={args.recipe}")
    for stage in recipe_stages(args.action):
        print(f"recipe stage={stage}")

    report_path = bundle_dir / "recipe_report.json"
    report = {
        "action": args.action,
        "recipe": args.recipe,
        "bundle_dir": str(bundle_dir),
        "started_at": started_at,
        "finished_at": now_iso(),
        "request_summary": summarize_request(args.action, request),
        "plan_step_count": len(plan.get("steps", [])) if isinstance(plan, dict) else 0,
        "status": "completed",
    }
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"recipe report={report_path}")
    print("recipe completed")
    return 0


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def recipe_stages(action: str) -> list[str]:
    if action == "training.run":
        return ["dataset-freeze", "trainer-config", "launch-train"]
    if action == "evaluation.run":
        return ["model-resolve", "eval-config", "launch-eval"]
    if action == "deployment.run":
        return ["release-check", "runtime-spec", "launch-rollout"]
    if action == "autolabel.run":
        return ["sample-batch", "adapter-config", "launch-autolabel"]
    return ["prepare", "execute", "finalize"]


def summarize_request(action: str, request: dict[str, Any]) -> dict[str, Any]:
    summary: dict[str, Any] = {"action": action}
    for key in ("dataset_id", "model_id", "target_task", "model_family", "target", "runtime"):
        value = request.get(key)
        if value not in (None, "", []):
            summary[key] = value
    return summary


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
