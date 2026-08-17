param(
    [string]$BaseUrl = "https://api.ldtv.dev",
    [string]$AdminToken = "",
    [string]$AdminEmail = "admin@avf.com",
    [string]$AdminPassword = "",
    [string]$SkuPrefix = "AVF111111",
    [string]$ImageDir = "",
    [string]$ArtifactDir = "",
    [switch]$SkipDownload,
    [switch]$DryRun
)
$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$ApiRoot = Split-Path (Split-Path $ScriptDir -Parent) -Parent
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'")

if (-not $ImageDir) {
    $ImageDir = Join-Path $ApiRoot "tmp\avf111111-images"
}
if (-not $ArtifactDir) {
    $ArtifactDir = Join-Path $ApiRoot "reports\e2e\avf111111-setup\$Ts\image-upload"
}
New-Item -ItemType Directory -Force -Path $ImageDir, $ArtifactDir | Out-Null

$ImageSources = @(
    @{
        File = "phuc-long.jpg"
        Url  = "https://upload.urbox.vn/strapi/phuc_long_5_c188a69da5.jpg"
        SlotFrom = 1
        SlotTo   = 15
    },
    @{
        File = "macchiato.webp"
        Url  = "https://spacet-release.s3.ap-southeast-1.amazonaws.com/img/blog/2023-11-06/macchiato-the-coffee-house-65486a865d22b5a617c3bfa0.webp"
        SlotFrom = 16
        SlotTo   = 30
    },
    @{
        File = "tra-sua-koi.webp"
        Url  = "https://hunufa.vn/wp-content/uploads/2025/10/tra-sua-koi-mon-nao-ngon-nhat-7.webp"
        SlotFrom = 31
        SlotTo   = 45
    },
    @{
        File = "tra-sua-dep.webp"
        Url  = "https://hunufa.vn/wp-content/uploads/2024/10/hinh-ly-tra-sua-dep.webp"
        SlotFrom = 46
        SlotTo   = 60
    }
)

$SlotCodes = @(
    "A1","A2","A3","A4","A5","A6","A7","A8","A9","A10",
    "B1","B2","B3","B4","B5","B6","B7","B8","B9","B10",
    "C1","C2","C3","C4","C5","C6","C7","C8","C9","C10",
    "D1","D2","D3","D4","D5","D6","D7","D8","D9","D10",
    "E1","E2","E3","E4","E5","E6","E7","E8","E9","E10",
    "F1","F2","F3","F4","F5","F6","F7","F8","F9","F10"
)

function Get-AdminToken {
    if ($AdminToken) { return $AdminToken.Trim() }
    if (-not $AdminEmail -or -not $AdminPassword) {
        throw "Set -AdminToken or -AdminEmail/-AdminPassword"
    }
    $body = @{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json
    $resp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/auth/login" `
        -ContentType "application/json" -Body $body
    return $resp.tokens.accessToken
}

function Get-ImagePathForSlotIndex([int]$Index) {
    foreach ($src in $ImageSources) {
        if ($Index -ge $src.SlotFrom -and $Index -le $src.SlotTo) {
            if ($Index -le 15) {
                $png = Join-Path $ImageDir "phuc-long.png"
                if (Test-Path $png) { return $png }
            }
            return Join-Path $ImageDir $src.File
        }
    }
    throw "No image mapping for slot index $Index"
}

if (-not $SkipDownload) {
    Write-Host "Download images -> $ImageDir"
    foreach ($src in $ImageSources) {
        $dest = Join-Path $ImageDir $src.File
        if ($DryRun) {
            Write-Host "DRY-RUN download $($src.Url) -> $dest"
            continue
        }
        Invoke-WebRequest -Uri $src.Url -OutFile $dest -UseBasicParsing
        Write-Host "Downloaded $($src.File)"
    }
    # Production API rejects some source JPEGs; normalize group-1 image to PNG.
    $jpg = Join-Path $ImageDir "phuc-long.jpg"
    $png = Join-Path $ImageDir "phuc-long.png"
    if ((Test-Path $jpg) -and -not (Test-Path $png)) {
        Add-Type -AssemblyName System.Drawing
        $img = [System.Drawing.Image]::FromFile($jpg)
        $img.Save($png, [System.Drawing.Imaging.ImageFormat]::Png)
        $img.Dispose()
        Write-Host "Converted phuc-long.jpg -> phuc-long.png"
    }
}

$token = if ($DryRun) { "dry-run-token" } else { Get-AdminToken }
$headers = @{ Authorization = "Bearer $token" }

Write-Host "List products sku prefix=$SkuPrefix"
$skuToProductId = @{}
$offset = 0
$limit = 500
do {
    if ($DryRun) { break }
    $resp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/admin/products?limit=$limit&offset=$offset" -Headers $headers
    foreach ($item in @($resp.items)) {
        if ($item.sku -like "$SkuPrefix-*") {
            $skuToProductId[$item.sku] = $item.id
        }
    }
    $count = @($resp.items).Count
    $offset += $count
} while ($count -ge $limit)

if (-not $DryRun -and $skuToProductId.Count -eq 0) {
    throw "No products found with prefix $SkuPrefix - run layout apply first"
}

$results = @()
$failed = 0
$ok = 0

for ($i = 0; $i -lt $SlotCodes.Count; $i++) {
    $slotCode = $SlotCodes[$i]
    $slotIndex = $i + 1
    $sku = "$SkuPrefix-$slotCode"
    $imagePath = Get-ImagePathForSlotIndex $slotIndex

    if ($DryRun) {
        Write-Host "DRY-RUN upload $sku index=$slotIndex file=$imagePath"
        continue
    }

    if (-not $skuToProductId.ContainsKey($sku)) {
        Write-Warning "SKIP $sku — product not found"
        $failed++
        continue
    }
    $productId = $skuToProductId[$sku]
    if (-not (Test-Path $imagePath)) {
        Write-Warning "SKIP $sku — image missing: $imagePath"
        $failed++
        continue
    }

    $idem = "avf111111-img-$productId"
    $outFile = Join-Path $ArtifactDir "upload-$sku.meta.json"
    $mime = "image/jpeg"
    if ($imagePath -like "*.png") { $mime = "image/png" }
    elseif ($imagePath -like "*.webp") { $mime = "image/webp" }
    elseif ($imagePath -like "*.gif") { $mime = "image/gif" }
    $fileUnix = $imagePath -replace '\\','/'
    $curlArgs = @(
        "-sS", "-w", "`n%{http_code}",
        "-X", "POST",
        "-H", "Authorization: Bearer $token",
        "-H", "Idempotency-Key: $idem",
        "-F", "file=@${fileUnix};type=$mime",
        "-F", "purpose=product_image",
        "-F", "productId=$productId",
        "-F", "isPrimary=true",
        "$BaseUrl/v1/admin/product-images"
    )
    $raw = & curl.exe @curlArgs 2>&1
    $lines = @($raw)
    $httpCode = $lines[-1].Trim()
    $bodyText = ($lines[0..([Math]::Max(0, $lines.Count - 2))] -join "`n").Trim()
    @{
        sku = $sku
        slotIndex = $slotIndex
        productId = $productId
        httpCode = $httpCode
        body = $bodyText
    } | ConvertTo-Json -Depth 6 | Set-Content -Path $outFile -Encoding UTF8

    if ($httpCode -eq "201" -or $httpCode -eq "200") {
        $ok++
        Write-Host "OK $sku http=$httpCode"
        $actHdr = @{ Authorization = "Bearer $token"; "Idempotency-Key" = "avf111111-act-$productId" }
        try {
            Invoke-RestMethod -Method Patch -Uri "$BaseUrl/v1/admin/products/$productId" -Headers $actHdr -ContentType "application/json" -Body '{"active":true}' | Out-Null
        } catch {
            Write-Warning "activate failed for $sku (upload ok)"
        }
    } else {
        $failed++
        Write-Warning "FAIL $sku http=$httpCode body=$bodyText"
    }
}

$summary = @{
    utc = $Ts
    baseUrl = $BaseUrl
    skuPrefix = $SkuPrefix
    uploaded = $ok
    failed = $failed
    artifactDir = $ArtifactDir
} | ConvertTo-Json -Depth 4
$summary | Set-Content -Path (Join-Path $ArtifactDir "UPLOAD_SUMMARY.json") -Encoding UTF8
Write-Host $summary
if ($failed -gt 0 -and -not $DryRun) { exit 2 }
exit 0
