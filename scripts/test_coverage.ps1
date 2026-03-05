# Test Coverage Analysis Script
# Analyzes test coverage and identifies untested packages

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Test Coverage Analysis" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

# Run tests with coverage
Write-Host "[1] Running tests with coverage..." -ForegroundColor Yellow
go test ./... -cover -coverprofile=coverage.out -covermode=atomic 2>&1 | Out-Null

if ($LASTEXITCODE -ne 0) {
    Write-Host "X Tests failed, cannot generate coverage report" -ForegroundColor Red
    exit 1
}

Write-Host "OK Tests completed" -ForegroundColor Green
Write-Host ""

# Generate coverage report
Write-Host "[2] Analyzing coverage..." -ForegroundColor Yellow
$coverageData = go tool cover -func=coverage.out

# Parse coverage data
$packages = @{}
$totalStatements = 0
$coveredStatements = 0

$coverageData | ForEach-Object {
    if ($_ -match '^(.+?):(\d+):\s+(\S+)\s+(\d+\.\d+)%$') {
        $file = $matches[1]
        $function = $matches[3]
        $coverage = [double]$matches[4]
        
        # Extract package name
        $package = $file -replace '\\', '/' -replace '/[^/]+$', ''
        
        if (-not $packages.ContainsKey($package)) {
            $packages[$package] = @{
                Files = @{}
                TotalCoverage = 0
                FileCount = 0
            }
        }
        
        if (-not $packages[$package].Files.ContainsKey($file)) {
            $packages[$package].Files[$file] = @{
                Functions = @()
                Coverage = 0
            }
            $packages[$package].FileCount++
        }
        
        $packages[$package].Files[$file].Functions += @{
            Name = $function
            Coverage = $coverage
        }
    }
}

# Calculate package coverage
foreach ($package in $packages.Keys) {
    $totalCov = 0
    $fileCount = 0
    
    foreach ($file in $packages[$package].Files.Keys) {
        $fileCov = 0
        $funcCount = 0
        
        foreach ($func in $packages[$package].Files[$file].Functions) {
            $fileCov += $func.Coverage
            $funcCount++
        }
        
        if ($funcCount -gt 0) {
            $packages[$package].Files[$file].Coverage = $fileCov / $funcCount
            $totalCov += $packages[$package].Files[$file].Coverage
            $fileCount++
        }
    }
    
    if ($fileCount -gt 0) {
        $packages[$package].TotalCoverage = $totalCov / $fileCount
    }
}

Write-Host "OK Coverage analysis complete" -ForegroundColor Green
Write-Host ""

# Display results
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Coverage by Package" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

# Sort packages by coverage
$sortedPackages = $packages.GetEnumerator() | Sort-Object { $_.Value.TotalCoverage }

foreach ($pkg in $sortedPackages) {
    $coverage = [math]::Round($pkg.Value.TotalCoverage, 1)
    $color = if ($coverage -ge 80) { "Green" } 
             elseif ($coverage -ge 50) { "Yellow" } 
             else { "Red" }
    
    $bar = "=" * [math]::Floor($coverage / 2)
    Write-Host ("{0,-60} {1,5}% " -f $pkg.Key, $coverage) -NoNewline
    Write-Host $bar -ForegroundColor $color
}

Write-Host ""

# Show overall coverage
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Overall Coverage" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

$overallLine = $coverageData | Select-Object -Last 1
if ($overallLine -match '(\d+\.\d+)%') {
    $overallCoverage = [double]$matches[1]
    $color = if ($overallCoverage -ge 80) { "Green" } 
             elseif ($overallCoverage -ge 50) { "Yellow" } 
             else { "Red" }
    
    Write-Host "Total Coverage: " -NoNewline
    Write-Host "$overallCoverage%" -ForegroundColor $color
}

Write-Host ""

# Identify packages with low coverage
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Packages Needing Tests" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

$lowCoverage = $packages.GetEnumerator() | 
    Where-Object { $_.Value.TotalCoverage -lt 50 } | 
    Sort-Object { $_.Value.TotalCoverage }

if ($lowCoverage.Count -gt 0) {
    Write-Host "Packages with <50% coverage:" -ForegroundColor Yellow
    foreach ($pkg in $lowCoverage) {
        $coverage = [math]::Round($pkg.Value.TotalCoverage, 1)
        Write-Host ("  - {0,-50} {1,5}%" -f $pkg.Key, $coverage) -ForegroundColor Red
    }
} else {
    Write-Host "OK All packages have >=50% coverage!" -ForegroundColor Green
}

Write-Host ""

# Generate HTML report
Write-Host "[3] Generating HTML report..." -ForegroundColor Yellow
go tool cover -html=coverage.out -o coverage.html
Write-Host "OK HTML report generated: coverage.html" -ForegroundColor Green
Write-Host ""

Write-Host "To view the report, run: start coverage.html" -ForegroundColor Cyan
