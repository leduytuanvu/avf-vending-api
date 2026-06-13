# Merge-safe bootstrap cabinet metadata repair for production first-install.
param(
    [string]$MachineId = "",
    [string]$SiteId = "",
    [string]$CabinetCode = "CAB-A",
    [switch]$DryRun,
    [switch]$NonInteractive
)

$ErrorActionPreference = "Stop"
foreach ($cmd in @("curl", "jq")) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) { throw "Required: $cmd" }
}

$ApiRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$WorkspaceRoot = Split-Path $ApiRoot -Parent
$ScriptsLib = Join-Path $WorkspaceRoot "scripts\lib"
. (Join-Path $ScriptsLib "autonomous-e2e-common.ps1")
. (Join-Path $ScriptsLib "admin-auth.ps1")
. (Join-Path $ScriptsLib "log-secrecy.ps1")
. (Join-Path $ScriptsLib "invoke-getbootstrap-direct.ps1")

$BaseUrl = Get-AvfApiBaseUrlNormalized
if ([string]::IsNullOrWhiteSpace($MachineId)) {
    $MachineId = if ($env:AVF_MACHINE_ID) { $env:AVF_MACHINE_ID } else { (Get-AutonomousTargetMachineId) }
}
if ([string]::IsNullOrWhiteSpace($SiteId)) {
    $SiteId = if ($env:AVF_SITE_ID) { $env:AVF_SITE_ID } else { (Get-AutonomousTargetSiteId) }
}
Assert-AutonomousTargetMachineId -MachineId $MachineId

$ContractMetadataKeys = @("board_protocol", "bill_protocol", "cash_topology", "transport_type")
$DryRunPointerPath = Join-Path $WorkspaceRoot "reports\latest-bootstrap-repair-dryrun.txt"
$DryRunArtifactTtlMinutes = 30

$ReportsDir = Join-Path $WorkspaceRoot "reports"
New-Item -ItemType Directory -Force -Path $ReportsDir | Out-Null
$RunDir = Join-Path $ReportsDir ("bootstrap-repair-" + (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ"))
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
$BeforePath = Join-Path $ReportsDir "bootstrap-metadata-before.json"
$AfterPath = Join-Path $ReportsDir "bootstrap-metadata-after.json"
$VerdictPath = Join-Path $RunDir "repair-verdict.json"

$TargetSummaryPath = Join-Path $RunDir "target-summary.json"
$BeforeMetaPath = Join-Path $RunDir "before-metadata.json"
$PatchMetaPath = Join-Path $RunDir "patch-metadata.json"
$AfterPreviewPath = Join-Path $RunDir "after-metadata-preview.json"
$TopologySafetyPath = Join-Path $RunDir "topology-safety-diff.json"
$MetadataDiffPath = Join-Path $RunDir "metadata-diff.json"
$LiveBeforePath = Join-Path $RunDir "live-before-response.json"
$LiveAfterPath = Join-Path $RunDir "live-after-response.json"
$LiveRefetchPath = Join-Path $RunDir "live-refetch-topology.json"

$script:AdminAuthPresent = $false
$script:WriteTokenPresent = Test-AutonomousProductionWriteAllowed

function Set-Utf8JsonFile {
    param([string]$Path, [string]$Content)
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($Path, $Content, $utf8NoBom)
}

function Set-Utf8TextFile {
    param([string]$Path, [string]$Content)
    Set-Utf8JsonFile -Path $Path -Content $Content
}

function Invoke-JqToFile {
    param(
        [string]$Path,
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$JqArgs
    )
    $output = & jq @JqArgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "jq failed: $output" }
    if ($output -is [System.Array]) { $output = ($output -join "`n") }
    Set-Utf8JsonFile -Path $Path -Content ([string]$output).TrimEnd()
}

function Get-MetadataSummary {
    param([string]$MetaFile)
    if (-not (Test-Path $MetaFile)) {
        return @{
            metadataKeys = @()
            boardProtocol = ""
            billProtocol = ""
            cashTopology = ""
            transportType = ""
            paymentAuthority = ""
            primaryPort = ""
            billBusKey = ""
        }
    }
    $raw = Get-Content $MetaFile -Raw
    return @{
        metadataKeys = @((& jq -r 'keys[]?' $MetaFile 2>$null) | Where-Object { $_ })
        boardProtocol = (& jq -r '.board_protocol // empty' $MetaFile)
        billProtocol = (& jq -r '.bill_protocol // empty' $MetaFile)
        cashTopology = (& jq -r '.cash_topology // empty' $MetaFile)
        transportType = (& jq -r '.transport_type // empty' $MetaFile)
        paymentAuthority = (& jq -r '.payment_authority // empty' $MetaFile)
        primaryPort = (& jq -r '.primary_port // .serial_port // empty' $MetaFile)
        billBusKey = (& jq -r '.driver_options.billBusKey // empty' $MetaFile)
    }
}

function Write-RepairDebugMarker {
    param(
        [string]$Name,
        [hashtable]$Fields
    )
    $parts = @($Name)
    foreach ($k in ($Fields.Keys | Sort-Object)) {
        $v = $Fields[$k]
        if ($v -is [array]) { $v = ($v -join ",") }
        if ($v -is [bool]) { $v = $v.ToString().ToLower() }
        $parts += "$k=$v"
    }
    Write-Host ($parts -join " ")
}

function Get-RedactedApiPath {
    param([string]$Url)
    $p = $Url
    if (-not [string]::IsNullOrWhiteSpace($BaseUrl)) {
        $p = $p -replace [regex]::Escape($BaseUrl), '<base>'
    }
    # never echo query-string secrets (defensive; this API uses header auth)
    $p = $p -replace '([?&](token|access_token|code|activationCode)=)[^&]*', '$1<redacted>'
    return $p
}

function Invoke-RepairApiCurl {
    param(
        [string]$Method,
        [string]$Url,
        [string[]]$CurlArgs
    )
    Write-RepairDebugMarker -Name "API_REQ" -Fields @{ method = $Method; path = (Get-RedactedApiPath $Url) }
    $code = & curl.exe @CurlArgs
    Write-RepairDebugMarker -Name "API_RES" -Fields @{ method = $Method; path = (Get-RedactedApiPath $Url); httpCode = $code }
    return $code
}

function Test-RepairContractKeysPresent {
    param([string]$MetaFile)
    foreach ($key in $ContractMetadataKeys) {
        $val = (& jq -r --arg k $key '.[$k] // empty' $MetaFile)
        if ([string]::IsNullOrWhiteSpace($val)) { return $false }
    }
    return $true
}

function Test-RepairArtifactsJson {
    param(
        [switch]$LiveRun,
        [switch]$SkipVerdict
    )
    $required = @(
        $TargetSummaryPath, $BeforeMetaPath, $PatchMetaPath, $AfterPreviewPath,
        $TopologySafetyPath, $MetadataDiffPath
    )
    if (-not $SkipVerdict) {
        $required += $VerdictPath
    }
    if ($LiveRun) {
        $required += @($LiveBeforePath, $LiveAfterPath, $LiveRefetchPath)
    }
    foreach ($path in $required) {
        if (-not (Test-Path $path)) {
            throw "REPAIR_ARTIFACT_JSON_INVALID missing=$path"
        }
        try {
            Get-Content $path -Raw | ConvertFrom-Json | Out-Null
        } catch {
            throw "REPAIR_ARTIFACT_JSON_INVALID path=$path err=$($_.Exception.Message)"
        }
    }
}

function Write-TargetSummaryArtifact {
    $payload = [ordered]@{
        machineId = $MachineId
        siteId = $SiteId
        cabinetCode = $CabinetCode
        dryRun = [bool]$DryRun
        baseUrl = $BaseUrl
        adminAuthPresent = $script:AdminAuthPresent
        writeTokenPresent = $script:WriteTokenPresent
        artifactDir = $RunDir
    }
    Set-Utf8JsonFile -Path $TargetSummaryPath -Content (($payload | ConvertTo-Json -Depth 4 -Compress))
    Write-RepairDebugMarker -Name "REPAIR_TARGET_SUMMARY" -Fields @{
        machineId = $MachineId
        siteId = $SiteId
        cabinetCode = $CabinetCode
        dryRun = [bool]$DryRun
        baseUrl = $BaseUrl
        adminAuthPresent = $script:AdminAuthPresent
        writeTokenPresent = $script:WriteTokenPresent
    }
}

function Write-RepairVerdict {
    param(
        [string]$Verdict,
        [bool]$SafeToLiveApply = $false,
        [hashtable]$Extra = @{}
    )
    $payload = [ordered]@{
        timestamp = (Get-Date).ToUniversalTime().ToString("o")
        verdict = $Verdict
        safeToLiveApply = $SafeToLiveApply
        artifactDir = $RunDir
        machine_id = $MachineId
        site_id = $SiteId
        cabinet_code = $CabinetCode
        dry_run = [bool]$DryRun
    }
    foreach ($k in $Extra.Keys) { $payload[$k] = $Extra[$k] }
    Set-Utf8JsonFile -Path $VerdictPath -Content (($payload | ConvertTo-Json -Depth 10 -Compress))
    Write-RepairDebugMarker -Name "REPAIR_RESULT" -Fields @{
        verdict = $Verdict
        safeToLiveApply = $SafeToLiveApply
        artifactDir = $RunDir
    }
    Write-Host "VERDICT=$Verdict safeToLiveApply=$SafeToLiveApply artifactDir=$RunDir"
}

function Exit-RepairVerdict {
    param(
        [string]$Verdict,
        [int]$ExitCode = 1,
        [bool]$SafeToLiveApply = $false,
        [hashtable]$Extra = @{}
    )
    Write-RepairVerdict -Verdict $Verdict -SafeToLiveApply $SafeToLiveApply -Extra $Extra
    exit $ExitCode
}

function Test-SafeToLiveApplyFromArtifacts {
    param(
        [string]$Verdict,
        [hashtable]$SafetySummary
    )
    if ($Verdict -ne "BACKEND_METADATA_REPAIR_DRY_RUN_OK") { return $false }
    if ($SafetySummary.unsafeReasons.Count -gt 0) { return $false }
    if (-not $SafetySummary.onlyTargetCabinetMetadataChanged) { return $false }
    if ($SafetySummary.layoutCountAfter -lt $SafetySummary.layoutCountBefore) { return $false }
    if ($SafetySummary.slotCountAfter -lt $SafetySummary.slotCountBefore) { return $false }
    if (-not (Test-RepairContractKeysPresent -MetaFile $AfterPreviewPath)) { return $false }
    try {
        Test-RepairArtifactsJson -SkipVerdict
    } catch {
        return $false
    }
    try {
        Assert-AvfLogSecrecyDirectory -Directory $RunDir
    } catch {
        return $false
    }
    return $true
}

function Assert-LiveDryRunGates {
    if (-not (Test-Path $DryRunPointerPath)) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{
            reason = "pointer_file_missing"
            pointer = $DryRunPointerPath
        }
    }
    $artifactRel = (Get-Content $DryRunPointerPath -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($artifactRel)) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{ reason = "pointer_empty" }
    }
    $artifactDir = if ([System.IO.Path]::IsPathRooted($artifactRel)) {
        $artifactRel
    } else {
        Join-Path $WorkspaceRoot ($artifactRel -replace '/', '\')
    }
    if (-not (Test-Path $artifactDir)) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{
            reason = "artifact_dir_missing"
            artifactDir = $artifactDir
        }
    }

    $dryTargetPath = Join-Path $artifactDir "target-summary.json"
    $dryVerdictPath = Join-Path $artifactDir "repair-verdict.json"
    if (-not (Test-Path $dryTargetPath) -or -not (Test-Path $dryVerdictPath)) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{ reason = "dry_run_artifacts_missing" }
    }

    $dryTarget = Get-Content $dryTargetPath -Raw | ConvertFrom-Json
    if ($dryTarget.machineId -ne $MachineId -or $dryTarget.siteId -ne $SiteId -or $dryTarget.cabinetCode -ne $CabinetCode) {
        Exit-RepairVerdict -Verdict "DRY_RUN_ARTIFACT_TARGET_MISMATCH" -ExitCode 3 -Extra @{
            expected_machineId = $MachineId
            expected_siteId = $SiteId
            expected_cabinetCode = $CabinetCode
            actual_machineId = $dryTarget.machineId
            actual_siteId = $dryTarget.siteId
            actual_cabinetCode = $dryTarget.cabinetCode
        }
    }

    $dryVerdict = Get-Content $dryVerdictPath -Raw | ConvertFrom-Json
    $dryTs = [datetime]::Parse($dryVerdict.timestamp).ToUniversalTime()
    $age = (Get-Date).ToUniversalTime() - $dryTs
    if ($age.TotalMinutes -gt $DryRunArtifactTtlMinutes) {
        Exit-RepairVerdict -Verdict "DRY_RUN_ARTIFACT_STALE" -ExitCode 3 -Extra @{
            age_minutes = [math]::Round($age.TotalMinutes, 1)
            ttl_minutes = $DryRunArtifactTtlMinutes
            dry_run_artifactDir = $artifactDir
        }
    }

    if ($dryVerdict.verdict -ne "BACKEND_METADATA_REPAIR_DRY_RUN_OK" -or -not $dryVerdict.safeToLiveApply) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{
            dry_verdict = $dryVerdict.verdict
            dry_safeToLiveApply = $dryVerdict.safeToLiveApply
        }
    }

    $dryBeforePath = Join-Path $artifactDir "before-metadata.json"
    if (-not (Test-Path $dryBeforePath)) {
        Exit-RepairVerdict -Verdict "DRY_RUN_NOT_SAFE_FOR_LIVE_APPLY" -ExitCode 3 -Extra @{ reason = "dry_before_metadata_missing" }
    }
    return @{
        artifactDir = $artifactDir
        dryBeforePath = $dryBeforePath
    }
}

function Test-BaselineMaterialDrift {
    param(
        [string]$DryBeforePath,
        [string]$CurrentBeforePath
    )
    $same = & jq -n --slurpfile a $DryBeforePath --slurpfile b $CurrentBeforePath '($a[0] // {}) == ($b[0] // {})'
    return ($same -eq "true")
}

function Convert-GrpcBootstrapToSnapshot {
    param([string]$GrpcJsonPath)
    $normFile = Join-Path $RunDir "bootstrap-normalized.json"
    Invoke-JqToFile -Path $normFile -f (Join-Path $RunDir "bootstrap-normalize.jq") $GrpcJsonPath
    $raw = Get-Content $normFile -Raw | ConvertFrom-Json
    if (-not $raw.topology) { throw "grpc bootstrap missing topology" }
    return (Get-Content $normFile -Raw)
}

function New-SynthesizedBootstrapSnapshot {
    param([string]$TargetCabinetCode)
    $payload = [ordered]@{
        bootstrapSource = "synthesized_admin_fallback"
        topology = [ordered]@{
            cabinets = @(
                [ordered]@{
                    code = $TargetCabinetCode
                    title = "Cabinet $TargetCabinetCode"
                    sortOrder = 1
                    metadata = [ordered]@{}
                }
            )
            layouts = @()
        }
    }
    return ($payload | ConvertTo-Json -Depth 8 -Compress)
}

function Invoke-FetchBootstrapViaGrpc {
    param([string]$MachineJwt)
    if ($MachineJwt -notmatch '^eyJ') { return $null }
    $grpc = Invoke-AvfGetBootstrapDirect -MachineToken $MachineJwt -MachineId $MachineId -OutDir $RunDir -RunPrefix "bootstrap-repair"
    if ($grpc.classification -ne "OK" -or -not $grpc.response_path -or -not (Test-Path $grpc.response_path)) {
        return $null
    }
    @'
.topology as $t |
{
  topology: {
    cabinets: [($t.cabinets // [])[] | {
      code: .code,
      title: (.title // ("Cabinet " + .code)),
      sortOrder: (.sortOrder // 1),
      metadata: (.metadata // {})
    }],
    layouts: []
  },
  bootstrapSource: "grpc_getbootstrap"
}
'@ | ForEach-Object { Set-Utf8TextFile -Path (Join-Path $RunDir "bootstrap-normalize.jq") -Content $_ }
    return (Convert-GrpcBootstrapToSnapshot -GrpcJsonPath $grpc.response_path)
}

function Ensure-MachineJwtForBootstrap {
    $jwt = if ($env:AVF_MACHINE_TOKEN -match '^eyJ') { $env:AVF_MACHINE_TOKEN.Trim() } else { "" }
    if ($jwt) { return $jwt }
    $adbSerial = if ($env:ANDROID_SERIAL) { $env:ANDROID_SERIAL } else { "PFT9UY4Y59" }
    $jwt = Get-AutonomousMachineJwtFromDevice -AdbSerial $adbSerial
    if ($jwt -match '^eyJ') { return $jwt }
    if ($env:AVF_ACTIVATION_CODE) {
        . (Join-Path $ScriptsLib "invoke-machine-claim-activation.ps1")
        $null = Invoke-AvfMachineClaimActivation -ActivationCode $env:AVF_ACTIVATION_CODE -AdbSerial $adbSerial -MachineId $MachineId -SiteId $SiteId -RunPrefix "bootstrap-repair"
        if ($env:AVF_MACHINE_TOKEN -match '^eyJ') { return $env:AVF_MACHINE_TOKEN.Trim() }
    }
    $createBodyFile = Join-Path $RunDir "activation-create-body.json"
    Set-Utf8JsonFile -Path $createBodyFile -Content '{"expiresInMinutes":120,"maxUses":1,"notes":"bootstrap-repair-refetch"}'
    $createOut = Join-Path $RunDir "activation-create.json"
    $createUrl = "$BaseUrl/v1/admin/machines/$MachineId/activation-codes"
    $createArgs = @("-sS") + $Headers + @("-o", $createOut, "-w", "%{http_code}", "-X", "POST", $createUrl, "-H", "Content-Type: application/json", "--data-binary", "@$createBodyFile")
    $createCode = Invoke-RepairApiCurl -Method "POST" -Url $createUrl -CurlArgs $createArgs
    if ($createCode -eq "201") {
        $plain = (Get-Content $createOut -Raw | ConvertFrom-Json).activationCode
        if ($plain) {
            . (Join-Path $ScriptsLib "invoke-machine-claim-activation.ps1")
            $null = Invoke-AvfMachineClaimActivation -ActivationCode $plain -AdbSerial $adbSerial -MachineId $MachineId -SiteId $SiteId -RunPrefix "bootstrap-repair"
            if ($env:AVF_MACHINE_TOKEN -match '^eyJ') { return $env:AVF_MACHINE_TOKEN.Trim() }
        }
    }
    return ""
}

function Resolve-BootstrapSnapshot {
    param(
        [string[]]$Headers,
        [string]$MachineJwt = "",
        [switch]$RequireGrpcAfterWrite
    )
    if (-not $RequireGrpcAfterWrite) {
        $bootUrl = "$BaseUrl/v1/setup/machines/$MachineId/bootstrap"
        $bootArgs = @("-sS") + $Headers + @("-o", $bootstrapFile, "-w", "%{http_code}", $bootUrl)
        $httpCode = Invoke-RepairApiCurl -Method "GET" -Url $bootUrl -CurlArgs $bootArgs
        if ($httpCode -eq "200") {
            $snap = Get-Content $bootstrapFile -Raw | ConvertFrom-Json
            $snap | Add-Member -NotePropertyName bootstrapSource -NotePropertyValue "http_setup_bootstrap" -Force
            Set-Utf8JsonFile -Path $bootstrapFile -Content (($snap | ConvertTo-Json -Depth 12 -Compress))
            return "http_setup_bootstrap"
        }
    }

    if ([string]::IsNullOrWhiteSpace($MachineJwt)) {
        $MachineJwt = Ensure-MachineJwtForBootstrap
    }
    $grpcJson = Invoke-FetchBootstrapViaGrpc -MachineJwt $MachineJwt
    if ($grpcJson) {
        Set-Utf8JsonFile -Path $bootstrapFile -Content $grpcJson
        return "grpc_getbootstrap"
    }

    if ($RequireGrpcAfterWrite) {
        throw "grpc bootstrap refetch unavailable after write"
    }

    $synth = New-SynthesizedBootstrapSnapshot -TargetCabinetCode $CabinetCode
    Set-Utf8JsonFile -Path $bootstrapFile -Content $synth
    return "synthesized_admin_fallback"
}

function Extract-CabinetMetadataFile {
    param(
        [string]$BootstrapPath,
        [string]$OutPath,
        [string]$TargetCabinetCode
    )
    $jqFile = Join-Path $RunDir "extract-cabinet-metadata.jq"
    @'
($cc) as $target |
(.topology.cabinets[]? | select(.code == $target) | .metadata // {})
'@ | ForEach-Object { Set-Utf8TextFile -Path $jqFile -Content $_ }
    Invoke-JqToFile -Path $OutPath --arg cc $TargetCabinetCode -f $jqFile $BootstrapPath
}

function Test-RefetchContractKeysFromSnapshot {
    param([string]$SnapshotFile)
    foreach ($key in $ContractMetadataKeys) {
        $val = (& jq -r --arg cc $CabinetCode --arg k $key '
            (.topology.cabinets[]? | select(.code == $cc) | .metadata[$k] // empty)
        ' $SnapshotFile)
        if ([string]::IsNullOrWhiteSpace($val)) { return $false }
    }
    return $true
}

if ($DryRun) {
    Write-Host "DRY-RUN target machineId=$MachineId siteId=$SiteId cabinetCode=$CabinetCode"
}

try {
    $Token = Get-AvfAdminAccessToken -BaseUrl $BaseUrl -NonInteractive:$NonInteractive -RunPrefix "bootstrap-repair"
    $script:AdminAuthPresent = $true
} catch {
    if ($_.Exception.Message -match "BLOCKED_ADMIN_ENV_MISSING") {
        Exit-RepairVerdict -Verdict "BLOCKED_ADMIN_ENV_MISSING" -ExitCode 4
    }
    throw
}

Write-TargetSummaryArtifact

$Headers = @("-H", "Authorization: Bearer $Token", "-H", "Accept: application/json")

# --- Preflight: admin machine ---
$machineFile = Join-Path $RunDir "machine-admin.json"
$machineUrl = "$BaseUrl/v1/admin/machines/$MachineId"
$machineArgs = @("-sS") + $Headers + @("-o", $machineFile, "-w", "%{http_code}", $machineUrl)
$mCode = Invoke-RepairApiCurl -Method "GET" -Url $machineUrl -CurlArgs $machineArgs
if ($mCode -eq "404") {
    Exit-RepairVerdict -Verdict "TARGET_MACHINE_NOT_FOUND" -ExitCode 5
}
if ($mCode -ne "200") {
    Exit-RepairVerdict -Verdict "TARGET_MACHINE_READ_FAILED" -ExitCode 5 -Extra @{ http_code = $mCode }
}

$machineSite = (Get-Content $machineFile -Raw | jq -r '.siteId // .site_id // empty')
if ($machineSite -and $machineSite -ne $SiteId) {
    Exit-RepairVerdict -Verdict "TARGET_SITE_MISMATCH" -ExitCode 5 -Extra @{
        expected_site_id = $SiteId
        actual_site_id = $machineSite
    }
}

# --- Preflight: bootstrap ---
$bootstrapFile = Join-Path $RunDir "bootstrap.json"
try {
    $bootstrapSource = Resolve-BootstrapSnapshot -Headers $Headers
} catch {
    Exit-RepairVerdict -Verdict "BOOTSTRAP_READ_FAILED" -ExitCode 5 -Extra @{ error = $_.Exception.Message }
}
Write-Host "Bootstrap source: $bootstrapSource"
Copy-Item $bootstrapFile $BeforePath -Force

$cabinetExists = (Get-Content $bootstrapFile -Raw | jq -r --arg cc $CabinetCode '
  [.topology.cabinets[]? | select(.code == $cc) | .code] | first // empty
')
if ([string]::IsNullOrWhiteSpace($cabinetExists)) {
    Exit-RepairVerdict -Verdict "TARGET_CABINET_NOT_FOUND" -ExitCode 6 -Extra @{ cabinet_code = $CabinetCode }
}

Extract-CabinetMetadataFile -BootstrapPath $bootstrapFile -OutPath $BeforeMetaPath -TargetCabinetCode $CabinetCode

$beforeSummary = Get-MetadataSummary -MetaFile $BeforeMetaPath
Write-RepairDebugMarker -Name "REPAIR_CURRENT_METADATA" -Fields @{
    cabinetCode = $CabinetCode
    metadataKeysBefore = ($beforeSummary.metadataKeys -join ",")
    boardProtocolBefore = $beforeSummary.boardProtocol
    billProtocolBefore = $beforeSummary.billProtocol
    cashTopologyBefore = $beforeSummary.cashTopology
    transportTypeBefore = $beforeSummary.transportType
    paymentAuthorityBefore = $beforeSummary.paymentAuthority
    primaryPortBefore = $beforeSummary.primaryPort
    billBusKeyBefore = $beforeSummary.billBusKey
}

$unexpectedKeys = @()
foreach ($k in @("board_protocol", "bill_protocol", "cash_topology", "transport_type", "payment_authority")) {
    $v = (& jq -r --arg k $k '.[$k] // empty' $BeforeMetaPath)
    if ($v -and $v -ne "tcn" -and $k -eq "board_protocol") { $unexpectedKeys += "$k=$v" }
    if ($v -and $v -notin @("ict_bc_v1", "ict-bc") -and $k -eq "bill_protocol") { $unexpectedKeys += "$k=$v" }
    if ($v -and $v -notin @("direct_bill", "hybrid", "cash_bridge") -and $k -eq "cash_topology") { $unexpectedKeys += "$k=$v" }
}
if ($unexpectedKeys.Count -gt 0) {
    Exit-RepairVerdict -Verdict "UNEXPECTED_CURRENT_METADATA" -ExitCode 6 -Extra @{ unexpected = ($unexpectedKeys -join "; ") }
}

# --- Preflight: slots (layout preservation source) ---
$slotsFile = Join-Path $RunDir "slots-admin.json"
$slotsUrl = "$BaseUrl/v1/admin/machines/$MachineId/slots"
$slotsArgs = @("-sS") + $Headers + @("-o", $slotsFile, "-w", "%{http_code}", $slotsUrl)
$sCode = Invoke-RepairApiCurl -Method "GET" -Url $slotsUrl -CurlArgs $slotsArgs
$slotsAvailable = ($sCode -eq "200")
if (-not $slotsAvailable) {
    Set-Utf8TextFile -Path $slotsFile -Content "{}"
}

# --- Metadata patch ---
@'
{
  "board_protocol": "tcn",
  "bill_protocol": "ict_bc_v1",
  "cash_topology": "direct_bill",
  "transport_type": "serial",
  "primary_port": "/dev/ttyS4",
  "serial_port": "/dev/ttyS4",
  "driver_options": {
    "billBusKey": "/dev/ttyS1",
    "billSharesBoardSerialBus": "false"
  }
}
'@ | ForEach-Object { Set-Utf8TextFile -Path $PatchMetaPath -Content $_ }

$mergedMetaFile = Join-Path $RunDir "merged-metadata.json"
$mergeJq = Join-Path $RunDir "merge-metadata.jq"
@'
. as $base |
($patch[0]) as $patch |
(($base | del(.driver_options)) + ($patch | del(.driver_options))) as $merged |
(($base.driver_options // {}) + ($patch.driver_options // {})) as $do |
($merged + {driver_options: $do}) as $merged2 |
($base.payment_authority // "") as $existingPa |
if ($existingPa == "local" or $existingPa == "remote" or $existingPa == "hybrid" or $existingPa == "backend")
then $merged2 + {payment_authority: $existingPa}
else $merged2 + {payment_authority: "backend"}
end
'@ | ForEach-Object { Set-Utf8TextFile -Path $mergeJq -Content $_ }

Invoke-JqToFile -Path $mergedMetaFile -f $mergeJq --slurpfile patch $PatchMetaPath $BeforeMetaPath
Copy-Item $mergedMetaFile $AfterPreviewPath -Force

$patchSummary = Get-MetadataSummary -MetaFile $PatchMetaPath
$mergedSummary = Get-MetadataSummary -MetaFile $mergedMetaFile
Write-RepairDebugMarker -Name "REPAIR_PATCH_METADATA" -Fields @{
    metadataKeysPatch = ($patchSummary.metadataKeys -join ",")
    boardProtocolPatch = "tcn"
    billProtocolPatch = "ict_bc_v1"
    cashTopologyPatch = "direct_bill"
    transportTypePatch = "serial"
    paymentAuthorityPatch = $(if ($mergedSummary.paymentAuthority) { $mergedSummary.paymentAuthority } else { "backend" })
    primaryPortPatch = "/dev/ttyS4"
    billBusKeyPatch = "/dev/ttyS1"
}

$diffJq = Join-Path $RunDir "metadata-diff.jq"
@'
($before[0] // {}) as $b |
($after[0] // {}) as $a |
{
  before_keys: ($b | keys),
  after_keys: ($a | keys),
  added: [($a | keys[]) | select(. as $k | ($b[$k] // null) == null)],
  updated: [($a | keys[]) | select(. as $k | ($b[$k] // null) != null and $b[$k] != $a[$k])],
  unchanged: [($a | keys[]) | select(. as $k | ($b[$k] // null) == $a[$k])],
  before: $b,
  after: $a
}
'@ | ForEach-Object { Set-Utf8TextFile -Path $diffJq -Content $_ }
Invoke-JqToFile -Path $MetadataDiffPath -f $diffJq -n --slurpfile before $BeforeMetaPath --slurpfile after $mergedMetaFile

# --- Merge-safe topology PUT body ---
$topologyFile = Join-Path $RunDir "topology-put.json"
$topologyJq = Join-Path $RunDir "topology-build.jq"
@'
($boot[0]) as $boot |
($slots[0] // {}) as $slotsDoc |
($mergedMeta[0]) as $mergedMeta |
$cc as $targetCode |
($boot.topology.cabinets // []) as $cabinets |
($cabinets | map(
  {
    code: .code,
    title: (.title // ("Cabinet " + .code)),
    sortOrder: (.sortOrder // 1),
    metadata: (
      if .code == $targetCode then $mergedMeta
      else (.metadata // {})
      end
    )
  }
)) as $putCabinets |
($slotsDoc.items // $slotsDoc.slots // []) as $slotItems |
($boot.topology.layouts // []) as $bootLayouts |
if ($slotItems | length) > 0 then
  ($slotItems | group_by(.cabinetCode // .cabinet_code) | map(
    .[0] as $first |
    ($first.cabinetCode // $first.cabinet_code) as $ccode |
    {
      cabinetCode: $ccode,
      layoutKey: ($first.layoutKey // $first.layout_key // "default"),
      revision: ($first.layoutRevision // $first.layout_revision // 1),
      layoutSpec: {
        slots: (
          reduce .[] as $s ({}; 
            . + {
              (($s.slotCode // $s.slot_code) | tostring): {
                slotCode: ($s.slotCode // $s.slot_code),
                lane: ($s.lane // $s.slotCode // $s.slot_code)
              }
            }
          )
        )
      },
      status: "published"
    }
  )) as $layoutsFromSlots |
  { cabinets: $putCabinets, layouts: $layoutsFromSlots }
elif ($bootLayouts | length) > 0 then
  { cabinets: $putCabinets, layouts: $bootLayouts }
else
  { cabinets: $putCabinets, layouts: [] }
end
'@ | ForEach-Object { Set-Utf8TextFile -Path $topologyJq -Content $_ }

Invoke-JqToFile -Path $topologyFile `
    -n --arg cc $CabinetCode `
    --slurpfile boot $bootstrapFile `
    --slurpfile slots $slotsFile `
    --slurpfile mergedMeta $mergedMetaFile `
    -f $topologyJq

$bootLayoutCount = [int](& jq -r '.topology.layouts // [] | length' $bootstrapFile)
$putLayoutCount = [int](& jq -r '.layouts // [] | length' $topologyFile)
$bootSlotCount = [int](& jq -r '
  [.topology.layouts[]?.layoutSpec.slots // {} | keys[]] | length
' $bootstrapFile)
$putSlotCount = [int](& jq -r '
  [.layouts[]?.layoutSpec.slots // {} | keys[]] | length
' $topologyFile)
$bootCabinetCount = [int](& jq -r '.topology.cabinets // [] | length' $bootstrapFile)
$putCabinetCount = [int](& jq -r '.cabinets // [] | length' $topologyFile)

if (-not $slotsAvailable -and $bootLayoutCount -gt 0 -and $putLayoutCount -lt $bootLayoutCount) {
    if (-not $DryRun) {
        Exit-RepairVerdict -Verdict "TOPOLOGY_LAYOUT_SOURCE_UNAVAILABLE" -ExitCode 8 -Extra @{
            slots_http = $sCode
            boot_layout_count = $bootLayoutCount
            put_layout_count = $putLayoutCount
        }
    }
}

$safetyJq = Join-Path $RunDir "topology-safety.jq"
@'
($boot[0].topology // {}) as $bt |
($put[0] // {}) as $p |
($targetCode) as $tc |
($bt.cabinets // []) as $bc |
($p.cabinets // []) as $pc |
($bt.layouts // []) as $bl |
($p.layouts // []) as $pl |
def slot_count($layouts):
  ($layouts | map(.layoutSpec.slots // {} | keys | length) | add // 0);
def cabinet_sig($cabinets):
  ($cabinets | map({id:(.id//""), code:(.code//""), name:(.name//.title//"")}) | sort_by(.code));
[]
| if ($bc|length) != ($pc|length) then . + ["cabinet_count_changed"] else . end
| if cabinet_sig($bc) != cabinet_sig($pc) then . + ["cabinet_id_code_name_changed"] else . end
| if slot_count($bl) > slot_count($pl) then . + ["slot_count_decreased"] else . end
| if ($bl|length) > ($pl|length) then . + ["layout_count_decreased"] else . end
| if ($bt.configVersion // null) != null and ($p.configVersion // null) == null then . + ["topology_config_version_disappeared"] else . end
| if ($bt.version // null) != null and ($p.version // null) == null then . + ["topology_version_disappeared"] else . end
| . as $reasons |
  ($bc | map(select(.code != $tc) | .metadata // {}) ) as $nonTargetBefore |
  ($pc | map(select(.code == $tc) | . as $c | ($bc[] | select(.code == $c.code) | .metadata // {})) ) as $nonTargetAfterPairs |
  (reduce range(0; ($nonTargetBefore|length)) as $i ($reasons;
    if ($nonTargetBefore[$i] != $nonTargetAfterPairs[$i]) then . + ["non_target_cabinet_metadata_changed"] else . end
  )) as $reasons2 |
  (if ($bl|length) > 0 and ($pl|length) > 0 then
    if ($bl | tostring) != ($pl | tostring) then $reasons2 + ["layout_or_slot_structure_changed"] else $reasons2 end
  else $reasons2 end) as $unsafe |
  ($bc[] | select(.code == $tc) | .metadata // {}) as $metaBefore |
  ($pc[] | select(.code == $tc) | .metadata // {}) as $metaAfter |
  {
    cabinetCountBefore: ($bc|length),
    cabinetCountAfter: ($pc|length),
    layoutCountBefore: ($bl|length),
    layoutCountAfter: ($pl|length),
    slotCountBefore: slot_count($bl),
    slotCountAfter: slot_count($pl),
    targetCabinetMetadataChanged: ($metaBefore != $metaAfter),
    onlyTargetCabinetMetadataChanged: (
      ($unsafe | length) == 0 and
      (all($bc[]; . as $c | if $c.code == $tc then true else ($c.metadata // {}) == (($pc[] | select(.code == $c.code) | .metadata) // {}) end))
    ),
    unsafeReasons: $unsafe,
    slots_fetch_available: true
  }
'@ | ForEach-Object { Set-Utf8TextFile -Path $safetyJq -Content $_ }

Invoke-JqToFile -Path $TopologySafetyPath `
    -n --arg targetCode $CabinetCode `
    --slurpfile boot $bootstrapFile `
    --slurpfile put $topologyFile `
    -f $safetyJq

$safety = Get-Content $TopologySafetyPath -Raw | ConvertFrom-Json
$safetyHash = @{
    cabinetCountBefore = [int]$safety.cabinetCountBefore
    cabinetCountAfter = [int]$safety.cabinetCountAfter
    layoutCountBefore = [int]$safety.layoutCountBefore
    layoutCountAfter = [int]$safety.layoutCountAfter
    slotCountBefore = [int]$safety.slotCountBefore
    slotCountAfter = [int]$safety.slotCountAfter
    targetCabinetMetadataChanged = [bool]$safety.targetCabinetMetadataChanged
    onlyTargetCabinetMetadataChanged = [bool]$safety.onlyTargetCabinetMetadataChanged
    unsafeReasons = @($safety.unsafeReasons)
}

Write-RepairDebugMarker -Name "REPAIR_SAFETY_DIFF" -Fields @{
    cabinetCountBefore = $safetyHash.cabinetCountBefore
    cabinetCountAfter = $safetyHash.cabinetCountAfter
    layoutCountBefore = $safetyHash.layoutCountBefore
    layoutCountAfter = $safetyHash.layoutCountAfter
    slotCountBefore = $safetyHash.slotCountBefore
    slotCountAfter = $safetyHash.slotCountAfter
    targetCabinetMetadataChanged = $safetyHash.targetCabinetMetadataChanged
    onlyTargetCabinetMetadataChanged = $safetyHash.onlyTargetCabinetMetadataChanged
    unsafeReasons = ($safetyHash.unsafeReasons -join ";")
}

if ($safetyHash.unsafeReasons.Count -gt 0) {
    Exit-RepairVerdict -Verdict "UNSAFE_TOPOLOGY_DIFF" -ExitCode 8 -SafeToLiveApply:$false -Extra @{
        unsafeReasons = $safetyHash.unsafeReasons
    }
}

if ($DryRun) {
    $safe = Test-SafeToLiveApplyFromArtifacts -Verdict "BACKEND_METADATA_REPAIR_DRY_RUN_OK" -SafetySummary $safetyHash
    Write-RepairVerdict -Verdict "BACKEND_METADATA_REPAIR_DRY_RUN_OK" -SafeToLiveApply:$safe -Extra @{
        before_path = $BeforePath
        diff_path = $MetadataDiffPath
        topology_preview_path = $topologyFile
        slots_fetch_http = $sCode
        payment_authority = $mergedSummary.paymentAuthority
        unsafeReasons = $safetyHash.unsafeReasons
        bootstrap_source = $bootstrapSource
    }
    try {
        Test-RepairArtifactsJson
        Assert-AvfLogSecrecyDirectory -Directory $RunDir
    } catch {
        Exit-RepairVerdict -Verdict "REPAIR_ARTIFACT_JSON_INVALID" -ExitCode 9 -Extra @{ error = $_.Exception.Message }
    }
    if ($safe) {
        $pointerRel = "reports/" + (Split-Path $RunDir -Leaf)
        Set-Content -Path $DryRunPointerPath -Value $pointerRel -Encoding utf8
    }
    Write-Host "DRY-RUN complete before=$BeforePath diff=$MetadataDiffPath"
    exit 0
}

# --- Live apply gates ---
$dryGate = Assert-LiveDryRunGates
if (-not (Test-BaselineMaterialDrift -DryBeforePath $dryGate.dryBeforePath -CurrentBeforePath $BeforeMetaPath)) {
    Exit-RepairVerdict -Verdict "DRY_RUN_BASELINE_CHANGED" -ExitCode 3 -Extra @{
        dry_run_artifactDir = $dryGate.artifactDir
    }
}

if (-not (Test-AutonomousProductionWriteAllowed)) {
    Write-Error "Live write blocked: set CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE"
    Exit-RepairVerdict -Verdict "BOOTSTRAP_METADATA_REPAIR_READY_BUT_NOT_APPLIED" -ExitCode 3
}

$preflightScript = Join-Path $ScriptsLib "prod-write-preflight.ps1"
if (Test-Path $preflightScript) {
    & $preflightScript -NonInteractive:$NonInteractive -AllowLiveWrite `
        -AffectedScripts @("repair-machine-bootstrap-metadata.ps1")
    if ($LASTEXITCODE -ne 0) {
        Exit-RepairVerdict -Verdict "PROD_WRITE_PREFLIGHT_FAILED" -ExitCode 3
    }
}

Copy-Item $bootstrapFile $LiveBeforePath -Force

$opBodyFile = Join-Path $RunDir "operator-start-body.json"
Set-Utf8JsonFile -Path $opBodyFile -Content '{"force_admin_takeover":true,"auth_method":"oidc"}'
$opOut = Join-Path $RunDir "operator.json"
$opUrl = "$BaseUrl/v1/admin/machines/$MachineId/operator-sessions/start"
$opArgs = @("-sS") + $Headers + @("-o", $opOut, "-w", "%{http_code}", "-X", "POST", $opUrl, "-H", "Content-Type: application/json", "--data-binary", "@$opBodyFile")
$opCode = Invoke-RepairApiCurl -Method "POST" -Url $opUrl -CurlArgs $opArgs
if ($opCode -ne "200") {
    Exit-RepairVerdict -Verdict "OPERATOR_SESSION_FAILED" -ExitCode 7 -Extra @{ http_code = $opCode }
}
$opSid = (Get-Content $opOut -Raw | jq -r '.session.id')
if ([string]::IsNullOrWhiteSpace($opSid)) {
    Exit-RepairVerdict -Verdict "OPERATOR_SESSION_FAILED" -ExitCode 7
}

$putWrapJq = Join-Path $RunDir "topology-wrap-session.jq"
Set-Utf8TextFile -Path $putWrapJq -Content '. + {operator_session_id: $sid}'
$putBodyFile = Join-Path $RunDir "topology-put-with-session.json"
Invoke-JqToFile -Path $putBodyFile --arg sid $opSid -f $putWrapJq $topologyFile

$putOut = Join-Path $RunDir "put-response.txt"
$putUrl = "$BaseUrl/v1/admin/machines/$MachineId/topology"
$putArgs = @("-sS") + $Headers + @("-o", $putOut, "-w", "%{http_code}", "-X", "PUT", $putUrl, "-H", "Content-Type: application/json", "--data-binary", "@$putBodyFile")
$putCode = Invoke-RepairApiCurl -Method "PUT" -Url $putUrl -CurlArgs $putArgs
$liveAfterPayload = @{
    http_code = $putCode
    response = (Get-Content $putOut -Raw -ErrorAction SilentlyContinue)
}
Set-Utf8JsonFile -Path $LiveAfterPath -Content (($liveAfterPayload | ConvertTo-Json -Depth 4 -Compress))

if ($putCode -ne "204") {
    Exit-RepairVerdict -Verdict "TOPOLOGY_PUT_FAILED" -ExitCode 7 -Extra @{
        http_code = $putCode
        response = (Get-Content $putOut -Raw -ErrorAction SilentlyContinue)
    }
}

try {
    $null = Resolve-BootstrapSnapshot -Headers $Headers -RequireGrpcAfterWrite
    Copy-Item $bootstrapFile $LiveRefetchPath -Force
    Copy-Item $bootstrapFile $AfterPath -Force
} catch {
    Exit-RepairVerdict -Verdict "BACKEND_METADATA_WRITE_NOT_VISIBLE_AFTER_REFETCH" -ExitCode 7 -Extra @{
        error = $_.Exception.Message
        stage = "grpc_refetch_unavailable"
    }
}

$refetchMetaFile = Join-Path $RunDir "refetch-target-metadata.json"
Extract-CabinetMetadataFile -BootstrapPath $LiveRefetchPath -OutPath $refetchMetaFile -TargetCabinetCode $CabinetCode
Copy-Item $refetchMetaFile $AfterPreviewPath -Force

$afterSummary = Get-MetadataSummary -MetaFile $refetchMetaFile
Write-RepairDebugMarker -Name "REPAIR_CURRENT_METADATA" -Fields @{
    cabinetCode = $CabinetCode
    metadataKeysBefore = ($beforeSummary.metadataKeys -join ",")
    boardProtocolBefore = $afterSummary.boardProtocol
    billProtocolBefore = $afterSummary.billProtocol
    cashTopologyBefore = $afterSummary.cashTopology
    transportTypeBefore = $afterSummary.transportType
    paymentAuthorityBefore = $afterSummary.paymentAuthority
    primaryPortBefore = $afterSummary.primaryPort
    billBusKeyBefore = $afterSummary.billBusKey
}

if (-not (Test-RefetchContractKeysFromSnapshot -SnapshotFile $LiveRefetchPath)) {
    Exit-RepairVerdict -Verdict "BACKEND_METADATA_WRITE_NOT_VISIBLE_AFTER_REFETCH" -ExitCode 7 -Extra @{
        after_metadata_keys = ($afterSummary.metadataKeys -join ",")
    }
}

try {
    Test-RepairArtifactsJson -LiveRun -SkipVerdict
    Assert-AvfLogSecrecyDirectory -Directory $RunDir
} catch {
    Exit-RepairVerdict -Verdict "REPAIR_ARTIFACT_JSON_INVALID" -ExitCode 9 -Extra @{ error = $_.Exception.Message }
}

Write-Host "SUCCESS before=$BeforePath after=$AfterPath targetMetadataKeys=$($afterSummary.metadataKeys -join ',')"
Exit-RepairVerdict -Verdict "BACKEND_METADATA_REPAIRED" -ExitCode 0 -SafeToLiveApply:$false -Extra @{
    before_path = $BeforePath
    after_path = $AfterPath
    after_metadata_keys = ($afterSummary.metadataKeys -join ",")
}
