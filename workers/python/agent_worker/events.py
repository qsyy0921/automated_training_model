from __future__ import annotations

import json
import sys

from agent_worker.contracts import utc_now_iso

WORKER_EVENT_PREFIX = "ATM_EVENT "


def emit_worker_event(payload: dict[str, object]) -> None:
    print(WORKER_EVENT_PREFIX + json.dumps(payload, ensure_ascii=False), file=sys.stderr, flush=True)


def emit_worker_log(level: str, message: str, at: str | None = None) -> None:
    emit_worker_event(
        {
            "type": "log",
            "at": at or utc_now_iso(),
            "level": level,
            "message": message,
        }
    )


def emit_worker_heartbeat(status: str, message: str, at: str | None = None) -> None:
    emit_worker_event(
        {
            "type": "heartbeat",
            "at": at or utc_now_iso(),
            "status": status,
            "message": message,
        }
    )


def emit_worker_stream(stream: str, text: str, at: str | None = None) -> None:
    emit_worker_event(
        {
            "type": "stream",
            "at": at or utc_now_iso(),
            "stream": stream,
            "text": text,
        }
    )
