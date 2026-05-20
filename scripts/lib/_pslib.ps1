# Shared helpers for scripts\local\*.ps1 — resolve repo root from any invocation path.
function Get-AvfRepoRoot {
    param(
        # Default: parent of scripts\local (repository root with go.mod)
        [string] $StartPath = $PSScriptRoot
    )
    $dir = Resolve-Path -LiteralPath $StartPath
    for ($i = 0; $i -lt 12; $i++) {
        $dirPath = if ($dir -is [string]) { $dir } else { $dir.Path }
        $gm = Join-Path $dirPath 'go.mod'
        if (Test-Path -LiteralPath $gm) {
            return (Resolve-Path -LiteralPath $dirPath).Path
        }
        $parent = Split-Path $dirPath -Parent
        if ([string]::IsNullOrEmpty($parent) -or $parent -eq $dirPath) {
            break
        }
        $dir = $parent
    }
    throw "Could not locate repository root (go.mod) starting from: $StartPath"
}

function Convert-ToGitBashPath {
    param([string] $WindowsPath)
    $p = (Resolve-Path -LiteralPath $WindowsPath).Path
    if ($p -match '^([A-Za-z]):[\\/](.+)$') {
        $drive = $Matches[1].ToLowerInvariant()
        $rest = $Matches[2] -replace '\\', '/'
        return "/$drive/$rest"
    }
    return $p -replace '\\', '/'
}

function Export-GoTestJsonlSummary {
    param(
        [Parameter(Mandatory)]
        [string] $JsonlPath,
        [Parameter(Mandatory)]
        [string] $OutCsv,
        [Parameter(Mandatory)]
        [string] $OutTxt,
        [Parameter(Mandatory)]
        [string] $OutFailed
    )
    if (-not (Test-Path -LiteralPath $JsonlPath)) {
        'No JSONL input file.' | Set-Content -LiteralPath $OutTxt -Encoding utf8
        'Package,Outcome,FailedTestCount' | Set-Content -LiteralPath $OutCsv -Encoding utf8
        '' | Set-Content -LiteralPath $OutFailed -Encoding utf8
        return
    }

    $finalPackage = @{}
    $failedTests = [System.Collections.Generic.List[string]]::new()

    Get-Content -LiteralPath $JsonlPath -Encoding utf8 -ErrorAction Stop | ForEach-Object {
        $line = $_.TrimEnd()
        if ([string]::IsNullOrWhiteSpace($line)) { return }
        try {
            $e = $line | ConvertFrom-Json -ErrorAction Stop
        } catch {
            return
        }
        if (-not $e.Package) { return }
        $testName = $null
        if ($e.PSObject.Properties.Name -contains 'Test') {
            $testName = $e.Test
        }
        if ([string]::IsNullOrEmpty($testName)) {
            if ($e.Action -in @('pass', 'fail', 'skip')) {
                $finalPackage[$e.Package] = $e.Action
            }
        } elseif ($e.Action -eq 'fail') {
            $failedTests.Add("$($e.Package)::$($testName)")
        }
    }

    $failCountByPkg = @{}
    foreach ($ft in $failedTests) {
        $pkg = $ft.Split('::')[0]
        if (-not $failCountByPkg.ContainsKey($pkg)) {
            $failCountByPkg[$pkg] = 0
        }
        $failCountByPkg[$pkg]++
    }

    $ordered = $finalPackage.Keys | Sort-Object
    $sbTxt = [System.Text.StringBuilder]::new()
    $null = $sbTxt.AppendLine('Package Outcome (from go test -json)')
    $null = $sbTxt.AppendLine('====================================')
    foreach ($pkg in $ordered) {
        $out = $finalPackage[$pkg]
        $nfc = 0
        if ($failCountByPkg.ContainsKey($pkg)) {
            $nfc = $failCountByPkg[$pkg]
        }
        $null = $sbTxt.AppendLine(('{0,-60} {1,-6} failed_tests={2}' -f $pkg, $out, $nfc))
    }
    $null = $sbTxt.AppendLine()
    $null = $sbTxt.AppendLine(('Total packages with outcome lines: {0}' -f $ordered.Count))
    $sbTxt.ToString() | Set-Content -LiteralPath $OutTxt -Encoding utf8

    'Package,Outcome,FailedTestCount' | Set-Content -LiteralPath $OutCsv -Encoding utf8
    foreach ($pkg in $ordered) {
        $out = $finalPackage[$pkg]
        $nfc = 0
        if ($failCountByPkg.ContainsKey($pkg)) {
            $nfc = $failCountByPkg[$pkg]
        }
        ('"{0}","{1}","{2}"' -f ($pkg -replace '"', '""'), $out, $nfc) | Add-Content -LiteralPath $OutCsv -Encoding utf8
    }

    $failedTests | Set-Content -LiteralPath $OutFailed -Encoding utf8
}

# ---- E2E report parsing (strict; avoids false positives from "P0" in prose or env-var names) ----

function Get-E2ERemediationStructuredFailureCount {
    param([string] $RemediationMarkdownPath)
    if (-not (Test-Path -LiteralPath $RemediationMarkdownPath)) {
        return -1
    }
    $txt = Get-Content -LiteralPath $RemediationMarkdownPath -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($txt)) {
        return -1
    }
    if ($txt -match '(?i)Structured hints for\s+\*\*(\d+)\*\*') {
        return [int]$Matches[1]
    }
    if ($txt -match '(?si)\*\*No failed steps') {
        return 0
    }
    if ($txt -match '(?i)^##\s+Result\b[^\r\n]*(?:\r?\n).*No failed steps') {
        return 0
    }
    return -1
}

function Get-E2EFlowImprovementSeverityTotals {
    param([string] $SnapshotDir)

    $scorePath = Join-Path $SnapshotDir 'flow-review-scorecard.json'
    if (Test-Path -LiteralPath $scorePath) {
        try {
            $json = Get-Content -LiteralPath $scorePath -Raw -Encoding UTF8 -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
            $rows = @($json)
            $p0Sum = 0
            $p1Sum = 0
            foreach ($row in $rows) {
                if ($null -eq $row) { continue }
                $fc = $row.finding_counts
                if ($null -ne $fc) {
                    if ($null -ne $fc.P0) { $p0Sum += [int]$fc.P0 }
                    if ($null -ne $fc.P1) { $p1Sum += [int]$fc.P1 }
                }
            }
            return [pscustomobject]@{
                P0Count       = $p0Sum
                P1Count       = $p1Sum
                Source        = 'flow-review-scorecard.json'
                SourcePath    = $scorePath
            }
        }
        catch {
            # Fall through to markdown fallbacks
        }
    }

    $summaryPath = Join-Path $SnapshotDir 'summary.md'
    if (Test-Path -LiteralPath $summaryPath) {
        $txt = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
        $p0m = [regex]::Match($txt, '(?im)^\s*-\s*P0:\s*\*\*\s*(\d+)\s*\*\*')
        $p1m = [regex]::Match($txt, '(?im)^\s*-\s*P1:\s*\*\*\s*(\d+)\s*\*\*')
        if ($p0m.Success -and $p1m.Success) {
            return [pscustomobject]@{
                P0Count       = [int]$p0m.Groups[1].Value
                P1Count       = [int]$p1m.Groups[1].Value
                Source        = 'summary.md (Flow Improvement Findings)'
                SourcePath    = $summaryPath
            }
        }
    }

    $impPath = Join-Path $SnapshotDir 'improvement-summary.md'
    if (Test-Path -LiteralPath $impPath) {
        $txt = Get-Content -LiteralPath $impPath -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
        $tbl0 = [regex]::Match($txt, '(?im)^\|\s*P0\s*\|\s*(\d+)\s*\|')
        $tbl1 = [regex]::Match($txt, '(?im)^\|\s*P1\s*\|\s*(\d+)\s*\|')
        if ($tbl0.Success -and $tbl1.Success) {
            return [pscustomobject]@{
                P0Count       = [int]$tbl0.Groups[1].Value
                P1Count       = [int]$tbl1.Groups[1].Value
                Source        = 'improvement-summary.md (count table)'
                SourcePath    = $impPath
            }
        }
        $exec0 = [regex]::Match($txt, '\*\*P0=(\d+)\*\*')
        $exec1 = [regex]::Match($txt, '\*\*P1=(\d+)\*\*')
        if ($exec0.Success -and $exec1.Success) {
            return [pscustomobject]@{
                P0Count       = [int]$exec0.Groups[1].Value
                P1Count       = [int]$exec1.Groups[1].Value
                Source        = 'improvement-summary.md (executive summary)'
                SourcePath    = $impPath
            }
        }
    }

    return [pscustomobject]@{
        P0Count       = -1
        P1Count       = -1
        Source        = 'none'
        SourcePath    = ''
    }
}
