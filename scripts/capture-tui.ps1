# Captures the installer's terminal interface as a person sees it.
#
# Used during development to look at the real thing: the form runs in a real
# console window, the screen is photographed, and the process is closed again.
# The Go tests drive the model with key messages, which is precise but has no
# colours and no layout; this is the other half.
#
# Two traps this script exists to avoid, both of which produced a colourless
# picture of a colourful interface:
#
#   * `NO_COLOR` in the developer's own shell. termenv honours it, lipgloss drops
#     every colour, and the screenshot then shows a monochrome product nobody
#     runs. A capture shell is not a user's shell: the variable is removed here.
#   * `TERM` and `COLORTERM` absent, which makes a terminal that can do
#     truecolor describe itself as a dumb one.
#
# Use -Probe to also read the console buffer back, cell by cell, which is the
# only way to tell "the theme asked for no colour" from "the terminal refused
# it" - a screenshot cannot answer that, because both look grey.

param(
    [string]$Exe = 'C:\Users\starx\Documents\Code\Theia\theia-setup.exe',
    [string]$Arguments = '',
    [int]$WaitSeconds = 3,
    [string]$Out = "$env:TEMP\theia-tui-shot.png",
    [string[]]$Keys = @(),
    # A fixed console size, so the window and the buffer agree and a screenshot
    # shows what the layout really does at a known width.
    [int]$Columns = 100,
    [int]$Lines = 30,
    # Read the console buffer and describe the colours actually drawn.
    [switch]$Probe,
    [string]$ProbeSource = "$env:TEMP\theia-v33-probe\ConsoleProbe.cs"
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms, System.Drawing

Add-Type -Namespace Tui -Name Keys -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hWnd, int x, int y, int w, int h, bool repaint);
'@

function Save-Screen([string]$path) {
    $bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
    $bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bmp.Dispose()
}

# A launcher rather than the executable directly: `mode con` is how a console is
# sized, and it has to run inside the console that will host the program.
$launcher = Join-Path $env:TEMP 'theia-tui-launch.cmd'
$argumentLine = $Arguments
@"
@echo off
mode con: cols=$Columns lines=$Lines
title Theia installer
"$Exe" $argumentLine
"@ | Set-Content -Path $launcher -Encoding ascii

# A colour-capable terminal, as a person's Windows Terminal would be, and no
# NO_COLOR inherited from the shell doing the capturing.
Remove-Item Env:\NO_COLOR -ErrorAction SilentlyContinue
$env:TERM = 'xterm-256color'
$env:COLORTERM = 'truecolor'

$started = Get-Date
$process = Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', $launcher -PassThru
Start-Sleep -Seconds $WaitSeconds

# The program under the console host, which is what the probe attaches to.
$name = [IO.Path]::GetFileNameWithoutExtension($Exe)
function Get-Target {
    Get-Process -Name $name -ErrorAction SilentlyContinue |
        Where-Object { $_.StartTime -ge $started } | Select-Object -First 1
}
$target = Get-Target

# The window belongs to the console host, so a console program's own
# MainWindowHandle is always zero. Attaching to its console is the only way to
# find the window this script is supposed to frame.
$window = [IntPtr]::Zero
if (Test-Path $ProbeSource) {
    Add-Type -Path $ProbeSource
    if ($target) {
        $handle = [ConsoleProbe]::Window([uint32]$target.Id)
        if ($handle -ne 0) { $window = [IntPtr]$handle }
    }
} else {
    Write-Warning "probe source not found, cannot find the console window: $ProbeSource"
}
if ($window -eq [IntPtr]::Zero) {
    $launcher = Get-Process -Id $process.Id -ErrorAction SilentlyContinue
    if ($launcher -and $launcher.MainWindowHandle -ne [IntPtr]::Zero) { $window = $launcher.MainWindowHandle }
}
if ($window -eq [IntPtr]::Zero -and $target) {
    if ($target.MainWindowHandle -ne [IntPtr]::Zero) { $window = $target.MainWindowHandle }
}
if ($window -eq [IntPtr]::Zero) {
    $host = Get-Process -Name conhost -ErrorAction SilentlyContinue |
        Where-Object { $_.StartTime -ge $started } |
        Sort-Object StartTime -Descending | Select-Object -First 1
    if ($host -and $host.MainWindowHandle -ne [IntPtr]::Zero) { $window = $host.MainWindowHandle }
}
if ($window -ne [IntPtr]::Zero) {
    [Tui.Keys]::ShowWindow($window, 5) | Out-Null
    [Tui.Keys]::MoveWindow($window, 40, 40, ($Columns * 10 + 30), ($Lines * 22 + 60), $true) | Out-Null
    [Tui.Keys]::SetForegroundWindow($window) | Out-Null
    Start-Sleep -Milliseconds 700
} else {
    Write-Warning 'no console window found; the screenshot is of the whole screen'
}

if ($Keys.Count -gt 0) {
    foreach ($key in $Keys) {
        [System.Windows.Forms.SendKeys]::SendWait($key)
        Start-Sleep -Milliseconds 600
    }
    Start-Sleep -Seconds 1
}

Save-Screen $Out
Write-Host "capture: $Out"

if ($Probe) {
    if (-not (Test-Path $ProbeSource)) {
        Write-Warning "probe source not found: $ProbeSource"
    } elseif (-not $target) {
        Write-Warning 'the program is not running any more; nothing to probe'
    } else {
        Write-Host "--- console text (proves which buffer was read) ---"
        [ConsoleProbe]::Dump($target.Id) | Select-Object -First 8 | ForEach-Object { Write-Host $_ }
        Write-Host "--- colours drawn ---"
        [ConsoleProbe]::Describe($target.Id) | ForEach-Object { Write-Host $_ }
    }
}

# Leave by the front door: escape cancels the form, the program exits, the
# launcher ends and the console closes itself. Killing the process instead leaves
# a dead tab behind in whatever terminal is hosting it, and a console host sweep
# would take the maintainer's own tabs down with it.
$remaining = Get-Target
if ($remaining) {
    [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
    Start-Sleep -Milliseconds 900
    $remaining = Get-Target
}
if ($remaining) {
    Write-Warning "the program did not exit on escape; killing PID $($remaining.Id)"
    Stop-Process -Id $remaining.Id -Force -ErrorAction SilentlyContinue
}
Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
Remove-Item $launcher -ErrorAction SilentlyContinue
