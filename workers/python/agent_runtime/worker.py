from __future__ import annotations

import json
import os
import sys
import traceback

from agent_runtime.contracts import RuntimeRequest, RuntimeResult
from agent_runtime.intent import classify_intent
from agent_runtime.main import run_runtime


def main() -> int:
    _write(
        {
            "type": "ready",
            "runtime": "agent_runtime.worker",
            "pid": os.getpid(),
        }
    )
    for line in sys.stdin:
        raw = line.strip()
        if not raw:
            continue
        if raw == "__quit__":
            _write({"type": "bye"})
            return 0
        try:
            payload = json.loads(raw)
            request_id = str(payload.get("request_id") or "")
            request = RuntimeRequest.from_dict(payload.get("request") or {})
            result = run_runtime(request)
            _write(
                {
                    "type": "result",
                    "request_id": request_id,
                    "result": result.to_dict(),
                }
            )
        except Exception as exc:
            intent = classify_intent(RuntimeRequest.from_dict({}))
            result = RuntimeResult(status="failed", intent=intent, reply_text=_safe_error(exc))
            _write(
                {
                    "type": "result",
                    "request_id": str(_safe_request_id(raw)),
                    "result": result.to_dict(),
                    "error": traceback.format_exc(limit=5),
                }
            )
    return 0


def _safe_request_id(raw: str) -> str:
    try:
        parsed = json.loads(raw)
        return str(parsed.get("request_id") or "")
    except Exception:
        return ""


def _safe_error(exc: Exception) -> str:
    text = str(exc)
    for name in ("ANTHROPIC_AUTH_TOKEN", "HF_TOKEN", "HUGGINGFACE_HUB_TOKEN"):
        token = os.getenv(name, "").strip()
        if token:
            text = text.replace(token, "***")
    return text


def _write(value: dict) -> None:
    sys.stdout.write(json.dumps(value, ensure_ascii=False) + "\n")
    sys.stdout.flush()


if __name__ == "__main__":
    raise SystemExit(main())
