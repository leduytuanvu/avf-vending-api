param(
    [string]$ArtifactDir = "",
    [switch]$SkipVerify
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$Examples = Join-Path $ScriptDir "examples"
$PilotMachineId = "019e702c-11c6-7ab0-89c7-5eb32f0b12cb"

Write-Host "TCN cash-only pilot setup (A1-A10) via multi-file layout harness"
$args = @{
    MachineId            = $PilotMachineId
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
