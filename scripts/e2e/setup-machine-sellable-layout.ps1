param(
    [Parameter(ParameterSetName = "Unified")][string]$LayoutConfigPath = "",
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$MachineId,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$CabinetLayoutPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$SlotAssignmentPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$InventoryPath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$PaymentProfilePath,
    [Parameter(ParameterSetName = "MultiFile", Mandatory = $true)][string]$HardwareProfilePath,
    [Parameter(ParameterSetName = "MultiFile")][string]$DestructiveScopePath = "",
    [Parameter(ParameterSetName = "MultiFile")][string]$CatalogDefaultsPath = "",
    [string]$ArtifactDir = "",
    [switch]$SkipVerify
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$Bash = "C:\Program Files\Git\bin\bash.exe"
if (-not (Test-Path $Bash)) { $Bash = "bash" }
$SchemaPy = Join-Path $ScriptDir "layout_config_schema.py"
$ExamplesDir = Join-Path $ScriptDir "examples"

function Resolve-LayoutPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    if (-not (Test-Path $Path)) { throw "layout file not found: $Path" }
    return (Resolve-Path $Path).Path
}

function Merge-LayoutManifest {
    param(
        [string]$Mid,
        [string]$Cabinet,
        [string]$Slots,
        [string]$Inventory,
        [string]$Payment,
        [string]$Hardware,
        [string]$Destructive,
        [string]$Catalog,
        [string]$OutPath
    )
    $mergeArgs = @(
        "merge",
        "--machine-id", $Mid,
        "--cabinet", $Cabinet,
        "--slots", $Slots,
        "--inventory", $Inventory,
        "--payment", $Payment,
        "--hardware", $Hardware,
        "-o", $OutPath
    )
    if ($Destructive) { $mergeArgs += @("--destructive", $Destructive) }
    if ($Catalog) { $mergeArgs += @("--catalog-defaults", $Catalog) }
    py -3 $SchemaPy @mergeArgs
    if ($LASTEXITCODE -ne 0) { throw "layout merge failed" }
}

$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")
if (-not $ArtifactDir) {
    $ArtifactDir = Join-Path $ApiRoot "reports\e2e\setup-machine-layout\$Ts"
}
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null

$effectiveLayout = ""
if ($PSCmdlet.ParameterSetName -eq "Unified") {
    if (-not $LayoutConfigPath) { throw "LayoutConfigPath required for Unified parameter set" }
    $effectiveLayout = Resolve-LayoutPath $LayoutConfigPath
    Write-Host "Validate unified layout: $effectiveLayout"
    py -3 $SchemaPy $effectiveLayout
    if ($LASTEXITCODE -ne 0) { throw "layout validation failed" }
} else {
    $cab = Resolve-LayoutPath $CabinetLayoutPath
    $slots = Resolve-LayoutPath $SlotAssignmentPath
    $inv = Resolve-LayoutPath $InventoryPath
    $pay = Resolve-LayoutPath $PaymentProfilePath
    $hw = Resolve-LayoutPath $HardwareProfilePath
    $dest = Resolve-LayoutPath $DestructiveScopePath
    $cat = Resolve-LayoutPath $CatalogDefaultsPath
    $effectiveLayout = Join-Path $ArtifactDir "merged-layout.json"
    Write-Host "Merge multi-file layout -> $effectiveLayout"
    Merge-LayoutManifest -Mid $MachineId -Cabinet $cab -Slots $slots -Inventory $inv -Payment $pay -Hardware $hw -Destructive $dest -Catalog $cat -OutPath $effectiveLayout
}

$layout = Get-Content $effectiveLayout -Raw | ConvertFrom-Json
$mid = if ($MachineId) { $MachineId } else { $layout.machine_id }
if (-not $mid) { throw "machine_id required" }

$env:E2E_ALLOW_WRITES = "true"
$env:E2E_PRODUCTION_WRITE_CONFIRMATION = "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION"
$env:E2E_RUN_DIR = $ArtifactDir
$env:E2E_RUN_TS = $Ts
$env:TEST_MACHINE_ID = $mid
$env:E2E_TEST_MACHINE_ID = $mid

Write-Host "Apply layout machine=$mid artifact=$ArtifactDir"
$bashEnv = @(
    "E2E_ALLOW_WRITES=true",
    "E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION",
    "E2E_RUN_DIR=$($ArtifactDir -replace '\\','/')",
    "E2E_RUN_TS=$Ts",
    "TEST_MACHINE_ID=$mid",
    "E2E_TEST_MACHINE_ID=$mid"
) -join " "
$applySh = (Join-Path $ScriptDir "setup-machine-sellable-layout-apply.sh") -replace '\\','/'
$layoutUnix = $effectiveLayout -replace '\\','/'
& $Bash -lc "$bashEnv bash '$applySh' '$layoutUnix'"
if ($LASTEXITCODE -ne 0) {
    $readiness = Join-Path $ArtifactDir "SETUP_READINESS.txt"
    if (Test-Path $readiness) { Get-Content $readiness | Write-Host }
    throw "setup apply failed exit=$LASTEXITCODE"
}

if (-not $SkipVerify) {
    Write-Host "Verify sellable layout"
    $scope = $layout.destructive_test_scope
    $destructive = "cabinet=$($scope.cabinet);slot_indexes=$($scope.slot_indexes)"
    & (Join-Path $ScriptDir "verify-machine-sellable-layout.ps1") `
        -LayoutConfigPath $effectiveLayout `
        -DestructiveSlotScope $destructive `
        -ArtifactDir (Join-Path $ArtifactDir "verify-$Ts")
    if ($LASTEXITCODE -ne 0) { throw "verify failed exit=$LASTEXITCODE" }
}

$readiness = Join-Path $ArtifactDir "SETUP_READINESS.txt"
if (Test-Path $readiness) { Get-Content $readiness | Write-Host }
Write-Host "Setup complete artifact=$ArtifactDir"
exit 0
