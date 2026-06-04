from __future__ import annotations

import json
from pathlib import Path
from typing import Any


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")


def bundle_paths(bundle_dir: Path) -> tuple[dict[str, Any], dict[str, Any], Path]:
    request = read_json(bundle_dir / "request.json")
    plan = read_json(bundle_dir / "plan.json")
    generated_dir = bundle_dir / "generated"
    generated_dir.mkdir(parents=True, exist_ok=True)
    return request, plan, generated_dir

