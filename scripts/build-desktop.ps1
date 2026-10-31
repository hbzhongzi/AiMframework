$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent $scriptDir
$outDir = Join-Path $root "dist"
$adminOut = Join-Path $outDir "gemma-desktop.exe"
$chatOut = Join-Path $outDir "gemma-chat.exe"
$env:GOCACHE = Join-Path $root ".cache\gocache-desktop"
New-Item -ItemType Directory -Force (Split-Path $env:GOCACHE) | Out-Null
New-Item -ItemType Directory -Force $outDir | Out-Null

Push-Location (Join-Path $root "desktop")
try {
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-H=windowsgui" -o $adminOut .
    go build -trimpath -ldflags "-H=windowsgui -X main.defaultPage=chat" -o $chatOut .
} finally {
    Pop-Location
}

Write-Host "Built: $adminOut"
Write-Host "Built: $chatOut"
