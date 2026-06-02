from __future__ import annotations

import io
import json
import unittest
from contextlib import redirect_stdout

from agent_worker.contracts import JobEnvelope
from agent_worker.main import main, run_job


class WorkerContractTests(unittest.TestCase):
    def test_dry_run_result_contains_observability_contract(self) -> None:
        result = run_job(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="training-agent",
                tool_id="training-runner",
                action="train",
                dataset_id="shanghaitech-original",
                dry_run=True,
            )
        ).to_dict()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertGreaterEqual(len(result["logs"]), 2)
        self.assertEqual(result["artifacts"][0]["kind"], "dry-run-plan")
        self.assertEqual(result["artifacts"][0]["metadata"]["dataset_id"], "shanghaitech-original")
        self.assertFalse(result["retryable"])

    def test_missing_task_id_is_non_retryable_failure(self) -> None:
        result = run_job(JobEnvelope(task_id="", workflow_id="wf", agent_id="agent", tool_id="tool", action="run")).to_dict()
        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["heartbeat"]["status"], "failed")
        self.assertFalse(result["retryable"])
        self.assertIn("task_id is required", result["message"])

    def test_health_command_prints_worker_heartbeat(self) -> None:
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            exit_code = main(["--health"])
        payload = json.loads(stdout.getvalue())

        self.assertEqual(exit_code, 0)
        self.assertEqual(payload["status"], "ok")
        self.assertEqual(payload["heartbeat"]["status"], "ok")
        self.assertIn("artifacts", payload["capabilities"])


if __name__ == "__main__":
    unittest.main()
