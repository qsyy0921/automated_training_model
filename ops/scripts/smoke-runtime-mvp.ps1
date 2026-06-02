param(
  [string]$Addr = "127.0.0.1:7910",
  [string]$Go = "",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [string]$ShanghaiTechRoot = "F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original",
  [string]$RuntimeRoot = "",
  [switch]$UseMimoPlanner
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet
. "$PSScriptRoot\resolve-go.ps1"
. "$PSScriptRoot\ensure-smoke-media-fixture.ps1"

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )
  if (-not $Condition) {
    throw $Message
  }
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

function Invoke-JSON {
  param(
    [string]$Method = "GET",
    [string]$Url,
    [object]$Body = $null,
    [int]$TimeoutSec = 10
  )
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec $TimeoutSec
  }
  $json = $Body | ConvertTo-Json -Depth 12
  return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json" -Body $json -TimeoutSec $TimeoutSec
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$Go = Resolve-Go -Candidate $Go
$mediaRoots = Ensure-SmokeMediaFixture -RepoRoot $repoRoot -MergeRoot $MergeRoot -FrameRoot $FrameRoot -MaskRoot $MaskRoot -AnnotationRoot $AnnotationRoot
$MergeRoot = $mediaRoots.MergeRoot
$FrameRoot = $mediaRoots.FrameRoot
$MaskRoot = $mediaRoots.MaskRoot
$AnnotationRoot = $mediaRoots.AnnotationRoot
Assert-True (Test-Path -LiteralPath $ShanghaiTechRoot) "ShanghaiTech root does not exist: $ShanghaiTechRoot"

if ($UseMimoPlanner) {
  . "$PSScriptRoot\load-mimo-env.ps1" -Quiet
  $env:AGENT_RUNTIME_PLANNER = "python"
  $env:AGENT_RUNTIME_PYTHON = "python"
  $env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
  $env:AGENT_RUNTIME_PLANNER_TIMEOUT_SECONDS = "180"
  $env:AGENT_RUNTIME_MIMO_TIMEOUT_SECONDS = "120"
} else {
  $env:AGENT_RUNTIME_PLANNER = "rule"
}
$plannerTimeoutSec = if ($UseMimoPlanner) { 120 } else { 10 }

$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"
$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "smoke-runtime-mvp.out.log"
$err = Join-Path $tmpDir "smoke-runtime-mvp.err.log"
if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $safeAddr = $Addr.Replace(":", "_").Replace(".", "_")
  $RuntimeRoot = Join-Path $tmpDir "runtime-smoke-$safeAddr"
}
if (Test-Path -LiteralPath $RuntimeRoot) {
  Remove-Item -LiteralPath $RuntimeRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $RuntimeRoot | Out-Null

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
  "-runtime-root", $RuntimeRoot
)

Push-Location $repoRoot
try {
  $server = Start-Process -FilePath $Go -ArgumentList $serverArgs -WorkingDirectory $repoRoot -RedirectStandardOutput $out -RedirectStandardError $err -WindowStyle Hidden -PassThru
  $ready = $false
  for ($i = 0; $i -lt 40; $i++) {
    try {
      Invoke-JSON -Url "$baseURL/healthz" | Out-Null
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

  $status = Invoke-JSON -Url "$baseURL/api/runtime/status"
  Assert-True ($status.runtime.runtime -eq "automated-training-agent-runtime") "runtime status did not return Agent Runtime"
  Assert-True (@($status.runtime.entry_points | Where-Object { $_.id -eq "qq" }).Count -eq 1) "runtime status missing QQ entry point"

  & $Go run .\cmd\labelctl -addr $baseURL runtime status | Out-Host
  & $Go run .\cmd\labelctl -addr $baseURL runtime model-jobs | Out-Host

  $ping = & $Go run .\cmd\labelctl -addr $baseURL runtime send /bot-ping | ConvertFrom-Json
  Assert-True ($ping.reply.text -eq "pong") "CLI runtime send /bot-ping did not return pong"

  $chatBody = @{
    id = "smoke-chat-1"
    channel = "qq"
    account_id = "default"
    peer = @{ channel = "qq"; account_id = "default"; kind = "direct"; id = "smoke-chat" }
    sender_id = "smoke-chat"
    text = "请帮我规划一个从数据接入到训练评估的 dry-run"
  }
  $chat = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $chatBody -TimeoutSec $plannerTimeoutSec
  if ($UseMimoPlanner) {
    Assert-True ($chat.reply.text -notmatch "回退") "Mimo planner fell back for free text: $($chat.reply.text)"
  } else {
    Assert-True ($chat.reply.text -match "planner-agent") "free text did not route to planner-agent"
  }

  $visionBody = @{
    id = "smoke-vision-1"
    channel = "qq"
    account_id = "default"
    peer = @{ channel = "qq"; account_id = "default"; kind = "group"; id = "smoke-group" }
    sender_id = "smoke-user"
    text = "请检查这张异常帧"
    attachments = @(
      @{ id = "att-image-1"; name = "frame_001.png"; media_type = "image/png"; source_uri = "qq://smoke/frame_001.png"; status = "received" }
    )
  }
  $vision = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $visionBody -TimeoutSec $plannerTimeoutSec
  Assert-True ($vision.reply.text -match "vision-agent") "image attachment did not route to vision-agent"

  $dataBody = @{
    id = "smoke-data-1"
    channel = "qq"
    account_id = "default"
    peer = @{ channel = "qq"; account_id = "default"; kind = "direct"; id = "smoke-data" }
    sender_id = "smoke-data"
    text = "请为 ShanghaiTech original 创建数据接入计划"
    attachments = @(
      @{ id = "att-data-1"; name = "shanghaitech-original.manifest"; media_type = "application/x-directory"; source_uri = $ShanghaiTechRoot; status = "received" }
    )
  }
  $data = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $dataBody -TimeoutSec $plannerTimeoutSec
  Assert-True ($data.reply.text -match "data-intake-agent") "data attachment did not route to data-intake-agent"

  $onebotBody = @{
    post_type = "message"
    message_type = "private"
    message_id = "smoke-onebot-status"
    user_id = 10001
    message = "/bot-status"
  }
  $onebot = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/onebot" -Body $onebotBody
  Assert-True ($onebot.onebot_reply.action -eq "send_msg") "OneBot response did not build send_msg"

  $desktop = Invoke-JSON -Url "$baseURL/api/desktop/status"
  Assert-True ($desktop.desktop.runtime -eq "automated-training-agent-runtime") "desktop status did not reuse runtime"

  $jobs = Invoke-JSON -Url "$baseURL/api/runtime/model-jobs"
  Assert-True ($null -ne $jobs.jobs) "runtime model jobs endpoint missing jobs"

  $sessions = Invoke-JSON -Url "$baseURL/api/runtime/sessions"
  $traces = Invoke-JSON -Url "$baseURL/api/runtime/traces?limit=30"
  Assert-True (@($sessions.sessions).Count -ge 4) "expected at least four runtime sessions"
  Assert-True (@($traces.traces | Where-Object { $_.agent_id -eq "planner-agent" }).Count -ge 1) "trace missing planner-agent"
  Assert-True (@($traces.traces | Where-Object { $_.agent_id -eq "vision-agent" }).Count -ge 1) "trace missing vision-agent"
  Assert-True (@($traces.traces | Where-Object { $_.agent_id -eq "data-intake-agent" }).Count -ge 1) "trace missing data-intake-agent"
  Assert-True (@($traces.traces | Where-Object { $_.tool_ids -contains "vlm.inspect" }).Count -ge 1) "trace missing vlm.inspect tool"
  $dataTrace = @($traces.traces | Where-Object { $_.tool_ids -contains "intake.plan" } | Select-Object -First 1)
  Assert-True ($dataTrace.Count -eq 1) "trace missing intake.plan tool"
  Assert-True ($dataTrace[0].metadata.dataset_name -eq "shanghaitech-original") "intake.plan trace missing ShanghaiTech dataset metadata"
  Assert-True ($dataTrace[0].metadata.source_uri -eq $ShanghaiTechRoot) "intake.plan trace missing ShanghaiTech source uri"

  Stop-LabelServer -Process $server -ListenAddr $Addr
  $server = Start-Process -FilePath $Go -ArgumentList $serverArgs -WorkingDirectory $repoRoot -RedirectStandardOutput $out -RedirectStandardError $err -WindowStyle Hidden -PassThru
  $ready = $false
  for ($i = 0; $i -lt 40; $i++) {
    try {
      Invoke-JSON -Url "$baseURL/healthz" | Out-Null
      $ready = $true
      break
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  Assert-True $ready "labelserver did not restart for runtime persistence check"
  $restoredSessions = Invoke-JSON -Url "$baseURL/api/runtime/sessions"
  $restoredTraces = Invoke-JSON -Url "$baseURL/api/runtime/traces?limit=30"
  Assert-True (@($restoredSessions.sessions).Count -ge 4) "runtime sessions did not persist across restart"
  Assert-True (@($restoredTraces.traces | Where-Object { $_.tool_ids -contains "intake.plan" }).Count -ge 1) "runtime traces did not persist across restart"

  Write-Host "smoke-runtime-mvp passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
