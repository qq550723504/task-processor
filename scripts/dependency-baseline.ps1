param(
    [string]$Root = ".",
    [string]$OutputDir = "docs/refactoring",
    [string]$DependencyBaselineFile = "dependency-baseline.generated.md",
    [string]$PackageMapFile = "package-map.generated.md",
    [string]$PackagesFile = "packages-baseline.txt",
    [string]$ModGraphFile = "mod-graph-baseline.txt",
    [switch]$RunGoTest,
    [switch]$RunCoverage,
    [switch]$FailOnViolation
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path $Root
Set-Location $repoRoot

$outputPath = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Path $outputPath -Force | Out-Null

function Get-GoFiles($Path) {
    if (-not (Test-Path $Path)) {
        return @()
    }
    return Get-ChildItem -Path $Path -Filter "*.go" -Recurse | Where-Object { $_.Name -notlike "*_test.go" }
}

function Get-GoPackageTargets {
    $targets = New-Object System.Collections.Generic.List[string]
    foreach ($path in @("cmd/...", "internal/...", "tests/...")) {
        $baseDir = $path.Substring(0, $path.IndexOf('/'))
        if (Test-Path $baseDir) {
            $targets.Add("./$path")
        }
    }
    return $targets
}

function Get-ImportsFromFile($File) {
    $content = Get-Content $File.FullName -Raw
    $imports = New-Object System.Collections.Generic.List[string]

    $blockMatches = [regex]::Matches($content, 'import\s*\((?s:.*?)\)')
    foreach ($block in $blockMatches) {
        $stringMatches = [regex]::Matches($block.Value, '"([^"]+)"')
        foreach ($match in $stringMatches) {
            $imports.Add($match.Groups[1].Value)
        }
    }

    $singleMatches = [regex]::Matches($content, 'import\s+"([^"]+)"')
    foreach ($match in $singleMatches) {
        $imports.Add($match.Groups[1].Value)
    }

    return $imports | Sort-Object -Unique
}

function Get-PackageName($File) {
    $relative = $File.FullName.Substring($repoRoot.Path.Length).TrimStart('\', '/')
    $dir = Split-Path $relative -Parent
    if ([string]::IsNullOrWhiteSpace($dir)) {
        return "."
    }
    return $dir.Replace('\', '/')
}

function Get-RepoRelativePath($File) {
    return ($File.FullName.Substring($repoRoot.Path.Length).TrimStart('\', '/') -replace '\\', '/')
}

function Get-BoundaryViolations($GoFiles) {
    $forbiddenRules = @(
        @{ From = '^internal/catalog(/|$)';       Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'catalog must not depend on listingkit' },
        @{ From = '^internal/asset(/|$)';         Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'asset must not depend on listingkit' },
        @{ From = '^internal/productimage(/|$)';  Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'productimage must not depend on listingkit' },
        @{ From = '^internal/publishing/';        Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'publishing packages must not depend on listingkit facade' },
        @{ From = '^internal/workspace/';         Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'workspace packages must not depend on listingkit facade' },
        @{ From = '^internal/amazon(/|$)';        Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'marketplace packages must not depend on listingkit facade' },
        @{ From = '^internal/shein(/|$)';         Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'marketplace packages must not depend on listingkit facade' },
        @{ From = '^internal/temu(/|$)';          Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'marketplace packages must not depend on listingkit facade' },
        @{ From = '^internal/walmart(/|$)';       Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'marketplace packages must not depend on listingkit facade' },
        @{ From = '^internal/infra/';             Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'infra must not depend on listingkit' },
        @{ From = '^internal/platform/';          Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'platform must not depend on listingkit' },
        @{ From = '^internal/integration/';       Import = '^task-processor/internal/listingkit(/|$)'; Reason = 'integration must not depend on listingkit' },
        @{ From = '^internal/(catalog|asset|productimage|publishing|workspace|amazon|shein|temu|walmart)(/|$)'; Import = '^github.com/gin-gonic/gin$'; Reason = 'domain/product/marketplace packages must not depend on Gin'; Exclude = '(^|/)(api|httpapi)(/|$)' }
    )

    $violations = New-Object System.Collections.Generic.List[object]

    foreach ($file in $GoFiles) {
        $fromPkg = Get-PackageName $file
        $imports = Get-ImportsFromFile $file
        foreach ($import in $imports) {
            foreach ($rule in $forbiddenRules) {
                if ($fromPkg -match $rule.From -and $import -match $rule.Import) {
                    if ($rule.ContainsKey('Exclude') -and $fromPkg -match $rule.Exclude) {
                        continue
                    }
                    $violations.Add([PSCustomObject]@{
                        File    = Get-RepoRelativePath $file
                        Package = $fromPkg
                        Import  = $import
                        Reason  = $rule.Reason
                    })
                }
            }
        }
    }

    return $violations
}

function Write-Utf8File($Path, $ContentLines) {
    $directory = Split-Path $Path -Parent
    if ($directory) {
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
    }
    [System.IO.File]::WriteAllLines($Path, $ContentLines, [System.Text.UTF8Encoding]::new($false))
}

$goFiles = Get-GoFiles "internal"
$packageTargets = Get-GoPackageTargets
$packages = & go list @packageTargets
Write-Utf8File (Join-Path $outputPath $PackagesFile) $packages

$modGraph = & go mod graph
Write-Utf8File (Join-Path $outputPath $ModGraphFile) $modGraph

$packageCounts = @{}
foreach ($file in $goFiles) {
    $pkg = Get-PackageName $file
    if (-not $packageCounts.ContainsKey($pkg)) {
        $packageCounts[$pkg] = 0
    }
    $packageCounts[$pkg]++
}

$largestFiles = $goFiles |
    ForEach-Object {
        [PSCustomObject]@{
            Path  = Get-RepoRelativePath $_
            Lines = (Get-Content $_.FullName | Measure-Object -Line).Lines
        }
    } |
    Sort-Object -Property Lines -Descending

$listingkitRootFiles = @()
if (Test-Path "internal/listingkit") {
    $listingkitRootFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | Where-Object { $_.Name -notlike "*_test.go" }
}

$listingkitImporters = New-Object System.Collections.Generic.HashSet[string]
foreach ($file in $goFiles) {
    $fromPkg = Get-PackageName $file
    $imports = Get-ImportsFromFile $file
    foreach ($import in $imports) {
        if ($import -match '^task-processor/internal/listingkit(/|$)' -and $fromPkg -ne 'internal/listingkit') {
            [void]$listingkitImporters.Add($fromPkg)
        }
    }
}

$violations = Get-BoundaryViolations $goFiles

$packageMapLines = @(
    "# Package Map",
    "",
    "Generated by `scripts/dependency-baseline.ps1`.",
    "",
    "## Packages",
    ""
)
$packageMapLines += $packages | ForEach-Object { "- ``$_``" }
Write-Utf8File (Join-Path $outputPath $PackageMapFile) $packageMapLines

$baselineLines = @(
    "# Dependency Baseline",
    "",
    "Generated by `scripts/dependency-baseline.ps1`.",
    "",
    "## Summary",
    "",
    "- Go packages: $($packages.Count)",
    "- `internal/` non-test Go files: $($goFiles.Count)",
    "- Root `internal/listingkit` non-test Go files: $($listingkitRootFiles.Count)",
    "- Packages importing `internal/listingkit*`: $($listingkitImporters.Count)",
    "- Advisory boundary violations: $($violations.Count)",
    "",
    "## Largest Packages By File Count",
    ""
)
$baselineLines += $packageCounts.GetEnumerator() |
    Sort-Object -Property Value -Descending |
    Select-Object -First 30 |
    ForEach-Object { "- ``$($_.Name)``: $($_.Value)" }

$baselineLines += @(
    "",
    "## Largest Files By Line Count",
    ""
)
$baselineLines += $largestFiles |
    Select-Object -First 30 |
    ForEach-Object { "- ``$($_.Path)``: $($_.Lines)" }

$baselineLines += @(
    "",
    "## ListingKit Importers",
    ""
)
if ($listingkitImporters.Count -eq 0) {
    $baselineLines += "- None"
} else {
    $baselineLines += ($listingkitImporters | Sort-Object | ForEach-Object { "- ``$_``" })
}

$baselineLines += @(
    "",
    "## Advisory Boundary Violations",
    ""
)
if ($violations.Count -eq 0) {
    $baselineLines += "- None detected by the current advisory rules."
} else {
    $baselineLines += $violations | ForEach-Object {
        "- ``$($_.File)`` imports ``$($_.Import)``. Reason: $($_.Reason)"
    }
}

Write-Utf8File (Join-Path $outputPath $DependencyBaselineFile) $baselineLines

if ($RunGoTest) {
    & go test ./... | Tee-Object -FilePath (Join-Path $outputPath "test-baseline.txt")
}

if ($RunCoverage) {
    & go test ./... -coverprofile=(Join-Path $outputPath "coverage-baseline.out")
}

Write-Host "Wrote baseline artifacts to $outputPath" -ForegroundColor Green
Write-Host "- $DependencyBaselineFile"
Write-Host "- $PackageMapFile"
Write-Host "- $PackagesFile"
Write-Host "- $ModGraphFile"

if ($RunGoTest) {
    Write-Host "- test-baseline.txt"
}

if ($RunCoverage) {
    Write-Host "- coverage-baseline.out"
}

if ($FailOnViolation -and $violations.Count -gt 0) {
    Write-Error "Boundary violations found: $($violations.Count)"
    exit 1
}
