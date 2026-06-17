param(
    [string]$ArtifactDir = "",
    [string]$MachineId = "",
    [string]$SiteId = "",
    [switch]$SkipVerify,
    [switch]$DryRun
)
# Defaults (via scripts/lib/autonomous-e2e-common.ps1):
#   MachineId -> 019eb4d7-f821-78f4-9b2c-48166006af73
#   SiteId    -> 019e550b-729d-7d30-9295-4d2bb8780203
# -DryRun exits 0 without HTTP writes. Live apply requires CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE.
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$WorkspaceRoot = Split-Path $ApiRoot -Parent
$ScriptsLib = Join-Path $WorkspaceRoot "scripts\lib"
. (Join-Path $ScriptsLib "autonomous-e2e-common.ps1")
$Examples = Join-Path $ScriptDir "examples"
$TargetMachineId = if ($MachineId) { $MachineId.Trim() }
                     elseif ($env:AVF_MACHINE_ID) { $env:AVF_MACHINE_ID.Trim() }
                     else { (Get-AutonomousTargetMachineId) }
$TargetSiteId = if ($SiteId) { $SiteId.Trim() }
                elseif ($env:AVF_SITE_ID) { $env:AVF_SITE_ID.Trim() }
                else { (Get-AutonomousTargetSiteId) }
Assert-AutonomousTargetMachineId -MachineId $TargetMachineId

if ($DryRun) {
    Write-Host "DRY-RUN: would setup TCN cash products A1-A10 for machine=$TargetMachineId site=$TargetSiteId"
    exit 0
}

if (-not $env:CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE) {
    Write-Error "Live write blocked: set CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE"
    exit 3
}

Write-Host "TCN cash-only setup (A1-A10) machine=$TargetMachineId site=$TargetSiteId"
$args = @{
    MachineId            = $TargetMachineId
    CabinetLayoutPath    = (Join-Path $Examples "pilot-cabinet-layout-a.json")
    SlotAssignmentPath   = (Join-Path $Examples "pilot-slot-assignments-a1-a10.json")
    InventoryPath        = (Join-Path $Examples "pilot-inventory-a1-a10.json")
    PaymentProfilePath   = (Join-Path $Examples "payment-profile-cash-only.json")
    HardwareProfilePath  = (Join-Path $Examples "hardware-profile-tcn.json")
    DestructiveScopePath = (Join-Path $Examples "destructive-scope-a1-a10.json")
    CatalogDefaultsPath  = (Join-Path $Examples "pilot-catalog-defaults.json")
}
if ($ArtifactDir) { $args.ArtifactDir = $ArtifactDir }
if ($SkipVerify) { $args.SkipVerify = $true }
& (Join-Path $ScriptDir "setup-machine-sellable-layout.ps1") @args
exit $LASTEXITCODE
