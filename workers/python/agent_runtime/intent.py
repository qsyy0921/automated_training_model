from __future__ import annotations

from agent_runtime.contracts import Intent, RuntimeRequest


def classify_intent(request: RuntimeRequest) -> Intent:
    text = request.text.strip()
    go_intent = (request.metadata.get("go_intent") or "").strip()
    if go_intent:
        intent = _intent_from_go(go_intent, request)
        if intent is not None:
            return intent
    if request.attachments:
        return Intent(
            kind="data_intake",
            raw_text=text,
            skill_id="channel-data-intake",
            tool_id="intake.plan",
            confidence=1.0,
        )
    if not text:
        return Intent(kind="unknown", confidence=1.0)
    if not text.startswith("/"):
        return Intent(
            kind="chat",
            raw_text=text,
            skill_id="agent-conversation",
            tool_id="llm.plan",
            confidence=0.7,
        )

    fields = text.split()
    command = fields[0].lower()
    args = fields[1:]
    if command == "/bot-ping":
        return Intent(kind="health_check", raw_text=text, command=command, args=args, tool_id="runtime.health", confidence=1.0)
    if command == "/bot-me":
        return Intent(kind="identify_actor", raw_text=text, command=command, args=args, tool_id="runtime.identify_actor", confidence=1.0)
    if command == "/bot-status":
        return Intent(kind="runtime_status", raw_text=text, command=command, args=args, tool_id="runtime.status", confidence=1.0)
    if command == "/bot-runs":
        return Intent(kind="list_runs", raw_text=text, command=command, args=args, tool_id="workflow.list_runs", confidence=1.0)
    if command == "/bot-run" and len(fields) >= 2 and fields[1].lower() == "dry":
        dataset_id = fields[2] if len(fields) >= 3 else "workspace-dataset"
        return Intent(
            kind="submit_dry_run",
            raw_text=text,
            command=command,
            args=args,
            dataset_id=dataset_id,
            skill_id="data-to-deployment-lifecycle",
            tool_id="workflow.submit_run",
            confidence=1.0,
        )
    return Intent(kind="unknown", raw_text=text, command=command, args=args, confidence=1.0)


def _intent_from_go(kind: str, request: RuntimeRequest) -> Intent | None:
    text = request.text.strip()
    if kind == "runtime_about":
        return Intent(kind=kind, raw_text=text, skill_id="runtime-self-description", confidence=0.95)
    if kind == "model_install":
        return Intent(
            kind=kind,
            raw_text=text,
            skill_id="huggingface-model-downloader",
            tool_id="model.download_hf",
            confidence=0.9,
        )
    if kind == "model_test":
        return Intent(
            kind=kind,
            raw_text=text,
            dataset_id=_dataset_id_from_text(text),
            skill_id="model-validation",
            tool_id="model.smoke_locateanything",
            confidence=0.9,
        )
    return None


def _dataset_id_from_text(text: str) -> str:
    lowered = text.lower()
    if "shanghaitech" in lowered or "上海" in lowered:
        return "shanghaitech-original"
    return ""
