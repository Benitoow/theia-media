# Builds the native player.
#
#   .\build-player.ps1              -> debug build, for running and testing
#   .\build-player.ps1 -Release     -> release build
#
# The order is not negotiable: tauri-build embeds player/ui/dist into the Rust
# binary at compile time, so the OSD has to exist first. Cargo will not tell you
# that in a useful way, which is the whole reason this script exists rather than
# a paragraph in a README nobody reads at the moment they need it.

param(
    [switch]$Release
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$profile = if ($Release) { 'release' } else { 'debug' }

Write-Host '==> Building the OSD' -ForegroundColor Cyan
Push-Location (Join-Path $root 'player\ui')
try {
    if (Test-Path 'package-lock.json') { npm ci } else { npm install }
    if ($LASTEXITCODE -ne 0) { throw 'npm install failed' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'the OSD build failed' }
}
finally {
    Pop-Location
}

if (-not (Test-Path (Join-Path $root 'player\ui\dist\index.html'))) {
    throw 'player/ui/dist/index.html is missing after the OSD build; cargo build would fail with a confusing error.'
}

# Find the Go-style toolchain resolution this project already uses: cargo is
# normally on PATH, but say something useful when it is not.
$cargo = (Get-Command cargo -ErrorAction SilentlyContinue).Source
if (-not $cargo) {
    $candidate = Join-Path $env:USERPROFILE '.cargo\bin\cargo.exe'
    if (Test-Path $candidate) { $cargo = $candidate }
}
if (-not $cargo) {
    throw "cargo was not found. Install Rust from https://rustup.rs, or put cargo on PATH."
}

Write-Host "==> Building theia-player ($profile, using $cargo)" -ForegroundColor Cyan
Push-Location $root
try {
    $arguments = @('build', '--manifest-path', 'player\Cargo.toml')
    if ($Release) { $arguments += '--release' }
    & $cargo @arguments
    if ($LASTEXITCODE -ne 0) { throw 'cargo build failed' }
}
finally {
    Pop-Location
}

$exe = Join-Path $root "player\target\$profile\theia-player.exe"
if (-not (Test-Path $exe)) { throw "the build reported success but $exe does not exist." }
$size = [math]::Round((Get-Item $exe).Length / 1MB, 1)
Write-Host "==> theia-player ready ($size MB, $profile)" -ForegroundColor Green
Write-Host '    The engine is not bundled yet: set THEIA_LIBMPV, or put libmpv-2.dll beside the executable.' -ForegroundColor DarkGray
