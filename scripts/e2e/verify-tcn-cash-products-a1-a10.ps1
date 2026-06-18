param(
    [string]$MachineId = "",
    [string]$ArtifactDir = "",
    [string]$ReportsDir = ""
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$WorkspaceRoot = Split-Path $ApiRoot -Parent
$ScriptsLib = Join-Path $WorkspaceRoot "scripts\lib"
. (Join-Path $ScriptsLib "autonomous-e2e-common.ps1")
$Examples = Join-Path $ScriptDir "examples"

$TargetMachineId = if ($MachineId) { $MachineId.Trim() }
                     elseif ($env:AVF_MACHINE_ID) { $env:AVF_MACHINE_ID.Trim() }
                     elseif ($env:AVF_AUTONOMOUS_TARGET_MACHINE_ID) { $env:AVF_AUTONOMOUS_TARGET_MACHINE_ID.Trim() }
                     else { (Get-AutonomousTargetMachineId) }
Assert-AutonomousTargetMachineId -MachineId $TargetMachineId -AllowOverride

Write-Host "TCN cash-only pilot verify (A1-A10) machine=$TargetMachineId"
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
if ($ReportsDir) { $args.ReportsDir = $ReportsDir }
& (Join-Path $ScriptDir "verify-machine-sellable-layout.ps1") @args
exit $LASTEXITCODE
