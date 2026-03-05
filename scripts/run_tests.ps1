# Test Runner Script
# Runs tests with various options and generates reports

param(
    [string]$Package = "./...",
    [switch]$Coverage,
    [switch]$Verbose,
    [switch]$Short,
    [switch]$Race,
    [string]$Run = "",
    [switch]$Html
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Test Runner" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

# Build test command
$testCmd = "go test"
$testArgs = @($Package)

if ($Verbose) {
    $testArgs += "-v"
}

if ($Short) {
    $testArgs += "-short"
}

if ($Race) {
    $testArgs += "-race"
}

if ($Run) {
    $testArgs += "-run"
    $testArgs += $Run
}

if ($Coverage) {
    $testArgs += "-cover"
    $testArgs += "-coverprofile=coverage.out"
    $testArgs += "-covermode=atomic"
}

# Run tests
Write-Host "Running: $testCmd $($testArgs -join ' ')" -ForegroundColor Yellow
Write-Host ""

$output = & $testCmd @testArgs 2>&1
$exitCode = $LASTEXITCODE

# Display output
$output | ForEach-Object {
    $line = $_.ToString()
    if ($line -match "PASS") {
        Write-Host $line -ForegroundColor Green
    } elseif ($line -match "FAIL") {
        Write-Host $line -ForegroundColor Red
    } elseif ($line -match "coverage:") {
        Write-Host $line -ForegroundColor Cyan
    } else {
        Write-Host $line
    }
}

Write-Host ""

# Generate coverage report if requested
if ($Coverage -and $exitCode -eq 0) {
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host "   Coverage Report" -ForegroundColor Blue
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host ""
    
    # Show coverage summary
    go tool cover -func=coverage.out | Select-Object -Last 1
    
    if ($Html) {
        Write-Host ""
        Write-Host "Generating HTML coverage report..." -ForegroundColor Yellow
        go tool cover -html=coverage.out -o coverage.html
        Write-Host "OK Coverage report generated: coverage.html" -ForegroundColor Green
        Write-Host "Open in browser: start coverage.html" -ForegroundColor Cyan
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Test Summary" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

if ($exitCode -eq 0) {
    Write-Host "OK All tests passed!" -ForegroundColor Green
} else {
    Write-Host "X Some tests failed" -ForegroundColor Red
}

exit $exitCode
