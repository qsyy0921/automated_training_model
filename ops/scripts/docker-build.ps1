$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Push-Location $root
try {
  docker build -f "$root\ops\deployments\docker\Dockerfile" -t video-label-tool-labelserver .
} finally {
  Pop-Location
}
