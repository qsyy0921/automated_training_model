param(
  [string]$Addr = "127.0.0.1:7920",
  [string]$Go = "F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe",
  [string]$ConfigPath = "C:\Users\10495\Desktop\mimo.txt",
  [string]$RepoId = "nvidia/LocateAnything-3B",
  [string]$Proxy = "http://127.0.0.1:7890",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [switch]$StartDownload,
  [switch]$WaitForCompletion,
  [int]$TimeoutMinutes = 480
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

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

if ($StartDownload -and -not $WaitForCompletion) {
  throw "Real downloads must use -WaitForCompletion so this script does not leave an unmanaged server/job running."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
if (-not (Test-Path -LiteralPath $Go)) {
  $Go = "go"
}
Assert-True (Test-Path -LiteralPath $ConfigPath) "Mimo config does not exist: $ConfigPath"

. "$PSScriptRoot\load-mimo-env.ps1" -ConfigPath $ConfigPath -Quiet
$env:AGENT_RUNTIME_PLANNER = "python"
$env:AGENT_RUNTIME_PYTHON = "python"
$env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
$env:AGENT_RUNTIME_PLANNER_TIMEOUT_SECONDS = "180"
$env:AGENT_RUNTIME_HF_DOWNLOAD_TIMEOUT_MINUTES = "$TimeoutMinutes"
$env:HTTP_PROXY = $Proxy
$env:HTTPS_PROXY = $Proxy
$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"
if ($StartDownload) {
  $env:AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL = "false"
} else {
  $env:AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL = "true"
}

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "runtime-hf-install.out.log"
$err = Join-Path $tmpDir "runtime-hf-install.err.log"

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
  "-agent-root", (Join-Path $repoRoot "data_lake\agents")
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

  $message = "Install $RepoId into data_lake/models/artifacts/huggingface through Agent Runtime. Do not commit model weights to Git."
  $reply = & $Go run .\cmd\labelctl -addr $baseURL runtime send $message | ConvertFrom-Json
  $reply | ConvertTo-Json -Depth 12

  $traces = Invoke-JSON -Url "${baseURL}/api/runtime/traces?limit=20"
  $downloadTrace = @($traces.traces | Where-Object { $_.tool_ids -contains "model.download_hf" } | Select-Object -First 1)
  Assert-True ($downloadTrace.Count -eq 1) "runtime did not produce a model.download_hf trace"

  if (-not $StartDownload) {
    Assert-True ($downloadTrace[0].status -eq "approval_required") "preflight should stop at approval_required"
    Write-Host "runtime-hf-install preflight passed"
    return
  }

  $jobs = Invoke-JSON -Url "${baseURL}/api/runtime/model-jobs"
  $job = @($jobs.jobs | Where-Object { $_.repo_id -eq $RepoId } | Select-Object -First 1)
  Assert-True ($job.Count -eq 1) "real download did not enqueue a model job"
  $jobID = $job[0].id
  Write-Host "download job queued: $jobID"

  $deadline = (Get-Date).AddMinutes($TimeoutMinutes)
  while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 15
    $jobs = Invoke-JSON -Url "${baseURL}/api/runtime/model-jobs" -TimeoutSec 10
    $job = @($jobs.jobs | Where-Object { $_.id -eq $jobID } | Select-Object -First 1)
    if ($job.Count -ne 1) {
      throw "job disappeared: $jobID"
    }
    Write-Host "job $jobID status=$($job[0].status) message=$($job[0].message)"
    if ($job[0].status -eq "succeeded") {
      Write-Host "runtime-hf-install download completed"
      return
    }
    if ($job[0].status -eq "failed") {
      throw "download failed: $($job[0].error)"
    }
  }
  throw "download did not finish before timeout: $TimeoutMinutes minutes"
} finally {
  if ($server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -Force
  }
  Pop-Location
}
