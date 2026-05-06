<#
.SYNOPSIS
  Run local (non-production-destructive) E2E scripts via Git Bash; tee logs under .test-runs.

.DESCRIPTION
  - Verifies /health/live is not Apache.
  - Uses repository .e2e-runs/run-* for harness output; copies key reports into latest .test-runs folder.
.PARAMETER BaseUrl
  API base URL (default http://127.0.0.1:18080).
.PARAMETER NoOpen
  Do not open artifact folders in Explorer.
#>
param(
    [string] $BaseUrl = 'http://127.0.0.1:18080',
    [switch] $NoOpen
)

$ErrorActionPreference = 'Stop'

$Lib = Join-Path $PSScriptRoot '_pslib.ps1'
. $Lib

$BashExe = 'C:\Program Files\Git\bin\bash.exe'
if (-not (Test-Path -LiteralPath $BashExe)) {
    throw "Git Bash not found at $BashExe. Install Git for Windows or adjust path."
}

$RepoRoot = Get-AvfRepoRoot -StartPath $PSScriptRoot
Set-Location -LiteralPath $RepoRoot

$BashRoot = Convert-ToGitBashPath $RepoRoot

# ---- Apache / wrong backend guard ----
$healthResp = ''
try {
    $healthResp = curl.exe -s -i "$BaseUrl/health/live" 2>&1 | Out-String
} catch {
    $healthResp = $_ | Out-String
}

if ([string]::IsNullOrWhiteSpace($healthResp.Trim())) {
    Write-Host ('FATAL: Empty response from ' + $BaseUrl + '/health/live — is the API running? Try ./scripts/local/start-api-local.ps1') -ForegroundColor Red
    exit 2
}

if ($healthResp -match '(?i)Server:\s*Apache') {
    Write-Host $healthResp
    Write-Host ""
    Write-Host 'FATAL: Response looks like Apache, not this Go API. Start the backend with ./scripts/local/start-api-local.ps1 (-HttpPort 18080) and set -BaseUrl accordingly.' -ForegroundColor Red
    exit 2
}
if ($healthResp -match '(?i)^HTTP/\S+\s+404' ) {
    Write-Host $healthResp
    Write-Host ""
    Write-Host "FATAL: /health/live returned 404 — API is not running on $BaseUrl (or wrong path)." -ForegroundColor Red
    exit 2
}

function Invoke-E2EBash {
    param(
        [Parameter(Mandatory)]
        [string] $Command,
        [Parameter(Mandatory)]
        [string] $LogPath
    )
    $full = "set -euo pipefail; cd '$BashRoot' && export E2E_TARGET=local && export BASE_URL='$BaseUrl' && export E2E_ENABLE_FLOW_REVIEW=true && $Command"
    Write-Host ">> $Command" -ForegroundColor DarkGray
    & $BashExe @('-lc', $full) 2>&1 | Tee-Object -FilePath $LogPath
    return $LASTEXITCODE
}

# ---- Artifact dir: reuse latest .test-runs or create ----
$testRunsRoot = Join-Path $RepoRoot '.test-runs'
if (-not (Test-Path -LiteralPath $testRunsRoot)) {
    New-Item -ItemType Directory -Path $testRunsRoot -Force | Out-Null
}
$latestExisting = Get-ChildItem -LiteralPath $testRunsRoot -Directory -ErrorAction SilentlyContinue |
    Sort-Object Name -Descending |
    Select-Object -First 1

if ($latestExisting) {
    $ArtifactDir = $latestExisting.FullName
    Write-Host "Reusing test artifacts folder: $ArtifactDir"
} else {
    $ts = Get-Date -Format 'yyyyMMddTHHmmss'
    $ArtifactDir = Join-Path $testRunsRoot $ts
    New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
    Write-Host "Created test artifacts folder: $ArtifactDir"
}

$e2eVerifyLog = Join-Path $ArtifactDir 'e2e-verify-assets.log'
$e2eRestLog = Join-Path $ArtifactDir 'e2e-rest-readonly.log'
$e2eFlowLog = Join-Path $ArtifactDir 'e2e-flow-review-static.log'
$e2eAllLog = Join-Path $ArtifactDir 'e2e-run-all-local.log'
$e2eStatus = Join-Path $ArtifactDir 'E2E_STATUS.txt'

$exVerify = Invoke-E2EBash -Command './scripts/ci/verify_e2e_assets.sh' -LogPath $e2eVerifyLog
$exRest = Invoke-E2EBash -Command 'export E2E_ALLOW_WRITES=false; ./tests/e2e/run-rest-local.sh --readonly' -LogPath $e2eRestLog
$exFlow = Invoke-E2EBash -Command 'export E2E_ALLOW_WRITES=false; ./tests/e2e/run-flow-review.sh --static-only' -LogPath $e2eFlowLog
$exAll = Invoke-E2EBash -Command 'export E2E_ALLOW_WRITES=true; ./tests/e2e/run-all-local.sh --fresh-data' -LogPath $e2eAllLog

# ---- Find newest E2E run directory ----
$e2eRunsRoot = Join-Path $RepoRoot '.e2e-runs'
$latestE2E = $null
if (Test-Path -LiteralPath $e2eRunsRoot) {
    $latestE2E = Get-ChildItem -LiteralPath $e2eRunsRoot -Directory -Filter 'run-*' -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
}

$snapshotDir = Join-Path $ArtifactDir 'e2e-latest-snapshot'
New-Item -ItemType Directory -Path $snapshotDir -Force | Out-Null

function Copy-IfExists {
    param([string] $Src, [string] $Dst)
    if (Test-Path -LiteralPath $Src) {
        Copy-Item -LiteralPath $Src -Destination $Dst -Force
    }
}

if ($latestE2E) {
    $runPath = $latestE2E.FullName
    $rep = Join-Path $runPath 'reports'
    Copy-IfExists (Join-Path $rep 'summary.md') (Join-Path $snapshotDir 'summary.md')
    Copy-IfExists (Join-Path $rep 'remediation.md') (Join-Path $snapshotDir 'remediation.md')
    Copy-IfExists (Join-Path $rep 'coverage.json') (Join-Path $snapshotDir 'coverage.json')
    Copy-IfExists (Join-Path $rep 'improvement-summary.md') (Join-Path $snapshotDir 'improvement-summary.md')
    Copy-IfExists (Join-Path $rep 'optimization-backlog.md') (Join-Path $snapshotDir 'optimization-backlog.md')

    Get-ChildItem -Path $runPath -Filter 'test-data.redacted.json' -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 |
        ForEach-Object { Copy-Item $_.FullName (Join-Path $snapshotDir 'test-data.redacted.json') -Force }

    $ev = Join-Path $runPath 'events.jsonl'
    if (Test-Path -LiteralPath $ev) {
        Get-Content -LiteralPath $ev -Tail 80 -ErrorAction SilentlyContinue |
            Set-Content -LiteralPath (Join-Path $snapshotDir 'events.jsonl.tail.txt') -Encoding utf8
    }
    $tev = Join-Path $runPath 'test-events.jsonl'
    if (Test-Path -LiteralPath $tev) {
        Get-Content -LiteralPath $tev -Tail 80 -ErrorAction SilentlyContinue |
            Set-Content -LiteralPath (Join-Path $snapshotDir 'test-events.jsonl.tail.txt') -Encoding utf8
    }
}

# ---- P0/P1 scan ----
$p0 = $false
$p1 = $false
foreach ($name in @('summary.md', 'remediation.md')) {
    $p = Join-Path $snapshotDir $name
    if (Test-Path -LiteralPath $p) {
        $txt = Get-Content -LiteralPath $p -Raw -ErrorAction SilentlyContinue
        if ($txt -match '\bP0\b') { $p0 = $true }
        if ($txt -match '\bP1\b') { $p1 = $true }
    }
}

@"
E2E local run summary
Generated: $(Get-Date -Format o)
BASE_URL: $BaseUrl

Steps (exit codes):
  verify_e2e_assets.sh          = $exVerify
  run-rest-local.sh --readonly = $exRest
  run-flow-review.sh --static   = $exFlow
  run-all-local.sh --fresh-data = $exAll

Latest .e2e-runs directory:
  $( if ($latestE2E) { $latestE2E.FullName } else { '(none found)' } )

Artifacts / logs directory (.test-runs):
  $ArtifactDir

P0 mention in snapshot summary/remediation: $p0
P1 mention in snapshot summary/remediation: $p1
"@ | Set-Content -LiteralPath $e2eStatus -Encoding utf8

Write-Host ""
Write-Host "Latest .e2e-runs: $( if ($latestE2E) { $latestE2E.FullName } else { '(none)' })"
Write-Host "Latest .test-runs (this session): $ArtifactDir"
Write-Host "P0 in reports (heuristic): $p0   P1: $p1"
Write-Host "Wrote $e2eStatus"

if (-not $NoOpen) {
    if (Test-Path -LiteralPath $ArtifactDir) {
        Start-Process explorer.exe @($ArtifactDir)
    }
    if ($latestE2E -and (Test-Path -LiteralPath $latestE2E.FullName)) {
        Start-Process explorer.exe @($latestE2E.FullName)
    }
}

$overall = 0
if ($exVerify -ne 0) { $overall = $exVerify }
elseif ($exRest -ne 0) { $overall = $exRest }
elseif ($exFlow -ne 0) { $overall = $exFlow }
elseif ($exAll -ne 0) { $overall = $exAll }
exit $overall
