# Verify generated avf-vending machine layout JSON for AVF000132.
param(
    [string]$Path = (Join-Path $PSScriptRoot "output\avf000132-machine-layout.json")
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Path)) {
    throw "Missing layout file. Run: python inventory_to_machine_layout.py"
}

python (Join-Path $PSScriptRoot "inventory_to_machine_layout.py") --output $Path 2>$null | Out-Null
python (Join-Path $PSScriptRoot "..\e2e\layout_config_schema.py") $Path
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$doc = Get-Content $Path -Raw | ConvertFrom-Json
$catalogCount = @($doc.catalog_products).Count
$withImage = @($doc.catalog_products | Where-Object { $_.primary_image_url -match '^https://' }).Count
$withPrice = @($doc.catalog_products | Where-Object { $_.unit_price_minor -gt 0 }).Count
if ($catalogCount -eq 0) {
    Write-Error "catalog_products is empty - run inventory_to_machine_layout.py with enriched manifest"
    exit 1
}
Write-Host "OK: layout schema valid at $Path"
Write-Host "catalog_products=$catalogCount with_image=$withImage with_price=$withPrice"
