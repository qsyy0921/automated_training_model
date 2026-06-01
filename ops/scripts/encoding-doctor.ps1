param(
  [string]$ProbePath = "README.md"
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$target = Join-Path $repoRoot $ProbePath
if (-not (Test-Path -LiteralPath $target)) {
  throw "ProbePath does not exist: $ProbePath"
}

$console = [Console]::OutputEncoding
$sample = Get-Content -LiteralPath $target -TotalCount 6
$raw = [System.IO.File]::ReadAllText($target, [System.Text.UTF8Encoding]::new($false))
$hasReplacement = $raw.Contains([char]0xfffd)
$mojibakeMarkers = @(
  [string][char]0x9359,
  [string][char]0x93c2,
  [string][char]0x5bb8,
  [string][char]0x6d93,
  [string][char]0x7ecb,
  [string][char]0x93cb
)
$looksMojibake = $false
foreach ($marker in $mojibakeMarkers) {
  if ($raw.Contains($marker)) {
    $looksMojibake = $true
    break
  }
}

Write-Host "PowerShell version: $($PSVersionTable.PSVersion)"
Write-Host "Console output encoding: $($console.WebName) / codepage $($console.CodePage)"
Write-Host "Current code page:"
chcp
Write-Host "Probe file: $ProbePath"
Write-Host "UTF-8 replacement chars found: $hasReplacement"
Write-Host "Common mojibake markers found: $looksMojibake"
Write-Host ""
Write-Host "Preview:"
$sample | ForEach-Object { Write-Host $_ }

if ($hasReplacement -or $looksMojibake) {
  throw "Encoding doctor found suspicious text in $ProbePath"
}

Write-Host ""
Write-Host "Encoding doctor passed."
