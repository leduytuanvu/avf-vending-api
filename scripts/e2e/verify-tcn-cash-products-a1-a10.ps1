param(
    [string]$MachineId = "019e702c-11c6-7ab0-89c7-5eb32f0b12cb",
    [string]$ArtifactDir = "",
    [string]$ReportsDir = ""
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$Examples = Join-Path $ScriptDir "examples"

Write-Host "TCN cash-only pilot verify (A1-A10)"
$args = @{
    MachineId            = $MachineId
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
