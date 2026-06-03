from __future__ import annotations

import io
import json
import subprocess
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

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

    @patch("agent_worker.main.subprocess.run")
    def test_download_hf_action_runs_snapshot_script(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"complete": True}),
            stderr="",
        )
        result = run_job(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="model-agent",
                tool_id="model.download_hf",
                action="download_hf",
                dry_run=False,
                params={
                    "repo_id": "sshleifer/tiny-gpt2",
                    "local_dir": "data_lake/models/artifacts/huggingface/sshleifer/tiny-gpt2",
                    "manifest": "data_lake/catalog/models/sshleifer_tiny-gpt2.download.json",
                },
            )
        ).to_dict()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertEqual(result["artifacts"][0]["kind"], "manifest")
        self.assertFalse(result["retryable"])

    @patch("agent_worker.main.subprocess.run")
    def test_verify_hf_action_runs_snapshot_script_with_verify_only(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"complete": True}),
            stderr="",
        )
        result = run_job(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="model-agent",
                tool_id="model.verify_hf",
                action="verify_hf",
                dry_run=False,
                params={
                    "repo_id": "sshleifer/tiny-gpt2",
                    "local_dir": "data_lake/models/artifacts/huggingface/sshleifer/tiny-gpt2",
                    "manifest": "data_lake/catalog/models/sshleifer_tiny-gpt2.download.json",
                },
            )
        ).to_dict()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertFalse(result["retryable"])
        called_args = run_mock.call_args.args[0]
        self.assertIn("--verify-only", called_args)

    @patch("agent_worker.main.subprocess.run")
    def test_smoke_locateanything_action_runs_smoke_script(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"status": "ok", "completed": {"model_load": True, "real_inference": False}}),
            stderr="",
        )
        result = run_job(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="model-agent",
                tool_id="model.smoke_locateanything",
                action="smoke_locateanything",
                dry_run=False,
                params={
                    "model_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
                    "data_root": "data_lake/raw/datasets/shanghaitech/original",
                    "output": "data_lake/catalog/models/nvidia_LocateAnything-3B.smoke.json",
                },
            )
        ).to_dict()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertEqual(result["artifacts"][0]["kind"], "smoke-report")
        called_args = run_mock.call_args.args[0]
        self.assertIn("locateanything_smoke.py", " ".join(called_args))

    @patch("agent_worker.main.subprocess.run")
    def test_download_hf_timeout_is_retryable_failure(self, run_mock) -> None:
        run_mock.side_effect = subprocess.TimeoutExpired(cmd=["python"], timeout=1)
        result = run_job(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="model-agent",
                tool_id="model.download_hf",
                action="download_hf",
                dry_run=False,
                params={
                    "repo_id": "sshleifer/tiny-gpt2",
                    "local_dir": "data_lake/models/artifacts/huggingface/sshleifer/tiny-gpt2",
                    "manifest": "data_lake/catalog/models/sshleifer_tiny-gpt2.download.json",
                },
            )
        ).to_dict()

        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["heartbeat"]["status"], "failed")
        self.assertTrue(result["retryable"])
        self.assertIn("timed out", result["message"])


if __name__ == "__main__":
    unittest.main()
