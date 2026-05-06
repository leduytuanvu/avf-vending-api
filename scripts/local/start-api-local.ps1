<#
.SYNOPSIS
  Start cmd/api with local development defaults (HTTP on 18080 by default).

.DESCRIPTION
  Resolves repository root via go.mod, sets APP_ENV=development and local disposable DATABASE_URL.
  If HttpPort is already listening, prints owning process and exits without starting the API.
.PARAMETER HttpPort
  HTTP listen port (default 18080 — avoids common Apache on 8080).
.PARAMETER GrpcPort
  gRPC listen port (default 9090).
.PARAMETER Force
  If set, skip the "port already in use" guard (not recommended).
#>
param(
    [int] $HttpPort = 18080,
    [int] $GrpcPort = 9090,
    [switch] $Force
)

$ErrorActionPreference = 'Stop'

$Lib = Join-Path $PSScriptRoot '_pslib.ps1'
. $Lib

$RepoRoot = Get-AvfRepoRoot -StartPath $PSScriptRoot
Set-Location -LiteralPath $RepoRoot

function Get-ListenersOnPort {
    param([int] $Port)
    try {
        Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
            Select-Object -ExpandProperty OwningProcess -Unique
    } catch {
        @()
    }
}

if (-not $Force) {
    $pids = @( Get-ListenersOnPort -Port $HttpPort )
    if ($pids.Count -gt 0) {
        Write-Host "Port $HttpPort is already in use. Owning process(es):" -ForegroundColor Yellow
        foreach ($procId in $pids) {
            if ($procId -gt 0) {
                try {
                    Get-Process -Id $procId -ErrorAction Stop | Format-List Id, ProcessName, Path
                } catch {
                    Write-Host "  PID $procId (process metadata unavailable)"
                }
            }
        }
        Write-Host "Refusing to start a second API on :$HttpPort (use -Force to override)." -ForegroundColor Yellow
        exit 1
    }
}

$env:APP_ENV = 'development'
$env:HTTP_ADDR = ":$HttpPort"
$env:PUBLIC_BASE_URL = "http://127.0.0.1:$HttpPort"
$env:BASE_URL = "http://127.0.0.1:$HttpPort"
$env:DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15432/avf_vending_test?sslmode=disable'
$env:TEST_DATABASE_URL = $env:DATABASE_URL
$env:GRPC_ADDR = ":$GrpcPort"
$env:GRPC_ENABLED = 'true'
$env:MACHINE_GRPC_ENABLED = 'true'
$env:GRPC_REFLECTION_ENABLED = 'true'
$env:HTTP_AUTH_JWT_SECRET = 'local-dev-http-secret-at-least-32-bytes-change-me'
$env:MACHINE_JWT_SECRET = 'local-dev-machine-secret-at-least-32-bytes-change-me'
$env:REDIS_ENABLED = 'true'
$env:REDIS_URL = 'redis://127.0.0.1:6379/0'
$env:NATS_URL = 'nats://127.0.0.1:4222'
$env:MQTT_BROKER_URL = 'tcp://127.0.0.1:1883'
$env:MQTT_CLIENT_ID_API = 'avf-api-local'
$env:MQTT_TOPIC_PREFIX = 'avf/devices'
$env:ENABLE_LEGACY_MACHINE_HTTP = 'true'
$env:MACHINE_REST_LEGACY_ENABLED = 'true'

Write-Host "Starting API from: $RepoRoot"
Write-Host "HTTP_ADDR=$($env:HTTP_ADDR)  GRPC_ADDR=$($env:GRPC_ADDR)"
Write-Host ""
Write-Host "Sanity checks (backend, not Apache on 8080):" -ForegroundColor Cyan
Write-Host "  curl.exe -i http://127.0.0.1:$HttpPort/health/live"
Write-Host "  curl.exe -i http://127.0.0.1:$HttpPort/health/ready"
Write-Host "  curl.exe -i http://127.0.0.1:$HttpPort/version"
Write-Host ""

& go run ./cmd/api
