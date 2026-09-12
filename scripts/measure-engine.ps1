param(
    [Parameter(Mandatory = $true)][string]$Binary,
    [Parameter(Mandatory = $true)][string]$DataDir,
    [int]$Port = 8395,
    [int]$Requests = 30,
    [string]$Output
)

$ErrorActionPreference = 'Stop'
$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
$dataPath = (Resolve-Path -LiteralPath $DataDir).Path
if ($Requests -lt 5) { throw 'Requests must be at least 5.' }

$stdout = Join-Path $env:TEMP ("theia-measure-{0}.stdout.log" -f [guid]::NewGuid())
$stderr = Join-Path $env:TEMP ("theia-measure-{0}.stderr.log" -f [guid]::NewGuid())
$arguments = "-data-dir `"$dataPath`" -port $Port"
$startup = [Diagnostics.Stopwatch]::StartNew()
$process = Start-Process -FilePath $binaryPath -ArgumentList $arguments `
    -WorkingDirectory $dataPath -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput $stdout -RedirectStandardError $stderr

try {
    $base = "http://127.0.0.1:$Port"
    $health = $null
    while ($startup.Elapsed -lt [TimeSpan]::FromSeconds(15)) {
        if ($process.HasExited) {
            throw "Theia exited during startup. See $stderr"
        }
        try {
            $health = Invoke-RestMethod -Uri "$base/api/health" -TimeoutSec 1
            if ($health.status -eq 'ok') { break }
        }
        catch { }
        Start-Sleep -Milliseconds 25
    }
    if (-not $health -or $health.status -ne 'ok') { throw 'Theia did not become healthy in 15 seconds.' }
    $startup.Stop()

    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.AutomaticDecompression = [System.Net.DecompressionMethods]::GZip
    $client = [System.Net.Http.HttpClient]::new($handler)
    $client.DefaultRequestHeaders.AcceptEncoding.ParseAdd('gzip')

    function Measure-Endpoint([string]$Name, [string]$Url) {
        foreach ($n in 1..3) {
            $warm = $client.GetAsync($Url).GetAwaiter().GetResult()
            $null = $warm.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
            $warm.Dispose()
        }
        $times = @()
        $payload = 0
        foreach ($n in 1..$Requests) {
            $timer = [Diagnostics.Stopwatch]::StartNew()
            $response = $client.GetAsync($Url).GetAwaiter().GetResult()
            $bytes = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
            $timer.Stop()
            if (-not $response.IsSuccessStatusCode) { throw "HTTP $($response.StatusCode) on $Url" }
            $times += $timer.Elapsed.TotalMilliseconds
            $payload = $bytes.Length
            $response.Dispose()
        }
        $sorted = @($times | Sort-Object)
        $p50 = $sorted[[math]::Floor(($sorted.Count - 1) * 0.50)]
        $p95 = $sorted[[math]::Floor(($sorted.Count - 1) * 0.95)]
        [pscustomobject]@{
            name = $Name
            p50_ms = [math]::Round($p50, 3)
            p95_ms = [math]::Round($p95, 3)
            mean_ms = [math]::Round(($times | Measure-Object -Average).Average, 3)
            payload_bytes = $payload
        }
    }

    $endpoints = @(
        (Measure-Endpoint 'health' "$base/api/health"),
        (Measure-Endpoint 'movies_500' "$base/api/library/movies?limit=500"),
        (Measure-Endpoint 'home_12' "$base/api/library/home?per_row=12"),
        (Measure-Endpoint 'search_heat' "$base/api/library/search?q=heat"),
        (Measure-Endpoint 'search_missing' "$base/api/library/search?q=zzzz-no-match-zzzz")
    )
    $process.Refresh()
    $file = Get-Item -LiteralPath $binaryPath
    $result = [pscustomobject]@{
        measured_at = [DateTime]::UtcNow.ToString('o')
        version = $health.version
        requests_per_endpoint = $Requests
        startup_ms = [math]::Round($startup.Elapsed.TotalMilliseconds, 3)
        binary_bytes = $file.Length
        binary_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $binaryPath).Hash
        process = [pscustomobject]@{
            working_set_bytes = $process.WorkingSet64
            private_bytes = $process.PrivateMemorySize64
            cpu_seconds = [math]::Round($process.TotalProcessorTime.TotalSeconds, 3)
        }
        endpoints = $endpoints
    }
    $json = $result | ConvertTo-Json -Depth 5
    if ($Output) { Set-Content -LiteralPath $Output -Value $json -Encoding utf8 }
    $json
}
finally {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }
    Remove-Item -LiteralPath $stdout, $stderr -Force -ErrorAction SilentlyContinue
}
