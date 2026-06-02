from __future__ import annotations

import json
import os
import sys
import traceback

from agent_runtime.contracts import RuntimeRequest, RuntimeResult
from agent_runtime.intent import classify_intent
from agent_runtime.main import _should_use_fast_chat, run_runtime
from agent_runtime.mimo import mimo_enabled, stream_chat_with_mimo
from agent_runtime.subagents import decide_sub_agent


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
            if bool(payload.get("stream")):
                result = _run_stream(request_id, request)
            else:
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


def _run_stream(request_id: str, request: RuntimeRequest) -> RuntimeResult:
    intent = classify_intent(request)
    delegation = decide_sub_agent(intent, request).to_dict()
    if not (mimo_enabled() and intent.kind == "chat" and _should_use_fast_chat(request)):
        _write({"type": "status", "request_id": request_id, "message": "stream fallback to planner"})
        return run_runtime(request)

    _write(
        {
            "type": "start",
            "request_id": request_id,
            "intent": intent.to_dict(),
            "delegation": delegation,
        }
    )

    def on_delta(delta: str) -> None:
        _write({"type": "delta", "request_id": request_id, "delta": delta})

    return stream_chat_with_mimo(request, intent, delegation, on_delta)


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
