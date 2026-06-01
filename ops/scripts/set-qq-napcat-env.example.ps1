. "$PSScriptRoot\utf8.ps1" -Quiet

$env:QQ_ONEBOT_OUTBOUND_ENABLED = "true"
$env:QQ_ONEBOT_HTTP_URL = "http://127.0.0.1:3000"
$env:QQ_ONEBOT_ACCESS_TOKEN = "replace_me_if_napcat_requires_token"

Write-Host "QQ NapCat OneBot outbound environment variables are set for this PowerShell session."
