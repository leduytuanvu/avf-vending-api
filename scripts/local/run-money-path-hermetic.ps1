<#
.SYNOPSIS
  Hermetic fresh-DB money-path test recipe. Creates a UNIQUE per-run Postgres database, applies goose
  migrations, runs the commerce money-path suite (vend evidence / idempotency / outbox / refund /
  reconciliation), then drops the database. No shared-DB pollution; safe to run concurrently.

.DESCRIPTION
  Why: `go test ./...` against a single shared DB can flake on cross-package interleaving and leftover
  rows. This recipe gives each run its own database so a green result is meaningful for the money path.

.PARAMETER PgContainer
  Docker container name for Postgres (default: avf-postgres, mapped host port 15432).

.PARAMETER Keep
  Keep the per-run database after the run (for debugging). Default: drop it.
#>
param(
    [string] $PgContainer = 'avf-postgres',
    [switch] $Keep
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $RepoRoot

$stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ").ToLower()
$Db = "avf_moneypath_$stamp"
$Dsn = "postgres://postgres:postgres@localhost:15432/${Db}?sslmode=disable"

Write-Host "[hermetic] Repo:     $RepoRoot"
Write-Host "[hermetic] Fresh DB: $Db"

docker exec $PgContainer psql -U postgres -c "CREATE DATABASE $Db;" | Out-Null

try {
    $env:TEST_DATABASE_URL = $Dsn
    $env:DATABASE_URL = $Dsn

    # Migrations are applied automatically by machineGRPCTestPool, but apply explicitly so a migration
    # failure is attributed here rather than inside a test.
    $MigDir = Join-Path $RepoRoot 'migrations'
    & go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir $MigDir postgres $Dsn up
    if ($LASTEXITCODE -ne 0) { throw "goose up failed ($LASTEXITCODE)" }

    # Money-path suite: commerce vend evidence (success + failure), idempotency replay, outbox,
    # refund/reconciliation. Serial (-p 1) within the fresh DB.
    $patterns = @(
        'TestMachineGRPC_Commerce_ConfirmVendSuccess',
        'TestMachineGRPC_Commerce_ReportVendSuccess',
        'TestMachineGRPC_Commerce_ReportVendFailure',
        'TestAckConfigVersion_persistsEffectiveDeviceConfigAndFieldAck'
    ) -join '|'

    go test ./internal/grpcserver/ -run $patterns -count=1 -p 1 -v
    $code = $LASTEXITCODE
    Write-Host "[hermetic] grpcserver money-path exit=$code"

    exit $code
}
finally {
    if (-not $Keep) {
        docker exec $PgContainer psql -U postgres -c "DROP DATABASE IF EXISTS $Db WITH (FORCE);" | Out-Null
        Write-Host "[hermetic] Dropped $Db"
    } else {
        Write-Host "[hermetic] Kept $Db (DSN: $Dsn)"
    }
}
