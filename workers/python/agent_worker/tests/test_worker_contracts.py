from __future__ import annotations

import io
import json
import tempfile
from pathlib import Path
import subprocess
import unittest
from contextlib import redirect_stderr, redirect_stdout
from unittest.mock import patch

from agent_worker.contracts import JobEnvelope
from agent_worker.main import main, run_job


class WorkerContractTests(unittest.TestCase):
    def run_job_quiet(self, envelope: JobEnvelope) -> dict[str, object]:
        with redirect_stderr(io.StringIO()):
            return run_job(envelope).to_dict()

    def test_dry_run_result_contains_observability_contract(self) -> None:
        result = self.run_job_quiet(
            JobEnvelope(
                task_id="task_000001",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="training-agent",
                tool_id="training-runner",
                action="train",
                dataset_id="shanghaitech-original",
                dry_run=True,
            )
        )

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertGreaterEqual(len(result["logs"]), 2)
        self.assertEqual(result["artifacts"][0]["kind"], "dry-run-plan")
        self.assertEqual(result["artifacts"][0]["metadata"]["dataset_id"], "shanghaitech-original")
        self.assertFalse(result["retryable"])

    def test_training_run_dry_run_builds_explicit_recipe_artifact(self) -> None:
        result = self.run_job_quiet(
            JobEnvelope(
                task_id="task_000010",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="training-agent",
                tool_id="training.run",
                action="training.run",
                dry_run=True,
                params={
                    "request_json": json.dumps(
                        {
                            "dataset_id": "shanghaitech-original",
                            "target_task": "detection",
                            "model_family": "yolo11n",
                            "split_config": "official-split",
                        }
                    )
                },
            )
        )

        self.assertEqual(result["status"], "completed")
        self.assertIn("training dry-run recipe ready", result["message"])
        self.assertEqual(result["artifacts"][0]["kind"], "training.run.plan")
        self.assertEqual(result["artifacts"][0]["metadata"]["dataset_id"], "shanghaitech-original")
        self.assertGreaterEqual(len(result["logs"]), 3)

    def test_deployment_run_validates_required_fields(self) -> None:
        result = self.run_job_quiet(
            JobEnvelope(
                task_id="task_000011",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="deployment-agent",
                tool_id="deployment.run",
                action="deployment.run",
                dry_run=True,
                params={"request_json": json.dumps({"model_id": "model-1"})},
            )
        )

        self.assertEqual(result["status"], "failed")
        self.assertFalse(result["retryable"])
        self.assertIn("target is required", result["message"])

    def test_evaluation_run_dry_run_uses_default_metrics(self) -> None:
        result = self.run_job_quiet(
            JobEnvelope(
                task_id="task_000012",
                workflow_id="data-to-deployment-lifecycle",
                agent_id="evaluation-agent",
                tool_id="evaluation.run",
                action="evaluation.run",
                dry_run=True,
                params={"request_json": json.dumps({"dataset_id": "shanghaitech-original", "model_id": "model-1"})},
            )
        )

        self.assertEqual(result["status"], "completed")
        self.assertIn("evaluation dry-run recipe ready", result["message"])
        self.assertEqual(result["artifacts"][0]["kind"], "evaluation.run.plan")

    def test_training_run_execution_uses_default_recipe_runner(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            artifact_root = Path(tmp) / "artifacts"
            result = self.run_job_quiet(
                JobEnvelope(
                    task_id="task_000013",
                    workflow_id="data-to-deployment-lifecycle",
                    agent_id="training-agent",
                    tool_id="training.run",
                    action="training.run",
                    dry_run=False,
                    params={
                        "artifact_root": str(artifact_root),
                        "request_json": json.dumps(
                            {
                                "dataset_id": "shanghaitech-original",
                                "target_task": "detection",
                                "model_family": "yolo11n",
                                "split_config": "official-split",
                            }
                        ),
                    },
                )
            )

            self.assertEqual(result["status"], "completed")
            self.assertIn("recipe completed", result["message"])
            self.assertEqual(result["heartbeat"]["status"], "completed")
            artifact_kinds = [item["kind"] for item in result["artifacts"]]
            self.assertIn("training.run.request", artifact_kinds)
            self.assertIn("training.run.plan", artifact_kinds)
            self.assertIn("training.run.result", artifact_kinds)
            self.assertIn("training.run.recipe_spec", artifact_kinds)
            self.assertIn("training.run.recipe_report", artifact_kinds)
            self.assertIn("training.run.generated", artifact_kinds)
            bundle_dir = artifact_root / "training.run" / "task_000013"
            self.assertTrue((bundle_dir / "request.json").exists())
            self.assertTrue((bundle_dir / "plan.json").exists())
            self.assertTrue((bundle_dir / "result.json").exists())
            self.assertTrue((bundle_dir / "recipe_spec.json").exists())
            self.assertTrue((bundle_dir / "recipe_report.json").exists())
            self.assertTrue((bundle_dir / "generated" / "train_summary.json").exists())
            self.assertTrue((bundle_dir / "generated" / "train_metrics.json").exists())
            self.assertTrue((bundle_dir / "generated" / "checkpoint.stub.json").exists())
            self.assertTrue((bundle_dir / "generated" / "train.log").exists())
            result_payload = json.loads((bundle_dir / "result.json").read_text(encoding="utf-8"))
            self.assertFalse(result_payload["dry_run"])
            self.assertEqual(result_payload["execution_mode"], "recipe-executed")
            self.assertEqual(result_payload["execution_recipe"], "default")
            self.assertTrue(result_payload["recipe_spec_path"].endswith("recipe_spec.json"))
            self.assertEqual(result_payload["request"]["dataset_id"], "shanghaitech-original")

    @patch("agent_worker.lifecycle.run_command_with_events")
    def test_training_run_execution_command_runs_real_recipe(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout="training started\ntraining finished\n",
            stderr="",
        )
        with tempfile.TemporaryDirectory() as tmp:
            artifact_root = Path(tmp) / "artifacts"
            result = self.run_job_quiet(
                JobEnvelope(
                    task_id="task_000014",
                    workflow_id="data-to-deployment-lifecycle",
                    agent_id="training-agent",
                    tool_id="training.run",
                    action="training.run",
                    dry_run=False,
                    params={
                        "artifact_root": str(artifact_root),
                        "request_json": json.dumps(
                            {
                                "dataset_id": "shanghaitech-original",
                                "target_task": "detection",
                                "model_family": "yolo11n",
                                "execution_command": ["python", "-c", "print('ok')"],
                                "execution_timeout_seconds": 30,
                            }
                        ),
                    },
                )
            )

            self.assertEqual(result["status"], "completed")
            self.assertIn("command completed", result["message"])
            bundle_dir = artifact_root / "training.run" / "task_000014"
            result_payload = json.loads((bundle_dir / "result.json").read_text(encoding="utf-8"))
            self.assertEqual(result_payload["execution_mode"], "command-executed")
            self.assertEqual(result_payload["returncode"], 0)
            self.assertEqual(result_payload["execution_command"], ["python", "-c", "print('ok')"])

    @patch("agent_worker.lifecycle.run_command_with_events")
    def test_training_run_execution_command_nonzero_exit_fails(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=5,
            stdout="",
            stderr="trainer crashed",
        )
        with tempfile.TemporaryDirectory() as tmp:
            artifact_root = Path(tmp) / "artifacts"
            result = self.run_job_quiet(
                JobEnvelope(
                    task_id="task_000015",
                    workflow_id="data-to-deployment-lifecycle",
                    agent_id="training-agent",
                    tool_id="training.run",
                    action="training.run",
                    dry_run=False,
                    params={
                        "artifact_root": str(artifact_root),
                        "request_json": json.dumps(
                            {
                                "dataset_id": "shanghaitech-original",
                                "target_task": "detection",
                                "model_family": "yolo11n",
                                "execution_command": ["python", "-c", "raise SystemExit(5)"],
                            }
                        ),
                    },
                )
            )

            self.assertEqual(result["status"], "failed")
            self.assertFalse(result["retryable"])
            self.assertEqual(result["heartbeat"]["status"], "failed")
            bundle_dir = artifact_root / "training.run" / "task_000015"
            result_payload = json.loads((bundle_dir / "result.json").read_text(encoding="utf-8"))
            self.assertEqual(result_payload["execution_mode"], "command-failed")
            self.assertEqual(result_payload["returncode"], 5)

    @patch("agent_worker.lifecycle.run_command_with_events")
    def test_training_run_execution_command_timeout_is_retryable(self, run_mock) -> None:
        run_mock.side_effect = subprocess.TimeoutExpired(cmd=["python"], timeout=3)
        with tempfile.TemporaryDirectory() as tmp:
            artifact_root = Path(tmp) / "artifacts"
            result = self.run_job_quiet(
                JobEnvelope(
                    task_id="task_000016",
                    workflow_id="data-to-deployment-lifecycle",
                    agent_id="training-agent",
                    tool_id="training.run",
                    action="training.run",
                    dry_run=False,
                    params={
                        "artifact_root": str(artifact_root),
                        "request_json": json.dumps(
                            {
                                "dataset_id": "shanghaitech-original",
                                "target_task": "detection",
                                "model_family": "yolo11n",
                                "execution_command": ["python", "-c", "import time; time.sleep(10)"],
                                "execution_timeout_seconds": 3,
                            }
                        ),
                    },
                )
            )

            self.assertEqual(result["status"], "failed")
            self.assertTrue(result["retryable"])
            self.assertEqual(result["heartbeat"]["status"], "failed")
            bundle_dir = artifact_root / "training.run" / "task_000016"
            result_payload = json.loads((bundle_dir / "result.json").read_text(encoding="utf-8"))
            self.assertEqual(result_payload["execution_mode"], "command-timeout")

    def test_missing_task_id_is_non_retryable_failure(self) -> None:
        result = self.run_job_quiet(JobEnvelope(task_id="", workflow_id="wf", agent_id="agent", tool_id="tool", action="run"))
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

    @patch("agent_worker.main.run_command_with_events")
    def test_download_hf_action_runs_snapshot_script(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"complete": True}),
            stderr="",
        )
        result = self.run_job_quiet(
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
        )

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertEqual(result["artifacts"][0]["kind"], "manifest")
        self.assertFalse(result["retryable"])

    @patch("agent_worker.main.run_command_with_events")
    def test_verify_hf_action_runs_snapshot_script_with_verify_only(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"complete": True}),
            stderr="",
        )
        result = self.run_job_quiet(
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
        )

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertFalse(result["retryable"])
        called_args = run_mock.call_args.args[0]
        self.assertIn("--verify-only", called_args)

    @patch("agent_worker.main.run_command_with_events")
    def test_smoke_locateanything_action_runs_smoke_script(self, run_mock) -> None:
        run_mock.return_value = subprocess.CompletedProcess(
            args=["python"],
            returncode=0,
            stdout=json.dumps({"status": "ok", "completed": {"model_load": True, "real_inference": False}}),
            stderr="",
        )
        result = self.run_job_quiet(
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
        )

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["heartbeat"]["status"], "completed")
        self.assertEqual(result["artifacts"][0]["kind"], "smoke-report")
        called_args = run_mock.call_args.args[0]
        self.assertIn("locateanything_smoke.py", " ".join(called_args))

    @patch("agent_worker.main.run_command_with_events")
    def test_download_hf_timeout_is_retryable_failure(self, run_mock) -> None:
        run_mock.side_effect = subprocess.TimeoutExpired(cmd=["python"], timeout=1)
        result = self.run_job_quiet(
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
        )

        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["heartbeat"]["status"], "failed")
        self.assertTrue(result["retryable"])
        self.assertIn("timed out", result["message"])


if __name__ == "__main__":
    unittest.main()
