param(
    [Parameter(Mandatory = $true)]
    [string]$Directory,

    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version,

    [ValidateSet('all', 'windows-amd64', 'darwin-arm64')]
    [string]$Target = 'all'
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path -LiteralPath $Directory).Path
$checked = 0

function Open-Archive([string]$name) {
    $path = Join-Path $root $name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "release parity: missing archive $name"
    }
    return [System.IO.Compression.ZipFile]::OpenRead($path)
}

function Normalize-EntryName([string]$name) {
    while ($name.StartsWith('./')) { $name = $name.Substring(2) }
    return $name
}

function Get-EntryHash($archive, [string]$name) {
    $matches = @($archive.Entries | Where-Object {
        (Normalize-EntryName $_.FullName) -eq $name
    })
    if ($matches.Count -ne 1) {
        throw "release parity: expected one $name in $($archive.Name), found $($matches.Count)"
    }
    $stream = $matches[0].Open()
    try { return (Get-FileHash -InputStream $stream -Algorithm SHA256).Hash }
    finally { $stream.Dispose() }
}

function Assert-FileMatchesEntry($archive, [string]$member, [string]$filename) {
    $file = Join-Path $root $filename
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
        throw "release parity: missing component $filename"
    }
    $inside = Get-EntryHash $archive $member
    $outside = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash
    if ($inside -ne $outside) {
        throw "release parity: $member in the offline archive differs from $filename"
    }
    $script:checked++
    Write-Host "equal: $member = $filename"
}

function Assert-EntriesMatch($offline, $component, [string]$member) {
    $inside = Get-EntryHash $offline $member
    $outside = Get-EntryHash $component $member
    if ($inside -ne $outside) {
        throw "release parity: $member differs between offline archive and player component"
    }
    $script:checked++
    Write-Host "equal: $member in both archives"
}

if ($Target -in @('all', 'windows-amd64')) {
    $offline = Open-Archive "theia-$Version-windows-amd64.zip"
    $player = Open-Archive 'theia-player-windows-amd64.zip'
    try {
        Assert-FileMatchesEntry $offline 'theia-server.exe' 'theia-server-windows-amd64.exe'
        Assert-FileMatchesEntry $offline 'theia-setup.exe' 'theia-setup-windows-amd64.exe'
        Assert-FileMatchesEntry $offline 'theia.exe' 'theia-launcher-windows-amd64.exe'
        foreach ($member in 'theia-player.exe', 'libmpv-2.dll', 'LICENSE-libmpv.txt', 'NOTICE.md') {
            Assert-EntriesMatch $offline $player $member
        }
    } finally {
        $player.Dispose()
        $offline.Dispose()
    }
}

if ($Target -in @('all', 'darwin-arm64')) {
    $offline = Open-Archive "theia-$Version-darwin-arm64.zip"
    $player = Open-Archive 'theia-player-darwin-arm64.zip'
    try {
        Assert-FileMatchesEntry $offline 'theia-server' 'theia-server-darwin-arm64'
        Assert-FileMatchesEntry $offline 'theia-setup' 'theia-setup-darwin-arm64'
        Assert-FileMatchesEntry $offline 'theia' 'theia-launcher-darwin-arm64'
        $appMembers = @($player.Entries | Where-Object {
            $name = Normalize-EntryName $_.FullName
            $name.StartsWith('Theia.app/') -and -not $name.EndsWith('/')
        } | ForEach-Object { Normalize-EntryName $_.FullName })
        if ($appMembers.Count -eq 0) { throw 'release parity: the macOS player archive has no application files' }
        foreach ($member in $appMembers) {
            Assert-EntriesMatch $offline $player $member
        }
    } finally {
        $player.Dispose()
        $offline.Dispose()
    }
}

Write-Host "release parity passed: $checked identical components for Theia $Version"
