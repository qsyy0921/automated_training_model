param(
  [string]$Addr = "127.0.0.1:7955",
  [string]$Go = "",
  [string]$DatasetId = "shanghaitech-original",
  [string]$TargetTask = "detection",
  [string]$ModelFamily = "yolo11n",
  [string]$EvaluationModelId = "model-1",
  [string]$EvaluationSplit = "validation",
  [string]$DeploymentModelId = "model-1",
  [string]$DeploymentTarget = "local-dry-run",
  [string]$DeploymentRuntime = "python-worker",
  [string]$DeploymentReplicas = "2",
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

function Wait-ModelJob {
  param([string]$BaseUrl, [string]$Kind, [hashtable]$MetadataMatch, [datetime]$Deadline)
  $job = $null
  while ((Get-Date) -lt $Deadline) {
    $jobs = Invoke-JSON -Url ("{0}/api/runtime/model-jobs?limit=50" -f $BaseUrl) -TimeoutSec 10
    $job = @($jobs.jobs | Where-Object {
      if ($_.kind -ne $Kind) { return $false }
      foreach ($key in $MetadataMatch.Keys) {
        if ($_.metadata.$key -ne $MetadataMatch[$key]) { return $false }
      }
      return $true
    } | Select-Object -First 1)
    if ($job.Count -eq 1) {
      if ($job[0].status -eq "succeeded") {
        return $job[0]
      }
      if ($job[0].status -eq "failed") {
        throw "$Kind worker job failed: $($job[0].error)"
      }
    }
    Start-Sleep -Seconds 2
  }
  throw "$Kind worker job did not complete before deadline"
}

function Assert-ExecutionLogs {
  param([string]$BaseUrl, [string]$JobId, [string]$ExpectedKind, [string]$ExpectedNeedle)
  $logs = Invoke-JSON -Url ("{0}/api/runtime/model-jobs/{1}/logs" -f $BaseUrl, $JobId) -TimeoutSec 30
  Assert-True ($logs.status -eq "succeeded") "model job did not succeed: $JobId"
  Assert-True ($logs.metadata.dry_run -eq "false") "model job metadata.dry_run should be false: $JobId"
  Assert-True ($logs.metadata.execution_recipe -eq "default") "model job missing execution_recipe=default: $JobId"
  Assert-True ($null -ne $logs.worker_heartbeat) "model job missing worker heartbeat: $JobId"
  Assert-True ($logs.worker_heartbeat.status -eq "completed") "model job heartbeat not completed: $JobId"
  Assert-True (($logs.artifacts | Measure-Object).Count -ge 5) "model job should emit at least five artifacts: $JobId"
  $specArtifact = @($logs.artifacts | Where-Object { $_.kind -like "*.recipe_spec" } | Select-Object -First 1)
  Assert-True ($specArtifact.Count -eq 1) "model job missing recipe spec artifact"
  Assert-True (Test-Path -LiteralPath $specArtifact[0].uri) "recipe spec path does not exist: $($specArtifact[0].uri)"
  $resultArtifact = @($logs.artifacts | Where-Object { $_.kind -eq $ExpectedKind } | Select-Object -First 1)
  Assert-True ($resultArtifact.Count -eq 1) "model job missing result artifact $ExpectedKind"
  Assert-True (Test-Path -LiteralPath $resultArtifact[0].uri) "result artifact path does not exist: $($resultArtifact[0].uri)"
  $resultPayload = Get-Content -LiteralPath $resultArtifact[0].uri -Raw | ConvertFrom-Json
  Assert-True ($resultPayload.execution_mode -eq "recipe-executed") "unexpected execution mode for ${JobId}: $($resultPayload.execution_mode)"
  Assert-True (Test-Path -LiteralPath $resultPayload.recipe_spec_path) "result payload recipe_spec_path does not exist: $($resultPayload.recipe_spec_path)"
  Assert-True ($resultPayload.summary -match [regex]::Escape($ExpectedNeedle)) "unexpected result summary for ${JobId}: $($resultPayload.summary)"
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
$runtimeRoot = Join-Path $tmpDir "runtime-execution-worker-smoke"
if (Test-Path -LiteralPath $runtimeRoot) {
  Remove-Item -LiteralPath $runtimeRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
$out = Join-Path $tmpDir "smoke-runtime-execution-worker.out.log"
$err = Join-Path $tmpDir "smoke-runtime-execution-worker.err.log"

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

  $trainReply = & $Go run .\cmd\labelctl -addr $baseURL runtime send "/bot-train-run $DatasetId $TargetTask $ModelFamily" | ConvertFrom-Json
  $trainReply | ConvertTo-Json -Depth 12
  $evalReply = & $Go run .\cmd\labelctl -addr $baseURL runtime send "/bot-eval-run $DatasetId $EvaluationModelId $EvaluationSplit" | ConvertFrom-Json
  $evalReply | ConvertTo-Json -Depth 12
  $deployReply = & $Go run .\cmd\labelctl -addr $baseURL runtime send "/bot-deploy-run $DeploymentModelId $DeploymentTarget $DeploymentRuntime $DeploymentReplicas" | ConvertFrom-Json
  $deployReply | ConvertTo-Json -Depth 12

  $traces = Invoke-JSON -Url ("{0}/api/runtime/traces?limit=20" -f $baseURL) -TimeoutSec 30
  Assert-True (@($traces.traces | Where-Object { $_.tool_ids -contains "training.run" }).Count -ge 1) "runtime trace missing training.run"
  Assert-True (@($traces.traces | Where-Object { $_.tool_ids -contains "evaluation.run" }).Count -ge 1) "runtime trace missing evaluation.run"
  Assert-True (@($traces.traces | Where-Object { $_.tool_ids -contains "deployment.run" }).Count -ge 1) "runtime trace missing deployment.run"

  $trainJob = Wait-ModelJob -BaseUrl $baseURL -Kind "training.run" -MetadataMatch @{ dataset_id = $DatasetId; dry_run = "false" } -Deadline $deadline
  Assert-ExecutionLogs -BaseUrl $baseURL -JobId $trainJob.id -ExpectedKind "training.run.result" -ExpectedNeedle "training.run recipe completed: recipe=default exit=0"

  $evalJob = Wait-ModelJob -BaseUrl $baseURL -Kind "evaluation.run" -MetadataMatch @{ dataset_id = $DatasetId; model_id = $EvaluationModelId; dry_run = "false" } -Deadline $deadline
  Assert-ExecutionLogs -BaseUrl $baseURL -JobId $evalJob.id -ExpectedKind "evaluation.run.result" -ExpectedNeedle "evaluation.run recipe completed: recipe=default exit=0"

  $deployJob = Wait-ModelJob -BaseUrl $baseURL -Kind "deployment.run" -MetadataMatch @{ model_id = $DeploymentModelId; target = $DeploymentTarget; dry_run = "false" } -Deadline $deadline
  Assert-ExecutionLogs -BaseUrl $baseURL -JobId $deployJob.id -ExpectedKind "deployment.run.result" -ExpectedNeedle "deployment.run recipe completed: recipe=default exit=0"

  Write-Host "smoke-runtime-execution-worker passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
