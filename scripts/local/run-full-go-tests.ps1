<#
.SYNOPSIS
  Full local Go test workflow: Docker deps, clean test DB, goose migrations, go test -short then full -json.

.DESCRIPTION
  Always changes directory to the repository root (detected via go.mod).
  Artifacts: .test-runs/<yyyyMMddTHHmmss>/

.PARAMETER NoOpen
  Do not open the artifact folder in Explorer when finished.
#>
param(
    [switch] $NoOpen
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

$Lib = Join-Path $PSScriptRoot '_pslib.ps1'
. $Lib

$RepoRoot = Get-AvfRepoRoot -StartPath $PSScriptRoot
Set-Location -LiteralPath $RepoRoot

$ts = Get-Date -Format 'yyyyMMddTHHmmss'
$RunDir = Join-Path $RepoRoot ".test-runs\$ts"
New-Item -ItemType Directory -Path $RunDir -Force | Out-Null

Write-Host "[run-full-go-tests] Repository root: $RepoRoot"
Write-Host "[run-full-go-tests] Artifact folder:  $RunDir"

# ---- Docker (broker profile: EMQX, MinIO, etc.) ----
$dcFile = Join-Path $RepoRoot 'deployments\docker\docker-compose.yml'
docker compose -f $dcFile --profile broker up -d 2>&1 | Tee-Object -FilePath (Join-Path $RunDir 'docker-up.log')

try {
    docker ps -a 2>&1 | Tee-Object -FilePath (Join-Path $RunDir 'docker-ps.log')
} catch {
    '' | Set-Content (Join-Path $RunDir 'docker-ps.log')
}

# ---- Postgres ready ----
$pgLog = Join-Path $RunDir 'postgres-ready.log'
$pgReady = $false
for ($i = 0; $i -lt 90; $i++) {
    $line = try {
        docker exec avf-postgres pg_isready -U postgres 2>&1
    } catch {
        $_ | Out-String
    }
    "$(Get-Date -Format o) $line" | Add-Content -LiteralPath $pgLog -Encoding utf8
    if ($LASTEXITCODE -eq 0) {
        $pgReady = $true
        break
    }
    Start-Sleep -Seconds 2
}
if (-not $pgReady) {
    Write-Warning "Postgres not ready after wait; see postgres-ready.log"
}

# ---- Redis ----
try {
    docker exec avf-redis redis-cli PING 2>&1 | Tee-Object -FilePath (Join-Path $RunDir 'redis-ping.log')
} catch {
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunDir 'redis-ping.log') -Encoding utf8
}

# ---- NATS monitoring ----
try {
    $natsUri = 'http://127.0.0.1:8222/healthz'
    $resp = Invoke-WebRequest -Uri $natsUri -UseBasicParsing -TimeoutSec 5
    "HTTP $($resp.StatusCode)`n$($resp.Content)" | Set-Content -LiteralPath (Join-Path $RunDir 'nats-health.log') -Encoding utf8
} catch {
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunDir 'nats-health.log') -Encoding utf8
}

# ---- Reset DB ----
$DbResetLog = Join-Path $RunDir 'db-reset.log'
$dbUrl = 'postgres://postgres:postgres@127.0.0.1:15432/avf_vending_test?sslmode=disable'
@"
DATABASE_URL (local disposable test DB):
$dbUrl
"@ | Set-Content -LiteralPath $DbResetLog -Encoding utf8

$resetSql = @'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'avf_vending_test' AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS avf_vending_test;
CREATE DATABASE avf_vending_test;
'@

try {
    $resetSql | docker exec -i avf-postgres psql -U postgres -d postgres -v ON_ERROR_STOP=1 2>&1 | Tee-Object -FilePath $DbResetLog -Append
} catch {
    $_ | Out-String | Tee-Object -FilePath $DbResetLog -Append
    Write-Warning "Database reset may have failed; check db-reset.log and Docker postgres container."
}

$env:DATABASE_URL = $dbUrl
$env:TEST_DATABASE_URL = $dbUrl

$MigDir = Join-Path $RepoRoot 'migrations'
$gooseUpArgs = @(
    'run', 'github.com/pressly/goose/v3/cmd/goose@v3.27.0',
    '-dir', $MigDir,
    'postgres', $env:DATABASE_URL,
    'up'
)
& go @gooseUpArgs 2>&1 | Tee-Object -FilePath (Join-Path $RunDir 'goose-up.log')
if ($LASTEXITCODE -ne 0) {
    throw "goose up failed with exit code $LASTEXITCODE (see goose-up.log)"
}

$gooseStatusArgs = @(
    'run', 'github.com/pressly/goose/v3/cmd/goose@v3.27.0',
    '-dir', $MigDir,
    'postgres', $env:DATABASE_URL,
    'status'
)
& go @gooseStatusArgs 2>&1 | Tee-Object -FilePath (Join-Path $RunDir 'goose-status.log')
# status is informational; do not fail the script on non-zero exit

go clean -testcache

$ShortJsonl = Join-Path $RunDir 'go-test-short.jsonl'
go test -short ./... -count=1 -json 2>&1 | Tee-Object -FilePath $ShortJsonl
$exitShort = $LASTEXITCODE

$FullJsonl = Join-Path $RunDir 'go-test-full.jsonl'
go test ./... -count=1 -json 2>&1 | Tee-Object -FilePath $FullJsonl
$exitFull = $LASTEXITCODE

$csv = Join-Path $RunDir 'go-test-packages.csv'
$txt = Join-Path $RunDir 'go-test-packages.txt'
$failedList = Join-Path $RunDir 'go-test-failed-tests.txt'
Export-GoTestJsonlSummary -JsonlPath $FullJsonl -OutCsv $csv -OutTxt $txt -OutFailed $failedList

$statusPath = Join-Path $RunDir 'STATUS.txt'
@"
AVF full Go test run
Generated (local): $(Get-Date -Format o)
Repository: $RepoRoot
Artifact directory: $RunDir

Disposable test database (non-secret local default):
  DATABASE_URL = $dbUrl

Exit codes:
  go test -short ./... -count=1  = $exitShort
  go test ./... -count=1         = $exitFull

Summary files:
  go-test-packages.txt / .csv / go-test-failed-tests.txt  (from full run JSONL)
"@ | Set-Content -LiteralPath $statusPath -Encoding utf8

Write-Host ""
Write-Host "STATUS.txt written. Exit codes: short=$exitShort full=$exitFull"
Write-Host "Artifacts: $RunDir"
if (-not $NoOpen) {
    Start-Process explorer.exe -ArgumentList $RunDir
}

exit $(if ($exitFull -ne 0) { $exitFull } elseif ($exitShort -ne 0) { $exitShort } else { 0 })
