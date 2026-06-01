# Python Agent Workers

This directory is the execution-layer boundary for model and data jobs.

The Go backend owns control-plane state: agent registry, tool registry, workflow
definitions, audit events, and task scheduling. Python workers own GPU-heavy or
research-heavy execution: tracking, segmentation, VLM grounding, training,
evaluation, and report materialization.

The current worker is intentionally small. It accepts a JSON job envelope and
prints a JSON result, so it can later be launched by a local process runner,
Docker worker, NATS consumer, or Kubernetes job without changing the Go domain
model.

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
