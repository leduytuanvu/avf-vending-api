<#
.SYNOPSIS
  Run local (non-production-destructive) E2E scripts via Git Bash; tee logs under .test-runs.

.DESCRIPTION
  - Verifies /health/live is not Apache.
  - Uses repository .e2e-runs/run-* for harness output; copies key reports into latest .test-runs folder.
  - Saves as UTF-8 with BOM so Windows PowerShell 5.1 parses the script correctly.

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

function Invoke-E2EIdempotentHarnessAuthSeed {
    param(
        [Parameter(Mandatory)][string]$LogPath
    )
    $ErrorActionPreference = 'Continue'
    docker exec avf-postgres pg_isready -U postgres -d avf_vending_test 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Warning '[run-local-e2e] avf-postgres / avf_vending_test not reachable; skipping deterministic harness admin seed (ensure Docker + migrations from run-full-go-tests).'
        return
    }
    $sql = @'
INSERT INTO platform_auth_accounts (id, email, password_hash, roles, status)
VALUES (
    '77777777-7777-7777-7777-777777777777'::uuid,
    'e2e-local-admin@invalid.local',
    '$2a$10$sHkBPyzFBWrVnUJBzzjEKOSKP55F6XjgTizrIoZuBHoo7Yred4ReK',
    ARRAY['admin']::text[],
    'active'
)
ON CONFLICT (id) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    email = EXCLUDED.email,
    roles = EXCLUDED.roles,
    status = EXCLUDED.status,
    updated_at = now();
'@
    $out = @()
    $dockerExit = -1
    try {
        $out = @( $sql | docker exec -i avf-postgres psql -U postgres -d avf_vending_test -v ON_ERROR_STOP=1 2>&1 )
        $dockerExit = $LASTEXITCODE
    }
    catch {
        Write-Warning ('[run-local-e2e] harness auth seed failed: {0}' -f $_)
        return
    }
    $out | Set-Content -LiteralPath $LogPath -Encoding utf8
    if ($dockerExit -ne 0) {
        Write-Warning ('[run-local-e2e] harness auth seed psql exited {0}: see {1}' -f $dockerExit, $LogPath)
        return
    }
}

function Invoke-E2EBashStep {
    param(
        [Parameter(Mandatory)][string]$StepName,
        [Parameter(Mandatory)][string]$BashSnippet,
        [Parameter(Mandatory)][string]$LogPath
    )

    $GrpcAddr = $env:E2E_LOCAL_GRPC_ADDR
    if ([string]::IsNullOrWhiteSpace($GrpcAddr)) { $GrpcAddr = '127.0.0.1:9090' }
    # Deterministic harness user; bcrypt for password E2E_LocalDev_9c3a! (Invoke-E2EIdempotentHarnessAuthSeed).
    $AdminMail = $env:E2E_LOCAL_ADMIN_EMAIL
    if ([string]::IsNullOrWhiteSpace($AdminMail)) { $AdminMail = 'e2e-local-admin@invalid.local' }
    $AdminPass = $env:E2E_LOCAL_ADMIN_PASSWORD
    if ([string]::IsNullOrWhiteSpace($AdminPass)) { $AdminPass = 'E2E_LocalDev_9c3a!' }

    # Single bash invocation (same chaining as upstream scripts). BashRoot/BaseUrl avoid embedded single-quotes today.
    $fullCmd = "set -euo pipefail; cd '$BashRoot' && export E2E_TARGET=local && export BASE_URL='$BaseUrl' && export GRPC_ADDR='$GrpcAddr' && export GRPC_USE_REFLECTION=true && export MQTT_HOST='127.0.0.1' && export MQTT_PORT='1883' && export ADMIN_EMAIL='$AdminMail' && export ADMIN_PASSWORD='$AdminPass' && export E2E_ENABLE_FLOW_REVIEW=true && $BashSnippet"

    Write-Host ('[{0}] {1}' -f $StepName, $BashSnippet) -ForegroundColor DarkGray

    if (Test-Path -LiteralPath $LogPath) {
        Remove-Item -LiteralPath $LogPath -Force -ErrorAction SilentlyContinue
    }

    # Git Bash can write WARN lines to stderr while still exiting 0. Do not let NativeCommandError bypass $ErrorActionPreference.
    $savedEap = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & $BashExe @('-lc', $fullCmd) 2>&1 | Tee-Object -FilePath $LogPath
        $nativeExit = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $savedEap
    }

    if ($null -eq $nativeExit) {
        $nativeExit = -1
    }

    return [pscustomobject]@{
        StepName   = $StepName
        Command    = $BashSnippet
        ExitCode   = $nativeExit
        LogPath    = $LogPath
    }
}

# ---- Apache / wrong backend guard ----
$healthResp = ''
try {
    $prev = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $healthResp = curl.exe -s -i ($BaseUrl + '/health/live') 2>&1 | Out-String
    }
    finally {
        $ErrorActionPreference = $prev
    }
}
catch {
    $healthResp = $_ | Out-String
}

if ([string]::IsNullOrWhiteSpace($healthResp.Trim())) {
    Write-Host ('FATAL: Empty response from ' + $BaseUrl + '/health/live - is the API running? Try ./scripts/local/start-api-local.ps1') -ForegroundColor Red
    exit 2
}

if ($healthResp -match '(?i)Server:\s*Apache') {
    Write-Host $healthResp
    Write-Host ''
    Write-Host 'FATAL: Response looks like Apache, not this Go API. Start the backend with ./scripts/local/start-api-local.ps1 (-HttpPort 18080) and set -BaseUrl accordingly.' -ForegroundColor Red
    exit 2
}
if ($healthResp -match '(?i)^HTTP/\S+\s+404\b') {
    Write-Host $healthResp
    Write-Host ''
    Write-Host ('FATAL: /health/live returned 404 - API is not running on ' + $BaseUrl + ' (or wrong path).') -ForegroundColor Red
    exit 2
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
    Write-Host ('Reusing test artifacts folder: {0}' -f $ArtifactDir)
}
else {
    $ts = Get-Date -Format 'yyyyMMddTHHmmss'
    $ArtifactDir = Join-Path $testRunsRoot $ts
    New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
    Write-Host ('Created test artifacts folder: {0}' -f $ArtifactDir)
}

$e2eVerifyLog = Join-Path $ArtifactDir 'e2e-verify-assets.log'
$e2eRestLog = Join-Path $ArtifactDir 'e2e-rest-readonly.log'
$e2eFlowLog = Join-Path $ArtifactDir 'e2e-flow-review-static.log'
$e2eAllLog = Join-Path $ArtifactDir 'e2e-run-all-local.log'
$e2eStatus = Join-Path $ArtifactDir 'E2E_STATUS.txt'

Invoke-E2EIdempotentHarnessAuthSeed -LogPath (Join-Path $ArtifactDir 'e2e-auth-seed.log')

$stepRecords = New-Object System.Collections.Generic.List[object]

$r1 = Invoke-E2EBashStep -StepName 'verify_e2e_assets' `
    -BashSnippet './scripts/ci/verify_e2e_assets.sh' `
    -LogPath $e2eVerifyLog
[void]$stepRecords.Add($r1)

$r2 = Invoke-E2EBashStep -StepName 'run-rest-local_readonly' `
    -BashSnippet 'export E2E_ALLOW_WRITES=false; ./tests/e2e/run-rest-local.sh --readonly' `
    -LogPath $e2eRestLog
[void]$stepRecords.Add($r2)

$r3 = Invoke-E2EBashStep -StepName 'run-flow-review_static-only' `
    -BashSnippet 'export E2E_ALLOW_WRITES=false; ./tests/e2e/run-flow-review.sh --static-only' `
    -LogPath $e2eFlowLog
[void]$stepRecords.Add($r3)

$r4 = Invoke-E2EBashStep -StepName 'run-all-local_fresh-data' `
    -BashSnippet 'export E2E_ALLOW_WRITES=true; ./tests/e2e/run-all-local.sh --fresh-data' `
    -LogPath $e2eAllLog
[void]$stepRecords.Add($r4)

$exVerify = [int]$r1.ExitCode
$exRest = [int]$r2.ExitCode
$exFlow = [int]$r3.ExitCode
$exAll = [int]$r4.ExitCode

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
    param([string]$Src, [string]$Dst)
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
    Copy-IfExists (Join-Path $rep 'flow-review-scorecard.json') (Join-Path $snapshotDir 'flow-review-scorecard.json')

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

# ---- Structured severity + remediation failures (strict) ----
$sev = Get-E2EFlowImprovementSeverityTotals -SnapshotDir $snapshotDir
$remCount = Get-E2ERemediationStructuredFailureCount -RemediationMarkdownPath (Join-Path $snapshotDir 'remediation.md')

$p0blocking = ($sev.P0Count -ne -1 -and [int]$sev.P0Count -gt 0)
$p1signal = ($sev.P1Count -ne -1 -and [int]$sev.P1Count -gt 0)
$remBlocking = ($remCount -ge 0 -and [int]$remCount -gt 0)

# ---- Compose E2E_STATUS.txt ----
$stepLines = @()
foreach ($sr in $stepRecords) {
    $stepLines += ('  - {0} | exit={1} | log={2} | cmd={3}' -f $sr.StepName, $sr.ExitCode, $sr.LogPath, $sr.Command)
}
$stepsBlock = $stepLines -join "`r`n"

@"
E2E local run summary
Generated: $( Get-Date -Format o )
BASE_URL: $BaseUrl

Steps (name, native exit code, log path):
$stepsBlock

Latest .e2e-runs directory:
$( if ($latestE2E) { $latestE2E.FullName } else { '(none found)' } )

Artifacts / logs directory (.test-runs):
$ArtifactDir

Structured flow-review severity totals (preferred source: flow-review-scorecard.json):
  P0 count: $($sev.P0Count)
  P1 count: $($sev.P1Count)
  Parsed from: $($sev.Source)
  Optional path: $($sev.SourcePath)

Remediation structured failure count (-1 if unknown): $remCount

Blocking heuristic (manual review): P0_gt_0=$p0blocking P1_gt_0=$p1signal remediation_structured_gt_0=$remBlocking
"@ | Set-Content -LiteralPath $e2eStatus -Encoding utf8

Write-Host ''
Write-Host ('Latest .e2e-runs: {0}' -f ($( if ($latestE2E) { $latestE2E.FullName } else { '(none)' } )))
Write-Host ('Latest .test-runs (this session): {0}' -f $ArtifactDir)
Write-Host ('Structured P0={0} P1={1} (source={2})' -f $sev.P0Count, $sev.P1Count, $sev.Source)
Write-Host ('Remediation structured failures: {0}' -f $remCount)
Write-Host ('Wrote {0}' -f $e2eStatus)

if (-not $NoOpen) {
    if (Test-Path -LiteralPath $ArtifactDir) {
        Start-Process explorer.exe @($ArtifactDir)
    }
    if ($latestE2E -and (Test-Path -LiteralPath $latestE2E.FullName)) {
        Start-Process explorer.exe @($latestE2E.FullName)
    }
}

$overall = 0
foreach ($sr in $stepRecords) {
    if ([int]$sr.ExitCode -ne 0) {
        $overall = [int]$sr.ExitCode
        break
    }
}
if ($overall -eq 0 -and $remBlocking) {
    $overall = 2
}
if ($overall -eq 0 -and $p0blocking) {
    Write-Host 'FATAL: structured E2E reports indicate P0 > 0 while bash steps exited 0.' -ForegroundColor Red
    $overall = 2
}

exit $overall
