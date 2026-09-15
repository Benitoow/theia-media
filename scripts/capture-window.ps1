# Launches a windowed program, brings it to the front, and photographs it.
#
# The sibling of capture-tui.ps1, for the artifacts that draw a window rather than
# a terminal: the player, and anything else with a WebView in it. It exists
# because photographing the player by hand produced a picture that looked like a
# layout fault and was not one.
#
# **The trap it was written for is DPI.** This machine scales at 200%, and a
# process that is not DPI-aware asking Windows for a 1100-pixel window gets one
# 2200 pixels wide: the picture is then clipped by the screen edge while the
# program inside has laid itself out perfectly for 1100. Three measurements in
# this session were wrong that way - a window edge mistaken for an overflow - and
# the OSD's own render check, which measures the DOM rather than the pixels,
# disagreed and was right. So this script declares per-monitor awareness *before*
# loading anything that touches a window, resizes in real pixels, and prints the
# DPI it found.
#
# Bringing a window forward is the second trap. SetForegroundWindow is refused
# unless the caller already owns the foreground, so the script attaches to the
# foreground thread first - the documented workaround - instead of leaving a
# translucent terminal across the picture.

param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [string]$Arguments = '',
    [int]$Width = 1100,
    [int]$Height = 700,
    [int]$WaitSeconds = 7,
    [string]$Out = "$env:TEMP\theia-window.png",
    # Photograph the whole screen rather than the window, to judge what a person
    # actually sees - including whatever is in front.
    [switch]$FullScreen,
    # Leave the program running instead of closing it when the picture is taken.
    [switch]$Keep
)

$ErrorActionPreference = 'Stop'

# Before System.Windows.Forms, before anything that asks the system how big the
# screen is: after that, the awareness is already settled and cannot be changed.
Add-Type -Namespace Dpi -Name Aware -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetProcessDpiAwarenessContext(IntPtr context);
'@
# DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
[Dpi.Aware]::SetProcessDpiAwarenessContext([IntPtr](-4)) | Out-Null

Add-Type -AssemblyName System.Windows.Forms, System.Drawing

Add-Type -Namespace Shot -Name Win -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hWnd, int x, int y, int w, int h, bool repaint);
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
[DllImport("user32.dll")] public static extern bool AttachThreadInput(uint attach, uint attachTo, bool fAttach);
[DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT r);
[DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hWnd);
public struct RECT { public int Left, Top, Right, Bottom; }
'@

$started = Get-Date
if ($Arguments -ne '') {
    $process = Start-Process -FilePath $Exe -ArgumentList $Arguments -PassThru
} else {
    $process = Start-Process -FilePath $Exe -PassThru
}
Start-Sleep -Seconds $WaitSeconds

$target = Get-Process -Id $process.Id -ErrorAction SilentlyContinue
if (-not $target) {
    throw "the program exited before it could be photographed: $Exe"
}
$window = $target.MainWindowHandle
if ($window -eq [IntPtr]::Zero) {
    throw "no window: $Exe has none, or it never drew one in $WaitSeconds seconds"
}

$dpi = [Shot.Win]::GetDpiForWindow($window)
Write-Host "pid $($target.Id)  window $window  dpi $dpi  (scale $([math]::Round($dpi / 96 * 100))%)"

if (-not $FullScreen) {
    [Shot.Win]::ShowWindow($window, 9) | Out-Null   # SW_RESTORE
    [Shot.Win]::MoveWindow($window, 40, 40, $Width, $Height, $true) | Out-Null
}

# Take the foreground by attaching to whoever holds it: an unattached call is
# refused, and the picture then shows whatever was in front instead.
$foreground = [Shot.Win]::GetForegroundWindow()
$holder = 0
$holderThread = [Shot.Win]::GetWindowThreadProcessId($foreground, [ref]$holder)
$mine = [Shot.Win]::GetCurrentThreadId()
[Shot.Win]::AttachThreadInput($mine, $holderThread, $true) | Out-Null
[Shot.Win]::BringWindowToTop($window) | Out-Null
[Shot.Win]::SetForegroundWindow($window) | Out-Null
[Shot.Win]::AttachThreadInput($mine, $holderThread, $false) | Out-Null
Start-Sleep -Milliseconds 900

$rect = New-Object Shot.Win+RECT
[Shot.Win]::GetWindowRect($window, [ref]$rect) | Out-Null
$windowWidth = $rect.Right - $rect.Left
$windowHeight = $rect.Bottom - $rect.Top
Write-Host "window rect $($rect.Left),$($rect.Top) -> $($rect.Right),$($rect.Bottom)  ($windowWidth x $windowHeight real pixels, $([math]::Round($windowWidth / ($dpi / 96))) CSS pixels)"

if ($FullScreen) {
    $area = [System.Windows.Forms.SystemInformation]::VirtualScreen
    $rect.Left = $area.Left; $rect.Top = $area.Top
    $windowWidth = $area.Width; $windowHeight = $area.Height
}
if ($rect.Left -lt 0) { $rect.Left = 0 }
if ($rect.Top -lt 0) { $rect.Top = 0 }

$bitmap = New-Object System.Drawing.Bitmap($windowWidth, $windowHeight)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bitmap.Size)
$bitmap.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose(); $bitmap.Dispose()
Write-Host "capture: $Out"

if (-not $Keep) {
    Stop-Process -Id $target.Id -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 400
    if (Get-Process -Id $target.Id -ErrorAction SilentlyContinue) {
        Write-Warning "pid $($target.Id) is still running"
    }
}
