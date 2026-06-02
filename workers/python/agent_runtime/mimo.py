from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from typing import Any

from agent_runtime.contracts import Intent, RuntimeRequest, RuntimeResult


def mimo_enabled() -> bool:
    return os.getenv("AGENT_RUNTIME_USE_MIMO", "").strip().lower() in {"1", "true", "yes", "on"}


def plan_with_mimo(request: RuntimeRequest, intent: Intent, delegation: dict[str, Any]) -> RuntimeResult:
    base_url = _anthropic_base_url()
    token = os.getenv("ANTHROPIC_AUTH_TOKEN", "").strip()
    model = _select_model(request, delegation)
    if not base_url or not token:
        raise RuntimeError("Mimo Anthropic-compatible environment is not configured")

    prompt = _planner_prompt(request, intent, delegation)
    body = {
        "model": model,
        "max_tokens": 1200,
        "temperature": 0.1,
        "messages": [{"role": "user", "content": prompt}],
    }
    raw = _post_json(base_url, token, body)
    text = _extract_anthropic_text(raw)
    parsed = _extract_json_or_repair(base_url, token, model, text)
    _validate_plan_contract(request, parsed)
    return RuntimeResult(
        status=str(parsed.get("status") or "planned"),
        intent=intent,
        reply_text=str(parsed.get("reply_text") or text or "Mimo planner 已完成规划。"),
        plan=list(parsed.get("plan") or []),
        delegations=[delegation],
    )


def _select_model(request: RuntimeRequest, delegation: dict[str, Any]) -> str:
    if _needs_vision_model(request, delegation):
        return (
            os.getenv("MIMO_VISION_MODEL")
            or os.getenv("VLM_MODEL")
            or os.getenv("ANTHROPIC_VISION_MODEL")
            or "mimo-v2.5"
        ).strip()
    return (
        os.getenv("MIMO_DEFAULT_MODEL")
        or os.getenv("ANTHROPIC_MODEL")
        or os.getenv("ANTHROPIC_DEFAULT_SONNET_MODEL")
        or "mimo-v2.5-pro"
    ).strip()


def _needs_vision_model(request: RuntimeRequest, delegation: dict[str, Any]) -> bool:
    if str(delegation.get("model_route") or "").strip().lower() == "vision":
        return True
    for attachment in request.attachments:
        media_type = attachment.media_type.strip().lower()
        name = attachment.name.strip().lower()
        if media_type.startswith("image") or "vision" in media_type:
            return True
        if name.endswith((".jpg", ".jpeg", ".png", ".webp", ".bmp")):
            return True
    return False


def _anthropic_base_url() -> str:
    base = os.getenv("ANTHROPIC_BASE_URL", "").strip().rstrip("/")
    if not base:
        return ""
    if base.endswith("/v1/messages"):
        return base
    if base.endswith("/v1"):
        return base + "/messages"
    return base + "/v1/messages"


def _planner_prompt(request: RuntimeRequest, intent: Intent, delegation: dict[str, Any]) -> str:
    attachments = [
        {
            "id": item.id,
            "name": item.name,
            "media_type": item.media_type,
            "source_uri": item.source_uri,
        }
        for item in request.attachments
    ]
    context = {
        "message": {
            "channel": request.channel,
            "account_id": request.account_id,
            "peer_kind": request.peer_kind,
            "peer_id": request.peer_id,
            "sender_id": request.sender_id,
            "text": request.text,
            "attachments": attachments,
            "session_key": request.session_key,
        },
        "intent": intent.to_dict(),
        "delegation": delegation,
        "available_tools": [
            "llm.plan",
            "intake.quarantine",
            "intake.plan",
            "vlm.inspect",
            "model.download_hf",
            "model.verify_hf",
            "workflow.submit_run",
            "workflow.list_runs",
            "runtime.status",
        ],
    }
    return (
        "你是 automated_training_model 的 Agent Runtime planner。"
        "只输出 JSON，不要 Markdown。JSON schema: "
        '{"status":"planned|tool_planned","reply_text":"中文回复","plan":[{"kind":"tool.id","params":{"k":"v"}}]}。'
        "所有 plan.params 的值必须是字符串，不能使用嵌套对象或数组；"
        "HuggingFace 模型安装请求只允许使用 model.download_hf 和 model.verify_hf，不能附加 workflow.submit_run；"
        "HuggingFace 模型下载使用 model.download_hf，参数包含 repo_id、local_dir、manifest；"
        "nvidia/LocateAnything-3B 的默认 local_dir 是 data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B，"
        "manifest 是 data_lake/catalog/models/nvidia_LocateAnything-3B.download.json；"
        "当用户要求用 ShanghaiTech original 或指定 data_root 测试 LocateAnything-3B 时，优先计划 model.verify_hf，"
        "然后计划 workflow.submit_run，且 params 必须包含 workflow_id=data-to-deployment-lifecycle、dataset_id=shanghaitech-original、dry_run=true、"
        "model_repo_id=nvidia/LocateAnything-3B、data_root=用户提供的数据路径；"
        "只有用户明确要求测试/运行并且 dry_run=true 时，才允许计划 workflow.submit_run；数据入湖必须先 intake.quarantine 和 intake.plan；"
        "图片或视觉数据先 vlm.inspect。当前上下文如下：\n"
        + json.dumps(context, ensure_ascii=False)
    )


def _post_json(url: str, token: str, body: dict[str, Any]) -> dict[str, Any]:
    raw = json.dumps(body, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(url, data=raw, method="POST")
    req.add_header("content-type", "application/json")
    req.add_header("anthropic-version", os.getenv("ANTHROPIC_VERSION", "2023-06-01"))
    req.add_header("x-api-key", token)
    req.add_header("authorization", f"Bearer {token}")
    timeout = float(os.getenv("AGENT_RUNTIME_MIMO_TIMEOUT_SECONDS", "30"))
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"Mimo HTTP {exc.code}: {detail}") from exc


def _extract_anthropic_text(value: dict[str, Any]) -> str:
    chunks = value.get("content") or []
    texts: list[str] = []
    for chunk in chunks:
        if isinstance(chunk, dict) and chunk.get("type") == "text":
            texts.append(str(chunk.get("text") or ""))
    return "\n".join(item for item in texts if item).strip()


def _extract_json(text: str) -> dict[str, Any]:
    stripped = text.strip()
    if stripped.startswith("```"):
        stripped = stripped.strip("`")
        if stripped.lower().startswith("json"):
            stripped = stripped[4:].strip()
    start = stripped.find("{")
    end = stripped.rfind("}")
    if start >= 0 and end >= start:
        stripped = stripped[start : end + 1]
    parsed = json.loads(stripped)
    if not isinstance(parsed, dict):
        raise ValueError("Mimo planner did not return a JSON object")
    return parsed


def _extract_json_or_repair(base_url: str, token: str, model: str, text: str) -> dict[str, Any]:
    try:
        return _extract_json(text)
    except Exception as first_error:
        repair_prompt = (
            "把下面内容修复为严格 JSON 对象，只输出 JSON，不要 Markdown，不要解释。"
            "schema: {\"status\":\"planned|tool_planned|blocked\",\"reply_text\":\"中文回复\",\"plan\":[{\"kind\":\"tool.id\",\"params\":{\"key\":\"value\"}}]}。"
            "所有 params 的值必须是字符串。原始内容：\n"
            + text[:4000]
        )
        raw = _post_json(
            base_url,
            token,
            {
                "model": model,
                "max_tokens": 800,
                "temperature": 0,
                "messages": [{"role": "user", "content": repair_prompt}],
            },
        )
        repaired = _extract_anthropic_text(raw)
        try:
            return _extract_json(repaired)
        except Exception as repair_error:
            raise ValueError(f"Mimo planner did not return valid JSON: {first_error}; repair failed: {repair_error}") from repair_error


def _validate_plan_contract(request: RuntimeRequest, parsed: dict[str, Any]) -> None:
    plan = parsed.get("plan") or []
    if not isinstance(plan, list):
        raise ValueError("Mimo planner returned non-list plan")
    for item in plan:
        if not isinstance(item, dict):
            raise ValueError("Mimo planner returned non-object tool call")
        params = item.get("params") or {}
        if not isinstance(params, dict):
            raise ValueError("Mimo planner returned non-object params")
        for key, value in params.items():
            if not isinstance(value, str):
                raise ValueError(f"Mimo planner returned non-string param value for {key}")

    text = request.text.lower()
    kinds = {str(item.get("kind") or "") for item in plan if isinstance(item, dict)}
    if "locateanything-3b" in text and ("下载" in text or "安装" in text or "download" in text or "install" in text):
        if "model.download_hf" not in kinds:
            raise ValueError("Mimo planner omitted model.download_hf for LocateAnything install request")
        if "model.verify_hf" in kinds:
            raise ValueError("Mimo planner must not verify before downloading for pure LocateAnything install request")
        if "workflow.submit_run" in kinds:
            raise ValueError("Mimo planner must not submit workflow for pure LocateAnything install request")
    if "locateanything-3b" in text and "shanghaitech" in text and ("dry-run" in text or "dry run" in text or "测试" in text):
        if "model.verify_hf" not in kinds or "workflow.submit_run" not in kinds:
            raise ValueError("Mimo planner omitted required ShanghaiTech dry-run tools")
