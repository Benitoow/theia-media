# Assembles what a person actually downloads: one archive, one file to run.
#
#   .\build-release.ps1                     -> dist\theia-<version>-windows-amd64.zip
#   .\build-release.ps1 -Version 3.3.3
#   .\build-release.ps1 -Version 3.3.4 -Target darwin-arm64
#                                           -> dist\theia-3.3.4-darwin-arm64.zip,
#                                              assembled from dist\ and not built
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
#
# macOS is the second platform with a whole product, from V3.3.4, and it is
# assembled rather than built - which is the one real difference between the two
# paths and the reason `-Target` exists instead of a second script. A Tauri app
# bundle is compiled by the machine it will run on, and the engine inside it is
# pinned for darwin-arm64 alone, so by the time this runs the bundle and the
# three programs are already in dist/ and nothing here may build anything. What
# this path owns is the archive: the bundle as it stands, the programs under the
# short names the installer looks for, and a START-HERE.txt that says the two
# things a Mac user meets and a Windows user does not - an unsigned download and
# a zip that arrives without its executable bit.
#
# Nothing is fetched or compiled to make the macOS archive, so it is offline by
# construction, and a missing bundle names the build that produces it rather
# than reaching for the network.

param(
    [string]$Version = 'dev',

    # The platform whose archive to assemble. `windows-amd64` builds the
    # programs (below); `darwin-arm64` assembles what a Mac built, and is what
    # the release workflow runs on its macOS runner.
    [ValidateSet('windows-amd64', 'darwin-arm64')]
    [string]$Target = 'windows-amd64',

    # Windows only, and vacuous on darwin: the macOS path never builds the
    # player, so there is nothing here for it to skip.
    [switch]$SkipPlayer
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

# The one step both platforms take: everything standing in the stage becomes
# dist\theia-<version>-<platform>.zip, and the sizes are printed.
#
# On Windows that is `Compress-Archive`, which writes no Unix permission bits -
# and none are wanted there, where an `.exe` is runnable by its name.
#
# On macOS the same call was the wrong tool, and the archive said so out loud: a
# zip download arrived readable but not runnable, and the START-HERE.txt had to
# teach a person to chmod four files before the product would start. That was
# honest and it was also avoidable: `zip` is on every Mac and writes the
# permission bits, and `-y` keeps the symlinks inside the bundle as symlinks
# instead of turning each one into a second copy of a 3.8 MB dylib. So the
# darwin path uses it, and what a Mac user unzips now runs.
function Publish-OfflineArchive {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Stage
    )

    $archive = Join-Path $root "dist/$Name.zip"
    if (Test-Path $archive) { Remove-Item -Force $archive }
    if ($IsMacOS) {
        # `-y` is not an optimisation: the engine is reached through symlinks
        # (libmpv.dylib -> libmpv.2.dylib), and a zip that stores the target
        # under both names ships the library twice and changes what the bundle
        # is. Run from the stage so the archive's members are relative.
        Push-Location $Stage
        try {
            & zip -r -y $archive .
            if ($LASTEXITCODE -ne 0) { throw "zip failed with exit code $LASTEXITCODE" }
        } finally {
            Pop-Location
        }
    } else {
        Compress-Archive -Path (Join-Path $Stage '*') -DestinationPath $archive
    }

    $total = [math]::Round(((Get-ChildItem $Stage -Recurse -File | Measure-Object -Property Length -Sum).Sum) / 1MB, 1)
    $zipped = [math]::Round((Get-Item $archive).Length / 1MB, 1)
    Write-Host "==> $Name.zip ready ($zipped MB zipped, $total MB unpacked)" -ForegroundColor Green
    Get-ChildItem $Stage | ForEach-Object {
        # A macOS stage holds a directory - the whole application - so the
        # listing measures it instead of printing nothing for it.
        $mb = if ($_.PSIsContainer) {
            ((Get-ChildItem $_.FullName -Recurse -File | Measure-Object -Property Length -Sum).Sum) / 1MB
        }
        else { $_.Length / 1MB }
        $kind = if ($_.PSIsContainer) { ' (directory, recursive)' } else { '' }
        Write-Host ("    {0}{1} ({2:N1} MB)" -f $_.Name, $kind, $mb)
    }
}

# Where the macOS programs are read from. The published name is what
# `make dist-programs` and the release workflow write
# (`theia-server-darwin-arm64`, the name that also has to appear in the published
# release); the short name is accepted because `go build -o dist/theia-server` is
# the other thing somebody does on a Mac, and reading it here is cheaper than
# explaining the difference.
function Get-DistProgram {
    param([string]$Published, [string]$Short)

    foreach ($candidate in @($Published, $Short)) {
        $path = Join-Path $root "dist/$candidate"
        if (Test-Path $path) { return $path }
    }
    throw "dist/$Published is missing (nor dist/$Short). Build the macOS programs with 'make dist-programs', which names them the way the release publishes them, or put the published assets there, and run this again."
}

# macOS is handled first and returns, because this side assembles while the
# Windows side below builds, and the only thing the two share is the zip at the
# end. Keeping the paths adjacent rather than interleaved is what stops a macOS
# change from moving a line in the path every Windows release depends on.
#
# This side's paths are written with forward slashes, and that is not a style
# choice: PowerShell on macOS does not read a backslash as a separator, so
# `dist\theia-player-darwin-arm64` names a directory that cannot exist there.
# Windows accepts both, which is why the Windows path below keeps its own.
if ($Target -eq 'darwin-arm64') {
    Write-Host '==> Assembling the macOS archive from dist/ (nothing is built)' -ForegroundColor Cyan

    $name = "theia-$Version-darwin-arm64"
    $stage = Join-Path $root "dist/$name"
    if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
    New-Item -ItemType Directory -Force -Path $stage | Out-Null

    # The player bundle as the macOS build leaves it, engine and licences
    # inside: the application is copied whole, so there is no second flat copy
    # of the engine or of its licence here to drift from the ones in the bundle.
    #
    # The engine is asked for by directory and not by one exact name on purpose.
    # The player build decides what it installs - player/libmpv.json's darwin pin
    # calls its engine library `libmpv.2.dylib` and its install_name is
    # `@rpath/libmpv.2.dylib`, while a bundle that renames it to `libmpv.dylib`
    # is what the loader reaches either way. What must not happen is a bundle
    # with no engine in it, because that one looks complete and plays nothing.
    $bundle = Join-Path $root 'dist/theia-player-darwin-arm64'
    foreach ($required in @(
        'Theia.app/Contents/MacOS/theia-player',
        'Theia.app/Contents/Resources/LICENSE-libmpv.txt',
        'Theia.app/Contents/Resources/NOTICE.md'
    )) {
        if (-not (Test-Path (Join-Path $bundle $required))) {
            throw "the macOS player bundle is missing $required under $bundle. This path assembles what a macOS build leaves behind; build the player on a Mac first."
        }
    }
    $frameworks = Join-Path $bundle 'Theia.app/Contents/Frameworks'
    $engines = @(Get-ChildItem -Path $frameworks -Filter 'libmpv*.dylib' -File -ErrorAction SilentlyContinue)
    if ($engines.Count -eq 0) {
        throw "the macOS player bundle holds no libmpv*.dylib in $frameworks. The engine and the application are one thing to ship: a bundle without it looks complete and plays nothing."
    }
    Copy-Item -Recurse -Force (Join-Path $bundle 'Theia.app') $stage

    # Short names inside the archive, exactly as the Windows archive does it:
    # the archive name already carries the platform, and the installer looks for
    # a short name on disk beside the published one (internal/setup
    # acceptedNames), because a working tree produces the short one.
    foreach ($program in @(
        @{ asset = 'theia-server-darwin-arm64'; installed = 'theia-server' },
        @{ asset = 'theia-setup-darwin-arm64'; installed = 'theia-setup' },
        @{ asset = 'theia-launcher-darwin-arm64'; installed = 'theia' }
    )) {
        $source = Get-DistProgram -Published $program.asset -Short $program.installed
        Copy-Item $source (Join-Path $stage $program.installed)
    }

    # Two paragraphs, one in each language, as on Windows - and the first thing
    # in each is what macOS refuses, because that happens before the installer
    # gets to say anything. Nothing here reaches for xattr: the instruction is
    # the documented way to allow an unsigned program, not a way to hide that it
    # is unsigned, and there is no notarisation step to describe because this
    # project has no Apple Developer certificate to notarise with.
    $readme = @"
Theia $Version
============$(('=' * $Version.Length))

EN - Unzip this archive, open Terminal in the folder it made, and run
     ./theia-setup. Two things about this download come first, because macOS
     says no to both. It is unsigned: the project has no Apple Developer
     certificate, so there is no notarised build to offer, and macOS therefore
     refuses the first launch of everything here with "cannot be opened because
     the developer cannot be verified". That refusal is expected and is not a
     broken download. Allow it once, the way Apple documents it: Control-click
     Theia.app, choose Open, then choose Open again in the dialog - and for the
     Terminal programs, let the first run be refused and then allow it in
     System Settings > Privacy & Security > Open Anyway. The programs arrive
     executable, so ./theia-setup runs as it is; nothing here needs chmod.
     The installer's first question is which language Theia should speak -
     English and French ship, and the answer is what both interfaces and the
     film metadata open in. Then it asks what this machine is for, where to keep
     its data, which port it listens on and which folders hold your films, and it
     shows the whole plan before writing anything. It installs the programs into
     ~/.local/lib/theia, links Theia.app into ~/Applications so Finder, Spotlight
     and Launchpad open it, and puts the theia command in ~/.local/bin so it is
     found by name. If ~/.local/bin is not on your PATH it prints the one line
     that adds it, rather than editing your shell profile behind your back. From
     a terminal, theia starts the server and opens the player, theia server and
     theia player start one half, and theia-server and theia-player run them in
     the console. Nothing is downloaded: everything is in this folder. It
     installs autostart only if you ask, and never asks for your password.

FR - Decompressez cette archive, ouvrez un Terminal dans le dossier obtenu et
     lancez ./theia-setup. Deux choses d'abord, parce que macOS refuse les deux.
     Ce fichier n'est pas signe : le projet n'a pas de certificat Apple, donc il
     n'y a pas de version notarisee a proposer, et macOS refuse donc le premier
     lancement de tout ce qui est ici, avec "cannot be opened because the
     developer cannot be verified". Ce refus est normal et ne veut pas dire que
     le telechargement est casse. Autorisez-le une fois, comme Apple le
     documente : Control-clic sur Theia.app, Ouvrir, puis Ouvrir encore dans la
     boite de dialogue - et pour les programmes en ligne de commande, laissez le
     premier lancement echouer puis autorisez-les dans Reglages du systeme >
     Confidentialite et securite > Ouvrir quand meme. Les programmes arrivent
     executables : ./theia-setup se lance tel quel, rien ici ne demande chmod.
     La premiere question de l'installateur est la langue que Theia doit parler
     - l'anglais et le francais sont livres, et la reponse est celle dans
     laquelle s'ouvrent les deux interfaces comme les fiches des films. Il
     demande ensuite a quoi sert cette machine, ou garder ses donnees, sur quel
     port elle ecoute et quels dossiers contiennent vos films, puis il affiche
     le plan complet avant d'ecrire quoi que ce soit. Il installe les programmes
     dans ~/.local/lib/theia, lie Theia.app dans ~/Applications pour que le
     Finder, Spotlight et Launchpad l'ouvrent, et met la commande theia dans
     ~/.local/bin pour qu'elle se trouve par son nom. Si ~/.local/bin n'est pas
     dans votre PATH, il affiche la seule ligne qui l'y ajoute plutot que de
     modifier votre profil de shell a votre place. Dans un terminal, theia
     demarre le serveur et ouvre le lecteur, theia server et theia player en
     lancent une moitie, et theia-server et theia-player les executent dans la
     console. Rien n'est telecharge : tout est dans ce dossier. Il installe un
     demarrage automatique seulement si vous le demandez, et ne demande jamais
     votre mot de passe.

LICENSE-libmpv.txt is the licence of the media engine (libmpv, LGPL-2.1+) and
NOTICE.md names the exact build and its SHA-256. On macOS both sit inside the
application, in Theia.app/Contents/Resources, beside the engine they cover.
"@
    Set-Content -Path (Join-Path $stage 'START-HERE.txt') -Value $readme

    Publish-OfflineArchive -Name $name -Stage $stage
    return
}

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
    #
    # `$build` and not `$target`: PowerShell compares variable names without
    # case, so a loop variable called `$target` *is* the -Target parameter, and
    # assigning a hashtable to it fails with a message about converting one into
    # a string. Same trap as `$bundle` in build-player.ps1.
    foreach ($build in @(
        @{ path = './cmd/theia-server'; out = 'theia-server.exe'; key = 'main.tmdbAPIKey' },
        @{ path = './cmd/theia-setup'; out = 'theia-setup.exe'; key = '' },
        # The launcher is built windowed: it is what the Start Menu entry starts,
        # and a console subsystem build would flash a black rectangle every time
        # somebody clicked Theia. See cmd/theia/main.go.
        @{ path = './cmd/theia'; out = 'theia.exe'; key = ''; gui = $true }
    )) {
        $ldflags = "-s -w -X main.version=$Version"
        if ($build.key -and $env:TMDB_API_KEY) { $ldflags += " -X $($build.key)=$($env:TMDB_API_KEY)" }
        if ($build.gui) { $ldflags += ' -H=windowsgui' }
        & $go build -buildvcs=false -trimpath -ldflags $ldflags -o (Join-Path $stage $build.out) $build.path
        if ($LASTEXITCODE -ne 0) { throw "building $($build.out) failed" }
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
     ``theia`` starts the server and opens the player, ``theia server`` and
     ``theia player`` start one half, and ``theia-server`` and
     ``theia-player`` run them in the console. Nothing is downloaded:
     everything is in this folder. It installs autostart only if you ask, and
     never requests administrator rights.

FR - Lancez theia-setup.exe. Sa premiere question est la langue que Theia doit
     parler - l'anglais et le francais sont livres, et la reponse est celle dans
     laquelle s'ouvrent les deux interfaces comme les fiches des films. Il
     demande ensuite a quoi sert cette machine, ou garder ses donnees, sur quel
     port elle ecoute et quels dossiers contiennent vos films, puis il affiche
     le plan complet avant d'ecrire quoi que ce soit. Il installe les programmes
     dans %LOCALAPPDATA%\Programs\Theia, met ce dossier dans votre PATH et pose
     des raccourcis dans le menu Demarrer et sur le bureau (Theia, Theia Server,
     Theia Player), pour qu'ils se lancent par leur nom. Dans un terminal,
     ``theia`` demarre le serveur et ouvre le lecteur, ``theia server`` et
     ``theia player`` en lancent une moitie, et ``theia-server`` et
     ``theia-player`` les executent dans la console. Rien n'est telecharge :
     tout est dans ce dossier. Il installe un demarrage automatique seulement
     si vous le demandez, et ne reclame jamais de droits administrateur.

LICENSE-libmpv.txt is the licence of the media engine (libmpv, LGPL-2.1+).
NOTICE.md names the exact build and its SHA-256.
"@
Set-Content -Path (Join-Path $stage 'START-HERE.txt') -Value $readme

Publish-OfflineArchive -Name $name -Stage $stage
