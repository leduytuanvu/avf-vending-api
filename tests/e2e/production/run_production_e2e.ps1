# Production release E2E harness (PowerShell wrapper)
# Usage:
#   .\tests\e2e\production\run_production_e2e.ps1 -Mode contract -DryRun
param(
    [ValidateSet('contract', 'preflight', 'live', 'route-matrix')]
    [string]$Mode = 'contract',
    [ValidateSet('all', 'rest', 'grpc', 'mqtt')]
    [string]$Suite = 'all',
    [switch]$DryRun,
    [switch]$SkipNewman
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..' '..' '..')).Path
$Bash = 'C:\Program Files\Git\bin\bash.exe'
if (-not (Test-Path $Bash)) {
    $Bash = (Get-Command bash -ErrorAction SilentlyContinue).Source
}
if (-not $Bash) {
    throw 'Git Bash required to run production E2E harness (bash scripts).'
}

$argsList = @('tests/e2e/production/run_production_e2e.sh', '--mode', $Mode, '--suite', $Suite)
if ($DryRun) { $argsList += '--dry-run' }
if ($SkipNewman) { $argsList += '--skip-newman' }

Push-Location $RepoRoot
try {
    $mosquitto = 'C:\Program Files\mosquitto'
    if ((Test-Path $mosquitto) -and ($env:PATH -notlike "*$mosquitto*")) {
        $env:PATH = "$mosquitto;$env:PATH"
    }
    & $Bash @argsList
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
