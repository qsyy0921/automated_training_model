param(
  [Parameter(Mandatory = $true)]
  [string]$DatasetId,

  [Parameter(Mandatory = $true)]
  [string]$SourceRoot,

  [string]$DataLakeRoot = "data_lake"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..\..")
. (Join-Path $repoRoot "ops\scripts\utf8.ps1") -Quiet

if (-not (Test-Path -LiteralPath $SourceRoot)) {
  throw "SourceRoot does not exist: $SourceRoot"
}

if ($DatasetId -notmatch "^[a-zA-Z0-9_.-]+$") {
  throw "DatasetId may only contain letters, digits, dot, underscore, and hyphen."
}

$target = Join-Path $DataLakeRoot (Join-Path "raw\datasets" (Join-Path $DatasetId "original"))
$catalogDir = Join-Path $DataLakeRoot "catalog\datasets"
$catalogPath = Join-Path $catalogDir "$DatasetId.json"

New-Item -ItemType Directory -Force -Path $target, $catalogDir | Out-Null

robocopy $SourceRoot $target /E /R:2 /W:2 /NFL /NDL /NP
$code = $LASTEXITCODE
if ($code -ge 8) {
  throw "robocopy failed with exit code $code"
}

$files = Get-ChildItem -LiteralPath $target -Recurse -File
$catalog = [ordered]@{
  id = $DatasetId
  source_root = (Resolve-Path -LiteralPath $SourceRoot).Path
  lake_root = (Resolve-Path -LiteralPath $target).Path
  file_count = @($files).Count
  bytes = ($files | Measure-Object -Property Length -Sum).Sum
  created_at = (Get-Date).ToUniversalTime().ToString("o")
  layout = "raw/datasets/<dataset-id>/original"
}

$catalog | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $catalogPath -Encoding UTF8
Write-Host "Dataset copied to: $target"
Write-Host "Catalog written to: $catalogPath"
