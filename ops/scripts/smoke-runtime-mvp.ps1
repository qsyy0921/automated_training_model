param(
  [string]$Addr = "127.0.0.1:7910",
  [string]$Go = "F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [string]$ShanghaiTechRoot = "F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original",
  [switch]$UseMimoPlanner
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )
  if (-not $Condition) {
    throw $Message
  }
}

function Invoke-JSON {
  param(
    [string]$Method = "GET",
    [string]$Url,
    [object]$Body = $null
  )
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec 10
  }
  $json = $Body | ConvertTo-Json -Depth 12
  return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json" -Body $json -TimeoutSec 10
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
if (-not (Test-Path -LiteralPath $Go)) {
  $Go = "go"
}
Assert-True (Test-Path -LiteralPath $ShanghaiTechRoot) "ShanghaiTech root does not exist: $ShanghaiTechRoot"

if ($UseMimoPlanner) {
  . "$PSScriptRoot\load-mimo-env.ps1" -Quiet
  $env:AGENT_RUNTIME_PLANNER = "python"
  $env:AGENT_RUNTIME_PYTHON = "python"
  $env:AGENT_RUNTIME_PYTHONPATH = Join-Path $repoRoot "workers\python"
} else {
  $env:AGENT_RUNTIME_PLANNER = "rule"
}

$env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"
$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "smoke-runtime-mvp.out.log"
$err = Join-Path $tmpDir "smoke-runtime-mvp.err.log"

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
  $chat = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $chatBody
  Assert-True ($chat.reply.text -match "planner-agent") "free text did not route to planner-agent"

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
  $vision = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $visionBody
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
  $data = Invoke-JSON -Method "POST" -Url "$baseURL/api/channels/qq/test-message" -Body $dataBody
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

  Write-Host "smoke-runtime-mvp passed"
} finally {
  if ($server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -Force
  }
  Pop-Location
}
