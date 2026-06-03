param(
  [string]$Addr = "127.0.0.1:7950",
  [string]$Go = "",
  [string]$RepoId = "nvidia/LocateAnything-3B",
  [string]$Proxy = "",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [int]$TimeoutMinutes = 30
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

function Resolve-LocalProxy {
  param([string]$ExplicitProxy = "")
  if (-not [string]::IsNullOrWhiteSpace($ExplicitProxy)) {
    return $ExplicitProxy
  }
  if (-not [string]::IsNullOrWhiteSpace($env:HTTPS_PROXY)) {
    return $env:HTTPS_PROXY
  }
  if (-not [string]::IsNullOrWhiteSpace($env:HTTP_PROXY)) {
    return $env:HTTP_PROXY
  }
  foreach ($candidate in @("http://127.0.0.1:7890", "http://127.0.0.1:7897")) {
    try {
      $uri = [Uri]$candidate
      $tcp = New-Object System.Net.Sockets.TcpClient
      try {
        $tcp.Connect($uri.Host, $uri.Port)
        if ($tcp.Connected) {
          return $candidate
        }
      } finally {
        $tcp.Dispose()
      }
    } catch {
      continue
    }
  }
  return ""
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
$env:AGENT_RUNTIME_HF_DOWNLOAD_TIMEOUT_MINUTES = "$TimeoutMinutes"
$resolvedProxy = Resolve-LocalProxy -ExplicitProxy $Proxy
if (-not [string]::IsNullOrWhiteSpace($resolvedProxy)) {
  $env:HTTP_PROXY = $resolvedProxy
  $env:HTTPS_PROXY = $resolvedProxy
}

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "smoke-hf-verify-worker.out.log"
$err = Join-Path $tmpDir "smoke-hf-verify-worker.err.log"

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

  $command = "/bot-verify-hf-job $RepoId"
  $reply = & $Go run .\cmd\labelctl -addr $baseURL runtime send $command | ConvertFrom-Json
  $reply | ConvertTo-Json -Depth 12

  $traces = Invoke-JSON -Url ("{0}/api/runtime/traces?limit=20" -f $baseURL) -TimeoutSec 30
  $trace = @($traces.traces | Where-Object { $_.tool_ids -contains "model.verify_hf" } | Select-Object -First 1)
  Assert-True ($trace.Count -eq 1) "runtime trace missing model.verify_hf"

  $deadline = (Get-Date).AddMinutes($TimeoutMinutes)
  $jobID = ""
  while ((Get-Date) -lt $deadline) {
    $jobs = Invoke-JSON -Url ("{0}/api/runtime/model-jobs?limit=20" -f $baseURL) -TimeoutSec 10
    $job = @($jobs.jobs | Where-Object { $_.kind -eq "model.verify_hf" -and $_.repo_id -eq $RepoId } | Select-Object -First 1)
    if ($job.Count -eq 1) {
      $jobID = $job[0].id
      if ($job[0].status -eq "succeeded") {
        break
      }
      if ($job[0].status -eq "failed") {
        throw "verify worker job failed: $($job[0].error)"
      }
    }
    Start-Sleep -Seconds 10
  }
  Assert-True (-not [string]::IsNullOrWhiteSpace($jobID)) "verify worker job was not created"

  $logs = Invoke-JSON -Url ("{0}/api/runtime/model-jobs/{1}/logs" -f $baseURL, $jobID) -TimeoutSec 30
  Assert-True ($logs.status -eq "succeeded") "verify worker job did not succeed"
  Assert-True ($null -ne $logs.worker_heartbeat) "verify worker logs missing worker heartbeat"
  Assert-True ($logs.worker_heartbeat.status -eq "completed") "verify worker heartbeat not completed"
  Assert-True (($logs.artifacts | Measure-Object).Count -ge 1) "verify worker artifacts missing"

  Write-Host "smoke-hf-verify-worker passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
