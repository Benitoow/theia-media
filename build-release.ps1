# Assembles what a person actually downloads: one archive, one file to run.
#
#   .\build-release.ps1                     -> dist\theia-<version>-windows-amd64.zip
#   .\build-release.ps1 -Version 3.3.2
#
# Why one archive and not three downloads. The first version of V3.3 published
# the installer, the server and the player separately and told the reader to put
# them together. Downloading the installer alone - which is what the README's
# first instruction says to do - produced a configuration and nothing to run it:
# a user with one file had no server and no player, and the installer could only
# report them missing. Measured by putting the release assets in an empty folder
# and following the instructions literally.
#
# This archive closes that: the programs, the pinned engine, its licence, and a
# line telling somebody which file to run. It stays a single download that needs
# no network at install time, which is also what "it works on my own machine"
# means.
#
# The published assets are still published separately, with their platform
# names, for people who want one piece and for the updater, which selects
# `theia-server-<os>-<arch>` by name.

param(
    [string]$Version = 'dev',
    [switch]$SkipPlayer
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) {
    $candidate = Join-Path $env:USERPROFILE 'go-toolchain\go\bin\go.exe'
    if (Test-Path $candidate) { $go = $candidate }
}
if (-not $go) { throw 'Go was not found. Install it, put it on PATH, or unpack it at $env:USERPROFILE/go-toolchain/go.' }

# The player first when it is not already built: it is by far the longest step,
# and failing there should not happen after the Go builds have run.
if (-not $SkipPlayer) {
    Write-Host '==> Building the player and its engine' -ForegroundColor Cyan
    & (Join-Path $root 'build-player.ps1') -Release -Bundle
    if ($LASTEXITCODE -ne 0) { throw 'the player build failed' }
}

$name = "theia-$Version-windows-amd64"
$stage = Join-Path $root "dist\$name"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

Write-Host '==> Building the server and the installer' -ForegroundColor Cyan
Push-Location $root
try {
    $env:CGO_ENABLED = '0'
    # Short names inside the archive: the archive name already carries the
    # platform, and `theia-server.exe` is the first name the installer looks for.
    #
    # Forward slashes and the `./` prefix: `cmd\theia-server` is a Windows path,
    # and Go reads it as a standard-library package - "package cmd/theia-server
    # is not in std" - which is how the first version of this failed.
    foreach ($target in @(
        @{ path = './cmd/theia-server'; out = 'theia-server.exe'; key = 'main.tmdbAPIKey' },
        @{ path = './cmd/theia-setup'; out = 'theia-setup.exe'; key = '' },
        # The launcher is built windowed: it is what the Start Menu entry starts,
        # and a console subsystem build would flash a black rectangle every time
        # somebody clicked Theia. See cmd/theia/main.go.
        @{ path = './cmd/theia'; out = 'theia.exe'; key = ''; gui = $true }
    )) {
        $ldflags = "-s -w -X main.version=$Version"
        if ($target.key -and $env:TMDB_API_KEY) { $ldflags += " -X $($target.key)=$($env:TMDB_API_KEY)" }
        if ($target.gui) { $ldflags += ' -H=windowsgui' }
        & $go build -buildvcs=false -trimpath -ldflags $ldflags -o (Join-Path $stage $target.out) $target.path
        if ($LASTEXITCODE -ne 0) { throw "building $($target.out) failed" }
    }
}
finally {
    Pop-Location
}

# The player bundle, already assembled with its engine by build-player.ps1.
$playerBundle = Join-Path $root 'dist\theia-player-windows-amd64'
if (-not (Test-Path (Join-Path $playerBundle 'theia-player.exe'))) {
    throw "the player bundle is missing from $playerBundle; build it with .\build-player.ps1 -Release -Bundle"
}
foreach ($file in 'theia-player.exe', 'libmpv-2.dll', 'LICENSE-libmpv.txt', 'NOTICE.md') {
    Copy-Item (Join-Path $playerBundle $file) $stage
}

# Two paragraphs, one in each language, because somebody who has just unzipped an
# archive has no other context. English leads and French follows: English is the
# base of the product (decision 137), and this file is the product's first
# surface - the only thing a person reads before anything runs.
$readme = @"
Theia $Version
============$(('=' * $Version.Length))

EN - Run theia-setup.exe. Its first question is which language Theia should
     speak - English and French ship, and the answer is what both interfaces and
     the film metadata open in. Then it asks what this machine is for, where to
     keep its data, which port it listens on and which folders hold your films,
     and it shows the whole plan before writing anything. It installs the
     programs into %LOCALAPPDATA%\Programs\Theia, puts that folder on your PATH
     and creates entries in the Start Menu and on the Desktop (Theia, Theia
     Server, Theia Player), so they can be launched by name. From a terminal,
     `theia` starts the server and opens the player, `theia server` and
     `theia player` start one half, and `theia-server` and `theia-player` run
     them in the console. Nothing is downloaded: everything is in this folder.
     It installs autostart only if you ask, and never requests administrator
     rights.

FR - Lancez theia-setup.exe. Sa premiere question est la langue que Theia doit
     parler - l'anglais et le francais sont livres, et la reponse est celle dans
     laquelle s'ouvrent les deux interfaces comme les fiches des films. Il
     demande ensuite a quoi sert cette machine, ou garder ses donnees, sur quel
     port elle ecoute et quels dossiers contiennent vos films, puis il affiche
     le plan complet avant d'ecrire quoi que ce soit. Il installe les programmes
     dans %LOCALAPPDATA%\Programs\Theia, met ce dossier dans votre PATH et pose
     des raccourcis dans le menu Demarrer et sur le bureau (Theia, Theia Server,
     Theia Player), pour qu'ils se lancent par leur nom. Dans un terminal,
     `theia` demarre le serveur et ouvre le lecteur, `theia server` et
     `theia player` en lancent une moitie, et `theia-server` et `theia-player`
     les executent dans la console. Rien n'est telecharge : tout est dans ce
     dossier. Il installe un demarrage automatique seulement si vous le
     demandez, et ne reclame jamais de droits administrateur.

LICENSE-libmpv.txt is the licence of the media engine (libmpv, LGPL-2.1+).
NOTICE.md names the exact build and its SHA-256.
"@
Set-Content -Path (Join-Path $stage 'START-HERE.txt') -Value $readme

$archive = Join-Path $root "dist\$name.zip"
if (Test-Path $archive) { Remove-Item -Force $archive }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive

$total = [math]::Round(((Get-ChildItem $stage -File | Measure-Object -Property Length -Sum).Sum) / 1MB, 1)
$zipped = [math]::Round((Get-Item $archive).Length / 1MB, 1)
Write-Host "==> $name.zip ready ($zipped MB zipped, $total MB unpacked)" -ForegroundColor Green
Get-ChildItem $stage | ForEach-Object { Write-Host ("    {0} ({1:N1} MB)" -f $_.Name, ($_.Length / 1MB)) }
