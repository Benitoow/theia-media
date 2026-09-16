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
    # Frame the window the terminal actually shows, by title, instead of the
    # window the console host reports. Required on Windows Terminal, where the
    # console host's own window is a pseudo-console and moving it moves nothing.
    [string]$FrameTitle = '',
    [string]$ProbeSource = "$env:TEMP\theia-v33-probe\ConsoleProbe.cs"
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms, System.Drawing

Add-Type -Namespace Tui -Name Keys -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hWnd, int x, int y, int w, int h, bool repaint);
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
[StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
'@

# The window, and nothing else.
#
# This used to photograph the whole virtual screen with the console parked at
# (40,40), which cropped the picture at the right and bottom edges whenever the
# console was wider than the space left of them. That produced a picture of a
# clipped installer, and a clipped installer reads as a layout fault - it was
# reported as one, and the width guard then proved the layout was fine all along.
# So the window is placed at the origin, which is always on screen, and the
# capture is cropped to its rectangle.
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

public class TuiFrames {
    public delegate bool Proc(IntPtr h, IntPtr l);
    [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
    [DllImport("user32.dll")] public static extern int GetWindowTextW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }

    // A visible top-level window whose title contains the text, largest first.
    //
    // The console host's own window is the wrong thing to frame on Windows
    // Terminal: `mode con` sizes the buffer, the terminal draws it, and the two
    // are different windows. Framing the pseudo-console produced a correctly
    // cropped picture of a region the interface was never in - which reads as a
    // clipped installer and is not one.
    public static IntPtr Find(string title) {
        IntPtr best = IntPtr.Zero; int bestArea = 0;
        EnumWindows((h, l) => {
            if (!IsWindowVisible(h)) return true;
            var t = new StringBuilder(512); GetWindowTextW(h, t, 512);
            if (t.ToString().IndexOf(title, StringComparison.OrdinalIgnoreCase) < 0) return true;
            RECT r; if (!GetWindowRect(h, out r)) return true;
            int area = (r.Right - r.Left) * (r.Bottom - r.Top);
            if (area > bestArea) { bestArea = area; best = h; }
            return true;
        }, IntPtr.Zero);
        return best;
    }

    public static string Describe(IntPtr h) {
        var t = new StringBuilder(512); GetWindowTextW(h, t, 512);
        var c = new StringBuilder(512); GetClassNameW(h, c, 512);
        RECT r; GetWindowRect(h, out r);
        uint pid; GetWindowThreadProcessId(h, out pid);
        return "pid=" + pid + " class=" + c + " title=" + t + " rect=" + r.Left + "," + r.Top + " " + (r.Right - r.Left) + "x" + (r.Bottom - r.Top);
    }
}
'@

function Save-Screen([string]$path, [IntPtr]$window) {
    $bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
    $bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)

    if ($window -ne [IntPtr]::Zero) {
        $rect = New-Object Tui.Keys+RECT
        if ([Tui.Keys]::GetWindowRect($window, [ref]$rect)) {
            $x = [Math]::Max(0, $rect.Left - $bounds.Left)
            $y = [Math]::Max(0, $rect.Top - $bounds.Top)
            $w = [Math]::Min($bounds.Width - $x, $rect.Right - $rect.Left)
            $h = [Math]::Min($bounds.Height - $y, $rect.Bottom - $rect.Top)
            if ($w -gt 0 -and $h -gt 0) {
                $crop = New-Object System.Drawing.Rectangle($x, $y, $w, $h)
                $windowShot = $bmp.Clone($crop, $bmp.PixelFormat)
                $bmp.Dispose()
                $bmp = $windowShot
                Write-Host "window: ${w}x${h} at ($x,$y); screen $($bounds.Width)x$($bounds.Height)"
            }
        }
    }

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

# The frame the terminal actually shows, when one is named.
#
# Everything above finds the *console host's* window, which is right for a classic
# conhost and wrong under Windows Terminal: there the console host owns a
# pseudo-console, moving it moves nothing on screen, and the picture is of a
# rectangle the interface was never in. Naming the title is how the visible frame
# gets found, and it is a parameter rather than a guess because the title is the
# caller's to choose.
if ($FrameTitle -ne '') {
    $frame = [TuiFrames]::Find($FrameTitle)
    if ($frame -eq [IntPtr]::Zero) {
        Write-Warning "no visible window titled like '$FrameTitle'; the console host's window is not the one on screen under Windows Terminal"
    } else {
        Write-Host "frame: $([TuiFrames]::Describe($frame))"
        $window = $frame
    }
}

if ($window -ne [IntPtr]::Zero) {
    [Tui.Keys]::ShowWindow($window, 5) | Out-Null
    # At the origin rather than at (40,40): the corner of the screen is the one
    # place a console is never partly outside it, whatever size it ends up.
    [Tui.Keys]::MoveWindow($window, 0, 0, ($Columns * 10 + 30), ($Lines * 22 + 60), $true) | Out-Null
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

Save-Screen $Out $window
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
