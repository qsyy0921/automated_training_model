# Python Agent Workers

This directory is the execution-layer boundary for model and data jobs.

The Go backend owns control-plane state: agent registry, tool registry, workflow
definitions, audit events, remote Gateway, channel routing, and task scheduling.
Python workers own LLM-heavy, GPU-heavy, or research-heavy execution: Agent
Runtime planning, tracking, segmentation, VLM grounding, training, evaluation,
and report materialization.

The current worker is intentionally small. It accepts a JSON job envelope and
prints a JSON result, so it can later be launched by a local process runner,
Docker worker, NATS consumer, or Kubernetes job without changing the Go domain
model. The result contract already includes heartbeat, logs, artifacts and
retry metadata, even though real task scheduling is still owned by the Go
control plane.

Health / heartbeat:

```powershell
$env:PYTHONPATH = (Resolve-Path .\workers\python).Path
python -m agent_worker.main --health
```

Example:

```powershell
$job = @{
  task_id = "task_000001"
  workflow_id = "human-loop-autolabel"
  agent_id = "tracking-agent"
  tool_id = "yolo26x-botsort"
  action = "track"
  dataset_id = "shanghaitech-original"
  dry_run = $true
} | ConvertTo-Json -Compress
Set-Content -Path tmp\agent-job.json -Value $job -Encoding UTF8
python -m agent_worker.main --job-file tmp\agent-job.json
```

Dry-run results include:

- `heartbeat`: worker status and timestamp.
- `logs`: ordered worker lifecycle messages.
- `artifacts`: dry-run artifact references, not large files.
- `retryable`, `attempt`, `max_attempts`: retry contract for the future Go task runner.

Lifecycle worker execution can now run an explicit command when a non-dry-run
request uses the default repo-owned recipe runner. In that mode the worker
materializes `request.json`, `plan.json`, `result.json`, `recipe_spec.json`,
and `recipe_report.json`. `recipe_spec.json` records the repo-owned stage list,
generated outputs, and command previews for the current action. Meanwhile
`result.json.execution_mode` becomes `recipe-executed`, `recipe-failed`, or
`recipe-timeout`. If the request explicitly includes `execution_command`, the
worker switches to operator-specified command execution and emits
`command-executed`, `command-failed`, or `command-timeout`.

The default recipe runner now executes repo-owned Python scripts under
`workers/python/lifecycle_recipes/` and writes real generated outputs into the
bundle `generated/` directory. For example, training emits
`train_summary.json`, `train_metrics.json`, `checkpoint.stub.json`, and
`train.log`; evaluation emits `evaluation_report.json` and
`metrics_summary.json`; deployment emits `deployment_release.json` and
`serving_manifest.json`.

Agent Runtime prototype:

```powershell
$request = @{
  message_id = "msg_001"
  channel = "qq"
  account_id = "default"
  peer_kind = "direct"
  peer_id = "12345"
  sender_id = "12345"
  text = "/bot-run dry shanghaitech-original"
} | ConvertTo-Json -Compress
python -m agent_runtime.main --request-json $request
```

LocateAnything-3B availability smoke:

```powershell
python -m pip install -r workers\python\requirements-locateanything.txt
python workers\python\agent_worker\locateanything_smoke.py `
  --model-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --data-root data_lake\raw\datasets\shanghaitech\original `
  --output data_lake\catalog\models\nvidia_LocateAnything-3B.smoke.json
```

The smoke verifies local files, dependencies, `AutoConfig`, `AutoProcessor`,
the first safetensors shard, and `AutoModel.from_pretrained`. It does not mark
real ShanghaiTech inference complete unless a dedicated inference harness runs.
