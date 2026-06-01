from __future__ import annotations

from dataclasses import asdict, dataclass, field


@dataclass
class SkillEvolutionConfig:
    enabled: bool = False
    draft_dir: str = "data_lake/agents/skill_drafts"
    controls: list[str] = field(
        default_factory=lambda: [
            "summarize successful traces only",
            "write drafts to quarantine",
            "require human approval before enablement",
            "strip secrets and raw private data",
        ]
    )

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


def default_skill_evolution_config() -> SkillEvolutionConfig:
    return SkillEvolutionConfig()
