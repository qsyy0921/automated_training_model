function Resolve-Go {
  param([string]$Candidate = "")

  $candidates = New-Object System.Collections.Generic.List[string]
  if (-not [string]::IsNullOrWhiteSpace($Candidate)) {
    $candidates.Add($Candidate)
  }
  if (-not [string]::IsNullOrWhiteSpace($env:ATM_GO)) {
    $candidates.Add($env:ATM_GO)
  }
  if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    $candidates.Add((Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe"))
  }
  if (-not [string]::IsNullOrWhiteSpace($env:ProgramFiles)) {
    $candidates.Add((Join-Path $env:ProgramFiles "Go\bin\go.exe"))
  }
  if (-not [string]::IsNullOrWhiteSpace(${env:ProgramFiles(x86)})) {
    $candidates.Add((Join-Path ${env:ProgramFiles(x86)} "Go\bin\go.exe"))
  }

  foreach ($path in $candidates) {
    if (Test-Path -LiteralPath $path) {
      return (Resolve-Path -LiteralPath $path).Path
    }
  }

  $fromPath = (where.exe go 2>$null | Select-Object -First 1)
  if (-not [string]::IsNullOrWhiteSpace($fromPath)) {
    return $fromPath
  }

  throw "Go executable was not found. Install Go MSI or set ATM_GO to go.exe."
}
