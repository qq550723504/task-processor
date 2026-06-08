param(
    [string]$Root = ".",
    [switch]$FailOnViolation
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path $Root
Set-Location $repoRoot

Write-Host "=== Task Processor Dependency Analysis ===" -ForegroundColor Cyan
Write-Host "Repository: $repoRoot"
Write-Host ""

function Get-GoFiles($Path) {
    if (-not (Test-Path $Path)) {
        return @()
    }
    return Get-ChildItem -Path $Path -Filter "*.go" -Recurse | Where-Object { $_.Name -notlike "*_test.go" }
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

$goFiles = Get-GoFiles "internal"
Write-Host "Go files under internal/ excluding tests: $($goFiles.Count)" -ForegroundColor Green

$listingkitRootFiles = @()
if (Test-Path "internal/listingkit") {
    $listingkitRootFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | Where-Object { $_.Name -notlike "*_test.go" }
}
Write-Host "Root internal/listingkit Go files excluding tests: $($listingkitRootFiles.Count)" -ForegroundColor Green
Write-Host ""

Write-Host "=== Package File Counts ===" -ForegroundColor Cyan
$packageCounts = @{}
foreach ($file in $goFiles) {
    $pkg = Get-PackageName $file
    if (-not $packageCounts.ContainsKey($pkg)) {
        $packageCounts[$pkg] = 0
    }
    $packageCounts[$pkg]++
}

$packageCounts.GetEnumerator() |
    Sort-Object -Property Value -Descending |
    Select-Object -First 30 |
    ForEach-Object {
        Write-Host ($_.Name.PadRight(55) + $_.Value)
    }
Write-Host ""

Write-Host "=== Largest Go Files ===" -ForegroundColor Cyan
$goFiles |
    ForEach-Object {
        [PSCustomObject]@{
            Path = $_.FullName.Substring($repoRoot.Path.Length).TrimStart('\', '/') -replace '\\', '/'
            Lines = (Get-Content $_.FullName | Measure-Object -Line).Lines
        }
    } |
    Sort-Object -Property Lines -Descending |
    Select-Object -First 30 |
    ForEach-Object {
        Write-Host ($_.Path.PadRight(80) + $_.Lines)
    }
Write-Host ""

Write-Host "=== Boundary Violation Scan ===" -ForegroundColor Cyan

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
    @{ From = '^internal/(catalog|asset|productimage|publishing|workspace|amazon|shein|temu|walmart)(/|$)'; Import = '^github.com/gin-gonic/gin$'; Reason = 'domain/product/marketplace packages must not depend on Gin' }
)

$violations = New-Object System.Collections.Generic.List[object]

foreach ($file in $goFiles) {
    $fromPkg = Get-PackageName $file
    $imports = Get-ImportsFromFile $file
    foreach ($import in $imports) {
        foreach ($rule in $forbiddenRules) {
            if ($fromPkg -match $rule.From -and $import -match $rule.Import) {
                $violations.Add([PSCustomObject]@{
                    File = $file.FullName.Substring($repoRoot.Path.Length).TrimStart('\', '/') -replace '\\', '/'
                    Package = $fromPkg
                    Import = $import
                    Reason = $rule.Reason
                })
            }
        }
    }
}

if ($violations.Count -eq 0) {
    Write-Host "No boundary violations found by advisory rules." -ForegroundColor Green
} else {
    Write-Host "Found $($violations.Count) potential boundary violation(s):" -ForegroundColor Yellow
    $violations | ForEach-Object {
        Write-Host "- $($_.File) imports $($_.Import)" -ForegroundColor Yellow
        Write-Host "  Reason: $($_.Reason)" -ForegroundColor DarkYellow
    }
}
Write-Host ""

Write-Host "=== ListingKit Import Pressure ===" -ForegroundColor Cyan
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

Write-Host "Packages importing internal/listingkit*: $($listingkitImporters.Count)" -ForegroundColor Green
$listingkitImporters | Sort-Object | ForEach-Object { Write-Host "- $_" }
Write-Host ""

Write-Host "=== Suggested Next Step ===" -ForegroundColor Cyan
Write-Host "Use this output to create docs/refactoring/dependency-baseline.md before broad package moves."
Write-Host "Known legacy violations should be documented before this script is promoted to CI enforcement."

if ($FailOnViolation -and $violations.Count -gt 0) {
    Write-Error "Boundary violations found: $($violations.Count)"
    exit 1
}
