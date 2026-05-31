$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
[Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)
chcp 65001 | Out-Null

$PSDefaultParameterValues["Get-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Set-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Add-Content:Encoding"] = "UTF8"
$PSDefaultParameterValues["Out-File:Encoding"] = "UTF8"
$PSDefaultParameterValues["Export-Csv:Encoding"] = "UTF8"

if ($PSStyle) {
  $PSStyle.OutputRendering = "PlainText"
}

Write-Host "PowerShell UTF-8 mode enabled. Use '. .\ops\scripts\utf8.ps1' at the start of a shell session."
