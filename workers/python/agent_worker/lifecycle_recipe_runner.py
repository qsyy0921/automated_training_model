from __future__ import annotations

import argparse
import json
import sys
import time
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
    spec = build_recipe_spec(args.action, args.recipe, bundle_dir, request, plan)
    spec_path = bundle_dir / "recipe_spec.json"
    write_json(spec_path, spec)
    print(f"recipe spec={spec_path}")

    stage_results: list[dict[str, Any]] = []
    generated_files: list[str] = [str(spec_path)]
    for index, stage in enumerate(spec["stages"], start=1):
        stage_id = str(stage["id"])
        stage_title = str(stage["title"])
        print(f"recipe stage[{index}] id={stage_id} title={stage_title}")
        result = execute_stage(args.action, args.recipe, bundle_dir, request, plan, stage)
        stage_results.append(result)
        generated_files.extend(result.get("generated_files", []))
        print(f"recipe stage[{index}] done status={result['status']} duration_ms={result['duration_ms']}")

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
        "recipe_spec_path": str(spec_path),
        "stage_count": len(stage_results),
        "stage_results": stage_results,
        "generated_files": unique_preserve_order(generated_files),
    }
    write_json(report_path, report)
    print(f"recipe report={report_path}")
    print("recipe completed")
    return 0


def build_recipe_spec(action: str, recipe: str, bundle_dir: Path, request: dict[str, Any], plan: dict[str, Any]) -> dict[str, Any]:
    action_dir = bundle_dir / "generated"
    action_dir.mkdir(parents=True, exist_ok=True)
    if action == "training.run":
        dataset_id = str(request.get("dataset_id", "")).strip()
        target_task = str(request.get("target_task", "")).strip()
        model_family = str(request.get("model_family", "")).strip()
        return {
            "action": action,
            "recipe": recipe,
            "bundle_dir": str(bundle_dir),
            "summary": f"repo-owned training recipe for dataset={dataset_id} model={model_family}",
            "stages": [
                {
                    "id": "dataset-freeze",
                    "title": "freeze dataset and annotation version",
                    "outputs": [str(action_dir / "dataset_freeze.json")],
                    "payload": {
                        "dataset_id": dataset_id,
                        "annotation_version": str(request.get("annotation_version", "")).strip() or "latest",
                        "split_config": str(request.get("split_config", "")).strip() or "default",
                    },
                },
                {
                    "id": "trainer-config",
                    "title": "render trainer config and output registry",
                    "outputs": [str(action_dir / "trainer_config.json")],
                    "payload": {
                        "target_task": target_task,
                        "model_family": model_family,
                        "training_config": string_map(request.get("training_config")),
                        "output_registry": str(request.get("output_registry", "")).strip() or "data_lake/models",
                    },
                },
                {
                    "id": "launch-plan",
                    "title": "render repo-owned training launch command",
                    "outputs": [str(action_dir / "launch_command.json"), str(action_dir / "artifact_manifest.json")],
                    "payload": {
                        "command": [
                            "python",
                            "-m",
                            "atm.training.recipe",
                            "--dataset-id",
                            dataset_id,
                            "--target-task",
                            target_task,
                            "--model-family",
                            model_family,
                        ],
                        "artifacts": ["checkpoint.stub", "metrics.stub", "train.log"],
                    },
                },
            ],
        }
    if action == "evaluation.run":
        dataset_id = str(request.get("dataset_id", "")).strip()
        model_id = str(request.get("model_id", "")).strip()
        split_name = str(request.get("split", "")).strip() or "validation"
        metrics = string_list(request.get("metrics")) or ["mAP", "recall"]
        return {
            "action": action,
            "recipe": recipe,
            "bundle_dir": str(bundle_dir),
            "summary": f"repo-owned evaluation recipe for model={model_id} split={split_name}",
            "stages": [
                {
                    "id": "model-resolve",
                    "title": "resolve model artifact and checkpoint set",
                    "outputs": [str(action_dir / "model_resolution.json")],
                    "payload": {"model_id": model_id, "dataset_id": dataset_id},
                },
                {
                    "id": "eval-config",
                    "title": "normalize metrics and failure mining settings",
                    "outputs": [str(action_dir / "evaluation_config.json")],
                    "payload": {
                        "split": split_name,
                        "metrics": metrics,
                        "save_visuals": bool(request.get("save_visuals", False)),
                        "failure_mining": bool(request.get("failure_mining", False)),
                    },
                },
                {
                    "id": "report-plan",
                    "title": "render repo-owned evaluation launch command",
                    "outputs": [str(action_dir / "launch_command.json"), str(action_dir / "report_stub.json")],
                    "payload": {
                        "command": [
                            "python",
                            "-m",
                            "atm.evaluation.recipe",
                            "--dataset-id",
                            dataset_id,
                            "--model-id",
                            model_id,
                            "--split",
                            split_name,
                        ],
                        "metrics": metrics,
                    },
                },
            ],
        }
    if action == "deployment.run":
        model_id = str(request.get("model_id", "")).strip()
        target = str(request.get("target", "")).strip()
        runtime = str(request.get("runtime", "")).strip() or "python-worker"
        replicas = int(request.get("replicas") or 1)
        return {
            "action": action,
            "recipe": recipe,
            "bundle_dir": str(bundle_dir),
            "summary": f"repo-owned deployment recipe for model={model_id} target={target}",
            "stages": [
                {
                    "id": "release-check",
                    "title": "validate model metadata and deployment target",
                    "outputs": [str(action_dir / "release_check.json")],
                    "payload": {
                        "model_id": model_id,
                        "model_version": str(request.get("model_version", "")).strip() or "latest",
                        "target": target,
                    },
                },
                {
                    "id": "runtime-spec",
                    "title": "normalize runtime and rollout settings",
                    "outputs": [str(action_dir / "runtime_spec.json")],
                    "payload": {
                        "runtime": runtime,
                        "replicas": replicas,
                        "strategy": str(request.get("strategy", "")).strip() or "rolling",
                        "rollback_policy": str(request.get("rollback_policy", "")).strip() or "manual",
                    },
                },
                {
                    "id": "rollout-plan",
                    "title": "render repo-owned deployment launch command",
                    "outputs": [str(action_dir / "launch_command.json"), str(action_dir / "serving_manifest.json")],
                    "payload": {
                        "command": [
                            "python",
                            "-m",
                            "atm.deployment.recipe",
                            "--model-id",
                            model_id,
                            "--target",
                            target,
                            "--runtime",
                            runtime,
                            "--replicas",
                            str(replicas),
                        ],
                    },
                },
            ],
        }
    if action == "autolabel.run":
        dataset_id = str(request.get("dataset_id", "")).strip()
        task_types = string_list(request.get("task_types"))
        return {
            "action": action,
            "recipe": recipe,
            "bundle_dir": str(bundle_dir),
            "summary": f"repo-owned autolabel recipe for dataset={dataset_id}",
            "stages": [
                {
                    "id": "input-scan",
                    "title": "resolve target videos and task types",
                    "outputs": [str(action_dir / "input_scan.json")],
                    "payload": {"dataset_id": dataset_id, "task_types": task_types},
                },
                {
                    "id": "model-route",
                    "title": "select worker model profile and prompt package",
                    "outputs": [str(action_dir / "model_route.json")],
                    "payload": {"model_profile": str(request.get("model_profile", "")).strip() or "default"},
                },
                {
                    "id": "review-plan",
                    "title": "render repo-owned autolabel launch command",
                    "outputs": [str(action_dir / "launch_command.json"), str(action_dir / "review_queue_stub.json")],
                    "payload": {
                        "command": [
                            "python",
                            "-m",
                            "atm.autolabel.recipe",
                            "--dataset-id",
                            dataset_id,
                        ],
                    },
                },
            ],
        }
    return {
        "action": action,
        "recipe": recipe,
        "bundle_dir": str(bundle_dir),
        "summary": f"generic repo-owned recipe for action={action}",
        "stages": [
            {
                "id": "prepare",
                "title": "prepare generic request context",
                "outputs": [str(action_dir / "prepare.json")],
                "payload": summarize_request(action, request),
            },
            {
                "id": "execute",
                "title": "render generic launch command",
                "outputs": [str(action_dir / "launch_command.json")],
                "payload": {"command": ["python", "-m", "atm.generic.recipe", "--action", action]},
            },
        ],
    }


def execute_stage(action: str, recipe: str, bundle_dir: Path, request: dict[str, Any], plan: dict[str, Any], stage: dict[str, Any]) -> dict[str, Any]:
    started = time.perf_counter()
    at = now_iso()
    generated_files: list[str] = []
    stage_payload = {
        "action": action,
        "recipe": recipe,
        "stage_id": stage["id"],
        "stage_title": stage["title"],
        "generated_at": at,
        "bundle_dir": str(bundle_dir),
        "request_summary": summarize_request(action, request),
        "plan_summary": plan.get("summary", ""),
        "payload": stage.get("payload", {}),
    }
    for output in stage.get("outputs", []):
        output_path = Path(output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        write_json(output_path, stage_payload)
        generated_files.append(str(output_path))
        print(f"recipe output={output_path}")
    duration_ms = int((time.perf_counter() - started) * 1000)
    return {
        "id": stage["id"],
        "title": stage["title"],
        "status": "completed",
        "started_at": at,
        "finished_at": now_iso(),
        "duration_ms": duration_ms,
        "generated_files": generated_files,
    }


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")


def summarize_request(action: str, request: dict[str, Any]) -> dict[str, Any]:
    summary: dict[str, Any] = {"action": action}
    for key in (
        "dataset_id",
        "model_id",
        "target_task",
        "model_family",
        "target",
        "runtime",
        "split",
        "model_version",
    ):
        value = request.get(key)
        if value not in (None, "", []):
            summary[key] = value
    return summary


def string_map(value: Any) -> dict[str, str]:
    if not isinstance(value, dict):
        return {}
    return {str(k): str(v) for k, v in value.items() if str(k).strip() and str(v).strip()}


def string_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item).strip() for item in value if str(item).strip()]


def unique_preserve_order(items: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for item in items:
        if item and item not in seen:
            seen.add(item)
            out.append(item)
    return out


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
