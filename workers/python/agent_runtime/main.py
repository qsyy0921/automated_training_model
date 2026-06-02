from __future__ import annotations

import argparse
import json
import os
import sys

from agent_runtime.contracts import RuntimeRequest, RuntimeResult
from agent_runtime.intent import classify_intent
from agent_runtime.mimo import mimo_enabled, plan_with_mimo
from agent_runtime.subagents import decide_sub_agent


def run_runtime(request: RuntimeRequest) -> RuntimeResult:
    intent = classify_intent(request)
    delegation = decide_sub_agent(intent, request)
    delegations = [delegation.to_dict()]

    if mimo_enabled() and intent.kind in {"chat", "data_intake"}:
        try:
            return plan_with_mimo(request, intent, delegations[0])
        except Exception as exc:
            guarded = _guarded_plan(request, intent, delegations)
            if guarded is not None:
                return guarded
            if not _allow_mimo_fallback():
                raise
            return RuntimeResult(
                status="planned",
                intent=intent,
                reply_text=f"Mimo planner 暂不可用，已回退到规则 planner：{exc}",
                delegations=delegations,
            )

    if intent.kind in {"health_check", "identify_actor", "runtime_status", "list_runs"}:
        return RuntimeResult(
            status="tool_planned",
            intent=intent,
            reply_text="",
            plan=[{"kind": intent.tool_id, "params": {"session_key": request.session_key}}],
            delegations=delegations,
        )

    if intent.kind == "data_intake":
        plan = [{"kind": "intake.quarantine", "params": {"count": str(len(request.attachments))}}]
        if delegation.tool_id == "vlm.inspect":
            plan.append({"kind": "vlm.inspect", "params": {"model_route": delegation.model_route}})
        plan.append({"kind": "intake.plan", "params": {"skill_id": intent.skill_id}})
        return RuntimeResult(
            status="tool_planned",
            intent=intent,
            reply_text=f"已收到 {len(request.attachments)} 个附件，先规划隔离、扫描和入湖审批。",
            plan=plan,
            delegations=delegations,
        )

    if intent.kind == "submit_dry_run":
        return RuntimeResult(
            status="tool_planned",
            intent=intent,
            reply_text=f"准备提交 dry-run workflow：dataset={intent.dataset_id}",
            plan=[
                {
                    "kind": "workflow.submit_run",
                    "params": {
                        "workflow_id": "data-to-deployment-lifecycle",
                        "dataset_id": intent.dataset_id,
                        "dry_run": "true",
                    },
                }
            ],
            delegations=delegations,
        )

    if intent.kind == "chat":
        return RuntimeResult(
            status="tool_planned",
            intent=intent,
            reply_text="",
            plan=[{"kind": "llm.plan", "params": {"model_route": delegation.model_route}}],
            delegations=delegations,
        )

    return RuntimeResult(
        status="planned",
        intent=intent,
        reply_text="未知命令或暂不支持的意图。发送 /bot-help 查看可用命令。",
        delegations=delegations,
    )


def _guarded_plan(request: RuntimeRequest, intent, delegations: list[dict[str, object]]) -> RuntimeResult | None:
    text = request.text.lower()
    if "locateanything-3b" in text and "shanghaitech" in text and ("dry-run" in text or "dry run" in text or "测试" in text):
        data_root = _extract_shanghaitech_root(request.text)
        return RuntimeResult(
            status="tool_planned_with_guard",
            intent=intent,
            reply_text="Mimo 响应不稳定，已使用受控 guard 生成 LocateAnything-3B + ShanghaiTech dry-run 计划。",
            plan=[
                {
                    "kind": "model.verify_hf",
                    "params": {
                        "repo_id": "nvidia/LocateAnything-3B",
                        "local_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
                        "manifest": "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
                        "verify_only": "true",
                    },
                },
                {
                    "kind": "workflow.submit_run",
                    "params": {
                        "workflow_id": "data-to-deployment-lifecycle",
                        "dataset_id": "shanghaitech-original",
                        "dry_run": "true",
                        "model_repo_id": "nvidia/LocateAnything-3B",
                        "data_root": data_root,
                    },
                },
            ],
            delegations=delegations,
        )
    if "locateanything-3b" in text and ("下载" in text or "安装" in text or "download" in text or "install" in text):
        return RuntimeResult(
            status="tool_planned_with_guard",
            intent=intent,
            reply_text="Mimo 响应不稳定，已使用受控 guard 生成 LocateAnything-3B 下载计划。",
            plan=[
                {
                    "kind": "model.download_hf",
                    "params": {
                        "repo_id": "nvidia/LocateAnything-3B",
                        "local_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
                        "manifest": "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
                    },
                }
            ],
            delegations=delegations,
        )
    return None


def _extract_shanghaitech_root(text: str) -> str:
    marker = "F:\\automated_training_model\\data_lake\\raw\\datasets\\shanghaitech\\original"
    if marker.lower() in text.lower():
        return marker
    marker_escaped = "F:\\\\automated_training_model\\\\data_lake\\\\raw\\\\datasets\\\\shanghaitech\\\\original"
    if marker_escaped.lower() in text.lower():
        return marker
    return marker


def _allow_mimo_fallback() -> bool:
    return os.getenv("AGENT_RUNTIME_MIMO_FALLBACK", "rule").strip().lower() not in {"0", "false", "off", "none"}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Automated Training Model Python Agent Runtime")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--request-json", help="JSON-encoded runtime request")
    source.add_argument("--request-file", help="Path to a JSON runtime request file")
    args = parser.parse_args(argv)

    request = RuntimeRequest.from_dict({})
    intent = classify_intent(request)
    try:
        raw = args.request_json
        if args.request_file:
            with open(args.request_file, "r", encoding="utf-8-sig") as handle:
                raw = handle.read()
        payload = json.loads(raw or "{}")
        request = RuntimeRequest.from_dict(payload)
        intent = classify_intent(request)
        result = run_runtime(request)
    except Exception as exc:
        result = RuntimeResult(status="failed", intent=intent, reply_text=_safe_error(exc))
    print(json.dumps(result.to_dict(), ensure_ascii=False))
    return 0 if result.status != "failed" else 1


def _safe_error(exc: Exception) -> str:
    text = str(exc)
    token = os.getenv("ANTHROPIC_AUTH_TOKEN", "").strip()
    if token:
        text = text.replace(token, "***")
    hf_token = os.getenv("HF_TOKEN", "").strip() or os.getenv("HUGGINGFACE_HUB_TOKEN", "").strip()
    if hf_token:
        text = text.replace(hf_token, "***")
    return text


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
