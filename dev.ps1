param(
    [Parameter(Position=0)]
    [ValidateSet('run','build','clean','watch','help')]
    [string]$Task = 'help'
)

function Invoke-Run {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go not found. Install Go and ensure it's on PATH."
        exit 1
    }
    go run .
}

function Invoke-Build {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go not found. Install Go and ensure it's on PATH."
        exit 1
    }
    $out = if ($IsWindows) { 'gtnh-updater-cli.exe' } else { 'gtnh-updater-cli' }
    go build -o $out .
    Write-Host "Built $out"
}

function Invoke-Clean {
    $exe = 'gtnh-updater-cli.exe'
    $bin = 'gtnh-updater-cli'
    if (Test-Path $exe) { Remove-Item $exe -Force }
    if (Test-Path $bin) { Remove-Item $bin -Force }
    Write-Host 'Cleaned build artifacts.'
}

switch ($Task) {
    'run'   { Invoke-Run }
    'build' { Invoke-Build }
    'clean' { Invoke-Clean }
    'watch' { Invoke-Watch }
    default {
        Write-Host "Usage: .\dev.ps1 [run|build|clean|watch|help]" -ForegroundColor Yellow
    }
}


