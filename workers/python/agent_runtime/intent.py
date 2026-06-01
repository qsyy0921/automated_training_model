from __future__ import annotations

from agent_runtime.contracts import Intent, RuntimeRequest


def classify_intent(request: RuntimeRequest) -> Intent:
    text = request.text.strip()
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

