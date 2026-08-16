# Windows equivalent of `make`: build the frontend, embed it, produce theia.exe.
#
#   .\build.ps1                 -> theia.exe, version "dev"
#   .\build.ps1 -Version 0.2.0  -> theia.exe, version "0.2.0"

param(
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

Write-Host '==> Building the frontend' -ForegroundColor Cyan
Push-Location (Join-Path $root 'web')
try {
    if (Test-Path 'package-lock.json') { npm ci } else { npm install }
    if ($LASTEXITCODE -ne 0) { throw 'npm install failed' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'npm run build failed' }
}
finally {
    Pop-Location
}

# The static adapter wipes web-dist/, including the placeholder that keeps the
# directory present in a fresh clone so //go:embed resolves.
$keep = Join-Path $root 'web-dist\.gitkeep'
if (-not (Test-Path $keep)) {
    git -C $root checkout -- 'web-dist/.gitkeep' 2>$null
    if (-not (Test-Path $keep)) { New-Item -ItemType File -Path $keep | Out-Null }
}

# Find the Go toolchain.
#
# This script used to assume `go` was on PATH and died with a bare
# CommandNotFoundException when it was not -- after the frontend had already
# been rebuilt, so the tree was left half-built and the cause was three screens
# up. A toolchain unpacked beside the profile rather than installed is the
# normal case on this machine, so look there before giving up, and say something
# useful if nothing is found.
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) {
    $candidates = @(
        (Join-Path $env:USERPROFILE 'go-toolchain/go/bin/go.exe'),
        (Join-Path $env:LOCALAPPDATA 'go/bin/go.exe'),
        'C:/Program Files/Go/bin/go.exe'
    )
    $go = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $go) {
    throw "Go was not found. Install it, put it on PATH, or unpack it at $env:USERPROFILE/go-toolchain/go."
}
Write-Host "==> Building the binary (using $go)" -ForegroundColor Cyan

Push-Location $root
try {
    $env:CGO_ENABLED = '0'
    & $go build -trimpath -ldflags "-s -w -X main.version=$Version" -o theia.exe ./cmd/theia
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
}
finally {
    Pop-Location
}

$size = [math]::Round((Get-Item (Join-Path $root 'theia.exe')).Length / 1MB, 1)
Write-Host "==> theia.exe ready ($size MB, version $Version)" -ForegroundColor Green
