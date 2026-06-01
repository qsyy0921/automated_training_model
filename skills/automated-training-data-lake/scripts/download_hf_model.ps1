param(
  [Parameter(Mandatory = $true)]
  [string]$RepoId,

  [string]$DestinationRoot = "data_lake\models\artifacts\huggingface",

  [switch]$PullLFS
)

$ErrorActionPreference = "Stop"

if ($RepoId -notmatch "^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$") {
  throw "RepoId must look like org/repo, got: $RepoId"
}

$parts = $RepoId.Split("/")
$target = Join-Path (Join-Path $DestinationRoot $parts[0]) $parts[1]
$repoUrl = "https://huggingface.co/$RepoId"

if (Test-Path -LiteralPath $target) {
  Write-Host "Target already exists: $target"
  exit 0
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null

if ($PullLFS) {
  git clone $repoUrl $target
  git -C $target lfs pull
} else {
  $env:GIT_LFS_SKIP_SMUDGE = "1"
  git clone $repoUrl $target
}

Write-Host "Model repository stored at: $target"
