param(
  [string]$Addr = "127.0.0.1:7954",
  [string]$Go = "",
  [string]$DatasetId = "shanghaitech-original",
  [string]$TargetTask = "detection",
  [string]$ModelFamily = "yolo11n",
  [string]$EvaluationModelId = "model-1",
  [string]$EvaluationSplit = "validation",
  [string]$DeploymentModelId = "model-1",
  [string]$DeploymentTarget = "local",
  [string]$DeploymentRuntime = "python-worker",
  [int]$DeploymentReplicas = 2,
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [int]$TimeoutMinutes = 10
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet
. "$PSScriptRoot\resolve-go.ps1"
. "$PSScriptRoot\ensure-smoke-media-fixture.ps1"

function Assert-True {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) {
    throw $Message
  }
}

function Invoke-JSON {
  param([string]$Method = "GET", [string]$Url, [object]$Body = $null, [int]$TimeoutSec = 30)
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec $TimeoutSec
  }
  $json = $Body | ConvertTo-Json -Depth 12
  return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json" -Body $json -TimeoutSec $TimeoutSec
}

function Stop-LabelServer {
  param([object]$Process, [string]$ListenAddr)
  if ($Process -and -not $Process.HasExited) {
    Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
  }
  Get-CimInstance Win32_Process -Filter "name='labelserver.exe'" |
    Where-Object { $_.CommandLine -and $_.CommandLine.Contains("-addr $ListenAddr") } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

function Wait-TaskCompleted {
  param([string]$BaseUrl, [string]$TaskId, [datetime]$Deadline, [string]$Label)
  $task = $null
  while ((Get-Date) -lt $Deadline) {
    $taskResponse = Invoke-JSON -Url ("{0}/api/tasks/{1}" -f $BaseUrl, $TaskId) -TimeoutSec 10
    $task = $taskResponse.task
    if ($task.status -eq "completed") {
      return $task
    }
    if ($task.status -eq "failed") {
      throw "$Label failed: $($task.error)"
    }
    Start-Sleep -Seconds 2
  }
  throw "$Label did not complete before deadline"
}

function Assert-ExecutionLogs {
  param(
    [string]$BaseUrl,
    [string]$TaskId,
    [string]$ExpectedResultKind,
    [string]$ExpectedSummaryNeedle,
    [string]$ExpectedExecutionMode = "recipe-executed"
  )
  $logs = Invoke-JSON -Url ("{0}/api/tasks/{1}/logs" -f $BaseUrl, $TaskId) -TimeoutSec 30
  Assert-True ($logs.status -eq "completed") "task logs did not complete for $TaskId"
  Assert-True ($logs.metadata.dry_run -eq "false") "task metadata.dry_run should be false for $TaskId"
  Assert-True ($logs.worker_heartbeat.status -eq "completed") "task heartbeat not completed for $TaskId"
  Assert-True (($logs.artifacts | Measure-Object).Count -ge 4) "task should emit at least four artifacts for $TaskId"
  Assert-True (-not [string]::IsNullOrWhiteSpace($logs.metadata.artifact_manifest)) "task missing artifact manifest for $TaskId"
  Assert-True (Test-Path -LiteralPath $logs.metadata.artifact_manifest) "artifact manifest path does not exist: $($logs.metadata.artifact_manifest)"
  foreach ($artifact in $logs.artifacts) {
    Assert-True (Test-Path -LiteralPath $artifact.uri) "artifact path does not exist: $($artifact.uri)"
  }
  $resultArtifact = @($logs.artifacts | Where-Object { $_.kind -eq $ExpectedResultKind } | Select-Object -First 1)
  Assert-True ($resultArtifact.Count -eq 1) "task missing result artifact of kind $ExpectedResultKind"
  $resultPayload = Get-Content -LiteralPath $resultArtifact[0].uri -Raw | ConvertFrom-Json
  Assert-True ($resultPayload.execution_mode -eq $ExpectedExecutionMode) "unexpected execution mode: $($resultPayload.execution_mode)"
  Assert-True ($resultPayload.summary -match [regex]::Escape($ExpectedSummaryNeedle)) "unexpected summary: $($resultPayload.summary)"
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$Go = Resolve-Go -Candidate $Go
$mediaRoots = Ensure-SmokeMediaFixture -RepoRoot $repoRoot -MergeRoot $MergeRoot -FrameRoot $FrameRoot -MaskRoot $MaskRoot -AnnotationRoot $AnnotationRoot
$MergeRoot = $mediaRoots.MergeRoot
$FrameRoot = $mediaRoots.FrameRoot
$MaskRoot = $mediaRoots.MaskRoot
$AnnotationRoot = $mediaRoots.AnnotationRoot

$env:AGENT_RUNTIME_PLANNER = "rule"
$env:AGENT_RUNTIME_PYTHON = "python"
$env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$runtimeRoot = Join-Path $tmpDir "runtime-lifecycle-execution-smoke"
if (Test-Path -LiteralPath $runtimeRoot) {
  Remove-Item -LiteralPath $runtimeRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
$out = Join-Path $tmpDir "smoke-lifecycle-execution-worker.out.log"
$err = Join-Path $tmpDir "smoke-lifecycle-execution-worker.err.log"

$serverArgs = @(
  "run", ".\cmd\labelserver",
  "-addr", $Addr,
  "-merge-root", $MergeRoot,
  "-frame-root", $FrameRoot,
  "-mask-root", $MaskRoot,
  "-annotation-root", $AnnotationRoot,
  "-web-root", (Join-Path $repoRoot "web"),
  "-data-root", (Join-Path $repoRoot "data_lake"),
  "-model-root", (Join-Path $repoRoot "data_lake\models"),
  "-agent-root", (Join-Path $repoRoot "data_lake\agents"),
  "-runtime-root", $runtimeRoot
)

Push-Location $repoRoot
try {
  $server = Start-Process -FilePath $Go -ArgumentList $serverArgs -WorkingDirectory $repoRoot -RedirectStandardOutput $out -RedirectStandardError $err -WindowStyle Hidden -PassThru
  $ready = $false
  for ($i = 0; $i -lt 60; $i++) {
    try {
      Invoke-JSON -Url "$baseURL/healthz" -TimeoutSec 2 | Out-Null
      $ready = $true
      break
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  if (-not $ready) {
    Write-Host (Get-Content -LiteralPath $out -Raw)
    Write-Host (Get-Content -LiteralPath $err -Raw)
    throw "labelserver did not become ready at $baseURL"
  }

  $deadline = (Get-Date).AddMinutes($TimeoutMinutes)

  $run = Invoke-JSON -Method "POST" -Url "$baseURL/api/training/runs" -Body @{
    dataset_id = $DatasetId
    target_task = $TargetTask
    model_family = $ModelFamily
    execution_recipe = "default"
    execution_timeout_seconds = 30
    dry_run = $false
  } -TimeoutSec 30
  Assert-True ($null -ne $run.run) "training execution response missing run"
  Assert-True (-not [string]::IsNullOrWhiteSpace($run.run.task_id)) "training execution task did not return task_id"
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $run.run.task_id -Deadline $deadline -Label "training execution task" | Out-Null
  Assert-ExecutionLogs -BaseUrl $baseURL -TaskId $run.run.task_id -ExpectedResultKind "training.run.result" -ExpectedSummaryNeedle "training.run recipe completed: recipe=default exit=0"

  $evaluation = Invoke-JSON -Method "POST" -Url "$baseURL/api/evaluation/runs" -Body @{
    dataset_id = $DatasetId
    model_id = $EvaluationModelId
    split = $EvaluationSplit
    execution_recipe = "default"
    execution_timeout_seconds = 30
    dry_run = $false
  } -TimeoutSec 30
  Assert-True ($null -ne $evaluation.run) "evaluation execution response missing run"
  Assert-True (-not [string]::IsNullOrWhiteSpace($evaluation.run.task_id)) "evaluation execution task did not return task_id"
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $evaluation.run.task_id -Deadline $deadline -Label "evaluation execution task" | Out-Null
  Assert-ExecutionLogs -BaseUrl $baseURL -TaskId $evaluation.run.task_id -ExpectedResultKind "evaluation.run.result" -ExpectedSummaryNeedle "evaluation.run recipe completed: recipe=default exit=0"

  $deployment = Invoke-JSON -Method "POST" -Url "$baseURL/api/deployments" -Body @{
    model_id = $DeploymentModelId
    target = $DeploymentTarget
    runtime = $DeploymentRuntime
    replicas = $DeploymentReplicas
    execution_recipe = "default"
    execution_timeout_seconds = 30
    dry_run = $false
  } -TimeoutSec 30
  Assert-True ($null -ne $deployment.deployment) "deployment execution response missing deployment"
  Assert-True (-not [string]::IsNullOrWhiteSpace($deployment.deployment.task_id)) "deployment execution task did not return task_id"
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $deployment.deployment.task_id -Deadline $deadline -Label "deployment execution task" | Out-Null
  Assert-ExecutionLogs -BaseUrl $baseURL -TaskId $deployment.deployment.task_id -ExpectedResultKind "deployment.run.result" -ExpectedSummaryNeedle "deployment.run recipe completed: recipe=default exit=0"

  Write-Host "smoke-lifecycle-execution-worker passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
