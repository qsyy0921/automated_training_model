from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from typing import Any


def main() -> int:
    parser = argparse.ArgumentParser(description="Download or verify a HuggingFace model snapshot into data_lake.")
    parser.add_argument("--repo-id", required=True, help="HuggingFace repo id, for example nvidia/LocateAnything-3B")
    parser.add_argument("--local-dir", required=True, help="Destination directory under data_lake/models/artifacts/huggingface")
    parser.add_argument("--manifest", required=True, help="Small JSON manifest path to write")
    parser.add_argument("--revision", default=None, help="Optional HuggingFace revision")
    parser.add_argument("--dry-run", action="store_true", help="Validate destination and write a dry-run manifest without downloading")
    parser.add_argument("--verify-only", action="store_true", help="Only inspect an existing local directory")
    args = parser.parse_args()

    local_dir = Path(args.local_dir).resolve()
    manifest = Path(args.manifest).resolve()
    remote = remote_summary(args.repo_id, args.revision)
    if args.dry_run:
        summary = dry_run_summary(args.repo_id, local_dir, args.revision, remote)
        manifest.parent.mkdir(parents=True, exist_ok=True)
        manifest.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return 0
    if not args.verify_only:
        snapshot_download(args.repo_id, local_dir, args.revision)
    summary = summarize(args.repo_id, local_dir, args.revision, remote)
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if args.verify_only and summary.get("missing_files"):
        raise SystemExit(f"verification failed: {len(summary['missing_files'])} remote files are missing or incomplete")
    return 0


def snapshot_download(repo_id: str, local_dir: Path, revision: str | None) -> None:
    try:
        from huggingface_hub import snapshot_download as hf_snapshot_download
    except ImportError as exc:
        raise SystemExit("huggingface_hub is required. Install with: python -m pip install huggingface_hub") from exc

    token = os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_HUB_TOKEN")
    local_dir.mkdir(parents=True, exist_ok=True)
    hf_snapshot_download(
        repo_id=repo_id,
        revision=revision,
        local_dir=str(local_dir),
        local_dir_use_symlinks=False,
        token=token,
        resume_download=True,
    )


def remote_summary(repo_id: str, revision: str | None) -> dict[str, Any]:
    try:
        from huggingface_hub import HfApi
    except ImportError as exc:
        raise SystemExit("huggingface_hub is required. Install with: python -m pip install huggingface_hub") from exc

    token = os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_HUB_TOKEN")
    api = HfApi(token=token)
    info = api.model_info(repo_id=repo_id, revision=revision, files_metadata=True)
    files: list[dict[str, Any]] = []
    for sibling in info.siblings:
        path = getattr(sibling, "rfilename", None)
        if not path:
            continue
        size = getattr(sibling, "size", None)
        files.append({"path": path, "bytes": int(size or 0)})
    largest = sorted(files, key=lambda item: item["bytes"], reverse=True)[:20]
    return {
        "repo_id": repo_id,
        "revision": revision or getattr(info, "sha", None) or "default",
        "sha": getattr(info, "sha", None),
        "file_count": len(files),
        "total_bytes": sum(item["bytes"] for item in files),
        "largest_files": largest,
        "files": files,
    }


def summarize(repo_id: str, local_dir: Path, revision: str | None, remote: dict[str, Any]) -> dict[str, Any]:
    if not local_dir.exists():
        raise SystemExit(f"local_dir does not exist: {local_dir}")
    files = [path for path in local_dir.rglob("*") if path.is_file()]
    total_bytes = sum(path.stat().st_size for path in files)
    largest = sorted(files, key=lambda path: path.stat().st_size, reverse=True)[:20]
    missing = compare_remote_files(local_dir, remote)
    return {
        "repo_id": repo_id,
        "revision": revision or "default",
        "local_dir": str(local_dir),
        "file_count": len(files),
        "total_bytes": total_bytes,
        "remote_file_count": remote["file_count"],
        "remote_total_bytes": remote["total_bytes"],
        "complete": len(missing) == 0,
        "missing_files": missing,
        "largest_files": [
            {
                "path": str(path.relative_to(local_dir)),
                "bytes": path.stat().st_size,
            }
            for path in largest
        ],
    }


def compare_remote_files(local_dir: Path, remote: dict[str, Any]) -> list[dict[str, Any]]:
    missing: list[dict[str, Any]] = []
    for item in remote.get("files", []):
        rel = item["path"]
        expected_size = int(item.get("bytes") or 0)
        path = local_dir / rel
        if not path.exists():
            missing.append({"path": rel, "expected_bytes": expected_size, "reason": "missing"})
            continue
        actual_size = path.stat().st_size
        if expected_size > 0 and actual_size != expected_size:
            missing.append({"path": rel, "expected_bytes": expected_size, "actual_bytes": actual_size, "reason": "size_mismatch"})
    return missing


def dry_run_summary(repo_id: str, local_dir: Path, revision: str | None, remote: dict[str, Any]) -> dict[str, Any]:
    return {
        "repo_id": repo_id,
        "revision": revision or "default",
        "local_dir": str(local_dir),
        "dry_run": True,
        "will_download": True,
        "remote_file_count": remote["file_count"],
        "remote_total_bytes": remote["total_bytes"],
        "remote_largest_files": remote["largest_files"],
    }


if __name__ == "__main__":
    raise SystemExit(main())
