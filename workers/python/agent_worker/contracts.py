from __future__ import annotations

from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from typing import Any


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


@dataclass
class JobEnvelope:
    task_id: str
    workflow_id: str
    agent_id: str
    tool_id: str
    action: str
    dataset_id: str = ""
    scene: str = ""
    dry_run: bool = True
    params: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, value: dict[str, Any]) -> "JobEnvelope":
        return cls(
            task_id=str(value.get("task_id") or ""),
            workflow_id=str(value.get("workflow_id") or ""),
            agent_id=str(value.get("agent_id") or ""),
            tool_id=str(value.get("tool_id") or ""),
            action=str(value.get("action") or ""),
            dataset_id=str(value.get("dataset_id") or ""),
            scene=str(value.get("scene") or ""),
            dry_run=bool(value.get("dry_run", True)),
            params={str(k): str(v) for k, v in dict(value.get("params") or {}).items()},
        )


@dataclass
class WorkerHeartbeat:
    at: str
    status: str
    message: str = ""


@dataclass
class JobLog:
    at: str
    level: str
    message: str


@dataclass
class JobArtifact:
    name: str
    uri: str
    kind: str = "generic"
    metadata: dict[str, str] = field(default_factory=dict)


@dataclass
class JobResult:
    task_id: str
    status: str
    artifacts: list[JobArtifact] = field(default_factory=list)
    metrics: dict[str, float] = field(default_factory=dict)
    logs: list[JobLog] = field(default_factory=list)
    heartbeat: WorkerHeartbeat | None = None
    attempt: int = 1
    max_attempts: int = 1
    retryable: bool = False
    message: str = ""
    started_at: str = ""
    finished_at: str = ""

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)
