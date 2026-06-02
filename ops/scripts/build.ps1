$ErrorActionPreference = "Stop"
. "$PSScriptRoot\utf8.ps1" -Quiet
. "$PSScriptRoot\resolve-go.ps1"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$go = Resolve-Go
Push-Location $root
try {
  & $go test ./...
  New-Item -ItemType Directory -Force -Path "$root\bin" | Out-Null
  & $go build -o "$root\bin\labelserver.exe" .\cmd\labelserver
} finally {
  Pop-Location
}
