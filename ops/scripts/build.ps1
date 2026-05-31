$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$go = "F:\keyan\token_compression\third_party\go1.26.3\go\bin\go.exe"
if (!(Test-Path $go)) {
  $go = "go"
}
Push-Location $root
try {
  & $go test ./...
  New-Item -ItemType Directory -Force -Path "$root\bin" | Out-Null
  & $go build -o "$root\bin\labelserver.exe" .\cmd\labelserver
} finally {
  Pop-Location
}
