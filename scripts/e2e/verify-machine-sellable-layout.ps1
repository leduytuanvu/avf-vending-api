param(
    [Parameter(ParameterSetName = "Unified")][string]$LayoutConfigPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$MachineId,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$CabinetLayoutPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$SlotAssignmentPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$InventoryPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$PaymentProfilePath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$HardwareProfilePath,
    [Parameter(ParameterSetName = "MultiFile")][string]$DestructiveScopePath = "",
    [Parameter(ParameterSetName = "MultiFile")][string]$CatalogDefaultsPath = "",
    [string]$HardwareProfile = "",
    [string]$DestructiveSlotScope = "",
    [string]$ArtifactDir = "",
    [string]$ReportsDir = ""
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$SchemaPy = Join-Path $ScriptDir "layout_config_schema.py"

function Resolve-LayoutPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    if (-not (Test-Path $Path)) { throw "layout file not found: $Path" }
    return (Resolve-Path $Path).Path
}

$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")
if (-not $ArtifactDir) {
    $runRoot = $env:MARKET_RUN_ROOT
    if (-not $runRoot) {
        $runRoot = Join-Path (Split-Path $ApiRoot -Parent) "market-readiness-runs\20260603T132538Z-tcn-cash-bill-tcn-init"
    }
    $ArtifactDir = Join-Path $runRoot "backend\verify-layout-$Ts"
}
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null

$effectiveLayout = ""
if ($PSCmdlet.ParameterSetName -eq "MultiFile" -or ($CabinetLayoutPath -and $SlotAssignmentPath)) {
    if (-not $MachineId) { throw "MachineId required for MultiFile verify" }
    $cab = Resolve-LayoutPath $CabinetLayoutPath
    $slots = Resolve-LayoutPath $SlotAssignmentPath
    $inv = Resolve-LayoutPath $InventoryPath
    $pay = Resolve-LayoutPath $PaymentProfilePath
    $hw = Resolve-LayoutPath $HardwareProfilePath
    $dest = Resolve-LayoutPath $DestructiveScopePath
    $cat = Resolve-LayoutPath $CatalogDefaultsPath
    $effectiveLayout = Join-Path $ArtifactDir "merged-layout.json"
    $mergeArgs = @(
        "merge",
        "--machine-id", $MachineId,
        "--cabinet", $cab,
        "--slots", $slots,
        "--inventory", $inv,
        "--payment", $pay,
        "--hardware", $hw,
        "-o", $effectiveLayout
    )
    if ($dest) { $mergeArgs += @("--destructive", $dest) }
    if ($cat) { $mergeArgs += @("--catalog-defaults", $cat) }
    py -3 $SchemaPy @mergeArgs
    if ($LASTEXITCODE -ne 0) { throw "layout merge failed" }
} else {
    if (-not $LayoutConfigPath) { throw "LayoutConfigPath required" }
    $effectiveLayout = Resolve-LayoutPath $LayoutConfigPath
    py -3 $SchemaPy $effectiveLayout
    if ($LASTEXITCODE -ne 0) { throw "layout validation failed" }
}

$layout = Get-Content $effectiveLayout -Raw | ConvertFrom-Json
$mid = if ($MachineId) { $MachineId } else { $layout.machine_id }
if (-not $mid) { throw "machine_id required" }

$scope = $layout.destructive_test_scope
if (-not $DestructiveSlotScope) {
    $DestructiveSlotScope = "cabinet=$($scope.cabinet);slot_indexes=$($scope.slot_indexes)"
}
if (-not $HardwareProfile) {
    $HardwareProfile = [string]$layout.hardware_profile
}

& (Join-Path $ScriptDir "diagnose-machine-sellable-layout.ps1") `
    -MachineId $mid `
    -HardwareProfile $HardwareProfile `
    -DestructiveSlotScope $DestructiveSlotScope `
    -ArtifactDir $ArtifactDir `
    -ReportsDir $ReportsDir
exit $LASTEXITCODE
