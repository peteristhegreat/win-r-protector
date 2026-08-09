[CmdletBinding()]
param(
    [string]$OutputDirectory = "dist"
)

$ErrorActionPreference = "Stop"
$projectRoot = $PSScriptRoot
$outputPath = Join-Path $projectRoot $OutputDirectory

function Assert-NativeSuccess([string]$Operation) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Operation failed with exit code $LASTEXITCODE."
    }
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
Push-Location $projectRoot
try {
    go mod download
    Assert-NativeSuccess "Dependency download"

    $goArch = go env GOARCH
    Assert-NativeSuccess "Architecture detection"

    go run github.com/tc-hib/go-winres@v0.3.3 simply `
        --arch $goArch `
        --out "cmd/win-r-protector/rsrc" `
        --manifest gui `
        --icon "icons/win-r-protect.ico" `
        --product-name "Win-R Protector" `
        --file-description "Win-R Protector service and tray application" `
        --original-filename "win-r-protector.exe"
    Assert-NativeSuccess "Windows resource generation"

    go build -trimpath -ldflags "-H windowsgui -s -w" -o (Join-Path $outputPath "win-r-protector.exe") ./cmd/win-r-protector
    Assert-NativeSuccess "Application build"
} finally {
    Pop-Location
}

Write-Host "Built $outputPath\win-r-protector.exe"
