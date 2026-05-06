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
