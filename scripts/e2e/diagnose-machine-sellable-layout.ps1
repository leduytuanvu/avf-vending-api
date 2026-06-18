param(
    [Parameter(Mandatory = $true)][string]$MachineId,
    [string]$HardwareProfile = "",
    [string]$DestructiveSlotScope = "cabinet=A;slot_indexes=1-10",
    [string]$ArtifactDir = "",
    [string]$ReportsDir = ""
)
$ErrorActionPreference = "Stop"
$ApiRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$Bash = "C:\Program Files\Git\bin\bash.exe"
function Parse-DestructiveScope([string]$Scope) {
    $cabinet = "A"; $slotRange = "1-10"
    foreach ($part in ($Scope -split ";")) {
        if ($part -match '^cabinet=(.+)$') { $cabinet = $Matches[1].Trim() }
        if ($part -match '^slot_indexes=(.+)$') { $slotRange = $Matches[1].Trim() }
    }
    if ($slotRange -notmatch '^(\d+)-(\d+)$') { throw "Invalid slot_indexes: $slotRange" }
    $codes = @()
    $prefix = if ($cabinet -match '^CAB-') { "A" } else { $cabinet }
    for ($i = [int]$Matches[1]; $i -le [int]$Matches[2]; $i++) { $codes += "$prefix$i" }
    return @{ Cabinet = $cabinet; SlotRange = $slotRange; SlotCodes = $codes }
}
$scope = Parse-DestructiveScope $DestructiveSlotScope
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")
if (-not $ArtifactDir) {
    $runRoot = $env:MARKET_RUN_ROOT
    if (-not $runRoot) { $runRoot = Join-Path (Split-Path $ApiRoot -Parent) "market-readiness-runs\20260603T132538Z-tcn-cash-bill-tcn-init" }
    $ArtifactDir = Join-Path $runRoot "backend\diagnose-layout-$Ts"
}
New-Item -ItemType Directory -Force -Path $ArtifactDir, (Join-Path $ArtifactDir "raw") | Out-Null
$env:E2E_TEST_MACHINE_ID = $MachineId; $env:TEST_MACHINE_ID = $MachineId
$env:E2E_TEST_SLOT_RANGE = $scope.SlotRange; $env:E2E_TEST_SLOT_CABINET = $scope.Cabinet
Write-Host "Diagnose layout machine=$MachineId dir=$ArtifactDir"
& $Bash (Join-Path $PSScriptRoot "diagnose-machine-sellable-layout-fetch.sh") $MachineId $ArtifactDir
if ($LASTEXITCODE -ne 0) { throw "fetch failed" }
$pyArgs = @((Join-Path $PSScriptRoot "diagnose_machine_sellable_layout_process.py"), "--artifact-dir", $ArtifactDir, "--machine-id", $MachineId, "--destructive-cabinet", $scope.Cabinet, "--destructive-slot-indexes", $scope.SlotRange)
if ($HardwareProfile) { $pyArgs += @("--hardware-profile", $HardwareProfile) }
py -3 @pyArgs | Write-Host
$procExit = $LASTEXITCODE
if (-not $ReportsDir) {
    if ($env:MARKET_RUN_ROOT) { $ReportsDir = Join-Path $env:MARKET_RUN_ROOT "reports" }
    else { $ReportsDir = Join-Path (Split-Path $ApiRoot -Parent) "market-readiness-runs\20260603T132538Z-tcn-cash-bill-tcn-init\reports" }
}
New-Item -ItemType Directory -Force -Path $ReportsDir | Out-Null
$summary = Get-Content (Join-Path $ArtifactDir "diagnose-summary.json") -Raw | ConvertFrom-Json
$lines = @("# Phase 2 — Dynamic machine layout and sellable catalog","","**UTC:** $Ts","**Machine:** ``$MachineId``","**Destructive scope:** $($scope.Cabinet)$($scope.SlotRange) ($(($scope.SlotCodes -join ', ')))","","## Verdict","","**$($summary.verdict)**","","| Metric | Value |","|--------|------:|","| App-facing catalog items | $($summary.app_facing_catalog_item_count) |","| Sellable slots (all machine) | $($summary.sellable_slots_all_machine_count) |","| Destructive test slots | $($summary.destructive_test_slots_count) |","| Hidden rows | $($summary.hidden_reason_count) |","| Cash-only | $($summary.cash_only_runtime) |","","## Artifacts","","``$ArtifactDir``","","## Failures","")
if ($summary.failures.Count -gt 0) { $lines += ($summary.failures | ForEach-Object { "- $_" }) } else { $lines += "- (none)" }
$lines += ""; if ($summary.verdict -eq "PASS") { $lines += "**Phase D prep:** YES" } else { $lines += "**Phase D prep:** NO" }
$lines | Set-Content (Join-Path $ReportsDir "PHASE_2_DYNAMIC_MACHINE_LAYOUT_CATALOG.md") -Encoding utf8
Write-Host "Report: $(Join-Path $ReportsDir 'PHASE_2_DYNAMIC_MACHINE_LAYOUT_CATALOG.md')"
exit $procExit
