from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

from agent_runtime.contracts import RuntimeRequest
from agent_runtime.main import run_runtime
from agent_runtime.smoke_mimo import load_env_file


def main() -> int:
    parser = argparse.ArgumentParser(description="Smoke test Mimo planner tool-call contracts.")
    parser.add_argument("--config", default=r"C:\Users\10495\Desktop\mimo.txt")
    args = parser.parse_args()

    load_env_file(Path(args.config))
    os.environ.setdefault("AGENT_RUNTIME_USE_MIMO", "true")
    os.environ.setdefault("AGENT_RUNTIME_MIMO_FALLBACK", "rule")

    install = _run_case(
        "smoke-install-locateanything",
        "请安装 nvidia/LocateAnything-3B 到 data_lake，不能把模型权重提交到 Git。",
    )
    _assert_contains(install, "model.download_hf")
    _assert_not_contains(install, "workflow.submit_run")

    shanghaitech_root = r"F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original"
    dry_run = _run_case(
        "smoke-shanghaitech-locateanything",
        f"用 {shanghaitech_root} 测试 LocateAnything-3B，先做 dry-run。",
    )
    _assert_contains(dry_run, "model.verify_hf")
    _assert_contains(dry_run, "model.smoke_locateanything")
    _assert_contains(dry_run, "workflow.submit_run")

    summary = {
        "install": {
            "status": install.status,
            "plan_kinds": _plan_kinds(install),
        },
        "shanghaitech": {
            "status": dry_run.status,
            "plan_kinds": _plan_kinds(dry_run),
        },
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


def _run_case(message_id: str, text: str):
    request = RuntimeRequest(
        message_id=message_id,
        channel="cli",
        account_id="local",
        peer_kind="direct",
        peer_id="smoke",
        sender_id="smoke",
        session_key="agent:go-control-plane:cli:direct:smoke",
        text=text,
        attachments=[],
    )
    return run_runtime(request)


def _plan_kinds(result) -> list[str]:
    return [str(item.get("kind") or "") for item in result.plan]


def _assert_contains(result, kind: str) -> None:
    kinds = _plan_kinds(result)
    if kind not in kinds:
        raise SystemExit(
            f"Expected plan kind {kind!r}, got {kinds!r}; "
            f"status={result.status!r}; reply={result.reply_text[:200]!r}"
        )


def _assert_not_contains(result, kind: str) -> None:
    kinds = _plan_kinds(result)
    if kind in kinds:
        raise SystemExit(f"Unexpected plan kind {kind!r}, got {kinds!r}")


if __name__ == "__main__":
    raise SystemExit(main())
