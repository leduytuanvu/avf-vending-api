<#
.SYNOPSIS
  Print paths and key files from the latest .test-runs and .e2e-runs directories.
.PARAMETER NoOpen
  Do not open Explorer windows.
#>
param(
    [switch] $NoOpen
)

$ErrorActionPreference = 'Stop'

$Lib = Join-Path $PSScriptRoot '_pslib.ps1'
. $Lib

$RepoRoot = Get-AvfRepoRoot -StartPath $PSScriptRoot

$testRuns = Join-Path $RepoRoot '.test-runs'
$latestTest = $null
if (Test-Path -LiteralPath $testRuns) {
    $latestTest = Get-ChildItem -LiteralPath $testRuns -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        Select-Object -First 1
}

$e2eRuns = Join-Path $RepoRoot '.e2e-runs'
$latestE2E = $null
if (Test-Path -LiteralPath $e2eRuns) {
    $latestE2E = Get-ChildItem -LiteralPath $e2eRuns -Directory -Filter 'run-*' -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
}

function Write-FileOrWarn {
    param([string] $Title, [string] $Path)
    Write-Host ""
    Write-Host "---- $Title ----" -ForegroundColor Cyan
    if (Test-Path -LiteralPath $Path) {
        Write-Host "Path: $Path"
        Get-Content -LiteralPath $Path -Encoding utf8
    } else {
        Write-Host "(missing) $Path" -ForegroundColor DarkYellow
    }
}

Write-Host "Repository: $RepoRoot"
Write-Host "Latest .test-runs: $( if ($latestTest) { $latestTest.FullName } else { '(none)' } )"
Write-Host "Latest .e2e-runs:  $( if ($latestE2E) { $latestE2E.FullName } else { '(none)' } )"

if ($latestTest) {
    $tr = $latestTest.FullName
    Write-FileOrWarn 'STATUS.txt' (Join-Path $tr 'STATUS.txt')
    Write-FileOrWarn 'go-test-packages.txt' (Join-Path $tr 'go-test-packages.txt')
    Write-FileOrWarn 'go-test-failed-tests.txt' (Join-Path $tr 'go-test-failed-tests.txt')
    Write-FileOrWarn 'E2E_STATUS.txt' (Join-Path $tr 'E2E_STATUS.txt')

    $snapSum = Join-Path $tr 'e2e-latest-snapshot\summary.md'
    $snapRem = Join-Path $tr 'e2e-latest-snapshot\remediation.md'
    Write-FileOrWarn 'e2e-latest-snapshot/summary.md' $snapSum
    Write-FileOrWarn 'e2e-latest-snapshot/remediation.md' $snapRem
} else {
    Write-Host ""
    Write-Host 'No .test-runs folders yet — run scripts/local/run-full-go-tests.ps1 first.' -ForegroundColor Yellow
}

if ($latestE2E) {
    $rep = Join-Path $latestE2E.FullName 'reports'
    $haveSnap = $false
    if ($latestTest) {
        $haveSnap = Test-Path -LiteralPath (Join-Path $latestTest.FullName 'e2e-latest-snapshot\summary.md')
    }
    if (-not $haveSnap) {
        Write-FileOrWarn 'reports/summary.md (latest .e2e-runs)' (Join-Path $rep 'summary.md')
        Write-FileOrWarn 'reports/remediation.md (latest .e2e-runs)' (Join-Path $rep 'remediation.md')
    }
}

if (-not $NoOpen) {
    if ($latestTest) {
        Start-Process explorer.exe @($latestTest.FullName)
    }
    if ($latestE2E) {
        Start-Process explorer.exe @($latestE2E.FullName)
    }
}
