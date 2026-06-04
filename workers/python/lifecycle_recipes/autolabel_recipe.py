from __future__ import annotations

import argparse
from pathlib import Path

from common import bundle_paths, write_json


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run repo-owned autolabel recipe.")
    parser.add_argument("--bundle-dir", required=True)
    args = parser.parse_args(argv)

    bundle_dir = Path(args.bundle_dir).resolve()
    request, plan, generated_dir = bundle_paths(bundle_dir)
    dataset_id = str(request.get("dataset_id", "")).strip()

    queue_path = generated_dir / "review_queue.json"
    summary_path = generated_dir / "autolabel_summary.json"

    write_json(
        queue_path,
        {
            "dataset_id": dataset_id,
            "task_types": request.get("task_types", []),
            "review_items": [],
        },
    )
    write_json(
        summary_path,
        {
            "dataset_id": dataset_id,
            "plan_summary": plan.get("summary", ""),
            "status": "queued-for-review",
        },
    )

    print(f"autolabel queue={queue_path}")
    print(f"autolabel summary={summary_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
