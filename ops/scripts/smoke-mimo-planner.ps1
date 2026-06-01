param(
  [string]$ConfigPath = "C:\Users\10495\Desktop\mimo.txt"
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$env:PYTHONPATH = Join-Path $repoRoot "workers\python"

python -m agent_runtime.smoke_planner --config $ConfigPath
if ($LASTEXITCODE -ne 0) {
  throw "smoke-mimo-planner failed"
}

Write-Host "smoke-mimo-planner passed"
