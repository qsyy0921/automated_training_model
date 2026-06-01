$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$crateRoot = Join-Path $repoRoot "crates\tracking-math"
$outDir = Join-Path $repoRoot "web\src\shared\wasm\pkg"
$targetRoot = if ($env:ATM_CARGO_TARGET_DIR) {
    $env:ATM_CARGO_TARGET_DIR
} else {
    Join-Path ([System.IO.Path]::GetTempPath()) "automated_training_model\cargo-target"
}
$targetDir = Join-Path $targetRoot "tracking-math"

New-Item -ItemType Directory -Force -Path $outDir | Out-Null
New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

$previousTargetDir = $env:CARGO_TARGET_DIR
$env:CARGO_TARGET_DIR = $targetDir
Push-Location $crateRoot
try {
    cargo build --target wasm32-unknown-unknown --release
    if ($LASTEXITCODE -ne 0) {
        throw "cargo build failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
    $env:CARGO_TARGET_DIR = $previousTargetDir
}

$wasm = Join-Path $targetDir "wasm32-unknown-unknown\release\tracking_math.wasm"
if (!(Test-Path $wasm)) {
    throw "tracking_math.wasm was not produced"
}

Copy-Item -Force $wasm (Join-Path $outDir "tracking_math.wasm")
Write-Host "WASM written to $outDir\tracking_math.wasm"
