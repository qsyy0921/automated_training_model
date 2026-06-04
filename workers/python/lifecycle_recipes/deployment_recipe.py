from __future__ import annotations

import argparse
from pathlib import Path

from common import bundle_paths, write_json


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run repo-owned deployment recipe.")
    parser.add_argument("--bundle-dir", required=True)
    args = parser.parse_args(argv)

    bundle_dir = Path(args.bundle_dir).resolve()
    request, plan, generated_dir = bundle_paths(bundle_dir)
    model_id = str(request.get("model_id", "")).strip()
    target = str(request.get("target", "")).strip()
    runtime = str(request.get("runtime", "")).strip() or "python-worker"
    replicas = int(request.get("replicas") or 1)

    release_path = generated_dir / "deployment_release.json"
    manifest_path = generated_dir / "serving_manifest.json"

    write_json(
        release_path,
        {
            "model_id": model_id,
            "target": target,
            "runtime": runtime,
            "replicas": replicas,
            "plan_summary": plan.get("summary", ""),
            "status": "ready-for-rollout",
        },
    )
    write_json(
        manifest_path,
        {
            "service_name": f"{model_id}-{target}",
            "runtime": runtime,
            "replicas": replicas,
            "route": f"/models/{model_id}",
        },
    )

    print(f"deployment release={release_path}")
    print(f"deployment manifest={manifest_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
