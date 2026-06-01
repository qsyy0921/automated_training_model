param(
  [string]$ConfigPath = "C:\Users\10495\Desktop\mimo.txt",
  [string]$Python = "python",
  [string]$Model = ""
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$env:PYTHONPATH = Join-Path $repoRoot "workers\python"

$args = @(
  "-m", "agent_runtime.smoke_mimo",
  "--config", $ConfigPath
)
if ($Model -ne "") {
  $args += @("--model", $Model)
}

Push-Location $repoRoot
try {
  & $Python @args
} finally {
  Pop-Location
}
