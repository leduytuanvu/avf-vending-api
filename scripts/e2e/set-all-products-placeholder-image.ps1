param(
    [string]$PlaceholderUrl = "https://res.cloudinary.com/dz4qz0tk9/image/upload/v1779376340/avf-vending/products/019e4b18-346b-78b7-a370-4b8339120062.png",
    [string]$ArtifactDir = ""
)
$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$Bash = "C:\Program Files\Git\bin\bash.exe"
if (-not (Test-Path $Bash)) { $Bash = "bash" }

$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")
if (-not $ArtifactDir) {
    $ArtifactDir = Join-Path $ApiRoot "reports\e2e\set-all-products-placeholder-image\$Ts"
}
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null

$env:E2E_ALLOW_WRITES = "true"
$env:E2E_RUN_DIR = $ArtifactDir
$env:E2E_RUN_TS = $Ts

$ApplySh = Join-Path $ScriptDir "set-all-products-placeholder-image.sh"
& $Bash $ApplySh $PlaceholderUrl
if ($LASTEXITCODE -ne 0) { throw "set-all-products-placeholder-image failed (exit=$LASTEXITCODE)" }
Write-Host "Artifacts: $ArtifactDir"
