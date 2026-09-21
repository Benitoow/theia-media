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

$expected = @(
    "theia-$Version-windows-amd64.zip"
    'theia-launcher-windows-amd64.exe'
    'theia-player-windows-amd64.zip'
    'theia-server-darwin-amd64'
    'theia-server-darwin-arm64'
    'theia-server-linux-amd64'
    'theia-server-linux-arm64'
    'theia-server-windows-amd64.exe'
    'theia-server-windows-arm64.exe'
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
