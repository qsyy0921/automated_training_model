from __future__ import annotations

import os
import unittest
from unittest.mock import patch

from agent_runtime.contracts import RuntimeRequest, RuntimeResult
from agent_runtime.intent import classify_intent
from agent_runtime.main import _should_use_fast_chat, run_runtime


def _request(text: str) -> RuntimeRequest:
    return RuntimeRequest(
        message_id="test",
        channel="cli",
        account_id="local",
        peer_kind="direct",
        peer_id="cli-runtime",
        sender_id="cli-runtime",
        text=text,
        session_key="agent:planner-agent:cli:direct:cli-runtime",
    )


class FastChatTests(unittest.TestCase):
    def setUp(self) -> None:
        os.environ.pop("AGENT_RUNTIME_FAST_CHAT", None)

    def test_should_use_fast_chat_for_ordinary_chat(self) -> None:
        self.assertTrue(_should_use_fast_chat(_request("你好，你是谁？")))

    def test_should_not_use_fast_chat_for_tool_or_long_task_request(self) -> None:
        self.assertFalse(_should_use_fast_chat(_request("下载 nvidia/LocateAnything-3B 并测试 ShanghaiTech")))
        self.assertFalse(_should_use_fast_chat(_request("帮我写一个 HuggingFace 模型下载 skill")))

    def test_should_not_use_fast_chat_when_disabled(self) -> None:
        os.environ["AGENT_RUNTIME_FAST_CHAT"] = "false"
        self.assertFalse(_should_use_fast_chat(_request("你好")))

    def test_runtime_routes_ordinary_chat_to_fast_chat(self) -> None:
        fake_intent = None

        def fake_chat(request, intent, delegation):
            nonlocal fake_intent
            fake_intent = intent
            return RuntimeResult(status="planned", intent=intent, reply_text="fast", delegations=[delegation])

        with patch("agent_runtime.main.mimo_enabled", return_value=True), patch(
            "agent_runtime.main.chat_with_mimo", side_effect=fake_chat
        ) as chat, patch("agent_runtime.main.plan_with_mimo") as planner:
            result = run_runtime(_request("你好"))

        self.assertEqual(result.reply_text, "fast")
        self.assertEqual(fake_intent.kind, "chat")
        chat.assert_called_once()
        planner.assert_not_called()

    def test_python_intent_honors_go_metadata(self) -> None:
        request = _request("请帮我下载 HuggingFace nvidia/LocateAnything-3B 模型")
        request.metadata = {"go_intent": "model_install"}

        intent = classify_intent(request)

        self.assertEqual(intent.kind, "model_install")
        self.assertEqual(intent.tool_id, "model.download_hf")

    def test_runtime_uses_go_model_intent_for_guard_plan(self) -> None:
        request = _request("请帮我下载 HuggingFace nvidia/LocateAnything-3B 模型")
        request.metadata = {"go_intent": "model_install"}

        with patch("agent_runtime.main.mimo_enabled", return_value=False):
            result = run_runtime(request)

        self.assertEqual(result.intent.kind, "model_install")
        self.assertEqual(result.plan[0]["kind"], "model.download_hf")


if __name__ == "__main__":
    unittest.main()
