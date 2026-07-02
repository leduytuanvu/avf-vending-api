$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $repo
if (-not $env:ENTERPRISE_FLOW_VERIFICATION_UTC) {
  $env:ENTERPRISE_FLOW_VERIFICATION_UTC = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
}
python tools/enterprise_flow/test_mqtt_full_coverage.py @args
exit $LASTEXITCODE
