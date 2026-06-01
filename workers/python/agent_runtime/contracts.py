from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass
class Attachment:
    id: str
    name: str = ""
    media_type: str = ""
    source_uri: str = ""

    @classmethod
    def from_dict(cls, value: dict[str, Any]) -> "Attachment":
        return cls(
            id=str(value.get("id") or ""),
            name=str(value.get("name") or ""),
            media_type=str(value.get("media_type") or ""),
            source_uri=str(value.get("source_uri") or ""),
        )


@dataclass
class RuntimeRequest:
    message_id: str
    channel: str
    account_id: str
    peer_kind: str
    peer_id: str
    sender_id: str
    text: str = ""
    mentioned: bool = False
    attachments: list[Attachment] = field(default_factory=list)
    session_key: str = ""

    @classmethod
    def from_dict(cls, value: dict[str, Any]) -> "RuntimeRequest":
        attachments = [Attachment.from_dict(item) for item in list(value.get("attachments") or [])]
        return cls(
            message_id=str(value.get("message_id") or value.get("id") or ""),
            channel=str(value.get("channel") or ""),
            account_id=str(value.get("account_id") or ""),
            peer_kind=str(value.get("peer_kind") or ""),
            peer_id=str(value.get("peer_id") or ""),
            sender_id=str(value.get("sender_id") or ""),
            text=str(value.get("text") or ""),
            mentioned=bool(value.get("mentioned", False)),
            attachments=attachments,
            session_key=str(value.get("session_key") or ""),
        )


@dataclass
class Intent:
    kind: str
    raw_text: str = ""
    command: str = ""
    args: list[str] = field(default_factory=list)
    dataset_id: str = ""
    skill_id: str = ""
    tool_id: str = ""
    mcp_server: str = ""
    confidence: float = 0.0

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


@dataclass
class RuntimeResult:
    status: str
    intent: Intent
    reply_text: str
    plan: list[dict[str, Any]] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        value = asdict(self)
        value["intent"] = self.intent.to_dict()
        return value

