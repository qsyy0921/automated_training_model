param(
  [string]$Addr = "127.0.0.1:7955",
  [string]$Go = "",
  [string]$DatasetId = "shanghaitech-original",
  [string]$TargetTask = "detection",
  [string]$ModelFamily = "yolo11n",
  [string]$EvaluationModelId = "model-1",
  [string]$DeploymentModelId = "model-1",
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

function Assert-TaskExecutionMode {
  param([string]$BaseUrl, [string]$TaskId)
  $logs = Invoke-JSON -Url ("{0}/api/tasks/{1}/logs" -f $BaseUrl, $TaskId) -TimeoutSec 30
  Assert-True ($logs.status -eq "completed") "task logs did not complete for $TaskId"
  $resultArtifact = @($logs.artifacts | Where-Object { $_.kind -like "*.result" } | Select-Object -First 1)
  Assert-True ($resultArtifact.Count -eq 1) "missing result artifact for $TaskId"
  $payload = Get-Content -LiteralPath $resultArtifact[0].uri -Raw | ConvertFrom-Json
  Assert-True ($payload.execution_mode -eq "recipe-executed") "unexpected execution_mode: $($payload.execution_mode)"
}

function Run-LabelctlJson {
  param([string]$Exe, [string[]]$CommandArgs)
  $output = & $Exe @CommandArgs
  return ($output | Out-String | ConvertFrom-Json)
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$Go = Resolve-Go -Candidate $Go
$mediaRoots = Ensure-SmokeMediaFixture -RepoRoot $repoRoot -MergeRoot $MergeRoot -FrameRoot $FrameRoot -MaskRoot $MaskRoot -AnnotationRoot $AnnotationRoot

$env:AGENT_RUNTIME_PLANNER = "rule"
$env:AGENT_RUNTIME_PYTHON = "python"
$env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$labelctlExe = Join-Path $tmpDir "labelctl-smoke.exe"
$runtimeRoot = Join-Path $tmpDir "runtime-lifecycle-cli-execution-smoke"
if (Test-Path -LiteralPath $runtimeRoot) {
  Remove-Item -LiteralPath $runtimeRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
$out = Join-Path $tmpDir "smoke-lifecycle-cli-execution-worker.out.log"
$err = Join-Path $tmpDir "smoke-lifecycle-cli-execution-worker.err.log"

$serverArgs = @(
  "run", ".\cmd\labelserver",
  "-addr", $Addr,
  "-merge-root", $mediaRoots.MergeRoot,
  "-frame-root", $mediaRoots.FrameRoot,
  "-mask-root", $mediaRoots.MaskRoot,
  "-annotation-root", $mediaRoots.AnnotationRoot,
  "-web-root", (Join-Path $repoRoot "web"),
  "-data-root", (Join-Path $repoRoot "data_lake"),
  "-model-root", (Join-Path $repoRoot "data_lake\models"),
  "-agent-root", (Join-Path $repoRoot "data_lake\agents"),
  "-runtime-root", $runtimeRoot
)

Push-Location $repoRoot
try {
  & $Go build -o $labelctlExe .\cmd\labelctl
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

  $train = Run-LabelctlJson -Exe $labelctlExe -CommandArgs @(
    "-addr", $baseURL,
    "training", "submit",
    "-dataset", $DatasetId,
    "-target-task", $TargetTask,
    "-model-family", $ModelFamily,
    "-exec-recipe", "default",
    "-exec-timeout", "30"
  )
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $train.run.task_id -Deadline $deadline -Label "training CLI task" | Out-Null
  Assert-TaskExecutionMode -BaseUrl $baseURL -TaskId $train.run.task_id

  $eval = Run-LabelctlJson -Exe $labelctlExe -CommandArgs @(
    "-addr", $baseURL,
    "evaluation", "submit",
    "-dataset", $DatasetId,
    "-model", $EvaluationModelId,
    "-exec-recipe", "default",
    "-exec-timeout", "30"
  )
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $eval.run.task_id -Deadline $deadline -Label "evaluation CLI task" | Out-Null
  Assert-TaskExecutionMode -BaseUrl $baseURL -TaskId $eval.run.task_id

  $deploy = Run-LabelctlJson -Exe $labelctlExe -CommandArgs @(
    "-addr", $baseURL,
    "deploy", "submit",
    "-model", $DeploymentModelId,
    "-target", "local",
    "-dry-run=false",
    "-exec-recipe", "default",
    "-exec-timeout", "30"
  )
  Wait-TaskCompleted -BaseUrl $baseURL -TaskId $deploy.deployment.task_id -Deadline $deadline -Label "deployment CLI task" | Out-Null
  Assert-TaskExecutionMode -BaseUrl $baseURL -TaskId $deploy.deployment.task_id

  Write-Host "smoke-lifecycle-cli-execution-worker passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
