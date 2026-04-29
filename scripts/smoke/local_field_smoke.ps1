param(
    [string]$BaseUrl = $env:BASE_URL,
    [string]$OrgId = $env:ORG_ID,
    [string]$AdminEmail = $env:ADMIN_EMAIL,
    [string]$AdminPassword = $env:ADMIN_PASSWORD,
    [string]$MachineId = $env:MACHINE_ID,
    [string]$EvidenceJson = $env:SMOKE_EVIDENCE_JSON,
    [switch]$SkipPaymentWebhook
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $root

$python = $env:PYTHON
if ([string]::IsNullOrWhiteSpace($python)) {
    $python = "python"
}

$argsList = @("tools/smoke_test.py", "local")
if (-not [string]::IsNullOrWhiteSpace($BaseUrl)) { $argsList += @("--base-url", $BaseUrl) }
if (-not [string]::IsNullOrWhiteSpace($OrgId)) { $argsList += @("--org-id", $OrgId) }
if (-not [string]::IsNullOrWhiteSpace($AdminEmail)) { $argsList += @("--admin-email", $AdminEmail) }
if (-not [string]::IsNullOrWhiteSpace($AdminPassword)) { $argsList += @("--admin-password", $AdminPassword) }
if (-not [string]::IsNullOrWhiteSpace($MachineId)) { $argsList += @("--machine-id", $MachineId) }
if (-not [string]::IsNullOrWhiteSpace($EvidenceJson)) { $argsList += @("--evidence-json", $EvidenceJson) }
if ($SkipPaymentWebhook) { $argsList += "--skip-payment-webhook" }

& $python @argsList
exit $LASTEXITCODE
