# Deterministic metadata contract validation tests (no HTTP writes).
$ErrorActionPreference = "Stop"
$TestDir = $PSScriptRoot
$E2eDir = Split-Path $TestDir -Parent
$ApiRoot = Split-Path (Split-Path $E2eDir -Parent) -Parent
$WorkspaceRoot = Split-Path $ApiRoot -Parent
$SchemaPy = Join-Path $E2eDir "layout_config_schema.py"
$Fixtures = Join-Path $TestDir "fixtures"
$ScriptsLib = Join-Path $WorkspaceRoot "scripts\lib"
$RepairScript = Join-Path $ApiRoot "scripts\repair\repair-machine-bootstrap-metadata.ps1"

. (Join-Path $ScriptsLib "autonomous-e2e-common.ps1")

$failures = @()
$passed = 0

function Assert-Test {
    param([string]$Name, [scriptblock]$Block)
    try {
        & $Block
        $script:passed++
        Write-Host "PASS $Name"
    } catch {
        $script:failures += "$Name :: $($_.Exception.Message)"
        Write-Host "FAIL $Name :: $($_.Exception.Message)"
    }
}

Assert-Test "tcn_cash_missing_metadata_fails" {
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & py -3 $SchemaPy (Join-Path $Fixtures "layout-tcn-cash-missing-metadata.json") 1>$null 2>$null
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $prevEap
    }
    if ($code -eq 0) { throw "expected validation failure exit=0" }
}

Assert-Test "tcn_cash_valid_metadata_passes" {
    & py -3 $SchemaPy (Join-Path $Fixtures "layout-tcn-cash-valid-metadata.json") | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "expected validation pass exit=$LASTEXITCODE" }
}

Assert-Test "pilot_merge_includes_contract_keys" {
    $out = Join-Path $env:TEMP ("merged-layout-" + [guid]::NewGuid().ToString() + ".json")
    $examples = Join-Path $E2eDir "examples"
    py -3 $SchemaPy merge `
        --machine-id (Get-AutonomousTargetMachineId) `
        --cabinet (Join-Path $examples "pilot-cabinet-layout-a.json") `
        --slots (Join-Path $examples "pilot-slot-assignments-a1-a10.json") `
        --inventory (Join-Path $examples "pilot-inventory-a1-a10.json") `
        --payment (Join-Path $examples "payment-profile-cash-only.json") `
        --hardware (Join-Path $examples "hardware-profile-tcn.json") `
        --destructive (Join-Path $examples "destructive-scope-a1-a10.json") `
        -o $out | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "merge failed exit=$LASTEXITCODE" }
    $meta = Get-Content $out -Raw | ConvertFrom-Json
    $cabMeta = $meta.cabinets[0].metadata
    foreach ($key in @("board_protocol", "bill_protocol", "cash_topology", "transport_type")) {
        $val = $cabMeta.$key
        if ([string]::IsNullOrWhiteSpace([string]$val)) { throw "missing $key after merge" }
    }
    Remove-Item $out -Force -ErrorAction SilentlyContinue
}

Assert-Test "placeholder_machine_id_rejected" {
    $bad = "00000000-0000-4000-8000-000000000099"
    try {
        Assert-AutonomousTargetMachineId -MachineId $bad
        throw "placeholder should be rejected"
    } catch {
        if ($_.Exception.Message -notmatch "placeholder") { throw $_ }
    }
}

Assert-Test "setup_tcn_dry_run_no_write_exit_zero" {
    & (Join-Path $E2eDir "setup-tcn-cash-products-a1-a10.ps1") -DryRun
    if ($LASTEXITCODE -ne 0) { throw "dry-run exit=$LASTEXITCODE" }
}

Assert-Test "repair_dry_run_blocked_without_admin_creds" {
    $savedEmail = $env:AVF_ADMIN_EMAIL
    $savedPass = $env:AVF_ADMIN_PASSWORD
    $savedToken = $env:AVF_ADMIN_TOKEN
    $savedAdminEmail = $env:ADMIN_EMAIL
    $savedAdminPass = $env:ADMIN_PASSWORD
    $savedE2eEmail = $env:E2E_PROD_ADMIN_EMAIL
    $savedE2ePass = $env:E2E_PROD_ADMIN_PASSWORD
    $env:AVF_ADMIN_EMAIL = $null
    $env:AVF_ADMIN_PASSWORD = $null
    $env:AVF_ADMIN_TOKEN = $null
    $env:ADMIN_EMAIL = $null
    $env:ADMIN_PASSWORD = $null
    $env:E2E_PROD_ADMIN_EMAIL = $null
    $env:E2E_PROD_ADMIN_PASSWORD = $null
    $env:AVF_NONINTERACTIVE = "1"
    try {
        & $RepairScript -DryRun -NonInteractive
        if ($LASTEXITCODE -ne 4) { throw "expected BLOCKED_ADMIN_ENV_MISSING exit=4 got=$LASTEXITCODE" }
    } finally {
        $env:AVF_ADMIN_EMAIL = $savedEmail
        $env:AVF_ADMIN_PASSWORD = $savedPass
        $env:AVF_ADMIN_TOKEN = $savedToken
        $env:ADMIN_EMAIL = $savedAdminEmail
        $env:ADMIN_PASSWORD = $savedAdminPass
        $env:E2E_PROD_ADMIN_EMAIL = $savedE2eEmail
        $env:E2E_PROD_ADMIN_PASSWORD = $savedE2ePass
    }
}

Assert-Test "repair_script_no_unsafe_json_write_pattern" {
    $content = Get-Content $RepairScript -Raw
    if ($content -match 'ForEach-Object\s*\{\s*Set-Utf8JsonFile') {
        throw "UNSAFE_JSON_WRITE_PATTERN_PRESENT"
    }
    if ($content -match 'jq[^\n\r]*\|\s*ForEach-Object') {
        throw "UNSAFE_JSON_WRITE_PATTERN_PRESENT jq pipeline ForEach-Object"
    }
    if ($content -match 'jq[^\n\r]*\|\s*%\s*\{') {
        throw "UNSAFE_JSON_WRITE_PATTERN_PRESENT jq pipeline %"
    }
}

Write-Host ""
Write-Host "SUMMARY passed=$passed failed=$($failures.Count)"
if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Host "  - $_" }
    exit 1
}
exit 0
