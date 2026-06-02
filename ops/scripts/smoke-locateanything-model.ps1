param(
  [string]$Addr = "127.0.0.1:7940",
  [string]$Go = "",
  [string]$ConfigPath = "C:\Users\10495\Desktop\mimo.txt",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [string]$ShanghaiTechRoot = "F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original"
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
Assert-True (Test-Path -LiteralPath $ConfigPath) "Mimo config does not exist: $ConfigPath"
Assert-True (Test-Path -LiteralPath $ShanghaiTechRoot) "ShanghaiTech root does not exist: $ShanghaiTechRoot"

. "$PSScriptRoot\load-mimo-env.ps1" -ConfigPath $ConfigPath -Quiet
$env:AGENT_RUNTIME_PLANNER = "python"
$env:AGENT_RUNTIME_PYTHON = "python"
$env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
$env:AGENT_RUNTIME_PLANNER_TIMEOUT_SECONDS = "180"
$env:AGENT_RUNTIME_MIMO_TIMEOUT_SECONDS = "120"
$env:AGENT_RUNTIME_HF_DOWNLOAD_TIMEOUT_MINUTES = "120"
$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "smoke-locateanything-model.out.log"
$err = Join-Path $tmpDir "smoke-locateanything-model.err.log"
$smokeReport = Join-Path $repoRoot "data_lake\catalog\models\nvidia_LocateAnything-3B.smoke.json"

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

  $message = "Run a dry-run ShanghaiTech LocateAnything-3B smoke test with data_root=$ShanghaiTechRoot"
  $reply = & $Go run .\cmd\labelctl -addr $baseURL runtime send $message | ConvertFrom-Json
  $reply | ConvertTo-Json -Depth 12

  $traces = Invoke-JSON -Url ("{0}/api/runtime/traces?limit=20" -f $baseURL) -TimeoutSec 30
  $trace = @($traces.traces | Where-Object { $_.tool_ids -contains "model.smoke_locateanything" } | Select-Object -First 1)
  Assert-True ($trace.Count -eq 1) "runtime trace missing model.smoke_locateanything"
  Assert-True ($trace[0].tool_ids -contains "model.verify_hf") "runtime trace missing model.verify_hf"
  Assert-True ($trace[0].tool_ids -contains "workflow.submit_run") "runtime trace missing workflow.submit_run"
  Assert-True ($trace[0].metadata.model_load -eq "true") "LocateAnything smoke did not load model"
  Assert-True ($trace[0].metadata.real_inference -eq "false") "real inference should remain false in CPU-only smoke"
  Assert-True (Test-Path -LiteralPath $smokeReport) "smoke report was not written: $smokeReport"

  $report = Get-Content -LiteralPath $smokeReport -Raw | ConvertFrom-Json
  Assert-True ($report.completed.model_load -eq $true) "report did not mark model_load=true"
  Assert-True ($report.completed.real_inference -eq $false) "report should mark real_inference=false"
  Assert-True ($report.checks.data_root.expected_shanghaitech_splits.training -eq $true) "report missing ShanghaiTech training split"
  Assert-True ($report.checks.data_root.expected_shanghaitech_splits.testing -eq $true) "report missing ShanghaiTech testing split"

  Write-Host "smoke-locateanything-model passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
