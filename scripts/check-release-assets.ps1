param(
    [Parameter(Mandatory = $true)]
    [string]$Directory,

    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path -LiteralPath $Directory).Path
$entries = @(Get-ChildItem -LiteralPath $root -Force)
$directories = @($entries | Where-Object { $_.PSIsContainer })
if ($directories.Count -gt 0) {
    $names = ($directories.Name | Sort-Object) -join ', '
    throw "release staging contains directories: $names"
}

# The whole product is published for Windows x64 and, from V3.3.4, for macOS
# Apple Silicon: those platforms have a player and an installer, so they publish
# an offline archive, a player bundle, a launcher and a setup program beside the
# server. The other four targets publish the server alone, because a platform
# nobody has run the product on gets a component and not a product.
#
# macOS Intel is the case that looks like an omission and is not: it has a
# published server, and no player, launcher, installer or archive, because there
# is no pinned engine and no verified build for it.
$expected = @(
    "theia-$Version-darwin-arm64.zip"
    "theia-$Version-windows-amd64.zip"
    'theia-launcher-darwin-arm64'
    'theia-launcher-windows-amd64.exe'
    'theia-player-darwin-arm64.zip'
    'theia-player-windows-amd64.zip'
    'theia-server-darwin-amd64'
    'theia-server-darwin-arm64'
    'theia-server-linux-amd64'
    'theia-server-linux-arm64'
    'theia-server-windows-amd64.exe'
    'theia-server-windows-arm64.exe'
    'theia-setup-darwin-arm64'
    'theia-setup-windows-amd64.exe'
) | Sort-Object

$actual = @($entries | Where-Object { -not $_.PSIsContainer } | ForEach-Object {
    if ($_.Length -le 0) {
        throw "release asset is empty: $($_.Name)"
    }
    $_.Name
}) | Sort-Object

$difference = @(Compare-Object -ReferenceObject $expected -DifferenceObject $actual)
if ($difference.Count -gt 0) {
    $missing = @($difference | Where-Object SideIndicator -eq '<=' | ForEach-Object InputObject)
    $unexpected = @($difference | Where-Object SideIndicator -eq '=>' | ForEach-Object InputObject)
    if ($missing.Count -gt 0) { Write-Error "release staging is missing: $($missing -join ', ')" }
    if ($unexpected.Count -gt 0) { Write-Error "release staging has unexpected files: $($unexpected -join ', ')" }
    throw 'release staging does not match the public asset contract'
}

Write-Host "release staging is exact: $($actual.Count) assets for Theia $Version"
$entries | Sort-Object Name | Select-Object Name, Length
