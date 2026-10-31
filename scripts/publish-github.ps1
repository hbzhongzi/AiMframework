$ErrorActionPreference = "Stop"

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$remote = "https://github.com/hbzhongzi/AiMframework.git"

Set-Location $root

git -c safe.directory=$root init
git -c safe.directory=$root branch -M main

$existing = git -c safe.directory=$root remote get-url origin 2>$null
if ($LASTEXITCODE -eq 0) {
    git -c safe.directory=$root remote set-url origin $remote
} else {
    git -c safe.directory=$root remote add origin $remote
}

git -c safe.directory=$root add .
git -c safe.directory=$root commit -m "Update Gemma backend and desktop application"
git -c safe.directory=$root push -u origin main

Write-Host "Published to $remote"
