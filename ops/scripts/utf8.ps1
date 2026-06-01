param(
  [switch]$Quiet
)

$utf8NoBom = [System.Text.UTF8Encoding]::new($false)

$OutputEncoding = $utf8NoBom
[Console]::OutputEncoding = $utf8NoBom
[Console]::InputEncoding = $utf8NoBom
chcp 65001 | Out-Null

$env:PYTHONUTF8 = "1"
$env:PYTHONIOENCODING = "utf-8"

$PSDefaultParameterValues["Get-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Set-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Add-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Out-File:Encoding"] = "UTF8"
$PSDefaultParameterValues["Export-Csv:Encoding"] = "UTF8"

if (Get-Variable -Name PSStyle -ErrorAction SilentlyContinue) {
  $PSStyle.OutputRendering = "PlainText"
}

if (-not $Quiet) {
  Write-Host "PowerShell UTF-8 mode enabled. Use '. .\ops\scripts\utf8.ps1' at the start of a shell session."
}
