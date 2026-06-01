from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any


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
class JobResult:
    task_id: str
    status: str
    artifacts: list[dict[str, str]] = field(default_factory=list)
    metrics: dict[str, float] = field(default_factory=dict)
    message: str = ""

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)
