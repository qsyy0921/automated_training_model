from __future__ import annotations

import argparse
from pathlib import Path

from common import bundle_paths, write_json


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run repo-owned training recipe.")
    parser.add_argument("--bundle-dir", required=True)
    args = parser.parse_args(argv)

    bundle_dir = Path(args.bundle_dir).resolve()
    request, plan, generated_dir = bundle_paths(bundle_dir)
    dataset_id = str(request.get("dataset_id", "")).strip()
    target_task = str(request.get("target_task", "")).strip() or "detection"
    model_family = str(request.get("model_family", "")).strip() or "default"

    summary_path = generated_dir / "train_summary.json"
    metrics_path = generated_dir / "train_metrics.json"
    checkpoint_path = generated_dir / "checkpoint.stub.json"
    log_path = generated_dir / "train.log"

    write_json(
        summary_path,
        {
            "dataset_id": dataset_id,
            "target_task": target_task,
            "model_family": model_family,
            "plan_summary": plan.get("summary", ""),
            "execution_mode": "repo-owned-default-recipe",
            "status": "prepared",
        },
    )
    write_json(
        metrics_path,
        {
            "loss": 0.42,
            "map50": 0.61,
            "map50_95": 0.37,
            "dataset_id": dataset_id,
            "model_family": model_family,
        },
    )
    write_json(
        checkpoint_path,
        {
            "checkpoint_id": f"{dataset_id}-{model_family}-stub",
            "format": "json-stub",
            "note": "placeholder checkpoint metadata for worker MVP",
        },
    )
    log_path.write_text(
        "\n".join(
            [
                f"dataset_id={dataset_id}",
                f"target_task={target_task}",
                f"model_family={model_family}",
                "status=prepared",
            ]
        )
        + "\n",
        encoding="utf-8",
    )

    print(f"training summary={summary_path}")
    print(f"training metrics={metrics_path}")
    print(f"training checkpoint={checkpoint_path}")
    print(f"training log={log_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
