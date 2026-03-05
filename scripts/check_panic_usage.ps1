# Panic Usage Detection Script
# Detects improper panic usage in the project

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Panic Usage Detection Tool" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

# Statistics
$totalIssues = 0
$criticalIssues = 0
$warningIssues = 0

# 1. Check runtime panic usage (exclude test files)
Write-Host "[1] Checking runtime panic usage..." -ForegroundColor Yellow

$runtimePanics = Get-ChildItem -Path . -Recurse -Include *.go -Exclude *_test.go | 
    Select-String -Pattern "panic\(" | 
    Where-Object { 
        $_.Line -notmatch "RecoverFromPanic" -and 
        $_.Line -notmatch "LogPanic" -and 
        $_.Line -notmatch "// panic" -and 
        $_.Path -notmatch "internal[\\/]core[\\/]errors[\\/]errors\.go" -and
        $_.Path -notmatch "internal[\\/]core[\\/]config[\\/]validator\.go"
    }

if ($runtimePanics) {
    Write-Host "X Found runtime panic usage:" -ForegroundColor Red
    $runtimePanics | ForEach-Object { Write-Host "  $($_.Path):$($_.LineNumber): $($_.Line.Trim())" }
    $count = ($runtimePanics | Measure-Object).Count
    $criticalIssues += $count
    $totalIssues += $count
    Write-Host ""
} else {
    Write-Host "OK No runtime panic usage found" -ForegroundColor Green
    Write-Host ""
}

# 2. Check ValidateOrPanic usage
Write-Host "[2] Checking ValidateOrPanic usage..." -ForegroundColor Yellow

$validatePanics = Get-ChildItem -Path . -Recurse -Include *.go | 
    Select-String -Pattern "\.ValidateOrPanic\(\)"

if ($validatePanics) {
    Write-Host "X Found ValidateOrPanic calls:" -ForegroundColor Red
    $validatePanics | ForEach-Object { Write-Host "  $($_.Path):$($_.LineNumber): $($_.Line.Trim())" }
    $count = ($validatePanics | Measure-Object).Count
    $criticalIssues += $count
    $totalIssues += $count
    Write-Host ""
    Write-Host "Suggestion: Use Validate() method and return error" -ForegroundColor Blue
    Write-Host ""
} else {
    Write-Host "OK No ValidateOrPanic calls found" -ForegroundColor Green
    Write-Host ""
}

# 3. Check Must/MustValue usage in non-initialization code
Write-Host "[3] Checking Must/MustValue in non-initialization code..." -ForegroundColor Yellow

$mustUsage = Get-ChildItem -Path . -Recurse -Include *.go | 
    Select-String -Pattern "errors\.Must\(|errors\.MustValue\(" | 
    Where-Object { 
        $_.Path -notmatch "cmd[\\/].*[\\/]main\.go" -and 
        $_.Path -notmatch "bootstrap" -and 
        $_.Line -notmatch "func init\(\)"
    }

if ($mustUsage) {
    Write-Host "! Found Must/MustValue in non-initialization code:" -ForegroundColor Yellow
    $mustUsage | ForEach-Object { Write-Host "  $($_.Path):$($_.LineNumber): $($_.Line.Trim())" }
    $count = ($mustUsage | Measure-Object).Count
    $warningIssues += $count
    $totalIssues += $count
    Write-Host ""
    Write-Host "Suggestion: Must/MustValue should only be used in main or init functions" -ForegroundColor Blue
    Write-Host ""
} else {
    Write-Host "OK Must/MustValue usage is correct" -ForegroundColor Green
    Write-Host ""
}

# 4. Check goroutines without panic recovery
Write-Host "[4] Checking goroutines for panic recovery..." -ForegroundColor Yellow

$goroutinesWithoutRecover = Get-ChildItem -Path . -Recurse -Include *.go | 
    Select-String -Pattern "go func\(\)" -Context 0,5 | 
    Where-Object { 
        $_.Context.PostContext -notmatch "defer.*recover|RecoverFromPanic|SafeGo"
    }

if ($goroutinesWithoutRecover) {
    Write-Host "! Found goroutines that may lack panic recovery" -ForegroundColor Yellow
    Write-Host "Tip: Check if these goroutines need panic recovery" -ForegroundColor Blue
    $goroutinesWithoutRecover | Select-Object -First 10 | ForEach-Object { 
        Write-Host "  $($_.Path):$($_.LineNumber): $($_.Line.Trim())" 
    }
    Write-Host "..." -ForegroundColor Gray
    Write-Host ""
} else {
    Write-Host "OK Goroutine panic recovery check passed" -ForegroundColor Green
    Write-Host ""
}

# 5. Check SafeGo and SafeExecute usage
Write-Host "[5] Checking safe execution function usage..." -ForegroundColor Yellow

$safeGoCount = (Get-ChildItem -Path . -Recurse -Include *.go | 
    Select-String -Pattern "SafeGo|SafeExecute" | 
    Measure-Object).Count

Write-Host "Info: SafeGo/SafeExecute usage count: $safeGoCount" -ForegroundColor Blue
Write-Host ""

# 6. Generate fix suggestions
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Fix Suggestions" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

if ($criticalIssues -gt 0) {
    Write-Host "CRITICAL ISSUES ($criticalIssues):" -ForegroundColor Red
    Write-Host "   1. Remove runtime panic, use error return instead"
    Write-Host "   2. Change ValidateOrPanic to Validate and handle errors"
    Write-Host ""
}

if ($warningIssues -gt 0) {
    Write-Host "WARNING ISSUES ($warningIssues):" -ForegroundColor Yellow
    Write-Host "   1. Ensure Must/MustValue only used in initialization phase"
    Write-Host "   2. Add panic recovery mechanism in goroutines"
    Write-Host ""
}

Write-Host "BEST PRACTICES:" -ForegroundColor Blue
Write-Host "   1. Only use panic in main and init functions"
Write-Host "   2. Use errors.SafeGo to start goroutines"
Write-Host "   3. Use errors.SafeExecute to wrap risky code"
Write-Host "   4. Use error return instead of panic in business logic"
Write-Host ""

# 7. Summary
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Detection Summary" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""
Write-Host "Total issues: $totalIssues"
Write-Host "Critical issues: $criticalIssues" -ForegroundColor $(if ($criticalIssues -gt 0) { "Red" } else { "Green" })
Write-Host "Warning issues: $warningIssues" -ForegroundColor $(if ($warningIssues -gt 0) { "Yellow" } else { "Green" })
Write-Host ""

if ($criticalIssues -eq 0 -and $warningIssues -eq 0) {
    Write-Host "OK Congratulations! No panic usage issues found" -ForegroundColor Green
    exit 0
} elseif ($criticalIssues -gt 0) {
    Write-Host "X Critical issues found, recommend immediate fix" -ForegroundColor Red
    exit 1
} else {
    Write-Host "! Warning issues found, recommend review and fix" -ForegroundColor Yellow
    exit 0
}
