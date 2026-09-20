# Builds the native player.
#
#   .\build-player.ps1              -> debug build, for running and testing
#   .\build-player.ps1 -Release     -> release build
#   .\build-player.ps1 -Release -Bundle
#                                   -> and a distribution directory to ship
#
# The order is not negotiable: tauri-build embeds player/ui/dist into the Rust
# binary at compile time, so the OSD has to exist first. Cargo will not tell you
# that in a useful way, which is the whole reason this script exists rather than
# a paragraph in a README nobody reads at the moment they need it.

param(
    [switch]$Release,
    [switch]$Bundle
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

# The binary has to carry the interface that was just built, and "cargo said ok"
# does not say that: tauri-build reads player/ui/dist at compile time, and cargo
# does not watch it on its own (see the note in player/theia-player/build.rs).
# Measured on 20 September 2026: a release binary carried the asset names of the
# build before it while dist/ already named the new ones, and the maintainer was
# shown a preview two revisions old.
#
# Checked by name, because Vite's asset names are content-hashed: a match means
# these bytes and not merely a build that ran. The file names are plain text in
# the executable's asset manifest even though the assets themselves are stored
# compressed, which is why this works where grepping for a class name does not.
$index = Get-Content (Join-Path $root 'player\ui\dist\index.html') -Raw
$assets = [regex]::Matches($index, 'assets/(index-[A-Za-z0-9_-]+\.(?:js|css))') | ForEach-Object { $_.Groups[1].Value }
if ($assets.Count -eq 0) {
    throw 'player/ui/dist/index.html names no hashed asset; the OSD build produced something unexpected.'
}
$exeText = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($exe))
$missing = $assets | Where-Object { -not $exeText.Contains($_) }
if ($missing) {
    throw ("the built player does not carry the OSD just built: {0} {1} in player/ui/dist and not in {2}. " -f ($missing -join ', '), $(if ($missing.Count -eq 1) { 'is' } else { 'are' }), $exe) +
        "Cargo reused a cached crate - rebuild, or run 'cargo clean -p theia-player' and rebuild."
}

$size = [math]::Round((Get-Item $exe).Length / 1MB, 1)
Write-Host "==> theia-player ready ($size MB, $profile, carrying $($assets -join ' and '))" -ForegroundColor Green

if (-not $Bundle) {
    Write-Host '    The engine is not bundled: set THEIA_LIBMPV, or put libmpv-2.dll beside the executable.' -ForegroundColor DarkGray
    Write-Host '    Use -Bundle to produce what gets shipped, with its licence.' -ForegroundColor DarkGray
    exit 0
}

# What actually gets shipped. Three files, and each one is an obligation rather
# than a nicety:
#
#   theia-player.exe   the program
#   libmpv-2.dll       the engine, a separate replaceable file (LGPL), pinned and
#                      digest-verified by scripts/fetch-libmpv
#   LICENSE-libmpv.txt the LGPL text, and NOTICE.md naming the source
#
# A player shipped without the engine is a player that does not play; one shipped
# without the licence is a licence breach. Both come from the same manifest, so
# neither can be forgotten separately.
Write-Host '==> Bundling the engine and its licence' -ForegroundColor Cyan

$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) {
    $candidate = Join-Path $env:USERPROFILE 'go-toolchain\go\bin\go.exe'
    if (Test-Path $candidate) { $go = $candidate }
}
if (-not $go) { throw 'Go was not found, and the bundling step needs it to fetch and verify the engine.' }

$bundleDir = Join-Path $root "dist\theia-player-windows-amd64"
# Not `$bundle`: PowerShell compares variable names case-insensitively, so
# `$bundle` *is* the -Bundle switch, and assigning a path to it fails with a
# message about converting a string into a switch. The first version of this did
# exactly that.
if (Test-Path $bundleDir) { Remove-Item -Recurse -Force $bundleDir }
New-Item -ItemType Directory -Force -Path $bundleDir | Out-Null

Push-Location $root
try {
    $env:CGO_ENABLED = '0'
    & $go run ./scripts/fetch-libmpv -out $bundleDir
    if ($LASTEXITCODE -ne 0) { throw 'fetching the pinned engine failed' }
}
finally {
    Pop-Location
}

Copy-Item $exe (Join-Path $bundleDir 'theia-player.exe')
Copy-Item (Join-Path $root 'player\LICENSE-libmpv.txt') $bundleDir
Copy-Item (Join-Path $root 'player\NOTICE.md') $bundleDir

$archive = Join-Path $root 'dist\theia-player-windows-amd64.zip'
if (Test-Path $archive) { Remove-Item -Force $archive }
Compress-Archive -Path (Join-Path $bundleDir '*') -DestinationPath $archive

$total = [math]::Round(((Get-ChildItem $bundleDir -File | Measure-Object -Property Length -Sum).Sum) / 1MB, 1)
$zipped = [math]::Round((Get-Item $archive).Length / 1MB, 1)
Write-Host "==> theia-player-windows-amd64 ready ($total MB, $zipped MB zipped)" -ForegroundColor Green
Get-ChildItem $bundleDir | ForEach-Object { Write-Host ("    {0} ({1:N1} MB)" -f $_.Name, ($_.Length / 1MB)) }
