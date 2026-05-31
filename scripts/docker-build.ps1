$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
  docker build -f "$root\deployments\docker\Dockerfile" -t video-label-tool-labelserver .
} finally {
  Pop-Location
}
