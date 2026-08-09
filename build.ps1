[CmdletBinding()]
param(
    [string]$OutputDirectory = "dist"
)

$ErrorActionPreference = "Stop"
$projectRoot = $PSScriptRoot
$outputPath = Join-Path $projectRoot $OutputDirectory

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
Push-Location $projectRoot
try {
    go mod download
    go build -trimpath -ldflags "-H windowsgui -s -w" -o (Join-Path $outputPath "WinRProtector.exe") ./cmd/win-r-protector
} finally {
    Pop-Location
}

Write-Host "Built $outputPath\WinRProtector.exe"
