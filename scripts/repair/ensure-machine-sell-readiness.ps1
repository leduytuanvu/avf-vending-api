param(
    [string]$LayoutJson = "",
    [string]$MachineId = "",
    [string]$SiteId = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$ApiRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$WorkspaceRoot = Split-Path $ApiRoot -Parent
$ScriptsLib = Join-Path $WorkspaceRoot "scripts\lib"
. (Join-Path $ScriptsLib "autonomous-e2e-common.ps1")

$TargetMachineId = if ($MachineId) { $MachineId.Trim() }
                   elseif ($env:AVF_MACHINE_ID) { $env:AVF_MACHINE_ID.Trim() }
                   else { (Get-AutonomousTargetMachineId) }
$TargetSiteId = if ($SiteId) { $SiteId.Trim() }
                elseif ($env:AVF_SITE_ID) { $env:AVF_SITE_ID.Trim() }
                else { (Get-AutonomousTargetSiteId) }
Assert-AutonomousTargetMachineId -MachineId $TargetMachineId

$ApiRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$ApplyScript = Join-Path $ApiRoot "scripts/e2e/setup-machine-sellable-layout-apply.sh"

if ([string]::IsNullOrWhiteSpace($LayoutJson)) {
    $LayoutJson = Join-Path $ApiRoot "tests/e2e/fixtures/tcn-cash-only-slots-a1-a10.layout.json"
}

if ($DryRun) {
    Write-Host "DRY-RUN: would apply sell readiness layout=$LayoutJson machine=$TargetMachineId site=$TargetSiteId"
    Write-Host "DRY-RUN complete (no writes)."
    exit 0
}

if (-not (Test-Path $ApplyScript)) {
    throw "Missing $ApplyScript"
}

if (-not (Test-Path $LayoutJson)) {
    Write-Warning "Layout fixture not found: $LayoutJson"
    Write-Host "Run repair-machine-bootstrap-metadata.ps1 first; provide -LayoutJson for sell readiness."
    exit 0
}

if (-not $env:CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE) {
    Write-Error "Live write blocked: set CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE"
    exit 3
}

$env:E2E_ALLOW_WRITES = "true"
$env:TEST_MACHINE_ID = $TargetMachineId
$env:AVF_MACHINE_ID = $TargetMachineId
$env:AVF_SITE_ID = $TargetSiteId
$env:BASE_URL = if ($env:AVF_BASE_URL) { $env:AVF_BASE_URL } else { "https://api.ldtv.dev" }

Write-Host "Applying sell-ready layout machine=$TargetMachineId via $ApplyScript"
& bash $ApplyScript $LayoutJson
if ($LASTEXITCODE -ne 0) { throw "Sell readiness apply failed exit=$LASTEXITCODE" }
Write-Host "Sell readiness script completed."
