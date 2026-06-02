from __future__ import annotations

import argparse
import importlib
import json
import platform
import sys
import time
from pathlib import Path
from typing import Any


REQUIRED_FILES = [
    "config.json",
    "processor_config.json",
    "model.safetensors.index.json",
    "model-00001-of-00002.safetensors",
    "model-00002-of-00002.safetensors",
]

DEPENDENCIES = [
    "torch",
    "transformers",
    "safetensors",
    "PIL",
    "numpy",
    "torchvision",
    "peft",
    "lmdb",
    "decord",
]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="LocateAnything-3B local availability smoke test.")
    parser.add_argument(
        "--model-dir",
        default="data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
        help="Local LocateAnything-3B model directory.",
    )
    parser.add_argument(
        "--data-root",
        default="data_lake/raw/datasets/shanghaitech/original",
        help="Dataset root used to prove the intended ShanghaiTech input exists.",
    )
    parser.add_argument("--output", help="Optional JSON report path.")
    parser.add_argument("--skip-model-load", action="store_true", help="Skip AutoModel.from_pretrained.")
    parser.add_argument("--run-inference", action="store_true", help="Reserved for a future real inference smoke.")
    args = parser.parse_args(argv)

    report = run_smoke(
        model_dir=Path(args.model_dir),
        data_root=Path(args.data_root),
        load_model=not args.skip_model_load,
        run_inference=args.run_inference,
    )
    if args.output:
        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report["status"] in {"ok", "partial"} else 1


def run_smoke(model_dir: Path, data_root: Path, load_model: bool, run_inference: bool) -> dict[str, Any]:
    started = time.time()
    model_dir = model_dir.resolve()
    data_root = data_root.resolve()
    checks: dict[str, Any] = {
        "environment": environment_summary(),
        "dependencies": dependency_summary(),
        "files": file_summary(model_dir),
        "data_root": data_root_summary(data_root),
    }
    errors: list[dict[str, str]] = []
    warnings: list[str] = []

    config = step("config_load", lambda: load_config(model_dir), errors)
    checks["config_load"] = config
    processor = step("processor_load", lambda: load_processor(model_dir), errors)
    checks["processor_load"] = processor
    safetensors = step("safetensors_index", lambda: inspect_safetensors(model_dir), errors)
    checks["safetensors_index"] = safetensors

    model_load: dict[str, Any] = {"status": "skipped", "reason": "skip_model_load=true"}
    if load_model:
        model_load = step("model_load", lambda: load_model_summary(model_dir), errors)
    checks["model_load"] = model_load

    inference = {"status": "skipped", "reason": "real inference is not enabled by default"}
    if run_inference:
        inference = {
            "status": "blocked",
            "reason": "LocateAnything real inference needs a separate image query harness and GPU/runtime budget.",
        }
        warnings.append("real inference was requested but is not implemented in this MVP smoke")
    checks["inference"] = inference

    missing_deps = [name for name, value in checks["dependencies"].items() if not value["available"]]
    if missing_deps:
        warnings.append("missing dependencies: " + ", ".join(missing_deps))
    if checks["environment"].get("cuda_available") is False:
        warnings.append("CUDA is not available; full ShanghaiTech inference is not marked complete")

    status = "ok"
    if errors:
        status = "failed"
    elif inference["status"] != "ok":
        status = "partial"

    return {
        "status": status,
        "model_id": "nvidia/LocateAnything-3B",
        "model_dir": str(model_dir),
        "data_root": str(data_root),
        "checks": checks,
        "warnings": warnings,
        "errors": errors,
        "completed": {
            "download": checks["files"]["required_files_present"],
            "config_load": config.get("status") == "ok",
            "processor_load": processor.get("status") == "ok",
            "safetensors_open": safetensors.get("status") == "ok",
            "model_load": model_load.get("status") == "ok",
            "real_inference": inference.get("status") == "ok",
        },
        "elapsed_seconds": round(time.time() - started, 3),
    }


def environment_summary() -> dict[str, Any]:
    summary: dict[str, Any] = {
        "python": sys.version.split()[0],
        "platform": platform.platform(),
    }
    try:
        import torch

        summary["torch"] = torch.__version__
        summary["cuda_available"] = bool(torch.cuda.is_available())
        summary["cuda_device_count"] = int(torch.cuda.device_count())
        if torch.cuda.is_available():
            summary["cuda_device_name"] = torch.cuda.get_device_name(0)
    except Exception as exc:  # pragma: no cover - defensive diagnostic path
        summary["torch_error"] = str(exc)
    try:
        import transformers

        summary["transformers"] = transformers.__version__
    except Exception as exc:  # pragma: no cover
        summary["transformers_error"] = str(exc)
    return summary


def dependency_summary() -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    for name in DEPENDENCIES:
        try:
            module = importlib.import_module(name)
            out[name] = {"available": True, "version": str(getattr(module, "__version__", ""))}
        except Exception as exc:
            out[name] = {"available": False, "error": str(exc)}
    return out


def file_summary(model_dir: Path) -> dict[str, Any]:
    files = []
    missing = []
    for name in REQUIRED_FILES:
        path = model_dir / name
        if not path.exists():
            missing.append(name)
            continue
        files.append({"path": name, "bytes": path.stat().st_size})
    return {
        "model_dir_exists": model_dir.exists(),
        "required_files_present": len(missing) == 0,
        "missing": missing,
        "required_files": files,
    }


def data_root_summary(data_root: Path) -> dict[str, Any]:
    children: list[str] = []
    if data_root.exists():
        children = sorted(path.name for path in data_root.iterdir())[:20]
    return {
        "exists": data_root.exists(),
        "children": children,
        "expected_shanghaitech_splits": {
            "training": (data_root / "training").exists(),
            "testing": (data_root / "testing").exists(),
            "testframemask": (data_root / "testframemask").exists(),
        },
    }


def step(name: str, fn, errors: list[dict[str, str]]) -> dict[str, Any]:
    started = time.time()
    try:
        value = fn()
        value["status"] = "ok"
        value["elapsed_seconds"] = round(time.time() - started, 3)
        return value
    except Exception as exc:
        errors.append({"step": name, "error": str(exc)})
        return {"status": "failed", "error": str(exc), "elapsed_seconds": round(time.time() - started, 3)}


def load_config(model_dir: Path) -> dict[str, Any]:
    from transformers import AutoConfig

    config = AutoConfig.from_pretrained(str(model_dir), trust_remote_code=True, local_files_only=True)
    text_config = getattr(config, "text_config", None)
    vision_config = getattr(config, "vision_config", None)
    return {
        "class": f"{config.__class__.__module__}.{config.__class__.__name__}",
        "model_type": getattr(config, "model_type", ""),
        "text_model_type": getattr(text_config, "model_type", "") if text_config is not None else "",
        "vision_model_type": getattr(vision_config, "model_type", "") if vision_config is not None else "",
    }


def load_processor(model_dir: Path) -> dict[str, Any]:
    from transformers import AutoProcessor

    processor = AutoProcessor.from_pretrained(str(model_dir), trust_remote_code=True, local_files_only=True)
    return {"class": f"{processor.__class__.__module__}.{processor.__class__.__name__}"}


def inspect_safetensors(model_dir: Path) -> dict[str, Any]:
    from safetensors import safe_open

    shard = model_dir / "model-00001-of-00002.safetensors"
    with safe_open(str(shard), framework="pt", device="cpu") as handle:
        keys = list(handle.keys())
    return {"first_shard": shard.name, "key_count": len(keys), "sample_keys": keys[:5]}


def load_model_summary(model_dir: Path) -> dict[str, Any]:
    import torch
    from transformers import AutoModel

    model = AutoModel.from_pretrained(
        str(model_dir),
        trust_remote_code=True,
        local_files_only=True,
        torch_dtype="auto",
        low_cpu_mem_usage=True,
    )
    try:
        first_param = next(model.parameters())
        parameter_count = sum(param.numel() for param in model.parameters())
        return {
            "class": f"{model.__class__.__module__}.{model.__class__.__name__}",
            "parameter_count": int(parameter_count),
            "device": str(first_param.device),
            "dtype": str(first_param.dtype),
            "cuda_available": bool(torch.cuda.is_available()),
        }
    finally:
        del model


if __name__ == "__main__":
    raise SystemExit(main())
