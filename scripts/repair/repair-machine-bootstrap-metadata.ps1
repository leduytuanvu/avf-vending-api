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

$BaseUrl = Get-AvfApiBaseUrlNormalized
if ([string]::IsNullOrWhiteSpace($MachineId)) {
    $MachineId = if ($env:AVF_MACHINE_ID) { $env:AVF_MACHINE_ID } else { (Get-AutonomousTargetMachineId) }
}
if ([string]::IsNullOrWhiteSpace($SiteId)) {
    $SiteId = if ($env:AVF_SITE_ID) { $env:AVF_SITE_ID } else { (Get-AutonomousTargetSiteId) }
}
Assert-AutonomousTargetMachineId -MachineId $MachineId

$ReportsDir = Join-Path $WorkspaceRoot "reports"
New-Item -ItemType Directory -Force -Path $ReportsDir | Out-Null
$RunDir = Join-Path $ReportsDir ("bootstrap-repair-" + (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ"))
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
$BeforePath = Join-Path $ReportsDir "bootstrap-metadata-before.json"
$AfterPath = Join-Path $ReportsDir "bootstrap-metadata-after.json"
$VerdictPath = Join-Path $RunDir "verdict.json"

function Write-RepairVerdict {
    param(
        [string]$Verdict,
        [hashtable]$Extra = @{}
    )
    $payload = [ordered]@{
        timestamp = (Get-Date).ToUniversalTime().ToString("o")
        verdict = $Verdict
        machine_id = $MachineId
        site_id = $SiteId
        cabinet_code = $CabinetCode
        dry_run = [bool]$DryRun
        run_dir = $RunDir
    }
    foreach ($k in $Extra.Keys) { $payload[$k] = $Extra[$k] }
    ($payload | ConvertTo-Json -Depth 8) | Set-Content $VerdictPath -Encoding utf8
    Write-Host "VERDICT=$Verdict run_dir=$RunDir"
}

function Exit-RepairVerdict {
    param(
        [string]$Verdict,
        [int]$ExitCode = 1,
        [hashtable]$Extra = @{}
    )
    Write-RepairVerdict -Verdict $Verdict -Extra $Extra
    exit $ExitCode
}

function Set-Utf8JsonFile {
    param(
        [string]$Path,
        [string]$Content
    )
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($Path, $Content, $utf8NoBom)
}

if ($DryRun) {
    Write-Host "DRY-RUN target machineId=$MachineId siteId=$SiteId cabinetCode=$CabinetCode"
}

try {
    $Token = Get-AvfAdminAccessToken -BaseUrl $BaseUrl -NonInteractive:$NonInteractive -RunPrefix "bootstrap-repair"
} catch {
    if ($_.Exception.Message -match "BLOCKED_ADMIN_ENV_MISSING") {
        Exit-RepairVerdict -Verdict "BLOCKED_ADMIN_ENV_MISSING" -ExitCode 4
    }
    throw
}

$Headers = @("-H", "Authorization: Bearer $Token", "-H", "Accept: application/json")

# --- Preflight: admin machine ---
$machineFile = Join-Path $RunDir "machine-admin.json"
$mCode = curl.exe -sS @Headers -o $machineFile -w "%{http_code}" "$BaseUrl/v1/admin/machines/$MachineId"
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
$bCode = curl.exe -sS @Headers -o $bootstrapFile -w "%{http_code}" "$BaseUrl/v1/setup/machines/$MachineId/bootstrap"
if ($bCode -eq "404") {
    Exit-RepairVerdict -Verdict "TARGET_MACHINE_NOT_FOUND" -ExitCode 5 -Extra @{ stage = "bootstrap" }
}
if ($bCode -ne "200") {
    Exit-RepairVerdict -Verdict "BOOTSTRAP_READ_FAILED" -ExitCode 5 -Extra @{ http_code = $bCode }
}
Copy-Item $bootstrapFile $BeforePath -Force

$cabinetExists = (Get-Content $bootstrapFile -Raw | jq -r --arg cc $CabinetCode '
  [.topology.cabinets[]? | select(.code == $cc) | .code] | first // empty
')
if ([string]::IsNullOrWhiteSpace($cabinetExists)) {
    Exit-RepairVerdict -Verdict "TARGET_CABINET_NOT_FOUND" -ExitCode 6 -Extra @{ cabinet_code = $CabinetCode }
}

$beforeMetaFile = Join-Path $RunDir "before-metadata.json"
Get-Content $bootstrapFile -Raw | jq --arg cc $CabinetCode '
  (.topology.cabinets[]? | select(.code == $cc) | .metadata // {})
' | ForEach-Object { Set-Utf8JsonFile -Path $beforeMetaFile -Content $_ }

$beforeKeys = @(Get-Content $beforeMetaFile -Raw | jq -r 'keys[]?' 2>$null)
$unexpectedKeys = @()
foreach ($k in @("board_protocol", "bill_protocol", "cash_topology", "transport_type", "payment_authority")) {
    $v = (Get-Content $beforeMetaFile -Raw | jq -r --arg k $k '.[$k] // empty')
    if ($v -and $v -ne "tcn" -and $k -eq "board_protocol") { $unexpectedKeys += "$k=$v" }
    if ($v -and $v -notin @("ict_bc_v1", "ict-bc") -and $k -eq "bill_protocol") { $unexpectedKeys += "$k=$v" }
    if ($v -and $v -notin @("direct_bill", "hybrid", "cash_bridge") -and $k -eq "cash_topology") { $unexpectedKeys += "$k=$v" }
}
if ($unexpectedKeys.Count -gt 0) {
    Exit-RepairVerdict -Verdict "UNEXPECTED_CURRENT_METADATA" -ExitCode 6 -Extra @{ unexpected = ($unexpectedKeys -join "; ") }
}

# --- Preflight: slots (layout preservation source) ---
$slotsFile = Join-Path $RunDir "slots-admin.json"
$sCode = curl.exe -sS @Headers -o $slotsFile -w "%{http_code}" "$BaseUrl/v1/admin/machines/$MachineId/slots"
$slotsAvailable = ($sCode -eq "200")
if (-not $slotsAvailable) {
    "{}" | Set-Content $slotsFile -Encoding utf8
}

# --- Metadata patch (target machine hints) ---
$patchFile = Join-Path $RunDir "metadata-patch.json"
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
'@ | Set-Content $patchFile -Encoding utf8

$mergedMetaFile = Join-Path $RunDir "merged-metadata.json"
$mergeJq = Join-Path $RunDir "merge-metadata.jq"
@'
. as $base |
($patch[0]) as $patch |
($base + $patch) as $merged |
($base.payment_authority // "") as $existingPa |
if ($existingPa == "local" or $existingPa == "remote" or $existingPa == "hybrid" or $existingPa == "backend")
then $merged + {payment_authority: $existingPa}
else $merged + {payment_authority: "backend"}
end
'@ | Set-Content $mergeJq -Encoding utf8

jq --slurpfile patch $patchFile -f $mergeJq $beforeMetaFile | ForEach-Object { Set-Utf8JsonFile -Path $mergedMetaFile -Content $_ }

$diffFile = Join-Path $RunDir "metadata-diff.json"
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
'@ | Set-Content $diffJq -Encoding utf8
jq -n --slurpfile before $beforeMetaFile --slurpfile after $mergedMetaFile -f $diffJq | ForEach-Object { Set-Utf8JsonFile -Path $diffFile -Content $_ }

# --- Merge-safe topology PUT body ---
$topologyFile = Join-Path $RunDir "topology-put.json"
$topologyJq = Join-Path $RunDir "topology-build.jq"
@'
$boot as $boot |
$slots as $slotsDoc |
$mergedMeta as $mergedMeta |
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
if ($slotItems | length) > 0 then
  ($slotItems | group_by(.cabinetCode // .cabinet_code) | map(
    .[0] as $first |
    ($first.cabinetCode // $first.cabinet_code) as $cc |
    {
      cabinetCode: $cc,
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
else
  { cabinets: $putCabinets, layouts: [] }
end
'@ | Set-Content $topologyJq -Encoding utf8

jq -n `
    --arg cc $CabinetCode `
    --argjson boot (Get-Content $bootstrapFile -Raw) `
    --argjson slots (Get-Content $slotsFile -Raw) `
    --argjson mergedMeta (Get-Content $mergedMetaFile -Raw) `
    -f $topologyJq | ForEach-Object { Set-Utf8JsonFile -Path $topologyFile -Content $_ }

$preview = (Get-Content $topologyFile -Raw | jq -c --arg cc $CabinetCode '{
  cabinetCount: (.cabinets|length),
  layoutCount: (.layouts|length),
  targetMetadataKeys: (.cabinets[] | select(.code==$cc) | .metadata | keys),
  slots_source_available: true
}')
Write-Host "Topology preview: $preview"
Copy-Item $diffFile (Join-Path $RunDir "metadata-diff-sanitized.json") -Force

if ($DryRun) {
    Write-Host "DRY-RUN complete before=$BeforePath diff=$diffFile"
    Exit-RepairVerdict -Verdict "BACKEND_METADATA_REPAIR_DRY_RUN_OK" -ExitCode 0 -Extra @{
        before_path = $BeforePath
        diff_path = $diffFile
        topology_preview_path = $topologyFile
        slots_fetch_http = $sCode
        payment_authority = (Get-Content $mergedMetaFile -Raw | jq -r '.payment_authority // empty')
    }
}

if (-not (Test-AutonomousProductionWriteAllowed)) {
    Write-Error "Live write blocked: set CONFIRM_PRODUCTION_TEST_WRITE_ON_TEST_MACHINE"
    Exit-RepairVerdict -Verdict "BOOTSTRAP_METADATA_REPAIR_READY_BUT_NOT_APPLIED" -ExitCode 3
}

$preflightScript = Join-Path $ScriptsLib "prod-write-preflight.ps1"
if (Test-Path $preflightScript) {
    & $preflightScript -NonInteractive:$NonInteractive -AffectedScripts @("repair-machine-bootstrap-metadata.ps1")
    if ($LASTEXITCODE -ne 0) {
        Exit-RepairVerdict -Verdict "PROD_WRITE_PREFLIGHT_FAILED" -ExitCode 3
    }
}

$opOut = Join-Path $RunDir "operator.json"
$opCode = curl.exe -sS @Headers -o $opOut -w "%{http_code}" -X POST "$BaseUrl/v1/admin/machines/$MachineId/operator-sessions/start" -H "Content-Type: application/json" -d '{"force_admin_takeover":true,"auth_method":"oidc"}'
if ($opCode -ne "200") {
    Exit-RepairVerdict -Verdict "OPERATOR_SESSION_FAILED" -ExitCode 7 -Extra @{ http_code = $opCode }
}
$opSid = (Get-Content $opOut -Raw | jq -r '.session.id')
if ([string]::IsNullOrWhiteSpace($opSid)) {
    Exit-RepairVerdict -Verdict "OPERATOR_SESSION_FAILED" -ExitCode 7
}

$putBodyFile = Join-Path $RunDir "topology-put-with-session.json"
Get-Content $topologyFile -Raw | jq --arg sid $opSid '. + {operator_session_id:$sid}' | ForEach-Object { Set-Utf8JsonFile -Path $putBodyFile -Content $_ }
$putOut = Join-Path $RunDir "put-response.txt"
$putBody = (Get-Content $putBodyFile -Raw)
$putCode = curl.exe -sS @Headers -o $putOut -w "%{http_code}" -X PUT "$BaseUrl/v1/admin/machines/$MachineId/topology" -H "Content-Type: application/json" -d $putBody
if ($putCode -ne "204") {
    Exit-RepairVerdict -Verdict "TOPOLOGY_PUT_FAILED" -ExitCode 7 -Extra @{
        http_code = $putCode
        response = (Get-Content $putOut -Raw -ErrorAction SilentlyContinue)
    }
}

curl.exe -sS @Headers -o $AfterPath "$BaseUrl/v1/setup/machines/$MachineId/bootstrap"
$afterKeys = (Get-Content $AfterPath -Raw | jq -r --arg cc $CabinetCode '
  (.topology.cabinets[]? | select(.code == $cc) | .metadata // {} | keys | join(","))
')
Write-Host "SUCCESS before=$BeforePath after=$AfterPath targetMetadataKeys=$afterKeys"
Exit-RepairVerdict -Verdict "BACKEND_METADATA_REPAIRED" -ExitCode 0 -Extra @{
    before_path = $BeforePath
    after_path = $AfterPath
    after_metadata_keys = $afterKeys
}
