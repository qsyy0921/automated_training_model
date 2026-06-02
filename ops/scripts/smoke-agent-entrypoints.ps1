param(
  [string]$Addr = "127.0.0.1:7873",
  [string]$Go = "F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe",
  [string]$MergeRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge",
  [string]$FrameRoot = "F:\keyan\token_compression\data\shanghai\data\testing\frames",
  [string]$MaskRoot = "F:\keyan\token_compression\data\shanghai\data\testframemask",
  [string]$AnnotationRoot = "F:\keyan\token_compression\data\shanghai\new_tracking\merge\annotations_review",
  [string]$RuntimeRoot = "",
  [switch]$UseConfiguredQQOutbound
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
if (-not (Test-Path -LiteralPath $Go)) {
  $Go = "go"
}

if (-not $UseConfiguredQQOutbound) {
  $env:QQ_ONEBOT_OUTBOUND_ENABLED = "false"
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

$baseURL = "http://$Addr"
$tmpDir = Join-Path $repoRoot "tmp"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$out = Join-Path $tmpDir "smoke-agent-entrypoints.out.log"
$err = Join-Path $tmpDir "smoke-agent-entrypoints.err.log"
if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $safeAddr = $Addr.Replace(":", "_").Replace(".", "_")
  $RuntimeRoot = Join-Path $tmpDir "runtime-entrypoints-$safeAddr"
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
      Invoke-RestMethod "$baseURL/healthz" -TimeoutSec 2 | Out-Null
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

  Write-Host "health ok"
  & $Go run .\cmd\labelctl -addr $baseURL runtime status
  & $Go run .\cmd\labelctl -addr $baseURL channel qq test /bot-ping

  $body = @{
    post_type = "message"
    message_type = "private"
    message_id = "smoke-onebot-1"
    user_id = 10001
    message = "/bot-status"
  } | ConvertTo-Json -Depth 5
  Invoke-RestMethod "$baseURL/api/channels/qq/onebot" -Method Post -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 8

  & $Go run .\cmd\agentdesktop -addr $baseURL
  & $Go run .\cmd\labelctl skill draft -id smoke-agent-entrypoints -summary "CLI Web desktop and QQ entrypoint smoke workflow" -draft-root (Join-Path $tmpDir "skill_drafts")
  Write-Host "smoke-agent-entrypoints passed"
} finally {
  Stop-LabelServer -Process $server -ListenAddr $Addr
  Pop-Location
}
