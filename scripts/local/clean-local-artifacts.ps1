<#
.SYNOPSIS
  Remove gitignored local temp/test/e2e artifacts. Dry-run by default; use -Apply to delete.

.DESCRIPTION
  Refuses to delete tracked files (git ls-files). Safe for developer workstations only.
#>
param(
    [switch] $Apply
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

$RepoRoot = (git -C $PSScriptRoot rev-parse --show-toplevel 2>$null)
if (-not $RepoRoot) {
    throw 'Not inside a git repository.'
}
Set-Location -LiteralPath $RepoRoot

$dirPatterns = @(
    '.tmp', '.go-tmp', 'tmp', 'temp', '.cache',
    '.test-runs', '.e2e-runs', 'tests/e2e/.e2e-runs', '.production-smoke-runs', '.production-latency-runs',
    'ci-reports', 'security-reports', 'coverage', 'dist', 'bin'
)

function Test-IsTracked([string] $Path) {
    $tracked = git ls-files -- $Path 2>$null
    return [bool]$tracked
}

$candidates = [System.Collections.Generic.List[string]]::new()

foreach ($name in $dirPatterns) {
    $full = Join-Path $RepoRoot $name
    if (Test-Path -LiteralPath $full) {
        $candidates.Add($name)
    }
}

Get-ChildItem -Force -Directory -Filter '.tmp-*' -ErrorAction SilentlyContinue | ForEach-Object {
    $rel = $_.Name
    if (-not $candidates.Contains($rel)) { $candidates.Add($rel) }
}

$migrationReport = Join-Path $RepoRoot 'migration-evidence/migration-safety-report.json'
if (Test-Path -LiteralPath $migrationReport) {
    $candidates.Add('migration-evidence/migration-safety-report.json')
}

$filePatterns = @('*.log', '*.bak', '*.old', '*.orig', 'repomix-output*.xml', 'newman-report.json', 'newman-junit.xml')
foreach ($pat in $filePatterns) {
    Get-ChildItem -Recurse -Force -File -Filter $pat -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '[\\/]\.git[\\/]' } |
        ForEach-Object {
            $rel = $_.FullName.Substring($RepoRoot.Length + 1) -replace '\\', '/'
            if (-not $candidates.Contains($rel)) { $candidates.Add($rel) }
        }
}

if ($candidates.Count -eq 0) {
    Write-Host 'No local artifact candidates found.'
    exit 0
}

Write-Host "Local artifact candidates ($($candidates.Count)):"
$toDelete = [System.Collections.Generic.List[string]]::new()
$skipped = 0
foreach ($p in ($candidates | Sort-Object -Unique)) {
    if (Test-IsTracked $p) {
        Write-Host "  SKIP (tracked): $p"
        $skipped++
    } else {
        Write-Host "  $p"
        $toDelete.Add($p)
    }
}

if ($skipped -gt 0) {
    Write-Warning "Refused to delete $skipped tracked path(s)."
}

if (-not $Apply) {
    Write-Host ''
    Write-Host "Dry-run only. Re-run with -Apply to delete $($toDelete.Count) path(s)."
    exit 0
}

foreach ($p in $toDelete) {
    $full = Join-Path $RepoRoot ($p -replace '/', [IO.Path]::DirectorySeparatorChar)
    if (Test-Path -LiteralPath $full) {
        Remove-Item -LiteralPath $full -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "Deleted $($toDelete.Count) path(s)."
git status --short
