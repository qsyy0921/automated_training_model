param(
  [string]$ConfigPath = "C:\Users\10495\Desktop\mimo.txt",
  [switch]$Quiet
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

if (-not (Test-Path -LiteralPath $ConfigPath)) {
  throw "Mimo config file not found: $ConfigPath"
}

$allowed = @(
  "ANTHROPIC_BASE_URL",
  "ANTHROPIC_AUTH_TOKEN",
  "ANTHROPIC_MODEL",
  "ANTHROPIC_DEFAULT_SONNET_MODEL",
  "ANTHROPIC_DEFAULT_OPUS_MODEL",
  "ANTHROPIC_DEFAULT_HAIKU_MODEL",
  "AGENT_RUNTIME_USE_MIMO",
  "AGENT_RUNTIME_MIMO_TIMEOUT_SECONDS",
  "AGENT_RUNTIME_MIMO_FALLBACK",
  "MIMO_DEFAULT_MODEL",
  "MIMO_VISION_MODEL",
  "VLM_MODEL",
  "ANTHROPIC_VISION_MODEL"
)

$loaded = New-Object System.Collections.Generic.List[string]
foreach ($line in Get-Content -LiteralPath $ConfigPath -Encoding UTF8) {
  $trimmed = $line.Trim()
  if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
    continue
  }
  if ($trimmed -match '^\$env:([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"(.*)"\s*$') {
    $name = $Matches[1]
    $value = $Matches[2]
  } elseif ($trimmed -match '^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$') {
    $name = $Matches[1]
    $value = $Matches[2].Trim('"')
  } else {
    continue
  }
  if ($allowed -notcontains $name) {
    continue
  }
  Set-Item -Path "Env:$name" -Value $value
  $loaded.Add($name) | Out-Null
}

if (-not $env:AGENT_RUNTIME_USE_MIMO) {
  $env:AGENT_RUNTIME_USE_MIMO = "true"
  $loaded.Add("AGENT_RUNTIME_USE_MIMO") | Out-Null
}
if (-not $env:AGENT_RUNTIME_MIMO_FALLBACK) {
  $env:AGENT_RUNTIME_MIMO_FALLBACK = "rule"
  $loaded.Add("AGENT_RUNTIME_MIMO_FALLBACK") | Out-Null
}

if (-not $Quiet) {
  $names = $loaded | Sort-Object -Unique
  Write-Host ("Loaded Mimo environment keys: " + ($names -join ", "))
  Write-Host "Secret values were not printed."
}
