# Apply AVF000132 machine layout to avf-vending-api, then pull on avf-vending-app.
param(
    [string]$MachineId = $env:AVF000132_MACHINE_ID,
    [string]$LayoutPath = (Join-Path $PSScriptRoot "output\avf000132-machine-layout.json"),
    [switch]$GenerateOnly,
    [switch]$SkipApply
)

$ErrorActionPreference = "Stop"
$apiRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

if (-not (Test-Path $LayoutPath)) {
    Write-Host "Generating layout JSON..."
    python (Join-Path $PSScriptRoot "inventory_to_machine_layout.py")
}

if ($MachineId) {
    Write-Host "Patching machine_id in layout JSON..."
    $doc = Get-Content $LayoutPath -Raw | ConvertFrom-Json
    $doc.machine_id = $MachineId
    $doc | ConvertTo-Json -Depth 20 | Set-Content $LayoutPath -Encoding UTF8
}

if ($GenerateOnly) {
    Write-Host "Generated: $LayoutPath"
    exit 0
}

if ($SkipApply) {
    Write-Host "SkipApply set. Layout ready at: $LayoutPath"
    exit 0
}

if (-not $env:E2E_ALLOW_WRITES) {
    Write-Host @"
Layout file is ready. To apply on server (avf-vending-api):
  `$env:E2E_ALLOW_WRITES = 'true'
  `$env:E2E_PRODUCTION_WRITE_CONFIRMATION = 'I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION'
  `$env:AVF000132_MACHINE_ID = '<machine-uuid>'
  bash $($apiRoot.Path -replace '\\','/')/scripts/e2e/setup-machine-sellable-layout-apply.sh $($LayoutPath -replace '\\','/')

On device (avf-vending-app, NOT legacy TIC):
  Technician -> Merchandise -> Planogram Editor -> Pull from server
  Or run catalog sync / re-bootstrap after server publish.
"@
    exit 0
}

$bash = Get-Command bash -ErrorAction SilentlyContinue
if (-not $bash) {
    throw "bash is required to run setup-machine-sellable-layout-apply.sh"
}

$layoutForBash = $LayoutPath -replace '\\', '/'
Push-Location $apiRoot
try {
    & bash "./scripts/e2e/setup-machine-sellable-layout-apply.sh" $layoutForBash
} finally {
    Pop-Location
}

Write-Host @"
Server apply finished.
On AVF000132 (avf-vending-app):
  1. Technician -> Planogram Editor -> Pull from server
  2. Product Catalog Sync (downloads images + base prices)
"@
