param(
    [Parameter(Mandatory = $true)][string]$MachineCode,
    [string]$BaseUrl = "https://api.ldtv.dev",
    [string]$AdminEmail = "admin@avf.com",
    [string]$AdminPassword = "",
    [string]$AdminToken = "",
    [string]$ReuseMediaFromPrefix = "AVF111111",
    [string]$LayoutJsonPath = "",
    [string]$ArtifactDir = "",
    [string]$SiteCode = "",
    [switch]$DryRun,
    [switch]$SkipLayoutApply,
    [switch]$SkipActivationCode
)
$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")
$SkuPrefix = $MachineCode

if (-not $LayoutJsonPath) {
    $LayoutJsonPath = Join-Path $ScriptDir "examples/machine-layout-${MachineCode}-60slots.json"
}
if (-not (Test-Path $LayoutJsonPath)) {
    throw "Layout JSON not found: $LayoutJsonPath"
}
if (-not $SiteCode) { $SiteCode = "$MachineCode-SITE" }
if (-not $ArtifactDir) {
    $ArtifactDir = Join-Path $ApiRoot "reports/e2e/$($MachineCode.ToLower())-setup/$Ts"
}
New-Item -ItemType Directory -Force -Path $ArtifactDir, "$ArtifactDir/raw" | Out-Null

$SlotCodes = @(
    "A1","A2","A3","A4","A5","A6","A7","A8","A9","A10",
    "B1","B2","B3","B4","B5","B6","B7","B8","B9","B10",
    "C1","C2","C3","C4","C5","C6","C7","C8","C9","C10",
    "D1","D2","D3","D4","D5","D6","D7","D8","D9","D10",
    "E1","E2","E3","E4","E5","E6","E7","E8","E9","E10",
    "F1","F2","F3","F4","F5","F6","F7","F8","F9","F10"
)

$MediaRefSkus = @{
    "1"  = "$ReuseMediaFromPrefix-A1"
    "16" = "$ReuseMediaFromPrefix-B6"
    "31" = "$ReuseMediaFromPrefix-C1"
    "46" = "$ReuseMediaFromPrefix-D1"
}

function Write-Log([string]$Msg) { Write-Host $Msg; Add-Content -Path (Join-Path $ArtifactDir "setup.log") -Value $Msg }

function Get-AdminToken {
    if ($AdminToken) { return $AdminToken.Trim() }
    if (-not $AdminEmail -or -not $AdminPassword) {
        throw "Set -AdminToken or -AdminEmail/-AdminPassword"
    }
    $body = @{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json
    $resp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/auth/login" -ContentType "application/json" -Body $body
    $resp | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $ArtifactDir "raw/B2-login.json") -Encoding UTF8
    return $resp.tokens.accessToken
}

function Invoke-AdminJson {
    param(
        [string]$Method, [string]$Path, [object]$Body, [string]$IdemKey, [string]$OutName
    )
    $token = Get-AdminToken
    $hdr = @{ Authorization = "Bearer $token" }
    if ($IdemKey) { $hdr["Idempotency-Key"] = $IdemKey }
    $uri = if ($Path -match '^https?://') { $Path } else { "$BaseUrl$Path" }
    $params = @{ Method = $Method; Uri = $uri; Headers = $hdr }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress)
    }
    if ($DryRun) {
        Write-Log "DRY-RUN $Method $Path"
        return $null
    }
    try {
        $resp = Invoke-RestMethod @params
        if ($OutName) {
            $resp | ConvertTo-Json -Depth 10 | Set-Content (Join-Path $ArtifactDir "raw/$OutName.json") -Encoding UTF8
        }
        return $resp
    } catch {
        $err = $_.ErrorDetails.Message
        if ($OutName) { $err | Set-Content (Join-Path $ArtifactDir "raw/$OutName.error.json") -Encoding UTF8 }
        throw
    }
}

function Get-AllProducts([string]$Token) {
    $items = @()
    $offset = 0
    $limit = 500
    do {
        $resp = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/products?limit=$limit&offset=$offset" -Headers @{ Authorization = "Bearer $Token" }
        $batch = @($resp.items)
        if ($batch.Count -eq 0) { break }
        $items += $batch
        $offset += $batch.Count
    } while ($batch.Count -ge $limit)
    return $items
}

function Get-ProductDetail([string]$Token, [string]$ProductId) {
    return Invoke-RestMethod -Uri "$BaseUrl/v1/admin/products/$ProductId" -Headers @{ Authorization = "Bearer $Token" }
}

function Resolve-MediaIdForSlotIndex([int]$SlotIndex, [hashtable]$MediaByGroup, [string]$Token) {
    $group = if ($SlotIndex -le 15) { "1" } elseif ($SlotIndex -le 30) { "16" } elseif ($SlotIndex -le 45) { "31" } else { "46" }
    if ($MediaByGroup.ContainsKey($group)) { return $MediaByGroup[$group] }
    $refSku = $MediaRefSkus[$group]
    throw "Missing media for group $group (reference SKU $refSku)"
}

Write-Log "=== Setup production machine $MachineCode ==="
Write-Log "ArtifactDir=$ArtifactDir"

if ($DryRun) {
    Write-Log "DRY-RUN complete (no HTTP writes)"
    exit 0
}

$token = Get-AdminToken
$h = @{ Authorization = "Bearer $token" }

# B1 version
$ver = Invoke-RestMethod -Uri "$BaseUrl/version"
$ver | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $ArtifactDir "raw/B1-version.json") -Encoding UTF8
Write-Log "version payment_mode=$($ver.payment_runtime.payment_mode)"

# B3 planogram org
$planos = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/planograms?limit=20" -Headers $h
$planos | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $ArtifactDir "raw/B3-planograms.json") -Encoding UTF8
$pubPlano = @($planos.items | Where-Object { $_.status -eq "published" } | Select-Object -First 1)
if (-not $pubPlano) { throw "No published org planogram found" }

# Resolve reuse media from reference prefix or any ready media on server
$allProds = Get-AllProducts $token
$refProds = @($allProds | Where-Object { $_.sku -like "$ReuseMediaFromPrefix-*" })
$mediaByGroup = @{}
if ($refProds.Count -gt 0) {
    foreach ($entry in $MediaRefSkus.GetEnumerator()) {
        $ref = @($refProds | Where-Object { $_.sku -eq $entry.Value } | Select-Object -First 1)
        if (-not $ref) { throw "Reference SKU not found: $($entry.Value)" }
        $detail = Get-ProductDetail $token $ref.id
        if (-not $detail.primaryMediaId) { throw "Reference SKU $($entry.Value) has no primaryMediaId" }
        $mediaByGroup[$entry.Key] = $detail.primaryMediaId
        Write-Log "Media group $($entry.Key): $($detail.primaryMediaId) from $($entry.Value)"
    }
} else {
    Write-Log "No products with prefix $ReuseMediaFromPrefix - scanning server catalog for ready media"
    $mediaIds = [System.Collections.Generic.List[string]]::new()
    foreach ($p in $allProds) {
        $mid = $p.primaryMediaId
        if (-not $mid) {
            try {
                $det = Get-ProductDetail $token $p.id
                $mid = $det.primaryMediaId
            } catch { continue }
        }
        if ($mid -and ($mediaIds -notcontains $mid)) { [void]$mediaIds.Add($mid) }
        if ($mediaIds.Count -ge 4) { break }
    }
    if ($mediaIds.Count -eq 0) {
        throw "No reusable primaryMediaId on server - upload product images first or restore reference catalog"
    }
    $groups = @("1", "16", "31", "46")
    for ($i = 0; $i -lt $groups.Count; $i++) {
        $mediaByGroup[$groups[$i]] = $mediaIds[[Math]::Min($i, $mediaIds.Count - 1)]
        Write-Log "Media group $($groups[$i]): $($mediaByGroup[$groups[$i]]) (server catalog index $i)"
    }
}
$mediaByGroup | ConvertTo-Json | Set-Content (Join-Path $ArtifactDir "raw/reused-media-ids.json") -Encoding UTF8

# B4 site
$sites = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/sites?limit=100" -Headers $h
$site = @($sites.items | Where-Object { $_.code -eq $SiteCode } | Select-Object -First 1)
if (-not $site) {
    $site = Invoke-AdminJson -Method Post -Path "/v1/admin/sites" -Body @{
        name = "$MachineCode Pilot Site"; code = $SiteCode
        timezone = "Asia/Ho_Chi_Minh"; address = @{}
    } -IdemKey "$MachineCode-site-create" -OutName "B4-site-create"
} else {
    Write-Log "Reuse site $($site.id)"
}
$siteId = $site.id

# B5/B6 machine
$machines = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/machines?limit=200" -Headers $h
$machine = @($machines.items | Where-Object { $_.code -eq $MachineCode } | Select-Object -First 1)
if (-not $machine) {
    $machine = Invoke-AdminJson -Method Post -Path "/v1/admin/machines" -Body @{
        siteId = $siteId; code = $MachineCode; serialNumber = "SN-$MachineCode"
        name = "$MachineCode TCN 60-slot"; model = "TCN"; cabinetType = "ambient"
        timezone = "Asia/Ho_Chi_Minh"; status = "draft"
    } -IdemKey "$MachineCode-machine-create" -OutName "B6-machine-create"
} else {
    Write-Log "Reuse machine $($machine.id) status=$($machine.status)"
}
$machineId = $machine.id
$machine | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $ArtifactDir "raw/machine.json") -Encoding UTF8

# B7 category + brand
$catSlug = "$($MachineCode.ToLower())-beverages"
$brSlug = $MachineCode.ToLower()
$cats = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/categories?limit=200" -Headers $h
$cat = @($cats.items | Where-Object { $_.slug -eq $catSlug } | Select-Object -First 1)
if (-not $cat) {
    $cat = Invoke-AdminJson -Method Post -Path "/v1/admin/categories" -Body @{
        name = "$MachineCode Beverages"; slug = $catSlug; parentId = $null; active = $true
    } -IdemKey "$MachineCode-cat" -OutName "B7-category"
}
$brands = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/brands?limit=200" -Headers $h
$brand = @($brands.items | Where-Object { $_.slug -eq $brSlug } | Select-Object -First 1)
if (-not $brand) {
    $brand = Invoke-AdminJson -Method Post -Path "/v1/admin/brands" -Body @{
        name = $MachineCode; slug = $brSlug; active = $true
    } -IdemKey "$MachineCode-brand" -OutName "B7-brand"
}
$catId = $cat.id
$brandId = $brand.id

# B8 products with reused primaryMediaId
$layout = Get-Content $LayoutJsonPath -Raw | ConvertFrom-Json
$existingSkus = @{}
foreach ($p in $allProds) { if ($p.sku) { $existingSkus[$p.sku] = $p.id } }
$created = 0; $skipped = 0; $activated = 0
foreach ($slot in $layout.slots) {
    $sku = $slot.product.sku
    $slotIndex = [int]$slot.slot_index
    $mediaId = Resolve-MediaIdForSlotIndex $slotIndex $mediaByGroup $token
    $productId = $null
    if ($existingSkus.ContainsKey($sku)) {
        $productId = $existingSkus[$sku]
        $skipped++
    } else {
        $body = @{
            name = $slot.product.name; sku = $sku; description = "$MachineCode slot"
            active = $false; primaryMediaId = $mediaId
            categoryId = $catId; brandId = $brandId
            ageRestricted = $false; allergenCodes = @()
        }
        $resp = Invoke-AdminJson -Method Post -Path "/v1/admin/products" -Body $body -IdemKey "$MachineCode-prod-$sku" -OutName "product-$sku"
        $productId = $resp.id
        $existingSkus[$sku] = $productId
        $created++
    }
    $detail = Get-ProductDetail $token $productId
    if ($detail.active -ne $true) {
        Invoke-AdminJson -Method Patch -Path "/v1/admin/products/$productId" -Body @{ active = $true } `
            -IdemKey "$MachineCode-act-$productId" -OutName "activate-$sku"
        $activated++
    }
}
Write-Log "Products created=$created skipped=$skipped activated=$activated"

# Patch machine active (required for planogram publish / MQTT)
Invoke-AdminJson -Method Patch -Path "/v1/admin/machines/$machineId" -Body @{ status = "active" } `
    -IdemKey "$MachineCode-machine-active" -OutName "B6-machine-active"

# Layout apply via bash script
if (-not $SkipLayoutApply) {
    $layoutOut = Join-Path $ArtifactDir "machine-layout-$MachineCode-60slots.json"
    (Get-Content $LayoutJsonPath -Raw) -replace 'REPLACE_WITH_MACHINE_ID', $machineId | Set-Content $layoutOut -NoNewline
    $bash = "C:\Program Files\Git\bin\bash.exe"
    if (-not (Test-Path $bash)) { $bash = "bash" }
    $layoutUnix = (Resolve-Path $layoutOut).Path -replace '\\','/'
    $applyDir = Join-Path $ArtifactDir "layout-apply"
    New-Item -ItemType Directory -Force -Path $applyDir | Out-Null
    $applyDirUnix = (Resolve-Path $applyDir).Path -replace '\\','/'
    $applySh = (Join-Path $ScriptDir "setup-machine-sellable-layout-apply.sh") -replace '\\','/'
    $envBlock = @(
        "E2E_ALLOW_WRITES=true",
        "E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION",
        "E2E_TARGET=production",
        "E2E_RUN_DIR=$applyDirUnix",
        "E2E_RUN_TS=$Ts",
        "TEST_MACHINE_ID=$machineId",
        "ADMIN_EMAIL=$AdminEmail",
        "ADMIN_PASSWORD=$AdminPassword",
        "BASE_URL=$BaseUrl"
    ) -join " "
    Write-Log "Running layout apply..."
    & $bash -lc "$envBlock bash '$applySh' '$layoutUnix'"
    $layoutExit = $LASTEXITCODE
    # layout apply may set online; restore active only if retry publish is needed below.
    # Final runtime status must be online (inventory/commerce gRPC gate).
    if ($layoutExit -ne 0) {
        Invoke-AdminJson -Method Patch -Path "/v1/admin/machines/$machineId" -Body @{ status = "active" } `
            -IdemKey "$MachineCode-machine-active-post-layout" -OutName "B6-machine-active-post-layout"
    }
    if ($layoutExit -ne 0) {
        Write-Log "Layout apply exit=$layoutExit — retrying planogram publish with active status"
        $token = Get-AdminToken
        $op = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/admin/machines/$machineId/operator-sessions/start" `
            -Headers @{ Authorization = "Bearer $token"; "Content-Type" = "application/json" } `
            -Body '{"force_admin_takeover":true,"auth_method":"oidc"}'
        $opSid = $op.session.id
        $allProds2 = Get-AllProducts $token
        $skuMap = @{}
        foreach ($p in $allProds2) { if ($p.sku -like "$SkuPrefix-*") { $skuMap[$p.sku] = $p.id } }
        $items = @()
        foreach ($slot in $layout.slots) {
            if (-not $slot.enabled -or -not $slot.sellable) { continue }
            $sku = $slot.product.sku
            if (-not $skuMap.ContainsKey($sku)) { continue }
            $items += @{
                cabinetCode = $slot.cabinet_code; slotCode = $slot.slot_code
                productId = $skuMap[$sku]; maxQuantity = $slot.inventory_quantity
                priceMinor = $slot.price_minor; layoutKey = "grid-10x6"; layoutRevision = 1
                legacySlotIndex = $slot.slot_index; metadata = @{}
            }
        }
        $draft = @{
            operator_session_id = $opSid; planogramId = $pubPlano.id
            planogramRevision = $pubPlano.revision; syncLegacyReadModel = $true; items = $items
        }
        try {
            Invoke-AdminJson -Method Put -Path "/v1/admin/machines/$machineId/planograms/draft" -Body $draft -OutName "planogram-draft-retry"
            Invoke-AdminJson -Method Post -Path "/v1/admin/machines/$machineId/planograms/publish" -Body $draft `
                -IdemKey "$MachineCode-planogram-retry-$Ts" -OutName "planogram-publish-retry"
        } catch {
            Write-Log "Planogram publish retry failed (slots may already be current): $($_.Exception.Message)"
        }
    }
}

# Runtime app requires online/offline for inventory gRPC (GetInventorySnapshot); active is MQTT-publish only.
Invoke-AdminJson -Method Patch -Path "/v1/admin/machines/$machineId" -Body @{ status = "online" } `
    -IdemKey "$MachineCode-machine-online-final" -OutName "B6-machine-online-final"

# Verify slots
$token = Get-AdminToken
$slots = Invoke-RestMethod -Uri "$BaseUrl/v1/admin/machines/$machineId/slots" -Headers @{ Authorization = "Bearer $token" }
$slots | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $ArtifactDir "raw/B15-slots.json") -Encoding UTF8
$withProd = @($slots.items | Where-Object { $_.productId }).Count
$withStock = @($slots.items | Where-Object { $_.currentQuantity -gt 0 }).Count
Write-Log "Slots total=$($slots.items.Count) withProduct=$withProd withStock=$withStock"

# B17 activation code
$activationCode = $null
if (-not $SkipActivationCode) {
    $act = Invoke-AdminJson -Method Post -Path "/v1/admin/machine-codes/$MachineCode/activation-codes" -Body @{
        expiresInMinutes = 1440; maxUses = 1; notes = "first install $MachineCode"
    } -IdemKey "$MachineCode-activation-$Ts" -OutName "B17-activation-code"
    $activationCode = $act.activationCode
    Write-Log "ActivationCode=$activationCode (also in raw/B17-activation-code.json)"
}

@{
    VERDICT = if ($withProd -ge 60 -and $withStock -ge 60) { "PASS" } else { "FAIL" }
    UTC = $Ts
    BASE_URL = $BaseUrl
    MACHINE_CODE = $MachineCode
    MACHINE_ID = $machineId
    SITE_ID = $siteId
    CATEGORY_ID = $catId
    BRAND_ID = $brandId
    PLANOGRAM_ID = $pubPlano.id
    SLOTS_WITH_PRODUCT = $withProd
    SLOTS_WITH_STOCK = $withStock
    ACTIVATION_CODE = $activationCode
    ARTIFACT_DIR = $ArtifactDir
}.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" } | Set-Content (Join-Path $ArtifactDir "SETUP_READINESS.txt")

Write-Log "=== DONE machine=$MachineCode id=$machineId activation=$activationCode ==="
if ($withProd -lt 60 -or $withStock -lt 60) { exit 2 }
exit 0
