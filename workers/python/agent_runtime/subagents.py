from __future__ import annotations

from dataclasses import asdict, dataclass, field

from agent_runtime.contracts import Intent, RuntimeRequest


@dataclass
class DelegationDecision:
    use_sub_agent: bool
    reason: str
    agent_id: str = ""
    required_capabilities: list[str] = field(default_factory=list)
    skill_id: str = ""
    tool_id: str = ""
    mcp_server: str = ""
    model_route: str = ""

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


def decide_sub_agent(intent: Intent, request: RuntimeRequest) -> DelegationDecision:
    if intent.kind in {"health_check", "identify_actor", "runtime_status", "list_runs", "submit_dry_run"}:
        return DelegationDecision(
            use_sub_agent=False,
            reason="low-risk deterministic runtime command handled by Go control plane",
            skill_id=intent.skill_id,
            tool_id=intent.tool_id,
        )
    if intent.kind == "runtime_about":
        return DelegationDecision(
            use_sub_agent=False,
            reason="local runtime self-description handled by Go control plane",
            skill_id=intent.skill_id,
            tool_id=intent.tool_id,
        )
    if intent.kind in {"model_install", "model_test"}:
        return DelegationDecision(
            use_sub_agent=True,
            agent_id="model-agent",
            reason="model lifecycle requests need controlled worker tools and observable jobs",
            required_capabilities=["model-download", "model-verify", "smoke-test"],
            skill_id=intent.skill_id,
            tool_id=intent.tool_id,
            model_route="text-planning",
        )
    if intent.kind == "data_intake":
        if _has_visual_attachment(request):
            return DelegationDecision(
                use_sub_agent=True,
                agent_id="vision-agent",
                reason="visual attachments require Mimo 2.5 inspection before data intake planning",
                required_capabilities=["image-understanding", "visual-data-check", "quarantine-plan"],
                skill_id="channel-data-intake",
                tool_id="vlm.inspect",
                model_route="vision",
            )
        return DelegationDecision(
            use_sub_agent=True,
            agent_id="data-intake-agent",
            reason="channel attachments must be quarantined and planned before entering the data lake",
            required_capabilities=["quarantine-plan", "data-governance"],
            skill_id="channel-data-intake",
            tool_id="intake.plan",
            model_route="text-planning",
        )
    if intent.kind == "chat":
        return DelegationDecision(
            use_sub_agent=True,
            agent_id="planner-agent",
            reason="free-form user text needs LLM planning before tool execution",
            required_capabilities=["intent-refine", "workflow-plan", "tool-plan"],
            skill_id=intent.skill_id,
            tool_id=intent.tool_id,
            model_route="text-planning",
        )
    return DelegationDecision(use_sub_agent=False, reason="unknown intent is not delegated until classified")


def _has_visual_attachment(request: RuntimeRequest) -> bool:
    for attachment in request.attachments:
        media_type = attachment.media_type.strip().lower()
        name = attachment.name.strip().lower()
        if media_type.startswith("image"):
            return True
        if name.endswith((".jpg", ".jpeg", ".png", ".webp", ".bmp")):
            return True
    return False
