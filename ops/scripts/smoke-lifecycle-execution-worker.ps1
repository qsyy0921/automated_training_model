param(
  [string]$Addr = "127.0.0.1:7954",
  [string]$Go = "",
  [string]$DatasetId = "shanghaitech-original",
  [string]$TargetTask = "detection",
  [string]$ModelFamily = "yolo11n",
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

  $run = Invoke-JSON -Method "POST" -Url "$baseURL/api/training/runs" -Body @{
    dataset_id = $DatasetId
    target_task = $TargetTask
    model_family = $ModelFamily
    dry_run = $false
  } -TimeoutSec 30
  Assert-True ($null -ne $run.run) "training execution response missing run"
  Assert-True (-not [string]::IsNullOrWhiteSpace($run.run.task_id)) "training execution task did not return task_id"

  $deadline = (Get-Date).AddMinutes($TimeoutMinutes)
  $task = $null
  while ((Get-Date) -lt $deadline) {
    $taskResponse = Invoke-JSON -Url ("{0}/api/tasks/{1}" -f $baseURL, $run.run.task_id) -TimeoutSec 10
    $task = $taskResponse.task
    if ($task.status -eq "completed") {
      break
    }
    if ($task.status -eq "failed") {
      throw "training execution task failed: $($task.error)"
    }
    Start-Sleep -Seconds 2
  }
  Assert-True ($null -ne $task) "training execution task was not readable"
  Assert-True ($task.status -eq "completed") "training execution task did not complete"

  $logs = Invoke-JSON -Url ("{0}/api/tasks/{1}/logs" -f $baseURL, $run.run.task_id) -TimeoutSec 30
  Assert-True ($logs.status -eq "completed") "training execution task logs did not complete"
  Assert-True ($logs.metadata.dry_run -eq "false") "training execution metadata.dry_run should be false"
  Assert-True ($logs.worker_heartbeat.status -eq "completed") "training execution heartbeat not completed"
  Assert-True (($logs.artifacts | Measure-Object).Count -eq 3) "training execution should emit three artifacts"
  Assert-True (-not [string]::IsNullOrWhiteSpace($logs.metadata.artifact_manifest)) "training execution missing artifact manifest"
  Assert-True (Test-Path -LiteralPath $logs.metadata.artifact_manifest) "artifact manifest path does not exist: $($logs.metadata.artifact_manifest)"

  foreach ($artifact in $logs.artifacts) {
    Assert-True (Test-Path -LiteralPath $artifact.uri) "artifact path does not exist: $($artifact.uri)"
  }

  $resultArtifact = @($logs.artifacts | Where-Object { $_.kind -eq "training.run.result" } | Select-Object -First 1)
  Assert-True ($resultArtifact.Count -eq 1) "training execution missing result artifact"
  $resultPayload = Get-Content -LiteralPath $resultArtifact[0].uri -Raw | ConvertFrom-Json
  Assert-True ($resultPayload.execution_mode -eq "materialized-recipe") "unexpected execution mode: $($resultPayload.execution_mode)"
  Assert-True ($resultPayload.request.dataset_id -eq $DatasetId) "result payload dataset_id mismatch"

  Write-Host "smoke-lifecycle-execution-worker passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
